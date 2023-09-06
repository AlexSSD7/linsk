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
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/AlexSSD7/linsk/utils"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

func GenerateSSHKey() (ssh.Signer, []byte, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, errors.Wrap(err, "generate rsa private key")
	}

	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		return nil, nil, errors.Wrap(err, "create signer from key")
	}

	return signer, ssh.MarshalAuthorizedKey(signer.PublicKey()), nil
}

func RunSSHCmd(ctx context.Context, sc *ssh.Client, cmd string) ([]byte, error) {
	var ret []byte
	err := NewSSHSession(ctx, time.Second*15, sc, func(sess *ssh.Session) error {
		stdout := bytes.NewBuffer(nil)
		stderr := bytes.NewBuffer(nil)

		sess.Stdout = stdout
		sess.Stderr = stderr

		err := sess.Run(cmd)
		if err != nil {
			return utils.WrapErrWithLog(err, "run cmd", stderr.String())
		}

		ret = stdout.Bytes()

		return nil
	})

	return ret, err
}

func NewSSHSession(ctx context.Context, timeout time.Duration, sc *ssh.Client, fn func(*ssh.Session) error) error {
	return NewSSHSessionWithDelayedTimeout(ctx, timeout, sc, func(sess *ssh.Session, startTimeout func(preTimeout func())) error {
		startTimeout(nil)
		return fn(sess)
	})
}

func NewSSHSessionWithDelayedTimeout(ctx context.Context, timeout time.Duration, sc *ssh.Client, fn func(sess *ssh.Session, startTimeout func(preTimeout func())) error) error {
	s, err := sc.NewSession()
	if err != nil {
		return errors.Wrap(err, "create new ssh session")
	}

	done := make(chan struct{})
	defer close(done)

	var timedOut bool

	// Start a thread to handle context cancelation.
	go func() {
		select {
		case <-ctx.Done():
			timedOut = true
			_ = sc.Close()
		case <-done:
		}
	}()

	err = fn(s, func(preTimeout func()) {
		// Now start a thread which will close the session
		// down when the timeout hits.
		go func() {
			select {
			case <-time.After(timeout):
				if preTimeout != nil {
					preTimeout()
				}
				timedOut = true
				_ = sc.Close()
			case <-done:
			}
		}()
	})
	if timedOut {
		return fmt.Errorf("timed out (%w)", err)
	}

	return err
}
