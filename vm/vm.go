package vm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"log/slog"

	"github.com/AlexSSD7/linsk/nettap"
	"github.com/AlexSSD7/linsk/sshutil"
	"github.com/AlexSSD7/linsk/utils"
	"github.com/alessio/shellescape"
	"github.com/bramvdbogaerde/go-scp"
	"github.com/phayes/freeport"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"golang.org/x/crypto/ssh"
)

type VM struct {
	logger *slog.Logger

	ctx       context.Context
	ctxCancel context.CancelFunc

	cmd *exec.Cmd

	sshMappedPort uint16
	sshConf       *ssh.ClientConfig
	sshReadyCh    chan struct{}
	installSSH    bool

	serialRead    *io.PipeReader
	serialReader  *bufio.Reader
	serialWrite   *io.PipeWriter
	serialWriteMu sync.Mutex
	qemuStderrBuf *bytes.Buffer

	osUpTimeout  time.Duration
	sshUpTimeout time.Duration

	serialStdoutCh chan []byte

	// These are to be interacted with using `atomic` package
	disposed uint32
	canceled uint32

	originalCfg VMConfig
}

type DriveConfig struct {
	Path         string
	SnapshotMode bool
}

type TapConfig struct {
	Name string
}

type VMConfig struct {
	CdromImagePath string
	BIOSPath       string
	Drives         []DriveConfig

	MemoryAlloc uint32 // In KiB.

	PassthroughConfig        PassthroughConfig
	ExtraPortForwardingRules []PortForwardingRule

	// Networking
	UnrestrictedNetworking bool
	Taps                   []TapConfig

	// Timeouts
	OSUpTimeout  time.Duration
	SSHUpTimeout time.Duration

	// Mostly debug-related options.
	ShowDisplay          bool
	InstallBaseUtilities bool
}

func NewVM(logger *slog.Logger, cfg VMConfig) (*VM, error) {
	cdromImagePath := filepath.Clean(cfg.CdromImagePath)
	_, err := os.Stat(cdromImagePath)
	if err != nil {
		return nil, errors.Wrap(err, "stat cdrom image path")
	}

	sshPort, err := freeport.GetFreePort()
	if err != nil {
		return nil, errors.Wrap(err, "get free port for ssh server")
	}

	cmdArgs := []string{"-serial", "stdio", "-m", fmt.Sprint(cfg.MemoryAlloc), "-smp", fmt.Sprint(runtime.NumCPU())}

	if cfg.BIOSPath != "" {
		cmdArgs = append(cmdArgs, "-bios", filepath.Clean(cfg.BIOSPath))
	}

	baseCmd := "qemu-system"

	var accel string
	switch runtime.GOOS {
	case "windows":
		// TODO: whpx accel is broken in Windows. Long term solution looks to be use Hyper-V.

		// For Windows, we need to install QEMU using an installer and add it to PATH.
		// Then, we should enable Windows Hypervisor Platform in "Turn Windows features on or off".
		// IMPORTANT: We should also install libusbK drivers for USB devices we want to pass through.
		// This can be easily done with a program called Zadiag by Akeo.
		accel = "whpx,kernel-irqchip=off"
	case "darwin":
		accel = "hvf"
	default:
		accel = "kvm"
	}

	switch runtime.GOARCH {
	case "amd64":
		baseCmd += "-x86_64"
	case "arm64":
		if cfg.BIOSPath == "" {
			logger.Warn("BIOS image path is not specified while attempting to run an aarch64 (arm64) VM. The VM will not boot.")
		}

		// ",highmem=off" is required for M1.
		cmdArgs = append(cmdArgs, "-M", "virt,highmem=off", "-cpu", "host")
		baseCmd += "-aarch64"
	default:
		return nil, fmt.Errorf("arch '%v' is not supported", runtime.GOARCH)
	}

	cmdArgs = append(cmdArgs, "-accel", accel)

	if runtime.GOOS == "windows" {
		baseCmd += ".exe"
	}

	netdevOpts := "user,id=net0,hostfwd=tcp:127.0.0.1:" + fmt.Sprint(sshPort) + "-:22"

	if !cfg.UnrestrictedNetworking {
		netdevOpts += ",restrict=on"
	} else {
		logger.Warn("Running with unrestricted networking")
	}

	for _, pf := range cfg.ExtraPortForwardingRules {
		hostIPStr := ""
		if pf.HostIP != nil {
			hostIPStr = pf.HostIP.String()
		}

		netdevOpts += ",hostfwd=tcp:" + hostIPStr + ":" + fmt.Sprint(pf.HostPort) + "-:" + fmt.Sprint(pf.VMPort)
	}

	cmdArgs = append(cmdArgs, "-device", "e1000,netdev=net0", "-netdev", netdevOpts)

	for i, tap := range cfg.Taps {
		err := nettap.ValidateTapName(tap.Name)
		if err != nil {
			return nil, errors.Wrapf(err, "validate network tap #%v name", i)
		}

		netdevName := "net" + fmt.Sprint(1+i)

		cmdArgs = append(cmdArgs, "-device", "e1000,netdev="+netdevName, "-netdev", "tap,id="+netdevName+",ifname="+shellescape.Quote(tap.Name)+",script=no,downscript=no")
	}

	if !cfg.ShowDisplay {
		cmdArgs = append(cmdArgs, "-display", "none")
	} else if runtime.GOARCH == "arm64" {
		// No video is configured by default in ARM. This will enable it.
		// TODO: This doesn't really work on arm64. It just shows a blank viewer.
		cmdArgs = append(cmdArgs, "-device", "virtio-gpu-device")
	}

	for i, extraDrive := range cfg.Drives {
		_, err = os.Stat(extraDrive.Path)
		if err != nil {
			return nil, errors.Wrapf(err, "stat extra drive #%v path", i)
		}

		driveArgs := "file=" + shellescape.Quote(strings.ReplaceAll(extraDrive.Path, "\\", "/")) + ",format=qcow2,if=none,id=disk" + fmt.Sprint(i)
		if extraDrive.SnapshotMode {
			driveArgs += ",snapshot=on"
		}

		devArgs := "virtio-blk-pci,drive=disk" + fmt.Sprint(i)

		if cfg.CdromImagePath == "" {
			devArgs += ",bootindex=" + fmt.Sprint(i)
		}

		cmdArgs = append(cmdArgs, "-drive", driveArgs, "-device", devArgs)
	}

	if len(cfg.PassthroughConfig.USB) != 0 {
		cmdArgs = append(cmdArgs, "-device", "nec-usb-xhci")

		for _, dev := range cfg.PassthroughConfig.USB {
			cmdArgs = append(cmdArgs, "-device", "usb-host,vendorid=0x"+hex.EncodeToString(utils.Uint16ToBytesBE(dev.VendorID))+",productid=0x"+hex.EncodeToString(utils.Uint16ToBytesBE(dev.ProductID)))
		}
	}

	if len(cfg.PassthroughConfig.Block) != 0 {
		logger.Warn("Detected raw block device passthrough. Please note that it's YOUR responsibility to ensure that no device is mounted in your OS and the VM at the same time. Otherwise, you run serious risks. No further warnings will be issued.")
	}

	for _, dev := range cfg.PassthroughConfig.Block {
		// It's always a user's responsibility to ensure that no drives are mounted
		// in both host and guest system. This should serve as the last resort.
		{
			seemsMounted, err := checkDeviceSeemsMounted(dev.Path)
			if err != nil {
				return nil, errors.Wrapf(err, "check whether device seems to be mounted (path '%v')", dev.Path)
			}

			if seemsMounted {
				return nil, fmt.Errorf("device '%v' seems to be already mounted in the host system", dev.Path)
			}
		}

		cmdArgs = append(cmdArgs, "-drive", "file="+shellescape.Quote(strings.ReplaceAll(dev.Path, "\\", "/"))+",format=raw,cache=none")
	}

	// We're not using clean `cdromImagePath` here because it is set to "."
	// when the original string is empty.
	if cfg.CdromImagePath != "" {
		cmdArgs = append(cmdArgs, "-boot", "d", "-cdrom", cdromImagePath)
	}

	if cfg.InstallBaseUtilities && !cfg.UnrestrictedNetworking {
		return nil, fmt.Errorf("cannot install base utilities with unrestricted networking disabled")
	}

	// NOTE: The default timeouts below have no relation to the default
	// timeouts set by the CLI. These work only if no timeout was supplied
	// in the config programmatically. Defaults set here are quite conservative.
	osUpTimeout := time.Second * 60
	if cfg.OSUpTimeout != 0 {
		osUpTimeout = cfg.OSUpTimeout
	}
	sshUpTimeout := time.Second * 120
	if cfg.SSHUpTimeout != 0 {
		sshUpTimeout = cfg.SSHUpTimeout
	}

	if sshUpTimeout < osUpTimeout {
		return nil, fmt.Errorf("vm ssh setup timeout cannot be lower than os up timeout")
	}

	// No errors beyond this point.

	sysRead, userWrite := io.Pipe()
	userRead, sysWrite := io.Pipe()

	cmd := exec.Command(baseCmd, cmdArgs...)

	cmd.Stdin = sysRead
	cmd.Stdout = sysWrite
	stderrBuf := bytes.NewBuffer(nil)
	cmd.Stderr = stderrBuf

	// This function is OS-specific.
	prepareVMCmd(cmd)

	userReader := bufio.NewReader(userRead)

	ctx, ctxCancel := context.WithCancel(context.Background())

	vm := &VM{
		logger: logger,

		ctx:       ctx,
		ctxCancel: ctxCancel,

		cmd: cmd,

		sshMappedPort: uint16(sshPort),
		sshReadyCh:    make(chan struct{}),
		installSSH:    cfg.InstallBaseUtilities,

		serialRead:    userRead,
		serialReader:  userReader,
		serialWrite:   userWrite,
		qemuStderrBuf: stderrBuf,

		osUpTimeout:  osUpTimeout,
		sshUpTimeout: sshUpTimeout,

		originalCfg: cfg,
	}

	vm.resetSerialStdout()

	return vm, nil
}

func (vm *VM) Run() error {
	if atomic.AddUint32(&vm.disposed, 1) != 1 {
		return fmt.Errorf("vm disposed")
	}

	err := vm.cmd.Start()
	if err != nil {
		return errors.Wrap(err, "start qemu cmd")
	}

	go vm.runPeriodicHostMountChecker()

	var globalErrsMu sync.Mutex
	var globalErrs []error

	globalErrFn := func(err error) {
		globalErrsMu.Lock()
		defer globalErrsMu.Unlock()

		globalErrs = append(globalErrs, err, errors.Wrap(vm.Cancel(), "cancel on error"))
	}

	bootReadyCh := make(chan struct{})

	go func() {
		select {
		case <-time.After(vm.osUpTimeout):
			vm.logger.Warn("A VM boot timeout detected, consider running with --vm-debug to investigate")
			globalErrFn(fmt.Errorf("vm boot timeout %v", utils.GetLogErrMsg(string(vm.consumeSerialStdout()), "serial log")))
		case <-bootReadyCh:
			vm.logger.Info("The VM is up, setting it up")
		}
	}()

	go func() {
		select {
		case <-time.After(vm.sshUpTimeout):
			globalErrFn(fmt.Errorf("vm setup timeout %v", utils.GetLogErrMsg(string(vm.consumeSerialStdout()), "serial log")))
		case <-vm.sshReadyCh:
			vm.logger.Info("The VM is ready")
		}
	}()

	vm.logger.Info("Booting the VM")

	go func() {
		_ = vm.runSerialReader()
		_ = vm.Cancel()
	}()

	go func() {
		err = vm.runVMLoginHandler()
		if err != nil {
			globalErrFn(errors.Wrap(err, "run vm login handler"))
			return
		}

		// This will disable the timeout-handling goroutine.
		close(bootReadyCh)

		sshSigner, err := vm.sshSetup()
		if err != nil {
			globalErrFn(errors.Wrap(err, "set up ssh"))
			return
		}

		vm.logger.Debug("Set up SSH server successfully")

		sshKeyScan, err := vm.scanSSHIdentity()
		if err != nil {
			globalErrFn(errors.Wrap(err, "scan ssh identity"))
			return
		}

		vm.logger.Debug("Scanned SSH identity")

		knownHosts, err := ParseSSHKeyScan(sshKeyScan)
		if err != nil {
			globalErrFn(errors.Wrap(err, "parse ssh key scan"))
			return
		}

		vm.sshConf = &ssh.ClientConfig{
			User:            "root",
			HostKeyCallback: knownHosts,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(sshSigner),
			},
			Timeout: time.Second * 5,
		}

		// This is to notify everyone waiting for SSH to be up that it's ready to go.
		close(vm.sshReadyCh)
	}()

	_, err = vm.cmd.Process.Wait()
	cancelErr := vm.Cancel()
	if err != nil {
		combinedErr := multierr.Combine(
			errors.Wrap(err, "wait for cmd to finish execution"),
			errors.Wrap(cancelErr, "cancel"),
		)

		return fmt.Errorf("%w %v", combinedErr, utils.GetLogErrMsg(vm.qemuStderrBuf.String(), "qemu stderr log"))
	}

	combinedErr := multierr.Combine(
		append(globalErrs, errors.Wrap(cancelErr, "cancel on exit"))...,
	)
	if combinedErr != nil {
		return fmt.Errorf("%w %v", combinedErr, utils.GetLogErrMsg(vm.qemuStderrBuf.String(), "qemu stderr log"))
	}

	return nil
}

func (vm *VM) Cancel() error {
	if atomic.AddUint32(&vm.canceled, 1) != 1 {
		return nil
	}

	vm.logger.Warn("Canceling the VM context")

	var gracefulOK bool

	sc, err := vm.DialSSH()
	if err != nil {
		if !errors.Is(err, ErrSSHUnavailable) {
			vm.logger.Warn("Failed to dial VM SSH to do graceful shutdown", "error", err.Error())
		}
	} else {
		vm.logger.Warn("Sending poweroff command to the VM")
		_, err = sshutil.RunSSHCmd(context.Background(), sc, "poweroff")
		_ = sc.Close()
		if err != nil {
			vm.logger.Warn("Could not power off the VM safely", "error", err.Error())
		} else {
			vm.logger.Info("Shutting the VM down safely")
		}
	}

	var interruptErr error

	if !gracefulOK {
		if vm.cmd.Process == nil {
			interruptErr = fmt.Errorf("process is not started")
		} else {
			interruptErr = terminateProcess(vm.cmd.Process.Pid)
		}
	}

	vm.ctxCancel()
	return multierr.Combine(
		errors.Wrap(interruptErr, "interrupt cmd"),
		errors.Wrap(vm.serialRead.Close(), "close serial read pipe"),
		errors.Wrap(vm.serialWrite.Close(), "close serial write pipe"),
	)
}

func (vm *VM) runSerialReader() error {
	for {
		raw, err := vm.serialReader.ReadBytes('\n')
		if err != nil {
			return errors.Wrap(err, "read from serial reader")
		}

		select {
		case vm.serialStdoutCh <- raw:
		default:
			// Message gets discarded if the buffer is full.
		}
	}
}

func (vm *VM) writeSerial(b []byte) error {
	vm.serialWriteMu.Lock()
	defer vm.serialWriteMu.Unlock()

	// What do you see below is a workaround for the way how serial console
	// is implemented in QEMU/Windows pair. Apparently they are using polling,
	// and this will ensure that we do not write faster than the polling rate.
	for i := range b {
		_, err := vm.serialWrite.Write([]byte{b[i]})
		time.Sleep(time.Millisecond)
		if err != nil {
			return errors.Wrapf(err, "write char #%v", i)
		}
	}

	return nil
}

func (vm *VM) runVMLoginHandler() error {
	for {
		select {
		case <-vm.ctx.Done():
			return vm.ctx.Err()
		case <-time.After(time.Second):
			peek, err := vm.serialReader.Peek(vm.serialReader.Buffered())
			if err != nil {
				return errors.Wrap(err, "peek stdout")
			}

			if bytes.Contains(peek, []byte("login:")) {
				err = vm.writeSerial([]byte("root\n"))
				if err != nil {
					return errors.Wrap(err, "failed to stdio write login")
				}

				vm.logger.Debug("Logged into the VM serial")

				return nil
			}
		}
	}
}

func (vm *VM) resetSerialStdout() {
	vm.serialStdoutCh = make(chan []byte, 32)
}

func (vm *VM) consumeSerialStdout() []byte {
	buf := bytes.NewBuffer(nil)

	for {
		select {
		case data := <-vm.serialStdoutCh:
			buf.Write(data)
		default:
			return buf.Bytes()
		}
	}
}

func (vm *VM) DialSSH() (*ssh.Client, error) {
	if vm.sshConf == nil {
		return nil, ErrSSHUnavailable
	}

	return ssh.Dial("tcp", "localhost:"+fmt.Sprint(vm.sshMappedPort), vm.sshConf)
}

func (vm *VM) DialSCP() (*scp.Client, error) {
	if vm.sshConf == nil {
		return nil, ErrSSHUnavailable
	}

	sc := scp.NewClient("localhost:"+fmt.Sprint(vm.sshMappedPort), vm.sshConf)
	err := sc.Connect()
	if err != nil {
		return nil, err
	}

	return &sc, nil
}

func (vm *VM) SSHUpNotifyChan() chan struct{} {
	return vm.sshReadyCh
}

// It's always a user's responsibility to ensure that no drives are mounted
// in both host and guest system. This should serve as the last resort.
func (vm *VM) runPeriodicHostMountChecker() {
	if len(vm.originalCfg.PassthroughConfig.Block) == 0 {
		return
	}

	for {
		select {
		case <-vm.ctx.Done():
			return
		case <-time.After(time.Second):
			for _, dev := range vm.originalCfg.PassthroughConfig.Block {
				seemsMounted, err := checkDeviceSeemsMounted(dev.Path)
				if err != nil {
					vm.logger.Warn("Failed to check if a passed device seems to be mounted", "dev-path", dev.Path)
					continue
				}

				if seemsMounted {
					_ = vm.cmd.Process.Kill()
					panic(fmt.Sprintf("CRITICAL: Passed-through device '%v' appears to have been mounted on the host OS. Forcefully exiting now to prevent data corruption.", dev.Path))
				}
			}
		}
	}
}
