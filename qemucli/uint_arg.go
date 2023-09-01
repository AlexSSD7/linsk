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
