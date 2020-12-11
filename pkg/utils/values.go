// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
)

// ToValues converts the given value v to a values map (map[string]interface{}), by first marshalling it to JSON,
// and then unmarshalling the result from JSON into map[string]interface{}. If the given value v cannot be marshalled to
// JSON, or if the result cannot be unmarshalled into map[string]interface{}, an error is returned.
func ToValues(v interface{}) (map[string]interface{}, error) {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// FromValues converts the given values map (map[string]interface{}) to the given value v, by first marshalling it to JSON,
// and then unmarshalling the result from JSON into v. If the given values map cannot be marshalled to
// JSON, or if the result cannot be unmarshalled into v, an error is returned.
func FromValues(m map[string]interface{}, v interface{}) error {
	jsonBytes, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(jsonBytes, v); err != nil {
		return err
	}
	return nil
}

// InitValues returns the given values map if it is non-nil, or a newly allocated values map if it is nil.
func InitValues(values map[string]interface{}) map[string]interface{} {
	if values == nil {
		values = make(map[string]interface{})
	}
	return values
}

// GetMapFromValues returns the map at the specified location in the given values map,
// e.g. GetMapFromValues(values, "a", "b", "c") returns values["a"]["b"]["c"], if such an element exists and it's a map.
// If such an element does not exist, it returns nil. If any of the keys exists but its element is not a map, an error is returned.
func GetMapFromValues(values map[string]interface{}, keys ...string) (map[string]interface{}, error) {
	for _, key := range keys {
		var ok bool
		if _, ok = values[key]; !ok {
			return nil, nil
		}
		if values, ok = values[key].(map[string]interface{}); !ok {
			return nil, fmt.Errorf("non-map value at key %q", key)
		}
	}
	return values, nil
}

// SetMapToValues sets the given map m to the specified location in the given values map,
// e.g. SetMapToValues(values, m, "a", "b", "c") sets values["a"]["b"]["c"] = m.
// If any of the keys before the last exists but its element is not a map, an error is returned.
func SetMapToValues(values map[string]interface{}, m map[string]interface{}, keys ...string) (map[string]interface{}, error) {
	if m == nil {
		return values, nil
	}
	if values == nil {
		values = make(map[string]interface{})
	}
	vs := values
	for _, key := range keys[:len(keys)-1] {
		var ok bool
		if _, ok = vs[key]; !ok {
			vs[key] = make(map[string]interface{})
		}
		if vs, ok = vs[key].(map[string]interface{}); !ok {
			return nil, fmt.Errorf("non-map value at key %q", key)
		}
	}
	vs[keys[len(keys)-1]] = m
	return values, nil
}
