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

func NewTapManager(logger *slog.Logger) (*TapManager, error) {
	return nil, ErrTapManagerUnimplemented
}

func NewRandomTapName() (string, error) {
	return "", ErrTapManagerUnimplemented
}

func (tm *TapManager) CreateNewTap(tapName string) error {
	return ErrTapManagerUnimplemented
}

func ValidateTapName(s string) error {
	return ErrTapManagerUnimplemented
}

func (tm *TapManager) DeleteTap(name string) error {
	return ErrTapManagerUnimplemented
}

func (tm *TapManager) ConfigureNet(tapName string, hostCIDR string) error {
	return ErrTapManagerUnimplemented
}
