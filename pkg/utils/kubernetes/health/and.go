// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
