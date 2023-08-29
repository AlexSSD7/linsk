package vm

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/AlexSSD7/linsk/utils"
	"github.com/alessio/shellescape"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

type FileManager struct {
	logger *slog.Logger

	vm *VM
}

func NewFileManager(logger *slog.Logger, vm *VM) *FileManager {
	return &FileManager{
		logger: logger,

		vm: vm,
	}
}

func (fm *FileManager) Init() error {
	sc, err := fm.vm.DialSSH()
	if err != nil {
		return errors.Wrap(err, "dial vm ssh")
	}

	defer func() { _ = sc.Close() }()

	_, err = runSSHCmd(sc, "vgchange -ay")
	if err != nil {
		return errors.Wrap(err, "run vgchange cmd")
	}

	return nil
}

func (fm *FileManager) Lsblk() ([]byte, error) {
	sc, err := fm.vm.DialSSH()
	if err != nil {
		return nil, errors.Wrap(err, "dial vm ssh")
	}

	defer func() { _ = sc.Close() }()

	sess, err := sc.NewSession()
	if err != nil {
		return nil, errors.Wrap(err, "create new vm ssh session")
	}

	defer func() { _ = sess.Close() }()

	ret := new(bytes.Buffer)

	sess.Stdout = ret

	err = sess.Run("lsblk -o NAME,SIZE,FSTYPE,LABEL -e 7,11,2,253")
	if err != nil {
		return nil, errors.Wrap(err, "run lsblk")
	}

	return ret.Bytes(), nil
}

type MountOptions struct {
	FSType string
	LUKS   bool
}

const luksDMName = "cryptmnt"

func (fm *FileManager) luksOpen(sc *ssh.Client, fullDevPath string) error {
	lg := fm.logger.With("vm-path", fullDevPath)

	sess, err := sc.NewSession()
	if err != nil {
		return errors.Wrap(err, "create new vm ssh session")
	}

	stdinPipe, err := sess.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "create vm ssh session stdin pipe")
	}

	stderrBuf := bytes.NewBuffer(nil)
	sess.Stderr = stderrBuf

	err = sess.Start("cryptsetup luksOpen " + shellescape.Quote(fullDevPath) + " " + luksDMName)
	if err != nil {
		return errors.Wrap(err, "start cryptsetup luksopen cmd")
	}

	lg.Info("Attempting to open a LUKS device")

	_, err = os.Stderr.Write([]byte("Enter Password: "))
	if err != nil {
		return errors.Wrap(err, "write prompt to stderr")
	}

	pwd, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return errors.Wrap(err, "read luks password")
	}

	fmt.Print("\n")

	var wErr error
	var wWG sync.WaitGroup

	wWG.Add(1)
	go func() {
		defer wWG.Done()

		_, err := stdinPipe.Write(pwd)
		_, err2 := stdinPipe.Write([]byte("\n"))
		wErr = errors.Wrap(multierr.Combine(err, err2), "write password to stdin")
	}()

	defer func() {
		// Clear the memory up.
		{
			for i := 0; i < len(pwd); i++ {
				pwd[i] = 0
			}

			for i := 0; i < 4; i++ {
				_, _ = rand.Read(pwd)
			}
		}
	}()

	done := make(chan struct{})
	defer close(done)

	var timedOut bool

	go func() {
		tm := func() {
			timedOut = true
			_ = sc.Close()
		}
		select {
		case <-fm.vm.ctx.Done():
			tm()
		case <-time.After(time.Second * 1):
			tm()
		case <-done:
		}
	}()

	checkTimeoutErr := func(err error) error {
		if timedOut {
			return fmt.Errorf("timed out (%w)", err)
		}

		return err
	}

	err = sess.Wait()
	if err != nil {
		if strings.Contains(stderrBuf.String(), "Not enough available memory to open a keyslot.") {
			fm.logger.Warn("Detected not enough memory to open a LUKS device, please allocate more memory using --vm-mem-alloc flag.")
		}

		return checkTimeoutErr(utils.WrapErrWithLog(err, "wait for cryptsetup luksopen cmd to finish", stderrBuf.String()))
	}

	lg.Info("LUKS device opened successfully")

	_ = stdinPipe.Close()
	wWG.Wait()

	return wErr
}

func (fm *FileManager) Mount(devName string, mo MountOptions) error {
	if devName == "" {
		return fmt.Errorf("device name is empty")
	}

	// It does allow mapper/ prefix for mapped devices.
	// This is to enable the support for LVM and LUKS.
	if !utils.ValidateDevName(devName) {
		return fmt.Errorf("bad device name")
	}

	fullDevPath := "/dev/" + devName

	if mo.FSType == "" {
		return fmt.Errorf("fs type is empty")
	}

	sc, err := fm.vm.DialSSH()
	if err != nil {
		return errors.Wrap(err, "dial vm ssh")
	}

	defer func() { _ = sc.Close() }()

	if mo.LUKS {
		err = fm.luksOpen(sc, fullDevPath)
		if err != nil {
			return errors.Wrap(err, "luks open")
		}

		fullDevPath = "/dev/mapper/" + luksDMName
	}

	done := make(chan struct{})
	defer close(done)

	var timedOut bool

	go func() {
		tm := func() {
			timedOut = true
			_ = sc.Close()
		}
		select {
		case <-fm.vm.ctx.Done():
			tm()
		case <-time.After(time.Second * 10):
			tm()
		case <-done:
		}
	}()

	checkTimeoutErr := func(err error) error {
		if timedOut {
			return fmt.Errorf("timed out (%w)", err)
		}

		return err
	}

	_, err = runSSHCmd(sc, "mount -t "+shellescape.Quote(mo.FSType)+" "+shellescape.Quote(fullDevPath)+" /mnt")
	if err != nil {
		return checkTimeoutErr(errors.Wrap(err, "run mount cmd"))
	}

	return nil
}

func (fm *FileManager) StartFTP(pwd []byte, passivePortStart uint16, passivePortCount uint16) error {
	scpCtx, scpCtxCancel := context.WithTimeout(fm.vm.ctx, time.Second*5)
	defer scpCtxCancel()

	scpClient, err := fm.vm.DialSCP()
	if err != nil {
		return errors.Wrap(err, "dial scp")
	}

	defer scpClient.Close()

	ftpdCfg := `anonymous_enable=NO
local_enable=YES
write_enable=YES
local_umask=022
chroot_local_user=YES
allow_writeable_chroot=YES
listen=YES
seccomp_sandbox=NO
pasv_min_port=` + fmt.Sprint(passivePortStart) + `
pasv_max_port=` + fmt.Sprint(passivePortStart+passivePortCount) + `
pasv_address=127.0.0.1
`

	err = scpClient.CopyFile(scpCtx, strings.NewReader(ftpdCfg), "/etc/vsftpd/vsftpd.conf", "0400")
	if err != nil {
		return errors.Wrap(err, "copy ftpd .conf file")
	}

	scpClient.Close()

	sc, err := fm.vm.DialSSH()
	if err != nil {
		return errors.Wrap(err, "dial ssh")
	}

	done := make(chan struct{})
	defer close(done)

	var timedOut bool

	go func() {
		tm := func() {
			timedOut = true
			_ = sc.Close()
		}
		select {
		case <-fm.vm.ctx.Done():
			tm()
		case <-time.After(time.Second * 15):
			tm()
		case <-done:
		}
	}()

	checkTimeoutErr := func(err error) error {
		if timedOut {
			return fmt.Errorf("timed out (%w)", err)
		}

		return err
	}

	defer func() { _ = sc.Close() }()

	_, err = runSSHCmd(sc, "rc-update add vsftpd && rc-service vsftpd start")
	if err != nil {
		return checkTimeoutErr(errors.Wrap(err, "add and start ftpd service"))
	}

	sess, err := sc.NewSession()
	if err != nil {
		return checkTimeoutErr(errors.Wrap(err, "create new ssh session"))
	}

	pwd = append(pwd, '\n')

	stderr := bytes.NewBuffer(nil)
	sess.Stderr = stderr

	stdinPipe, err := sess.StdinPipe()
	if err != nil {
		return checkTimeoutErr(errors.Wrap(err, "stdin pipe"))
	}

	err = sess.Start("passwd linsk")
	if err != nil {
		return checkTimeoutErr(errors.Wrap(err, "start change user password cmd"))
	}

	go func() {
		_, err = stdinPipe.Write(pwd)
		if err != nil {
			fm.vm.logger.Error("Failed to write FTP password to passwd stdin", "error", err.Error())
		}
		_, err = stdinPipe.Write(pwd)
		if err != nil {
			fm.vm.logger.Error("Failed to write repeated FTP password to passwd stdin", "error", err.Error())
		}
	}()

	err = sess.Wait()
	if err != nil {
		return checkTimeoutErr(utils.WrapErrWithLog(err, "wait for change user password cmd", stderr.String()))
	}

	return nil
}
