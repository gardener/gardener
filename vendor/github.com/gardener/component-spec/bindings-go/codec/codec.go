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

package codec

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"sigs.k8s.io/yaml"

	"github.com/gardener/component-spec/bindings-go/apis"
	v2 "github.com/gardener/component-spec/bindings-go/apis/v2"
	"github.com/gardener/component-spec/bindings-go/apis/v2/jsonscheme"
	"github.com/gardener/component-spec/bindings-go/apis/v2/validation"
)

// Decode decodes a component into the given object.
// The obj is expected to be of type v2.ComponentDescriptor or v2.ComponentDescriptorList.
func Decode(data []byte, obj interface{}, opts ...DecodeOption) error {
	objType := reflect.TypeOf(obj)
	if objType.Kind() != reflect.Ptr {
		return fmt.Errorf("object is expected to be of type pointer but is of type %T", obj)
	}

	o := &DecodeOptions{}
	o.ApplyOptions(opts)

	raw := make(map[string]json.RawMessage)
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}

	var metadata apis.Metadata
	if err := yaml.Unmarshal(raw["meta"], &metadata); err != nil {
		return err
	}

	// handle v2
	if metadata.Version == v2.SchemaVersion && objType.Elem() == reflect.TypeOf(v2.ComponentDescriptor{}) {
		if !o.DisableValidation {
			if err := jsonscheme.Validate(data); err != nil {
				return err
			}
		}

		if o.StrictMode {
			if err := yaml.UnmarshalStrict(data, obj); err != nil {
				return err
			}
		} else {
			if err := yaml.Unmarshal(data, obj); err != nil {
				return err
			}
		}

		comp := obj.(*v2.ComponentDescriptor)
		if err := v2.DefaultComponent(comp); err != nil {
			return err
		}

		if o.DisableValidation {
			return nil
		}
		return validation.Validate(comp)
	}

	if metadata.Version == v2.SchemaVersion && objType.Elem() == reflect.TypeOf(v2.ComponentDescriptorList{}) {
		if o.StrictMode {
			if err := yaml.UnmarshalStrict(data, obj); err != nil {
				return err
			}
		} else {
			if err := yaml.Unmarshal(data, obj); err != nil {
				return err
			}
		}
		list := obj.(*v2.ComponentDescriptorList)
		if err := v2.DefaultList(list); err != nil {
			return err
		}
		if o.DisableValidation {
			return nil
		}
		return validation.ValidateList(list)
	}

	// todo: implement conversion
	return errors.New("invalid version")
}

// Encode encodes a component or component list into the given object.
// The obj is expected to be of type v2.ComponentDescriptor or v2.ComponentDescriptorList.
func Encode(obj interface{}) ([]byte, error) {
	objType := reflect.TypeOf(obj)
	if objType.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("object is expected to be of type pointer but is of type %T", obj)
	}

	if objType.Elem() == reflect.TypeOf(v2.ComponentDescriptor{}) {
		comp := obj.(*v2.ComponentDescriptor)
		comp.Metadata.Version = v2.SchemaVersion
		if err := v2.DefaultComponent(comp); err != nil {
			return nil, err
		}
		return json.Marshal(comp)
	}

	if objType.Elem() == reflect.TypeOf(v2.ComponentDescriptorList{}) {
		list := obj.(*v2.ComponentDescriptorList)
		list.Metadata.Version = v2.SchemaVersion
		if err := v2.DefaultList(list); err != nil {
			return nil, err
		}
		return json.Marshal(list)
	}

	// todo: implement conversion
	return nil, errors.New("invalid version")
}

// DecodeOptions defines decode options for the codec.
type DecodeOptions struct {
	DisableValidation bool
	StrictMode        bool
}

// ApplyOptions applies the given list options on these options,
// and then returns itself (for convenient chaining).
func (o *DecodeOptions) ApplyOptions(opts []DecodeOption) *DecodeOptions {
	for _, opt := range opts {
		if opt != nil {
			opt.ApplyOption(o)
		}
	}
	return o
}

// DecodeOption is the interface to specify different cache options
type DecodeOption interface {
	ApplyOption(options *DecodeOptions)
}

// StrictMode enables or disables strict mode parsing.
type StrictMode bool

// ApplyOption applies the configured strict mode.
func (s StrictMode) ApplyOption(options *DecodeOptions) {
	options.StrictMode = bool(s)
}

// DisableValidation enables or disables validation of the component descriptor.
type DisableValidation bool

// ApplyOption applies the validation disable option.
func (v DisableValidation) ApplyOption(options *DecodeOptions) {
	options.DisableValidation = bool(v)
}
