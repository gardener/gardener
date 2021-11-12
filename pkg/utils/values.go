// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Options are options for marshalling
type Options struct {
	// LowerCaseKeys forces the keys to be lower case
	// this is not applied recursively
	LowerCaseKeys bool
}

// ToValuesMap converts the given value v to a values map, by first marshalling it to JSON,
// and then unmarshalling the result from JSON into a values map.
// If v cannot be marshalled to JSON, or if the result cannot be unmarshalled into a values map, an error is returned.
func ToValuesMap(v interface{}) (map[string]interface{}, error) {
	var m map[string]interface{}
	if err := convert(v, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// ToValuesMapWithOptions converts the given value v to a values map, by first marshalling it to JSON,
// and then unmarshalling the result from JSON into a values map.
// If v cannot be marshalled to JSON, or if the result cannot be unmarshalled into a values map, an error is returned.
func ToValuesMapWithOptions(v interface{}, opt Options) (map[string]interface{}, error) {
	var m map[string]interface{}
	if err := convert(v, &m); err != nil {
		return nil, err
	}

	if opt.LowerCaseKeys {
		m = toLowerCase(m)
	}

	return m, nil
}

// toLowerCase recursively ensures that the keys in a map[string]interface{} are lower-case
func toLowerCase(input map[string]interface{}) map[string]interface{} {
	if input == nil {
		return nil
	}

	if len(input) == 0 {
		return input
	}

	lowCase := make(map[string]interface{})
	for k, v := range input {
		if v == nil {
			continue
		}

		newKey := strings.ToLower(k)

		if m, ok := v.(map[string]interface{}); ok {
			v = toLowerCase(m)
		}

		if m, ok := v.([]interface{}); ok {
			for k2, v2 := range m {
				m[k2] = toLowerCase(toMap(v2))
			}
			v = m
		}
		lowCase[newKey] = v
	}
	return lowCase
}

// FromValuesMap converts the given values map values to the given value v, by first marshalling it to JSON,
// and then unmarshalling the result from JSON into v.
// If values cannot be marshalled to JSON, or if the result cannot be unmarshalled into v, an error is returned.
func FromValuesMap(values map[string]interface{}, v interface{}) error {
	return convert(values, v)
}

// InitValuesMap returns the given values map if it is non-nil, or a newly allocated values map if it is nil.
func InitValuesMap(values map[string]interface{}) map[string]interface{} {
	if values == nil {
		values = make(map[string]interface{})
	}
	return values
}

// GetFromValuesMap returns the element at the specified location in the given values map,
// e.g. GetFromValuesMap(values, "a", 0, "b") returns values["a"][0]["b"], if such an element exists.
// If such an element does not exist, it returns nil.
// All keys must be of type either string (for map keys) or int (for slice indexes).
// If a key type doesn't match the corresponding element type (string for map, int for slice), an error is returned.
func GetFromValuesMap(values map[string]interface{}, keys ...interface{}) (interface{}, error) {
	return getFromValues(fromMap(values), keys...)
}

// SetToValuesMap sets the given element v to the specified location in the given values map,
// e.g. SetToValuesMap(values, v, "a", 0, "b") sets values["a"][0]["b"] = v.
// All map and slice elements along the way that don't exist are created and set at their corresponding locations.
// All keys must be of type either string (for map keys) or int (for slice indexes).
// Slice indexes must refer to existing slice elements or the first element beyond the end of a slice.
// If a key type doesn't match the corresponding element type (string for map, int for slice), an error is returned.
func SetToValuesMap(values map[string]interface{}, v interface{}, keys ...interface{}) (map[string]interface{}, error) {
	x, err := setToValues(fromMap(values), v, keys...)
	return toMap(x), err
}

// DeleteFromValuesMap deletes the element at the specified location in the given values map,
// e.g. DeleteFromValuesMap(values, "a", 0, "b") deletes values["a"][0]["b"].
// If such an element does not exist, it returns the given values map unmodified.
// All keys must be of type either string (for map keys) or int (for slice indexes).
// If a key type doesn't match the corresponding element type (string for map, int for slice), an error is returned.
func DeleteFromValuesMap(values map[string]interface{}, keys ...interface{}) (map[string]interface{}, error) {
	x, err := deleteFromValues(fromMap(values), keys...)
	return toMap(x), err
}

// convert converts x to y by first marshalling x to JSON, and then unmarshalling the result from JSON into y.
// If x cannot be marshalled to JSON, or if the result cannot be unmarshalled into y, an error is returned.
func convert(x, y interface{}) error {
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
func getFromValues(values interface{}, keys ...interface{}) (interface{}, error) {
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
		var m map[string]interface{}
		if m, ok = values.(map[string]interface{}); !ok {
			return nil, fmt.Errorf("cannot use key '%s' with a non-map value", key)
		}
		if _, ok = m[key]; !ok {
			return nil, nil
		}
		return getFromValues(m[key], keys[1:]...)
	case int:
		index := keys[0].(int)
		var s []interface{}
		if s, ok = values.([]interface{}); !ok {
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
func setToValues(values interface{}, v interface{}, keys ...interface{}) (interface{}, error) {
	if len(keys) == 0 {
		return values, nil
	}
	var ok bool
	switch keys[0].(type) {
	case string:
		key := keys[0].(string)
		if values == nil {
			values = map[string]interface{}{}
		}
		var m map[string]interface{}
		if m, ok = values.(map[string]interface{}); !ok {
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
			values = []interface{}{}
		}
		var s []interface{}
		if s, ok = values.([]interface{}); !ok {
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
func deleteFromValues(values interface{}, keys ...interface{}) (interface{}, error) {
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
		var m map[string]interface{}
		if m, ok = values.(map[string]interface{}); !ok {
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
		var s []interface{}
		if s, ok = values.([]interface{}); !ok {
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

func fromMap(values map[string]interface{}) interface{} {
	if values == nil {
		return nil
	}
	return values
}

func toMap(x interface{}) map[string]interface{} {
	if x == nil {
		return nil
	}
	return x.(map[string]interface{})
}
