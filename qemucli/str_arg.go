package qemucli

import (
	"github.com/pkg/errors"
)

type StringArg struct {
	key   string
	value string
}

func MustNewStringArg(key string, value string) *StringArg {
	a, err := NewStringArg(key, value)
	if err != nil {
		panic(err)
	}

	return a
}

func NewStringArg(key string, value string) (*StringArg, error) {
	a := &StringArg{
		key:   key,
		value: value,
	}

	// Preflight arg key/type check.
	err := validateArgKey(a.key, a.ValueType())
	if err != nil {
		return nil, errors.Wrap(err, "validate arg key")
	}

	err = validateArgStrValue(a.value)
	if err != nil {
		return nil, errors.Wrap(err, "validate str value")
	}

	return a, nil
}

func (a *StringArg) StringKey() string {
	return a.key
}

func (a *StringArg) StringValue() string {
	// We're not validating anything here because
	// we expect that the string value was validated
	// at the creation of the StringArg.
	return a.value
}

func (a *StringArg) ValueType() ArgAcceptedValue {
	return ArgAcceptedValueString
}
