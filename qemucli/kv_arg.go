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
	"strings"

	"github.com/pkg/errors"
)

type KeyValueArgItem struct {
	Key   string
	Value string
}

type KeyValueArg struct {
	key   string
	items []KeyValueArgItem
}

func MustNewKeyValueArg(key string, items []KeyValueArgItem) *KeyValueArg {
	a, err := NewKeyValueArg(key, items)
	if err != nil {
		panic(err)
	}

	return a
}

func NewKeyValueArg(key string, items []KeyValueArgItem) (*KeyValueArg, error) {
	a := &KeyValueArg{
		key: key,
		// We're creating a copy here because we do not
		// want to reference to any external source
		// that can be modified after we've done checks.
		items: make([]KeyValueArgItem, len(items)),
	}

	// Preflight arg key/type check.
	err := validateArgKey(key, a.ValueType())
	if err != nil {
		return nil, errors.Wrap(err, "validate arg key")
	}

	for i, item := range items {
		// We're making a copy here because we don't want to
		// leave the possibility to modify the value remotely
		// after the checks are done. Slices are pointers, and
		// no copies are made when passing a slice is passed
		// through to a function.
		item := item

		if len(item.Key) == 0 {
			return nil, fmt.Errorf("empty key not allowed")
		}

		err := validateArgStrValue(item.Key)
		if err != nil {
			return nil, errors.Wrapf(err, "validate key '%v'", item.Key)
		}

		err = validateArgStrValue(item.Value)
		if err != nil {
			return nil, errors.Wrapf(err, "validate map value '%v'", item.Value)
		}

		a.items[i] = item
	}

	return a, nil
}

func (a *KeyValueArg) StringKey() string {
	return a.key
}

func (a *KeyValueArg) StringValue() string {
	sb := new(strings.Builder)
	for i, item := range a.items {
		// We're not validating anything here because
		// we expect that the keys/values were validated
		// at the creation of the MapArg.

		if item.Key == "" {
			// But if for whatever reason it happens that
			// a key is blank, we skip the entire item because
			// otherwise it's bad syntax.
			continue
		}

		if i != 0 {
			sb.WriteString(",")
		}

		sb.WriteString(item.Key)
		if len(item.Value) > 0 {
			// Item values can theoretically be empty.
			sb.WriteString("=" + item.Value)
		}
	}

	return sb.String()
}

func (a *KeyValueArg) ValueType() ArgAcceptedValue {
	return ArgAcceptedValueKeyValue
}
