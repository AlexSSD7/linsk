package qemucli

import (
	"github.com/pkg/errors"
)

type FlagArg struct {
	key string
}

func MustNewFlagArg(key string) *FlagArg {
	a, err := NewFlagArg(key)
	if err != nil {
		panic(err)
	}

	return a
}

func NewFlagArg(key string) (*FlagArg, error) {
	a := &FlagArg{
		key: key,
	}

	// Preflight arg key/type check.
	err := validateArgKey(a.key, a.ValueType())
	if err != nil {
		return nil, errors.Wrap(err, "validate arg key")
	}

	return a, nil
}

func (a *FlagArg) StringKey() string {
	return a.key
}

func (a *FlagArg) StringValue() string {
	// Boolean flags have no value.
	return ""
}

func (a *FlagArg) ValueType() ArgAcceptedValue {
	return ArgAcceptedValueNone
}
