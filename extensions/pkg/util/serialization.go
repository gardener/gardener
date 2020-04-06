// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
