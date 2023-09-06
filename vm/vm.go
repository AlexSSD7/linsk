// Linsk - A utility to access Linux-native file systems on non-Linux operating systems.
// Copyright (c) 2023 The Linsk Authors.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package vm

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"log/slog"

	"github.com/AlexSSD7/linsk/osspecifics"
	"github.com/AlexSSD7/linsk/qemucli"
	"github.com/AlexSSD7/linsk/sshutil"
	"github.com/AlexSSD7/linsk/utils"
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

	originalCfg Config
}

type DriveConfig struct {
	Path         string
	SnapshotMode bool
}

type TapConfig struct {
	Name string
}

type Config struct {
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
	Debug                bool // This will show the display and forward all QEMU warnings/errors to stderr.
	InstallBaseUtilities bool
}

func NewVM(logger *slog.Logger, cfg Config) (*VM, error) {
	sshPort, err := freeport.GetFreePort()
	if err != nil {
		return nil, errors.Wrap(err, "get free port for ssh server")
	}

	baseCmd, cmdArgs, err := configureBaseVMCmd(logger, cfg)
	if err != nil {
		return nil, errors.Wrap(err, "configure base vm cmd")
	}

	netCmdArgs, err := configureVMCmdNetworking(logger, cfg, uint16(sshPort))
	if err != nil {
		return nil, errors.Wrap(err, "configure vm cmd networking")
	}

	cmdArgs = append(cmdArgs, netCmdArgs...)

	driveCmdArgs, err := configureVMCmdDrives(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "configure vm cmd drives")
	}

	cmdArgs = append(cmdArgs, driveCmdArgs...)

	usbCmdArgs := configureVMCmdUSBPassthrough(cfg)

	cmdArgs = append(cmdArgs, usbCmdArgs...)

	blockDevArgs, err := configureVMCmdBlockDevicePassthrough(logger, cfg)
	if err != nil {
		return nil, errors.Wrap(err, "configure vm cmd block device passthrough")
	}

	cmdArgs = append(cmdArgs, blockDevArgs...)

	if cfg.InstallBaseUtilities && !cfg.UnrestrictedNetworking {
		return nil, fmt.Errorf("installation of base utilities is impossible with unrestricted networking disabled")
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

	encodedCmdArgs, err := qemucli.EncodeArgs(cmdArgs)
	if err != nil {
		return nil, errors.Wrap(err, "encode qemu cli args")
	}

	// No errors beyond this point.

	sysRead, userWrite := io.Pipe()
	userRead, sysWrite := io.Pipe()

	cmd := exec.Command(baseCmd, encodedCmdArgs...) //#nosec G204 // I know, it's generally a bad idea to include variables into shell commands, but QEMU unfortunately does not accept anything else.

	cmd.Stdin = sysRead
	cmd.Stdout = sysWrite
	stderrBuf := bytes.NewBuffer(nil)
	cmd.Stderr = stderrBuf

	if cfg.Debug {
		cmd.Stderr = io.MultiWriter(cmd.Stderr, os.Stderr)
	}

	// This function is OS-specific.
	osspecifics.SetNewProcessGroupCmd(cmd)

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
			interruptErr = osspecifics.TerminateProcess(vm.cmd.Process.Pid)
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
					return errors.Wrap(err, "stdio write login")
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
		return nil, errors.Wrap(err, "connect")
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
				seemsMounted, err := osspecifics.CheckDeviceSeemsMounted(dev.Path)
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
