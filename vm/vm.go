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
	"time"

	"log/slog"

	"github.com/alessio/shellescape"
	"github.com/phayes/freeport"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"golang.org/x/crypto/ssh"
)

type USBDevicePassthroughConfig struct {
	HostBus  uint8
	HostPort uint8
}

type Instance struct {
	logger *slog.Logger

	ctx       context.Context
	ctxCancel context.CancelFunc

	cmd *exec.Cmd

	sshMappedPort uint16
	sshConf       *ssh.ClientConfig
	sshReadyCh    chan struct{}

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

func NewInstance(logger *slog.Logger, alpineImagePath string, usbDevices []USBDevicePassthroughConfig, debug bool) (*Instance, error) {
	alpineImagePath = filepath.Clean(alpineImagePath)
	_, err := os.Stat(alpineImagePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to stat alpine image path")
	}

	sshPort, err := freeport.GetFreePort()
	if err != nil {
		return nil, errors.Wrap(err, "get free port for ssh server")
	}

	// TODO: Disable internet access

	// TODO: Configurable memory allocation

	baseCmd := "qemu-system-x86_64"
	cmdArgs := []string{"-serial", "stdio", "-enable-kvm", "-m", "2048", "-smp", fmt.Sprint(runtime.NumCPU()),
		"-device", "e1000,netdev=net0", "-netdev", "user,id=net0,hostfwd=tcp::" + fmt.Sprint(sshPort) + "-:22"}

	cmdArgs = append(cmdArgs, "-drive", "file="+shellescape.Quote(alpineImagePath)+",format=qcow2,if=virtio", "-snapshot")

	if !debug {
		cmdArgs = append(cmdArgs, "-display", "none")
	}

	if len(usbDevices) != 0 {
		cmdArgs = append(cmdArgs, "-usb", "-device", "nec-usb-xhci,id=xhci")

		for _, dev := range usbDevices {
			cmdArgs = append(cmdArgs, "-device", "usb-host,hostbus="+strconv.FormatUint(uint64(dev.HostBus), 10)+",hostport="+strconv.FormatUint(uint64(dev.HostPort), 10))
		}
	}

	sysRead, userWrite := io.Pipe()
	userRead, sysWrite := io.Pipe()

	cmd := exec.Command(baseCmd, cmdArgs...)

	cmd.Stdin = sysRead
	cmd.Stdout = sysWrite
	stderrBuf := bytes.NewBuffer(nil)
	cmd.Stderr = stderrBuf

	userReader := bufio.NewReader(userRead)

	ctx, ctxCancel := context.WithCancel(context.Background())

	vi := &Instance{
		logger: logger,

		ctx:       ctx,
		ctxCancel: ctxCancel,

		cmd: cmd,

		sshMappedPort: uint16(sshPort),
		sshReadyCh:    make(chan struct{}),

		serialRead:   userRead,
		serialReader: userReader,
		serialWrite:  userWrite,
		stderrBuf:    stderrBuf,
	}

	vi.resetSerialStdout()

	return vi, nil
}

func (vi *Instance) Run() error {
	if atomic.AddUint32(&vi.disposed, 1) != 1 {
		return fmt.Errorf("vm disposed")
	}

	err := vi.cmd.Start()
	if err != nil {
		return errors.Wrap(err, "start qemu cmd")
	}

	var globalErrsMu sync.Mutex
	var globalErrs []error

	globalErrFn := func(err error) {
		globalErrsMu.Lock()
		defer globalErrsMu.Unlock()

		globalErrs = append(globalErrs, err, errors.Wrap(vi.Cancel(), "cancel on error"))
	}

	vi.logger.Info("Booting the VM")

	go func() {
		_ = vi.runSerialReader()
		_ = vi.Cancel()
	}()

	go func() {
		err = vi.runVMLoginHandler()
		if err != nil {
			globalErrFn(errors.Wrap(err, "run vm login handler"))
			return
		}

		vi.logger.Info("Setting the VM up")

		sshSigner, err := vi.sshSetup()
		if err != nil {
			globalErrFn(errors.Wrap(err, "set up ssh"))
			return
		}

		vi.logger.Debug("Set up SSH server successfully")

		sshKeyScan, err := vi.scanSSHIdentity()
		if err != nil {
			globalErrFn(errors.Wrap(err, "scan ssh identity"))
			return
		}

		vi.logger.Debug("Scanned SSH identity")

		knownHosts, err := ParseSSHKeyScan(sshKeyScan)
		if err != nil {
			// TODO: Test what actually happens in inline critical errors like this.
			globalErrFn(errors.Wrap(err, "parse ssh key scan"))
			return
		}

		vi.sshConf = &ssh.ClientConfig{
			User:            "root",
			HostKeyCallback: knownHosts,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(sshSigner),
			},
			Timeout: time.Second * 5,
		}

		// This is to notify everyone waiting for SSH to be up that it's ready to go.
		close(vi.sshReadyCh)

		vi.logger.Info("The VM is ready")
	}()

	_, err = vi.cmd.Process.Wait()
	cancelErr := vi.Cancel()
	if err != nil {
		combinedErr := multierr.Combine(
			errors.Wrap(err, "wait for cmd to finish execution"),
			errors.Wrap(cancelErr, "cancel"),
		)

		return fmt.Errorf("%w %v", combinedErr, getLogErrMsg(vi.stderrBuf.String()))
	}

	combinedErr := multierr.Combine(
		append(globalErrs, errors.Wrap(cancelErr, "cancel on exit"))...,
	)
	if combinedErr != nil {
		return fmt.Errorf("%w %v", combinedErr, getLogErrMsg(vi.stderrBuf.String()))
	}

	return nil
}

func (vi *Instance) Cancel() error {
	if atomic.AddUint32(&vi.canceled, 1) != 1 {
		return nil
	}

	vi.ctxCancel()
	return multierr.Combine(
		errors.Wrap(vi.cmd.Process.Signal(os.Interrupt), "cancel cmd"),
		errors.Wrap(vi.serialRead.Close(), "close serial read pipe"),
		errors.Wrap(vi.serialWrite.Close(), "close serial write pipe"),
	)
}

func (vi *Instance) runSerialReader() error {
	for {
		raw, err := vi.serialReader.ReadBytes('\n')
		if err != nil {
			return errors.Wrap(err, "read from serial reader")
		}

		select {
		case vi.serialStdoutCh <- raw:
		default:
			// Message gets discarded if the buffer is full.
		}
	}
}

func (vi *Instance) writeSerial(b []byte) error {
	vi.serialWriteMu.Lock()
	defer vi.serialWriteMu.Unlock()

	_, err := vi.serialWrite.Write(b)
	return err
}

func (vi *Instance) runVMLoginHandler() error {
	for {
		select {
		case <-vi.ctx.Done():
			return nil
		case <-time.After(time.Second):
			peek, err := vi.serialReader.Peek(vi.serialReader.Buffered())
			if err != nil {
				return errors.Wrap(err, "peek stdout")
			}

			if bytes.Contains(peek, []byte("login:")) {
				err = vi.writeSerial([]byte("root\n"))
				if err != nil {
					return errors.Wrap(err, "failed to stdio write login")
				}

				vi.logger.Debug("Logged into the VM serial")

				return nil
			}
		}
	}
}

func (vi *Instance) resetSerialStdout() {
	vi.serialStdoutCh = make(chan []byte, 32)
}

func (vi *Instance) DialSSH() (*ssh.Client, error) {
	if vi.sshConf == nil {
		return nil, ErrSSHUnavailable
	}

	return ssh.Dial("tcp", "localhost:"+fmt.Sprint(vi.sshMappedPort), vi.sshConf)
}

func (vi *Instance) SSHUpNotifyChan() chan struct{} {
	return vi.sshReadyCh
}
