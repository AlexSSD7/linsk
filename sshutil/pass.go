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

type ChangePassFunc func(ctx context.Context, sc *ssh.Client, user string, pwd string) error

func ChangeUnixPass(ctx context.Context, sc *ssh.Client, user string, pwd string) error {
	return genericChangePass(ctx, sc, user, pwd, "passwd")
}

func ChangeSambaPass(ctx context.Context, sc *ssh.Client, user string, pwd string) error {
	return genericChangePass(ctx, sc, user, pwd, "smbpasswd -a")
}
