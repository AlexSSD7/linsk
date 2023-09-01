package vm

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/AlexSSD7/linsk/sshutil"
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

	_, err = sshutil.RunSSHCmd(fm.vm.ctx, sc, "vgchange -ay")
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

	ret, err := sshutil.RunSSHCmd(fm.vm.ctx, sc, "lsblk -o NAME,SIZE,FSTYPE,LABEL -e 7,11,2")
	if err != nil {
		return nil, errors.Wrap(err, "run lsblk")
	}

	return ret, nil
}

type MountOptions struct {
	FSType string
	LUKS   bool
}

const luksDMName = "cryptmnt"

func (fm *FileManager) luksOpen(sc *ssh.Client, fullDevPath string) error {
	lg := fm.logger.With("vm-path", fullDevPath)

	return sshutil.NewSSHSessionWithDelayedTimeout(fm.vm.ctx, time.Second*15, sc, func(sess *ssh.Session, startTimeout func(preTimeout func())) error {
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

		// We start the timeout countdown now only to avoid timing out
		// while the user is entering the password, or shortly after that.
		startTimeout(func() {
			lg.Warn("LUKS open command timed out. If you are using large-memory key derivation function, try increasing the VM memory allocation using --vm-mem-alloc flag.")
		})

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
			// Clear the memory up for security.
			{
				for i := 0; i < len(pwd); i++ {
					pwd[i] = 0
				}

				// This is my paranoia.
				_, _ = rand.Read(pwd)
				_, _ = rand.Read(pwd)
			}
		}()

		err = sess.Wait()
		if err != nil {
			if strings.Contains(stderrBuf.String(), "Not enough available memory to open a keyslot.") {
				fm.logger.Warn("Detected not enough memory to open a LUKS device, please allocate more memory using --vm-mem-alloc flag.")
			}

			return utils.WrapErrWithLog(err, "wait for cryptsetup luksopen cmd to finish", stderrBuf.String())
		}

		lg.Info("LUKS device opened successfully")

		_ = stdinPipe.Close()
		wWG.Wait()

		return wErr
	})
}

func (fm *FileManager) Mount(devName string, mo MountOptions) error {
	if devName == "" {
		return fmt.Errorf("device name is empty")
	}

	// It does allow "mapper/" prefix for mapped devices.
	// This is to enable the support for LVM and LUKS.
	if !utils.ValidateDevName(devName) {
		return fmt.Errorf("bad device name")
	}

	// We're intentionally not calling filepath.Clean() as
	// this causes unintended consequences when run on Windows.
	// (Windows Go standard library treats the path as it's for
	// Windows, but we're targeting a Linux VM.)
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

	_, err = sshutil.RunSSHCmd(fm.vm.ctx, sc, "mount -t "+shellescape.Quote(mo.FSType)+" "+shellescape.Quote(fullDevPath)+" /mnt")
	if err != nil {
		return errors.Wrap(err, "run mount cmd")
	}

	return nil
}

func (fm *FileManager) StartFTP(pwd string, passivePortStart uint16, passivePortCount uint16, extIP net.IP) error {
	// This timeout is for the SCP client exclusively.
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
pasv_address=` + extIP.String() + `
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

	defer func() { _ = sc.Close() }()

	_, err = sshutil.RunSSHCmd(fm.vm.ctx, sc, "rc-update add vsftpd && rc-service vsftpd start")
	if err != nil {
		return errors.Wrap(err, "add and start ftpd service")
	}

	err = sshutil.ChangeUnixPass(fm.vm.ctx, sc, "linsk", pwd)
	if err != nil {
		return errors.Wrap(err, "change unix pass")
	}

	return nil
}

func (fm *FileManager) StartSMB(pwd string) error {
	// This timeout is for the SCP client exclusively.
	scpCtx, scpCtxCancel := context.WithTimeout(fm.vm.ctx, time.Second*5)
	defer scpCtxCancel()

	scpClient, err := fm.vm.DialSCP()
	if err != nil {
		return errors.Wrap(err, "dial scp")
	}

	defer scpClient.Close()

	sambaCfg := `[global]
workgroup = WORKGROUP
dos charset = cp866
unix charset = utf-8
client min protocol = SMB2
client max protocol = SMB3

read raw = yes
write raw = yes
socket options = TCP_NODELAY IPTOS_LOWDELAY SO_RCVBUF=131072 SO_SNDBUF=131072
min receivefile size = 16384
use sendfile = true
aio read size = 16384
aio write size = 16384
server signing = no

[linsk]
browseable = yes
writeable = yes
path = /mnt
force user = linsk
force group = linsk
create mask = 0664
`

	err = scpClient.CopyFile(scpCtx, strings.NewReader(sambaCfg), "/etc/samba/smb.conf", "0400")
	if err != nil {
		return errors.Wrap(err, "copy samba config file")
	}

	scpClient.Close()

	sc, err := fm.vm.DialSSH()
	if err != nil {
		return errors.Wrap(err, "dial ssh")
	}

	defer func() { _ = sc.Close() }()

	_, err = sshutil.RunSSHCmd(fm.vm.ctx, sc, "rc-update add samba && rc-service samba start")
	if err != nil {
		return errors.Wrap(err, "add and start samba service")
	}

	err = sshutil.ChangeSambaPass(fm.vm.ctx, sc, "linsk", pwd)
	if err != nil {
		return errors.Wrap(err, "change samba pass")
	}

	return nil
}

func (fm *FileManager) StartAFP(pwd string) error {
	// This timeout is for the SCP client exclusively.
	scpCtx, scpCtxCancel := context.WithTimeout(fm.vm.ctx, time.Second*5)
	defer scpCtxCancel()

	scpClient, err := fm.vm.DialSCP()
	if err != nil {
		return errors.Wrap(err, "dial scp")
	}

	defer scpClient.Close()

	afpCfg := `[Global]

[linsk]
path = /mnt
file perm = 0664
directory perm = 0775
valid users = linsk
force user = linsk
force group = linsk
`

	err = scpClient.CopyFile(scpCtx, strings.NewReader(afpCfg), "/etc/afp.conf", "0400")
	if err != nil {
		return errors.Wrap(err, "copy netatalk config file")
	}

	scpClient.Close()

	sc, err := fm.vm.DialSSH()
	if err != nil {
		return errors.Wrap(err, "dial ssh")
	}

	defer func() { _ = sc.Close() }()

	_, err = sshutil.RunSSHCmd(fm.vm.ctx, sc, "rc-update add netatalk && rc-service netatalk start")
	if err != nil {
		return errors.Wrap(err, "add and start netatalk service")
	}

	err = sshutil.ChangeUnixPass(fm.vm.ctx, sc, "linsk", pwd)
	if err != nil {
		return errors.Wrap(err, "change unix pass")
	}

	return nil
}
