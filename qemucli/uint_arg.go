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

package qemucli

import (
	"github.com/AlexSSD7/linsk/utils"
	"github.com/pkg/errors"
	"golang.org/x/exp/constraints"
)

type UintArg struct {
	key   string
	value uint64
}

func MustNewUintArg[T constraints.Integer](key string, value T) *UintArg {
	a, err := NewUintArg(key, uint64(value))
	if err != nil {
		panic(err)
	}

	return a
}

func NewUintArg(key string, value uint64) (*UintArg, error) {
	a := &UintArg{
		key:   key,
		value: value,
	}

	// Preflight arg key/type check.
	err := validateArgKey(key, a.ValueType())
	if err != nil {
		return nil, errors.Wrap(err, "validate arg key")
	}

	return a, nil
}

func (a *UintArg) StringKey() string {
	return a.key
}

func (a *UintArg) StringValue() string {
	return utils.UintToStr(a.value)
}

func (a *UintArg) ValueType() ArgAcceptedValue {
	return ArgAcceptedValueUint
}
