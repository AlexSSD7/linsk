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
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/AlexSSD7/linsk/sshutil"
	"github.com/AlexSSD7/linsk/utils"
	"github.com/alessio/shellescape"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

func ParseSSHKeyScan(knownHosts []byte) (ssh.HostKeyCallback, error) {
	knownKeysMap := make(map[string][]byte)
	for _, line := range strings.Split(string(knownHosts), "\n") {
		if len(line) == 0 {
			continue
		}

		lineSplit := strings.Split(line, " ")
		if want, have := 3, len(lineSplit); want != have {
			return nil, fmt.Errorf("bad split ssh identity string length: want %v, have %v ('%v')", want, have, line)
		}

		b, err := base64.StdEncoding.DecodeString(lineSplit[2])
		if err != nil {
			return nil, errors.Wrap(err, "decode base64 public key")
		}

		knownKeysMap[lineSplit[1]] = b
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		knownKey, ok := knownKeysMap[key.Type()]
		if !ok {
			return fmt.Errorf("unknown key type '%v'", key.Type())
		}

		if !bytes.Equal(key.Marshal(), knownKey) {
			return fmt.Errorf("public key mismatch")
		}

		return nil
	}, nil
}

func (vm *VM) scanSSHIdentity() ([]byte, error) {
	vm.resetSerialStdout()

	err := vm.writeSerial([]byte(`ssh-keyscan -H 127.0.0.1 && echo "SERIAL STATUS: $?" && rm /root/.ash_history` + "\n"))
	if err != nil {
		return nil, errors.Wrap(err, "write keyscan command to serial")
	}

	deadline := time.Now().Add(time.Second * 5)

	stdOutErrBuf := bytes.NewBuffer(nil)
	ret := bytes.NewBuffer(nil)

	for {
		select {
		case <-vm.ctx.Done():
			return nil, vm.ctx.Err()
		case <-time.After(time.Until(deadline)):
			return nil, fmt.Errorf("keyscan command timed out %v", utils.GetLogErrMsg(stdOutErrBuf.String(), "stdout/stderr log"))
		case data := <-vm.serialStdoutCh:
			if len(data) == 0 {
				continue
			}

			stdOutErrBuf.Write(data)

			// This isn't clean at all, but there is no better
			// way to achieve an exit status check like this.
			prefix := []byte("SERIAL STATUS: ")
			if bytes.HasPrefix(data, prefix) {
				if len(data) == len(prefix) {
					return nil, fmt.Errorf("keyscan command status code did not show up")
				}

				if data[len(prefix)] != '0' {
					return nil, fmt.Errorf("non-zero keyscan command status code: '%v' %v", string(data[len(prefix)]), utils.GetLogErrMsg(stdOutErrBuf.String(), "stdout/stderr log"))
				}

				return ret.Bytes(), nil
			} else if data[0] == '|' {
				ret.Write(data)
			}
		}
	}
}

func (vm *VM) sshSetup() (ssh.Signer, error) {
	vm.resetSerialStdout()

	sshSigner, sshPublicKey, err := sshutil.GenerateSSHKey()
	if err != nil {
		return nil, errors.Wrap(err, "generate ssh key")
	}

	installSSHDCmd := ""
	if vm.installSSH {
		installSSHDCmd = "apk add openssh; "
	}

	cmd := `do_setup () { sh -c "set -ex; ifconfig eth0 up && ifconfig lo up && udhcpc; ` + installSSHDCmd + `mkdir -p ~/.ssh; echo ` + shellescape.Quote(string(sshPublicKey)) + ` > ~/.ssh/authorized_keys; rc-update add sshd; rc-service sshd start"; echo "SERIAL"" ""STATUS: $?"; }; do_setup` + "\n"

	err = vm.writeSerial([]byte(cmd))
	if err != nil {
		return nil, errors.Wrap(err, "write ssh setup serial command")
	}

	deadline := time.Now().Add(time.Second * 30)

	stdOutErrBuf := bytes.NewBuffer(nil)

	for {
		select {
		case <-vm.ctx.Done():
			return nil, vm.ctx.Err()
		case <-time.After(time.Until(deadline)):
			return nil, fmt.Errorf("setup command timed out %v", utils.GetLogErrMsg(stdOutErrBuf.String(), "stdout/stderr log"))
		case data := <-vm.serialStdoutCh:
			// This isn't clean at all, but there is no better
			// way to achieve an exit status check like this.
			prefix := []byte("SERIAL STATUS: ")
			stdOutErrBuf.WriteString(utils.ClearUnprintableChars(string(data), true))
			if bytes.HasPrefix(data, prefix) {
				if len(data) == len(prefix) {
					return nil, fmt.Errorf("setup command status code did not show up")
				}

				if data[len(prefix)] != '0' {
					// A non-pretty yet effective debug print to assist with debugging
					// in case something ever goes wrong.
					fmt.Fprintf(os.Stderr, "SSH SETUP FAILURE:\n%v", stdOutErrBuf.String())

					return nil, fmt.Errorf("non-zero setup command status code: '%v' %v", string(data[len(prefix)]), utils.GetLogErrMsg(stdOutErrBuf.String(), "stdout/stderr log"))
				}

				return sshSigner, nil
			}
		}
	}
}
