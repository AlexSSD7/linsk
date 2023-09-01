package nettap

import "errors"

var (
	ErrTapNotFound             = errors.New("tap not found")
	ErrTapManagerUnimplemented = errors.New("tap manager is implemented on windows only")
)
