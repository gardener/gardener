// Copyright 2020 Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
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

package selector

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/ghodss/yaml"
	"github.com/xeipuuv/gojsonschema"
)

// Interface defines a selector interface that
// matches a map of string to string.
type Interface interface {
	Match(obj map[string]string) (bool, error)
}

// MatchSelectors checks whether all selectors matches the given obj.
func MatchSelectors(obj map[string]string, selectors ...Interface) (bool, error) {
	for _, sel := range selectors {
		ok, err:= sel.Match(obj)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

// DefaultSelector defines the selector for the identity of an object.
// The default selector is a map of identity key to selector
// All keys are validated against the given selector.
//
// Valid selectors are
// - raw string value: identity value is compared to the selector value
// - array of strings: or-operator identity value must be one of the defined strings in the array
type DefaultSelector map[string]interface{}

var _ Interface = DefaultSelector{}

// ParseDefaultSelector creates a Identity selector from a
// - json encoded selector
// - map[string]Selector
//
// A selector can be
// - a string: the value is directly matched
// - a array of strings: one selector in the array must match
func ParseDefaultSelector(value interface{}) (DefaultSelector, error) {
	switch v := value.(type) {
	case map[string]interface{}:
		return v, nil
	case string:
		selector := DefaultSelector{}
		if err := json.Unmarshal([]byte(v), &selector); err != nil {
			return nil, err
		}
		return selector, nil
	default:
		return nil, fmt.Errorf("unknown type %T", value)
	}
}

func (is DefaultSelector) Match(obj map[string]string) (bool, error) {
	for key, selector := range is {
		value, ok := obj[key]
		if !ok {
			return false, nil
		}
		ok, err := matchValue(selector, value)
		if err != nil {
			return false, fmt.Errorf("error while trying to match '%s': %w", key, err)
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

func matchValue(selector interface{}, val string) (bool, error) {
	switch s := selector.(type) {
	case string:
		return s == val, nil
	case []interface{}:
		for _, orVal := range s {
			v, ok := orVal.(string)
			if !ok {
				return false, fmt.Errorf("invalid selector type '%T'", val)
			}
			if val == v {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("unknown selector type '%T' only string or a list of strings is supported", val)
	}
}

// RegexSelector defines the selector for the identity of an object.
// The regex selector is a map of identity key to selector
// All keys are validated against the given selector.
//
// Valid selectors are
// - raw string value: identity value is compared to the selector value
// - array of strings: or-operator identity value must be one of the defined strings in the array
type RegexSelector map[string]interface{}

var _ Interface = RegexSelector{}

// ParseRegexSelector creates a Identity selector from a
// - json encoded selector
// - map[string]Selector
//
// A selector can be
// - a string: the value is directly matched
// - a array of strings: one selector in the array must match
func ParseRegexSelector(value interface{}) (RegexSelector, error) {
	switch v := value.(type) {
	case map[string]interface{}:
		return v, nil
	case string:
		selector := RegexSelector{}
		if err := json.Unmarshal([]byte(v), &selector); err != nil {
			return nil, err
		}
		return selector, nil
	default:
		return nil, fmt.Errorf("unknown type %T", value)
	}
}

func (is RegexSelector) Match(obj map[string]string) (bool, error) {
	for key, selector := range is {
		value, ok := obj[key]
		if !ok {
			return false, nil
		}
		ok, err := matchValueByRegex(selector, value)
		if err != nil {
			return false, fmt.Errorf("error while trying to match '%s': %w", key, err)
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

func matchValueByRegex(selector interface{}, val string) (bool, error) {
	switch s := selector.(type) {
	case string:
		return regexp.MatchString(s, val)
	case []interface{}:
		for _, orVal := range s {
			v, ok := orVal.(string)
			if !ok {
				return false, fmt.Errorf("invalid selector type '%T'", val)
			}
			if val == v {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("unknown selector type '%T' only string or a list of strings is supported", val)
	}
}

// JSONSchemaSelector uses a jsonschema to match a specific object.
type JSONSchemaSelector struct {
	Scheme *gojsonschema.Schema
}

// NewJSONSchemaSelector creates a new jsonschema selector from a gojsonschema
func NewJSONSchemaSelector(scheme *gojsonschema.Schema) JSONSchemaSelector {
	return JSONSchemaSelector{Scheme: scheme}
}

// NewJSONSchemaSelectorFromBytes creates a new jsonschema selector from a gojsonschema
func NewJSONSchemaSelectorFromBytes(src []byte) (JSONSchemaSelector, error) {
	data, err := yaml.YAMLToJSON(src)
	if err != nil {
		return JSONSchemaSelector{}, err
	}
	scheme, err := gojsonschema.NewSchema(gojsonschema.NewBytesLoader(data))
	if err != nil {
		return JSONSchemaSelector{}, err
	}
	return JSONSchemaSelector{Scheme: scheme}, nil
}

// NewJSONSchemaSelectorFromString creates a new jsonschema selector from a gojsonschema
func NewJSONSchemaSelectorFromString(src string) (JSONSchemaSelector, error) {
	return NewJSONSchemaSelectorFromBytes([]byte(src))
}

// NewJSONSchemaSelectorFromString creates a new jsonschema selector from a gojsonschema
func NewJSONSchemaSelectorFromGoStruct(src interface{}) (JSONSchemaSelector, error) {
	scheme, err := gojsonschema.NewSchema(gojsonschema.NewGoLoader(src))
	if err != nil {
		return JSONSchemaSelector{}, err
	}
	return NewJSONSchemaSelector(scheme), nil
}

var _ Interface = JSONSchemaSelector{}

func (J JSONSchemaSelector) Match(obj map[string]string) (bool, error) {
	documentLoader := gojsonschema.NewGoLoader(obj)
	res, err := J.Scheme.Validate(documentLoader)
	if err != nil {
		return false, err
	}
	return res.Valid(), nil
}
