package vm

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/AlexSSD7/vldisk/utils"
	"github.com/alessio/shellescape"
	"github.com/pkg/errors"
)

type FileManager struct {
	vi *Instance
}

func NewFileManager(vi *Instance) *FileManager {
	return &FileManager{
		vi: vi,
	}
}

func (fm *FileManager) Init() error {
	c, err := fm.vi.DialSSH()
	if err != nil {
		return errors.Wrap(err, "dial vm ssh")
	}

	_, err = runSSHCmd(c, "apk add util-linux lvm2")
	if err != nil {
		return errors.Wrap(err, "install utilities")
	}

	_, err = runSSHCmd(c, "vgchange -ay")
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

	sess, err := sc.NewSession()
	if err != nil {
		return nil, errors.Wrap(err, "create new vm ssh session")
	}

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

	sess, err := sc.NewSession()
	if err != nil {
		return errors.Wrap(err, "create new vm ssh session")
	}

	err = sess.Run("mount -t " + shellescape.Quote(mo.FSType) + " " + shellescape.Quote(fullDevPath) + " /mnt")
	if err != nil {
		return errors.Wrap(err, "run mount cmd")
	}

	return nil
}
