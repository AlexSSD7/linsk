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

func (fm *FileManager) InitLVM() error {
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

type MountConfig struct {
	LUKSContainerPreopen string

	FSTypeOverride string
	LUKS           bool
	MountOptions   string
}

func (fm *FileManager) luksOpen(sc *ssh.Client, fullDevPath string, luksDMName string) error {
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

		pwd, err := term.ReadPassword(int(syscall.Stdin)) //nolint:unconvert // On Windows it's a different non-int type.
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

func (fm *FileManager) PreopenLUKSContainer(containerDevPath string) error {
	sc, err := fm.vm.DialSSH()
	if err != nil {
		return errors.Wrap(err, "dial vm ssh")
	}

	defer func() { _ = sc.Close() }()

	return fm.preopenLUKSContainerWithSSH(sc, containerDevPath)
}

func (fm *FileManager) preopenLUKSContainerWithSSH(sc *ssh.Client, containerDevPath string) error {
	if !utils.ValidateDevName(containerDevPath) {
		return fmt.Errorf("bad luks container device name")
	}

	fullContainerDevPath := "/dev/" + containerDevPath

	fm.logger.Info("Preopening a LUKS container", "container", fullContainerDevPath)

	err := fm.luksOpen(sc, fullContainerDevPath, "cryptcontainer")
	if err != nil {
		return errors.Wrap(err, "luks (pre)open container")
	}

	err = fm.InitLVM()
	if err != nil {
		return errors.Wrap(err, "reinit lvm")
	}

	return nil
}

func (fm *FileManager) Mount(devName string, mc MountConfig) error {
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

	var fsOverride string
	if mc.FSTypeOverride != "" {
		if !utils.ValidateFsType(mc.FSTypeOverride) {
			return fmt.Errorf("bad fs type override (contains illegal characters)")
		}
		fsOverride = mc.FSTypeOverride
	}

	var mountOptions string
	if mc.MountOptions != "" {
		if !utils.ValidateMountOptions(mc.MountOptions) {
			return fmt.Errorf("invalid mount options (contains illegal characters)")
		}
		mountOptions = mc.MountOptions
	}

	sc, err := fm.vm.DialSSH()
	if err != nil {
		return errors.Wrap(err, "dial vm ssh")
	}

	defer func() { _ = sc.Close() }()

	if mc.LUKSContainerPreopen != "" {
		err := fm.preopenLUKSContainerWithSSH(sc, mc.LUKSContainerPreopen)
		if err != nil {
			return errors.Wrap(err, "preopen luks container")
		}
	}

	if mc.LUKS {
		luksDMName := "cryptmnt"

		err = fm.luksOpen(sc, fullDevPath, luksDMName)
		if err != nil {
			return errors.Wrap(err, "luks open")
		}

		fullDevPath = "/dev/mapper/" + luksDMName
	}

	cmd := "mount "
	if fsOverride != "" {
		cmd += "-t " + shellescape.Quote(fsOverride) + " "
	}
	if mountOptions != "" {
		cmd += "-o " + shellescape.Quote(mountOptions) + " "
	}
	cmd += shellescape.Quote(fullDevPath) + " /mnt"

	_, err = sshutil.RunSSHCmd(fm.vm.ctx, sc, cmd)
	if err != nil {
		return errors.Wrap(err, "run mount cmd")
	}

	return nil
}

func (fm *FileManager) StartFTP(pwd string, passivePortStart uint16, passivePortCount uint16, extIP net.IP) error {
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

	return fm.startGenericShare(pwd, ftpdCfg, "/etc/vsftpd/vsftpd.conf", "vsftpd", sshutil.ChangeUnixPass)
}

func (fm *FileManager) StartSMB(pwd string) error {
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
	return fm.startGenericShare(pwd, sambaCfg, "/etc/samba/smb.conf", "samba", sshutil.ChangeSambaPass)
}

func (fm *FileManager) StartAFP(pwd string) error {
	afpCfg := `[Global]

[linsk]
path = /mnt
file perm = 0664
directory perm = 0775
valid users = linsk
force user = linsk
force group = linsk
`

	return fm.startGenericShare(pwd, afpCfg, "/etc/afp.conf", "netatalk", sshutil.ChangeUnixPass)
}

func (fm *FileManager) startGenericShare(pwd string, cfg string, cfgPath string, rcServiceName string, changePassFunc sshutil.ChangePassFunc) error {
	// This timeout is for the SCP client exclusively.
	scpCtx, scpCtxCancel := context.WithTimeout(fm.vm.ctx, time.Second*5)
	defer scpCtxCancel()

	scpClient, err := fm.vm.DialSCP()
	if err != nil {
		return errors.Wrap(err, "dial scp")
	}

	defer scpClient.Close()

	err = scpClient.CopyFile(scpCtx, strings.NewReader(cfg), cfgPath, "0400")
	if err != nil {
		return errors.Wrap(err, "copy config file")
	}

	scpClient.Close()

	sc, err := fm.vm.DialSSH()
	if err != nil {
		return errors.Wrap(err, "dial ssh")
	}

	defer func() { _ = sc.Close() }()

	_, err = sshutil.RunSSHCmd(fm.vm.ctx, sc, "rc-update add "+shellescape.Quote(rcServiceName)+" && rc-service "+shellescape.Quote(rcServiceName)+" start")
	if err != nil {
		return errors.Wrap(err, "add and start rc service")
	}

	err = changePassFunc(fm.vm.ctx, sc, "linsk", pwd)
	if err != nil {
		return errors.Wrap(err, "change pass")
	}

	return nil
}
