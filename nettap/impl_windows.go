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

//go:build windows

package nettap

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"log/slog"

	"github.com/AlexSSD7/linsk/utils"
	"github.com/alessio/shellescape"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

func Available() bool {
	return true
}

type TapManager struct {
	logger *slog.Logger

	tapctlPath string
}

func NewTapManager(logger *slog.Logger) (*TapManager, error) {
	tapctlPath := `C:\Program Files\OpenVPN\bin\tapctl.exe`
	_, err := os.Stat(tapctlPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logger.Warn("Required OpenVPN tap networking Windows drivers do not appear to be installed. The easiest way to get them is to install OpenVPN: https://openvpn.net/community-downloads/")
		}
		return nil, errors.Wrapf(err, "stat tapctl path '%v'", tapctlPath)
	}

	return &TapManager{
		logger: logger,

		tapctlPath: tapctlPath,
	}, nil
}

// We need some sort of format to avoid conflicting with other Windows interfaces.
var tapNameRegexp = regexp.MustCompile(`^LinskTap-\d+$`)

func NewUniqueTapName() (string, error) {
	time.Sleep(time.Millisecond)
	return fmt.Sprintf("LinskTap-%v", time.Now().UnixNano()), nil
}

func (tm *TapManager) CreateNewTap(tapName string) error {
	err := ValidateTapName(tapName)
	if err != nil {
		return errors.Wrap(err, "validate tap name")
	}

	out, err := exec.Command(tm.tapctlPath, "create", "--name", tapName).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "exec tapctl create cmd (out '%v')", utils.ClearUnprintableChars(string(out), false))
	}

	tm.logger.Info("Created network tap", "name", tapName)

	return nil
}

func ValidateTapName(s string) error {
	if !tapNameRegexp.MatchString(s) {
		return fmt.Errorf("invalid tap name '%v'", s)
	}

	return nil
}

func (tm *TapManager) DeleteTap(name string) error {
	stderr := bytes.NewBuffer(nil)
	cmd := exec.Command(tm.tapctlPath, "list")
	cmd.Stderr = stderr
	tapList, err := cmd.Output()
	if err != nil {
		return errors.Wrapf(err, "exec tapctl list cmd (out '%v')", utils.ClearUnprintableChars(stderr.String(), false))
	}

	for _, line := range strings.Split(string(tapList), "\n") {
		if line == "" {
			continue
		}

		line = strings.ReplaceAll(line, "\t", " ")
		line = utils.ClearUnprintableChars(line, false)

		split := strings.Split(line, " ")
		if want, have := 2, len(split); want > have {
			return fmt.Errorf("bad tap list item split length: want %v > have %v (line '%v')", want, have, line)
		}

		if name != split[1] {
			continue
		}

		lineTapUUIDStr := strings.TrimPrefix(split[0], "{")
		lineTapUUIDStr = strings.TrimSuffix(lineTapUUIDStr, "}")
		lineTapUUID, err := uuid.Parse(lineTapUUIDStr)
		if err != nil {
			return errors.Wrapf(err, "parse found line tap uuid (value '%v', line '%v')", lineTapUUIDStr, line)
		}

		deleteOut, err := exec.Command(tm.tapctlPath, "delete", "{"+lineTapUUID.String()+"}").CombinedOutput()
		if err != nil {
			return errors.Wrapf(err, "exec tapctl delete (out '%v')", utils.ClearUnprintableChars(string(deleteOut), false))
		}

		tm.logger.Info("Deleted network tap", "name", name)

		return nil
	}

	return ErrTapNotFound
}

func (tm *TapManager) ConfigureNet(tapName string, hostCIDR string) error {
	err := ValidateTapName(tapName)
	if err != nil {
		return errors.Wrap(err, "validate tap name")
	}

	ip, _, err := net.ParseCIDR(hostCIDR)
	if err != nil {
		return errors.Wrap(err, "parse cidr")
	}

	if !utils.IsIPv6IP(ip) {
		return fmt.Errorf("ipv6 is accepted only (have '%v')", ip)
	}

	out, err := exec.Command("netsh", "interface", "ipv6", "set", "address", shellescape.Quote(tapName), shellescape.Quote(hostCIDR)).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "exec netsh cmd (out '%v')", utils.ClearUnprintableChars(string(out), false))
	}

	tm.logger.Info("Configured network tap", "name", tapName, "cidr", hostCIDR)

	return nil
}
