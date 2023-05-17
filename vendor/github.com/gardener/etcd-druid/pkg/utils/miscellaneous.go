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
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MergeStringMaps merges the content of the newMaps with the oldMap. If a key already exists then
// it gets overwritten by the last value with the same key.
func MergeStringMaps(oldMap map[string]string, newMaps ...map[string]string) map[string]string {
	var out map[string]string

	if oldMap != nil {
		out = make(map[string]string)
	}
	for k, v := range oldMap {
		out[k] = v
	}

	for _, newMap := range newMaps {
		if newMap != nil && out == nil {
			out = make(map[string]string)
		}

		for k, v := range newMap {
			out[k] = v
		}
	}

	return out
}

func nameAndNamespace(namespaceOrName string, nameOpt ...string) (namespace, name string) {
	if len(nameOpt) > 1 {
		panic(fmt.Sprintf("more than name/namespace for key specified: %s/%v", namespaceOrName, nameOpt))
	}
	if len(nameOpt) == 0 {
		name = namespaceOrName
		return
	}
	namespace = namespaceOrName
	name = nameOpt[0]
	return
}

// Key creates a new client.ObjectKey from the given parameters.
// There are only two ways to call this function:
//   - If only namespaceOrName is set, then a client.ObjectKey with name set to namespaceOrName is returned.
//   - If namespaceOrName and one nameOpt is given, then a client.ObjectKey with namespace set to namespaceOrName
//     and name set to nameOpt[0] is returned.
//
// For all other cases, this method panics.
func Key(namespaceOrName string, nameOpt ...string) client.ObjectKey {
	namespace, name := nameAndNamespace(namespaceOrName, nameOpt...)
	return client.ObjectKey{Namespace: namespace, Name: name}
}

// Max returns the larger of x or y.
func Max(x, y int) int {
	if y > x {
		return y
	}
	return x
}
