// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/json-iterator/go"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

var json = jsoniter.ConfigFastest

// CreateTwoWayMergePatch creates a two way merge patch of the given objects.
// The two objects have to be pointers implementing the interfaces.
func CreateTwoWayMergePatch(obj1 metav1.Object, obj2 metav1.Object) ([]byte, error) {
	t1, t2 := reflect.TypeOf(obj1), reflect.TypeOf(obj2)
	if t1 != t2 {
		return nil, fmt.Errorf("cannot patch two objects of different type: %q - %q", t1, t2)
	}
	if t1.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("type has to be of kind pointer but got %q", t1)
	}

	obj1Data, err := json.Marshal(obj1)
	if err != nil {
		return nil, err
	}

	obj2Data, err := json.Marshal(obj2)
	if err != nil {
		return nil, err
	}

	dataStructType := t1.Elem()
	dataStruct := reflect.New(dataStructType).Elem().Interface()

	return strategicpatch.CreateTwoWayMergePatch(obj1Data, obj2Data, dataStruct)
}

// IsEmptyPatch checks if the given patch is empty. A patch is considered empty if it is
// the empty string or if it json-decodes to an empty json map.
func IsEmptyPatch(patch []byte) bool {
	if len(strings.TrimSpace(string(patch))) == 0 {
		return true
	}

	var m map[string]interface{}
	if err := json.Unmarshal(patch, &m); err != nil {
		return false
	}

	return len(m) == 0
}
