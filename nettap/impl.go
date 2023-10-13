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

//go:build !windows

package nettap

import (
	"log/slog"
)

func Available() bool {
	return false
}

type TapManager struct {
	logger *slog.Logger
}

func NewTapManager(_ *slog.Logger) (*TapManager, error) {
	return nil, ErrTapManagerUnimplemented
}

func NewUniqueTapName() (string, error) {
	return "", ErrTapManagerUnimplemented
}

func (tm *TapManager) CreateNewTap(_ string) error {
	return ErrTapManagerUnimplemented
}

func ValidateTapName(_ string) error {
	return ErrTapManagerUnimplemented
}

func (tm *TapManager) DeleteTap(_ string) error {
	return ErrTapManagerUnimplemented
}

func (tm *TapManager) ConfigureNet(_ string, _ string) error {
	return ErrTapManagerUnimplemented
}
