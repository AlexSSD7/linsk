package vm

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"log/slog"

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
	stderrBuf     *bytes.Buffer

	serialStdoutCh chan []byte

	// These are to be interacted with using `atomic` package
	disposed uint32
	canceled uint32
}

type DriveConfig struct {
	Path         string
	SnapshotMode bool
}

type VMConfig struct {
	CdromImagePath string
	Drives         []DriveConfig

	USBDevices               []USBDevicePassthroughConfig
	ExtraPortForwardingRules []PortForwardingRule

	// Mostly debug-related options.
	UnrestrictedNetworking bool
	ShowDisplay            bool
	InstallBaseUtilities   bool
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

	// TODO: Configurable memory allocation

	baseCmd := "qemu-system-x86_64"
	cmdArgs := []string{"-serial", "stdio", "-enable-kvm", "-m", "2048", "-smp", fmt.Sprint(runtime.NumCPU())}

	netdevOpts := "user,id=net0,hostfwd=tcp:127.0.0.1:" + fmt.Sprint(sshPort) + "-:22"

	if !cfg.UnrestrictedNetworking {
		netdevOpts += ",restrict=on"
	} else {
		logger.Warn("Running with unsafe unrestricted networking")
	}

	for _, pf := range cfg.ExtraPortForwardingRules {
		hostIPStr := ""
		if pf.HostIP != nil {
			hostIPStr = pf.HostIP.String()
		}

		netdevOpts += ",hostfwd=tcp:" + hostIPStr + ":" + fmt.Sprint(pf.HostPort) + "-:" + fmt.Sprint(pf.VMPort)
	}

	cmdArgs = append(cmdArgs, "-device", "e1000,netdev=net0", "-netdev", netdevOpts)

	if !cfg.ShowDisplay {
		cmdArgs = append(cmdArgs, "-display", "none")
	}

	if len(cfg.USBDevices) != 0 {
		cmdArgs = append(cmdArgs, "-usb", "-device", "nec-usb-xhci,id=xhci")

		for _, dev := range cfg.USBDevices {
			cmdArgs = append(cmdArgs, "-device", "usb-host,hostbus="+strconv.FormatUint(uint64(dev.HostBus), 10)+",hostport="+strconv.FormatUint(uint64(dev.HostPort), 10))
		}
	}

	for i, extraDrive := range cfg.Drives {
		_, err = os.Stat(extraDrive.Path)
		if err != nil {
			return nil, errors.Wrapf(err, "stat extra drive #%v path", i)
		}

		driveArgs := "file=" + shellescape.Quote(extraDrive.Path) + ",format=qcow2,if=virtio"
		if extraDrive.SnapshotMode {
			driveArgs += ",snapshot=on"
		}

		cmdArgs = append(cmdArgs, "-drive", driveArgs)
	}

	// We're not using clean `cdromImagePath` here because it is set to "."
	// when the original string is empty.
	if cfg.CdromImagePath != "" {
		cmdArgs = append(cmdArgs, "-boot", "d", "-cdrom", cdromImagePath)
	}

	if cfg.InstallBaseUtilities && !cfg.UnrestrictedNetworking {
		return nil, fmt.Errorf("cannot install base utilities with unrestricted networking disabled")
	}

	sysRead, userWrite := io.Pipe()
	userRead, sysWrite := io.Pipe()

	cmd := exec.Command(baseCmd, cmdArgs...)

	cmd.Stdin = sysRead
	cmd.Stdout = sysWrite
	stderrBuf := bytes.NewBuffer(nil)
	cmd.Stderr = stderrBuf

	// This is to prevent Ctrl+C propagating to the child process.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

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

		serialRead:   userRead,
		serialReader: userReader,
		serialWrite:  userWrite,
		stderrBuf:    stderrBuf,
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

	var globalErrsMu sync.Mutex
	var globalErrs []error

	globalErrFn := func(err error) {
		globalErrsMu.Lock()
		defer globalErrsMu.Unlock()

		globalErrs = append(globalErrs, err, errors.Wrap(vm.Cancel(), "cancel on error"))
	}

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

		vm.logger.Info("Setting the VM up")

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
			// TODO: Test what actually happens in inline critical errors like this.
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

		vm.logger.Info("The VM is ready")
	}()

	_, err = vm.cmd.Process.Wait()
	cancelErr := vm.Cancel()
	if err != nil {
		combinedErr := multierr.Combine(
			errors.Wrap(err, "wait for cmd to finish execution"),
			errors.Wrap(cancelErr, "cancel"),
		)

		return fmt.Errorf("%w %v", combinedErr, utils.GetLogErrMsg(vm.stderrBuf.String()))
	}

	combinedErr := multierr.Combine(
		append(globalErrs, errors.Wrap(cancelErr, "cancel on exit"))...,
	)
	if combinedErr != nil {
		return fmt.Errorf("%w %v", combinedErr, utils.GetLogErrMsg(vm.stderrBuf.String()))
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
			vm.logger.Warn("Failed to dial VM ssh to do graceful shutdown", "error", err)
		}
	} else {
		_, err = runSSHCmd(sc, "poweroff")
		_ = sc.Close()
		if err != nil {
			vm.logger.Warn("Could not power off the VM safely", "error", err)
		} else {
			vm.logger.Info("Shutting the VM down safely")
		}
	}

	var interruptErr error

	if !gracefulOK {
		interruptErr = vm.cmd.Process.Signal(os.Interrupt)
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

	_, err := vm.serialWrite.Write(b)
	return err
}

func (vm *VM) runVMLoginHandler() error {
	for {
		select {
		case <-vm.ctx.Done():
			return nil
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
