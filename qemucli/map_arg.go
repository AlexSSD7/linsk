package qemucli

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

type MapArg struct {
	key    string
	values map[string]string
}

func MustNewMapArg(key string, values map[string]string) *MapArg {
	a, err := NewMapArg(key, values)
	if err != nil {
		panic(err)
	}

	return a
}

func NewMapArg(key string, values map[string]string) (*MapArg, error) {
	a := &MapArg{
		key:    key,
		values: make(map[string]string),
	}

	// Preflight arg key/type check.
	err := validateArgKey(key, a.ValueType())
	if err != nil {
		return nil, errors.Wrap(err, "validate arg key")
	}

	for k, v := range values {
		// The reason why we're making copies here and creating
		// a whole other copy of the entire map is because maps
		// are pointers, and we do not want to reference anything
		// that will not be able to validate except at this stage
		// of MapArg creation.
		k := k
		v := v

		if len(k) == 0 {
			return nil, fmt.Errorf("empty map key not allowed")
		}

		if len(v) == 1 {
			// Values *can* be empty, though. We do not allow them for consistency.
			return nil, fmt.Errorf("empty map value for key '%v' is not allowed", k)
		}

		err := validateArgStrValue(k)
		if err != nil {
			return nil, errors.Wrapf(err, "validate map key '%v'", k)
		}

		err = validateArgStrValue(v)
		if err != nil {
			return nil, errors.Wrapf(err, "validate map value '%v'", v)
		}

		a.values[k] = v
	}

	return a, nil
}

func (a *MapArg) StringKey() string {
	return a.key
}

func (a *MapArg) StringValue() string {
	sb := new(strings.Builder)
	for k, v := range a.values {
		// We're not validating anything here because
		// we expect that the keys/values were validated
		// at the creation of the MapArg.

		sb.WriteString(k)
		if len(v) > 0 {
			sb.WriteString("=" + v)
		}
	}

	return sb.String()
}

func (a *MapArg) ValueType() ArgAcceptedValue {
	return ArgAcceptedValueMap
}
