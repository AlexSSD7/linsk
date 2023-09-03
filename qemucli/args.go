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
	"fmt"
	"reflect"

	"github.com/alessio/shellescape"
	"github.com/pkg/errors"
)

type ArgAcceptedValue string

const (
	ArgAcceptedValueUint     ArgAcceptedValue = "uint"
	ArgAcceptedValueString   ArgAcceptedValue = "string"
	ArgAcceptedValueKeyValue ArgAcceptedValue = "kv"
	ArgAcceptedValueNone     ArgAcceptedValue = "none"
)

var safeArgs = map[string]ArgAcceptedValue{
	"accel":   ArgAcceptedValueKeyValue,
	"boot":    ArgAcceptedValueString,
	"m":       ArgAcceptedValueUint,
	"smp":     ArgAcceptedValueUint,
	"device":  ArgAcceptedValueKeyValue,
	"netdev":  ArgAcceptedValueKeyValue,
	"serial":  ArgAcceptedValueString,
	"cdrom":   ArgAcceptedValueString,
	"machine": ArgAcceptedValueKeyValue,
	"cpu":     ArgAcceptedValueString,
	"display": ArgAcceptedValueString,
	"drive":   ArgAcceptedValueKeyValue,
	"bios":    ArgAcceptedValueString,
}

type Arg interface {
	StringKey() string
	StringValue() string
	ValueType() ArgAcceptedValue
}

func EncodeArgs(args []Arg) ([]string, error) {
	var cmdArgs []string

	for i, arg := range args {
		flag, value, err := EncodeArg(arg)
		if err != nil {
			return nil, errors.Wrapf(err, "encode flag #%v", i)
		}

		cmdArgs = append(cmdArgs, flag)
		if value != nil {
			cmdArgs = append(cmdArgs, *value)
		}
	}

	return cmdArgs, nil
}

func EncodeArg(a Arg) (string, *string, error) {
	// We're making copies because we don't want to trust
	// that Arg always returns the same value.
	argKey := a.StringKey()
	argValueType := a.ValueType()

	err := validateArgKey(argKey, argValueType)
	if err != nil {
		return "", nil, errors.Wrap(err, "validate arg key")
	}

	if argValueType == ArgAcceptedValueNone {
		if a.StringValue() != "" {
			return "", nil, fmt.Errorf("arg returned a value while declaring no value (type %v)", reflect.TypeOf(a))
		}

		return argKey, nil, nil
	}

	argValueStr := a.StringValue()
	if argValueStr == "" {
		return "", nil, fmt.Errorf("empty string value while declaring non-empty value (type %v)", reflect.TypeOf(a))
	}

	argVal := shellescape.Quote(argValueStr)

	return "-" + argKey, &argVal, nil
}
