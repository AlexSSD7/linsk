package vm

import (
	"bytes"

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
	c, err := fm.vi.DialSSH()
	if err != nil {
		return nil, errors.Wrap(err, "dial vm ssh")
	}

	sess, err := c.NewSession()
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
