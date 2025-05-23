// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
)

// Decode takes a `decoder` and decodes the provided `data` into the provided object.
// The underlying `into` address is used to assign the decoded object.
func Decode(decoder runtime.Decoder, data []byte, into runtime.Object) error {
	// By not providing an `into` it is necessary that the serialized `data` is configured with
	// a proper `apiVersion` and `kind` field. This also makes sure that the conversion logic to
	// the internal version is called.
	output, _, err := decoder.Decode(data, nil, nil)
	if err != nil {
		return err
	}

	intoType := reflect.TypeOf(into)

	if reflect.TypeOf(output) == intoType {
		reflect.ValueOf(into).Elem().Set(reflect.ValueOf(output).Elem())
		return nil
	}

	return fmt.Errorf("is not of type %s", intoType)
}
