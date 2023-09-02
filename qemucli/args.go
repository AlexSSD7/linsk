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
	"accel":   ArgAcceptedValueString,
	"boot":    ArgAcceptedValueString,
	"m":       ArgAcceptedValueUint,
	"smp":     ArgAcceptedValueUint,
	"device":  ArgAcceptedValueKeyValue,
	"netdev":  ArgAcceptedValueKeyValue,
	"serial":  ArgAcceptedValueString,
	"cdrom":   ArgAcceptedValueString,
	"machine": ArgAcceptedValueString,
	"cpu":     ArgAcceptedValueString,
	"display": ArgAcceptedValueString,
	"drive":   ArgAcceptedValueKeyValue,
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
