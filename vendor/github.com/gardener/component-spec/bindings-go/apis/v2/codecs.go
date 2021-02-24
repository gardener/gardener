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

package v2

import (
	"encoding/json"
	"fmt"
	"strings"
)

// KnownTypeValidationFunc defines a function that can validate types.
type KnownTypeValidationFunc func(ttype string) error

// KnownTypes defines a set of known types.
type KnownTypes map[string]TypedObjectCodec

// Register adds a codec for a specific type to the list of known types.
// It returns if the operation has overwritten an already registered type.
func (kt *KnownTypes) Register(ttype string, codec TypedObjectCodec) (overwritten bool) {
	if _, ok := (*kt)[ttype]; ok {
		overwritten = true
	}
	(*kt)[ttype] = codec
	return
}

// TypedObjectCodec describes a known component type and how it is decoded and encoded
type TypedObjectCodec interface {
	TypedObjectDecoder
	TypedObjectEncoder
}

// TypedObjectCodecWrapper is a simple struct that implements the TypedObjectCodec interface
type TypedObjectCodecWrapper struct {
	TypedObjectDecoder
	TypedObjectEncoder
}

// TypedObjectDecoder defines a decoder for a typed object.
type TypedObjectDecoder interface {
	Decode(data []byte, into TypedObjectAccessor) error
}

// TypedObjectEncoder defines a encoder for a typed object.
type TypedObjectEncoder interface {
	Encode(accessor TypedObjectAccessor) ([]byte, error)
}

// TypedObjectDecoderFunc is a simple function that implements the TypedObjectDecoder interface.
type TypedObjectDecoderFunc func(data []byte, obj TypedObjectAccessor) error

// Decode is the Decode implementation of the TypedObjectDecoder interface.
func (e TypedObjectDecoderFunc) Decode(data []byte, obj TypedObjectAccessor) error {
	return e(data, obj)
}

// TypedObjectEncoderFunc is a simple function that implements the TypedObjectEncoder interface.
type TypedObjectEncoderFunc func(accessor TypedObjectAccessor) ([]byte, error)

// Encode is the Encode implementation of the TypedObjectEncoder interface.
func (e TypedObjectEncoderFunc) Encode(accessor TypedObjectAccessor) ([]byte, error) {
	return e(accessor)
}

// DefaultJSONTypedObjectCodec implements TypedObjectCodec interface with the json decoder and json encoder.
var DefaultJSONTypedObjectCodec = TypedObjectCodecWrapper{
	TypedObjectDecoder: DefaultJSONTypedObjectDecoder{},
	TypedObjectEncoder: DefaultJSONTypedObjectEncoder{},
}

// DefaultJSONTypedObjectDecoder is a simple decoder that implements the TypedObjectDecoder interface.
// It simply decodes the access using the json marshaller.
type DefaultJSONTypedObjectDecoder struct{}

var _ TypedObjectDecoder = DefaultJSONTypedObjectDecoder{}

// Decode is the Decode implementation of the TypedObjectDecoder interface.
func (e DefaultJSONTypedObjectDecoder) Decode(data []byte, obj TypedObjectAccessor) error {
	return json.Unmarshal(data, obj)
}

// DefaultJSONTypedObjectEncoder is a simple decoder that implements the TypedObjectDecoder interface.
// It encodes the access type using the default json marshaller.
type DefaultJSONTypedObjectEncoder struct{}

var _ TypedObjectEncoder = DefaultJSONTypedObjectEncoder{}

// Encode is the Encode implementation of the TypedObjectEncoder interface.
func (e DefaultJSONTypedObjectEncoder) Encode(obj TypedObjectAccessor) ([]byte, error) {
	return json.Marshal(obj)
}

// ValidateAccessType validates that a type is known or of a generic type.
// todo: revisit; currently "x-" specifies a generic type
func ValidateAccessType(ttype string) error {
	if _, ok := KnownAccessTypes[ttype]; ok {
		return nil
	}

	if !strings.HasPrefix(ttype, "x-") {
		return fmt.Errorf("unknown non generic types %s", ttype)
	}
	return nil
}

type codec struct {
	knownTypes     KnownTypes
	defaultCodec   TypedObjectCodec
	validationFunc KnownTypeValidationFunc
}

// NewDefaultCodec creates a new default typed object codec.
func NewDefaultCodec() TypedObjectCodec {
	return &codec{
		defaultCodec: DefaultJSONTypedObjectCodec,
		knownTypes:   KnownAccessTypes,
	}
}

// NewCodec creates a new typed object codec.
func NewCodec(knownTypes KnownTypes, defaultCodec TypedObjectCodec, validationFunc KnownTypeValidationFunc) TypedObjectCodec {
	if knownTypes == nil {
		knownTypes = KnownAccessTypes
	}

	if defaultCodec == nil {
		defaultCodec = DefaultJSONTypedObjectCodec
	}

	return &codec{
		defaultCodec:   defaultCodec,
		knownTypes:     knownTypes,
		validationFunc: validationFunc,
	}
}

// Decode unmarshals a unstructured typed object into a TypedObjectAccessor.
// The given known types are used to decode the data into a specific.
// The given defaultCodec is used if no matching type is known.
// An error is returned when the type is unknown and the default codec is nil.
func (c *codec) Decode(data []byte, into TypedObjectAccessor) error {
	accessType := &ObjectType{}
	if err := json.Unmarshal(data, accessType); err != nil {
		return err
	}

	if c.validationFunc != nil {
		if err := c.validationFunc(accessType.GetType()); err != nil {
			return err
		}
	}

	codec, ok := c.knownTypes[accessType.GetType()]
	if !ok {
		codec = c.defaultCodec
	}

	return codec.Decode(data, into)
}

// Encode marshals a typed object into a unstructured typed object.
// The given known types are used to decode the data into a specific.
// The given defaultCodec is used if no matching type is known.
// An error is returned when the type is unknown and the default codec is nil.
func (c *codec) Encode(acc TypedObjectAccessor) ([]byte, error) {
	if c.validationFunc != nil {
		if err := c.validationFunc(acc.GetType()); err != nil {
			return nil, err
		}
	}

	codec, ok := c.knownTypes[acc.GetType()]
	if !ok {
		codec = c.defaultCodec
	}

	return codec.Encode(acc)
}
