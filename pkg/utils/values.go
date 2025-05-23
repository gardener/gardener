// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"encoding/json"
	"fmt"
	"reflect"
	"unicode"
	"unicode/utf8"
)

// Options are options for marshalling
type Options struct {
	// LowerCaseKeys forces the keys to be lower case for the first character
	LowerCaseKeys bool
	// RemoveZeroEntries removes the map entry if the value is the zero value for its type
	// For example: removes the map entry if the value is string(""), bool(false) or int(0)
	RemoveZeroEntries bool
}

// ToValuesMap converts the given value v to a values map, by first marshalling it to JSON,
// and then unmarshalling the result from JSON into a values map.
// If v cannot be marshalled to JSON, or if the result cannot be unmarshalled into a values map, an error is returned.
func ToValuesMap(v any) (map[string]any, error) {
	var m map[string]any
	if err := convert(v, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// ToValuesMapWithOptions converts the given value v to a values map, by first marshalling it to JSON,
// and then unmarshalling the result from JSON into a values map.
// If v cannot be marshalled to JSON, or if the result cannot be unmarshalled into a values map, an error is returned.
func ToValuesMapWithOptions(v any, opt Options) (map[string]any, error) {
	var m map[string]any
	if err := convert(v, &m); err != nil {
		return nil, err
	}

	if hasOptions(opt) {
		m = opt.applyOptions(m)
	}

	return m, nil
}

// hasOptions returns true if there are any enabled options
func hasOptions(opt Options) bool {
	return opt.LowerCaseKeys || opt.RemoveZeroEntries
}

// applyOptions recursively ensures that the keys in a map[string]any are lower-case
func (opt *Options) applyOptions(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}

	if len(input) == 0 {
		return input
	}

	result := make(map[string]any)
	for key, value := range input {
		if value == nil {
			continue
		}

		v := reflect.ValueOf(value)
		if opt.RemoveZeroEntries && v.IsZero() {
			continue
		}

		if opt.LowerCaseKeys {
			r, n := utf8.DecodeRuneInString(key)
			key = string(unicode.ToLower(r)) + key[n:]
		}

		if m, ok := value.(map[string]any); ok {
			value = opt.applyOptions(m)
		} else if m, ok := value.([]any); ok {
			value = opt.sliceToValues(m)
		}

		result[key] = value
	}
	return result
}

func (opt *Options) sliceToValues(input []any) []any {
	var result = make([]any, len(input))
	for index, v2 := range input {
		if m2, ok := v2.(map[string]any); ok {
			result[index] = opt.applyOptions(m2)
			continue
		}
		result[index] = v2
	}
	return result
}

// FromValuesMap converts the given values map values to the given value v, by first marshalling it to JSON,
// and then unmarshalling the result from JSON into v.
// If values cannot be marshalled to JSON, or if the result cannot be unmarshalled into v, an error is returned.
func FromValuesMap(values map[string]any, v any) error {
	return convert(values, v)
}

// InitValuesMap returns the given values map if it is non-nil, or a newly allocated values map if it is nil.
func InitValuesMap(values map[string]any) map[string]any {
	if values == nil {
		values = make(map[string]any)
	}
	return values
}

// GetFromValuesMap returns the element at the specified location in the given values map,
// e.g. GetFromValuesMap(values, "a", 0, "b") returns values["a"][0]["b"], if such an element exists.
// If such an element does not exist, it returns nil.
// All keys must be of type either string (for map keys) or int (for slice indexes).
// If a key type doesn't match the corresponding element type (string for map, int for slice), an error is returned.
func GetFromValuesMap(values map[string]any, keys ...any) (any, error) {
	return getFromValues(fromMap(values), keys...)
}

// SetToValuesMap sets the given element v to the specified location in the given values map,
// e.g. SetToValuesMap(values, v, "a", 0, "b") sets values["a"][0]["b"] = v.
// All map and slice elements along the way that don't exist are created and set at their corresponding locations.
// All keys must be of type either string (for map keys) or int (for slice indexes).
// Slice indexes must refer to existing slice elements or the first element beyond the end of a slice.
// If a key type doesn't match the corresponding element type (string for map, int for slice), an error is returned.
func SetToValuesMap(values map[string]any, v any, keys ...any) (map[string]any, error) {
	x, err := setToValues(fromMap(values), v, keys...)
	return toMap(x), err
}

// DeleteFromValuesMap deletes the element at the specified location in the given values map,
// e.g. DeleteFromValuesMap(values, "a", 0, "b") deletes values["a"][0]["b"].
// If such an element does not exist, it returns the given values map unmodified.
// All keys must be of type either string (for map keys) or int (for slice indexes).
// If a key type doesn't match the corresponding element type (string for map, int for slice), an error is returned.
func DeleteFromValuesMap(values map[string]any, keys ...any) (map[string]any, error) {
	x, err := deleteFromValues(fromMap(values), keys...)
	return toMap(x), err
}

// convert converts x to y by first marshalling x to JSON, and then unmarshalling the result from JSON into y.
// If x cannot be marshalled to JSON, or if the result cannot be unmarshalled into y, an error is returned.
func convert(x, y any) error {
	jsonBytes, err := json.Marshal(x)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonBytes, y)
}

// getFromValues returns the element at the specified location in the given values (either map or slice),
// e.g. getFromValues(values, "a", 0, "b") returns values["a"][0]["b"], if such an element exists.
// If such an element does not exist, it returns nil.
// All keys must be of type either string (for map keys) or int (for slice indexes).
// If a key type doesn't match the corresponding element type (string for map, int for slice), an error is returned.
func getFromValues(values any, keys ...any) (any, error) {
	if values == nil {
		return nil, nil
	}
	if len(keys) == 0 {
		return values, nil
	}

	var ok bool

	switch keys[0].(type) {
	case string:
		key := keys[0].(string)
		var m map[string]any

		if m, ok = values.(map[string]any); !ok {
			return nil, fmt.Errorf("cannot use key '%s' with a non-map value", key)
		}
		if _, ok = m[key]; !ok {
			return nil, nil
		}
		return getFromValues(m[key], keys[1:]...)
	case int:
		var (
			index = keys[0].(int)
			s     []any
		)

		if s, ok = values.([]any); !ok {
			return nil, fmt.Errorf("cannot use index '%d' with a non-slice value", index)
		}
		if index < 0 || index >= len(s) {
			return nil, nil
		}
		return getFromValues(s[index], keys[1:]...)
	default:
		return nil, fmt.Errorf("key '%v' must be of type string or int", keys[0])
	}
}

// setToValues sets the given element v to the specified location in the given values (either map or slice),
// e.g. setToValues(values, v, "a", 0, "b") sets values["a"][0]["b"] = v.
// All map and slice elements along the way that don't exist are created and set at their corresponding locations.
// All keys must be of type either string (for map keys) or int (for slice indexes).
// Slice indexes must refer to existing slice elements or the first element beyond the end of a slice.
// If a key type doesn't match the corresponding element type (string for map, int for slice), an error is returned.
func setToValues(values any, v any, keys ...any) (any, error) {
	if len(keys) == 0 {
		return values, nil
	}

	var ok bool

	switch keys[0].(type) {
	case string:
		key := keys[0].(string)

		if values == nil {
			values = map[string]any{}
		}

		var m map[string]any
		if m, ok = values.(map[string]any); !ok {
			return values, fmt.Errorf("cannot use key '%s' with a non-map value", key)
		}
		if len(keys) == 1 {
			m[key] = v
		} else {
			x, err := setToValues(m[key], v, keys[1:]...)
			if err != nil {
				return m, err
			}
			m[key] = x
		}
		return m, nil
	case int:
		index := keys[0].(int)

		if values == nil {
			values = []any{}
		}

		var s []any

		if s, ok = values.([]any); !ok {
			return values, fmt.Errorf("cannot use index '%d' with a non-slice value", index)
		}
		if index >= 0 && index < len(s) {
			if len(keys) == 1 {
				s[index] = v
			} else {
				x, err := setToValues(s[index], v, keys[1:]...)
				if err != nil {
					return s, err
				}
				s[index] = x
			}
		} else if index == len(s) {
			if len(keys) == 1 {
				s = append(s, v)
			} else {
				x, err := setToValues(nil, v, keys[1:]...)
				if err != nil {
					return s, err
				}

				s = append(s, x)
			}
		} else {
			return s, fmt.Errorf("index '%d' out of range", index)
		}
		return s, nil
	default:
		return values, fmt.Errorf("key '%v' must be of type string or int", keys[0])
	}
}

// deleteFromValues deletes the element at the specified location in the given values (either map or slice),
// e.g. deleteFromValues(values, "a", 0, "b") deletes values["a"][0]["b"].
// If such an element does not exist, it returns the given values unmodified.
// All keys must be of type either string (for map keys) or int (for slice indexes).
// If a key type doesn't match the corresponding element type (string for map, int for slice), an error is returned.
func deleteFromValues(values any, keys ...any) (any, error) {
	if values == nil {
		return nil, nil
	}
	if len(keys) == 0 {
		return values, nil
	}

	var ok bool

	switch keys[0].(type) {
	case string:
		key := keys[0].(string)
		var m map[string]any

		if m, ok = values.(map[string]any); !ok {
			return values, fmt.Errorf("cannot use key '%s' with a non-map value", key)
		}
		if _, ok = m[key]; ok {
			if len(keys) == 1 {
				delete(m, key)
			} else {
				x, err := deleteFromValues(m[key], keys[1:]...)
				if err != nil {
					return m, err
				}
				m[key] = x
			}
		}
		return m, nil
	case int:
		index := keys[0].(int)
		var s []any

		if s, ok = values.([]any); !ok {
			return values, fmt.Errorf("cannot use index '%d' with a non-slice value", index)
		}
		if index >= 0 && index < len(s) {
			if len(keys) == 1 {
				s = append(s[:index], s[index+1:]...)
			} else {
				x, err := deleteFromValues(s[index], keys[1:]...)
				if err != nil {
					return s, err
				}
				s[index] = x
			}
		}
		return s, nil
	default:
		return values, fmt.Errorf("key '%v' must be of type string or int", keys[0])
	}
}

func fromMap(values map[string]any) any {
	if values == nil {
		return nil
	}
	return values
}

func toMap(x any) map[string]any {
	if x == nil {
		return nil
	}
	return x.(map[string]any)
}
