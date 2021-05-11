// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"io"
	"strings"

	"github.com/coreos/go-systemd/v22/unit"
)

// UnitSerializer contains methods for serializing and deserializing a slice of systemd unit options to and from a string.
type UnitSerializer interface {
	// Serialize serializes the given slice of systemd unit options to a string.
	Serialize([]*unit.UnitOption) (string, error)
	// Deserialize deserializes a slice of systemd unit options from the given string.
	Deserialize(string) ([]*unit.UnitOption, error)
}

// NewUnitSerializer creates and returns a new UnitSerializer.
func NewUnitSerializer() UnitSerializer {
	return &unitSerializer{}
}

type unitSerializer struct{}

// Serialize serializes the given slice of systemd unit options to a string.
func (us *unitSerializer) Serialize(opts []*unit.UnitOption) (string, error) {
	bytes, err := io.ReadAll(unit.Serialize(opts))
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// Deserialize deserializes a slice of systemd unit options from the given string.
func (us *unitSerializer) Deserialize(s string) ([]*unit.UnitOption, error) {
	return unit.Deserialize(strings.NewReader(s))
}
