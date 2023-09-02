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

package nettap

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/process"
)

type Alloc struct {
	TapName string
	PID     int
}

func (a *Alloc) Validate() error {
	err := ValidateTapName(a.TapName)
	if err != nil {
		return errors.Wrap(err, "validate tap name")
	}

	if a.PID == 0 {
		return fmt.Errorf("pid is zero")
	}

	if a.PID > int(int32(a.PID)) {
		return fmt.Errorf("pid int32 overflow (%v)", a.PID)
	}

	return nil
}

// The taps removed slice always returns the taps removed, even after
// an error has occurred sometime while deleting non-first interfaces.
func (tm *TapManager) PruneTaps(knownAllocs []Alloc) ([]string, error) {
	var tapsRemoved []string

	for i, alloc := range knownAllocs {
		err := alloc.Validate()
		if err != nil {
			return tapsRemoved, errors.Wrapf(err, "validate alloc #%v", i)
		}
	}

	runningPids, err := process.Pids()
	if err != nil {
		return tapsRemoved, errors.Wrap(err, "get running pids")
	}

	runningPidsMap := make(map[int32]struct{})
	for _, pid := range runningPids {
		runningPidsMap[pid] = struct{}{}
	}

	var tapsToRemove []string

	for _, alloc := range knownAllocs {
		if _, exists := runningPidsMap[int32(alloc.PID)]; !exists {
			tm.logger.Info("Found a dangling network tap", "name", alloc.TapName, "pid", alloc.PID)
			tapsToRemove = append(tapsToRemove, alloc.TapName)
		}
	}

	for _, tapToRemove := range tapsToRemove {
		err = tm.DeleteTap(tapToRemove)
		if err != nil {
			if errors.Is(err, ErrTapNotFound) {
				tm.logger.Warn("Attempted to prune a network tap that doesn't exist, skipping", "name", tapToRemove)
			} else {
				return tapsRemoved, errors.Wrapf(err, "delete tap '%v'", tapToRemove)
			}
		}

		tapsRemoved = append(tapsRemoved, tapToRemove)
	}

	return tapsRemoved, nil
}
