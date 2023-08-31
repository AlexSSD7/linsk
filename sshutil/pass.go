package sshutil

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/AlexSSD7/linsk/utils"
	"github.com/alessio/shellescape"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"golang.org/x/crypto/ssh"
)

func genericChangePass(ctx context.Context, sc *ssh.Client, user string, pwd string, cmd string) error {
	if !utils.ValidateUnixUsername(user) {
		return fmt.Errorf("invalid unix username")
	}

	return NewSSHSession(ctx, time.Second*10, sc, func(sess *ssh.Session) error {
		stderr := bytes.NewBuffer(nil)
		sess.Stderr = stderr

		stdinPipe, err := sess.StdinPipe()
		if err != nil {
			return errors.Wrap(err, "stdin pipe")
		}

		err = sess.Start(cmd + " " + shellescape.Quote(user))
		if err != nil {
			return errors.Wrap(err, "start change user password cmd")
		}

		pwdBytes := []byte(pwd + "\n")
		defer func() {
			// Clearing the memory up for security.
			for i := range pwdBytes {
				pwdBytes[i] = 0
			}
		}()

		go func() {
			// Writing the password. We're doing this two times
			// as we need to confirm the password.
			_, _ = stdinPipe.Write(pwdBytes)
			_, _ = stdinPipe.Write(pwdBytes)
		}()

		err = sess.Wait()
		if err != nil {
			return multierr.Combine(utils.WrapErrWithLog(err, "wait for change user password cmd", stderr.String()))
		}

		return nil
	})
}

func ChangeUnixPass(ctx context.Context, sc *ssh.Client, user string, pwd string) error {
	return genericChangePass(ctx, sc, user, pwd, "passwd")
}

func ChangeSambaPass(ctx context.Context, sc *ssh.Client, user string, pwd string) error {
	return genericChangePass(ctx, sc, user, pwd, "smbpasswd -a")
}
