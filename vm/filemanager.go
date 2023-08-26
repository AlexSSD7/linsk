package vm

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/AlexSSD7/linsk/utils"
	"github.com/alessio/shellescape"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

type FileManager struct {
	logger *slog.Logger

	vi *Instance
}

func NewFileManager(logger *slog.Logger, vi *Instance) *FileManager {
	return &FileManager{
		logger: logger,

		vi: vi,
	}
}

func (fm *FileManager) Init() error {
	sc, err := fm.vi.DialSSH()
	if err != nil {
		return errors.Wrap(err, "dial vm ssh")
	}

	defer func() { _ = sc.Close() }()

	_, err = runSSHCmd(sc, "apk add util-linux lvm2")
	if err != nil {
		return errors.Wrap(err, "install utilities")
	}

	_, err = runSSHCmd(sc, "vgchange -ay")
	if err != nil {
		return errors.Wrap(err, "run vgchange cmd")
	}

	return nil
}

func (fm *FileManager) Lsblk() ([]byte, error) {
	sc, err := fm.vi.DialSSH()
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

	defer func() { sess.Close() }()

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

	lg.Info("Attempting to open LUKS device")

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

	// TODO: Timeout for this command

	err = sess.Wait()
	if err != nil {
		return wrapErrWithLog(err, "wait for cryptsetup luksopen cmd to finish", stderrBuf.String())
	}

	lg.Info("LUKS device opened successfully")

	// Clear the memory up
	{
		for i := 0; i < len(pwd); i++ {
			pwd[i] = 0
		}

		for i := 0; i < 16; i++ {
			_, _ = rand.Read(pwd)
		}
	}

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

	fullDevPath := filepath.Clean("/dev/" + devName)

	if mo.FSType == "" {
		return fmt.Errorf("fs type is empty")
	}

	sc, err := fm.vi.DialSSH()
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

	_, err = runSSHCmd(sc, "mount -t "+shellescape.Quote(mo.FSType)+" "+shellescape.Quote(fullDevPath)+" /mnt")
	if err != nil {
		return errors.Wrap(err, "run mount cmd")
	}

	return nil
}

func (fm *FileManager) StartSMB(pwd []byte) error {
	scpClient, err := fm.vi.DialSCP()
	if err != nil {
		return errors.Wrap(err, "dial scp")
	}

	defer scpClient.Close()

	sambaCfg := `[global]
workgroup = WORKGROUP
dos charset = cp866
unix charset = utf-8
	
[linsk]
browseable = yes
writeable = yes
path = /mnt
force user = linsk
force group = linsk
create mask = 0664`

	err = scpClient.CopyFile(fm.vi.ctx, strings.NewReader(sambaCfg), "/etc/samba/smb.conf", "0400")
	if err != nil {
		return errors.Wrap(err, "copy samba config file")
	}

	scpClient.Close()

	sc, err := fm.vi.DialSSH()
	if err != nil {
		return errors.Wrap(err, "dial ssh")
	}

	defer func() { _ = sc.Close() }()

	_, err = runSSHCmd(sc, "rc-update add samba && rc-service samba start")
	if err != nil {
		return errors.Wrap(err, "add and start samba service")
	}

	sess, err := sc.NewSession()
	if err != nil {
		return errors.Wrap(err, "create new ssh session")
	}

	pwd = append(pwd, '\n')

	stderr := bytes.NewBuffer(nil)
	sess.Stderr = stderr

	stdinPipe, err := sess.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "stdin pipe")
	}

	// TODO: Timeout for this command

	err = sess.Start("smbpasswd -a linsk")
	if err != nil {
		return errors.Wrap(err, "start change samba password cmd")
	}

	go func() {
		_, err = stdinPipe.Write(pwd)
		if err != nil {
			fm.vi.logger.Error("Failed to write SMB password to smbpasswd stdin", "error", err)
		}
		_, err = stdinPipe.Write(pwd)
		if err != nil {
			fm.vi.logger.Error("Failed to write repeated SMB password to smbpasswd stdin", "error", err)
		}
	}()

	err = sess.Wait()
	if err != nil {
		return wrapErrWithLog(err, "wait for change samba password cmd", stderr.String())
	}

	return nil
}
