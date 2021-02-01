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

package health

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Func is a type for a function that checks the health of a runtime.Object.
type Func func(client.Object) error

// And combines multiple health check funcs to a single func, checking all funcs sequentially and return the first
// error that occurs or nil if no error occurs.¬
func And(fns ...Func) Func {
	return func(o client.Object) error {
		for _, fn := range fns {
			if err := fn(o); err != nil {
				return err
			}
		}
		return nil
	}
}
