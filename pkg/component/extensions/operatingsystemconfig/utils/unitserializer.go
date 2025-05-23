// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
