// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"

	gardensecurity "github.com/gardener/gardener/pkg/apis/security"
)

// Accessor tries to create a `gardensecurity.Object` from the given runtime.Object.
//
// If the given object already implements Object, it is returned as-is.
// Otherwise, an error with the type of the object is returned.
func Accessor(obj runtime.Object) (gardensecurity.Object, error) {
	switch v := obj.(type) {
	case gardensecurity.Object:
		return v, nil
	default:
		return nil, fmt.Errorf("value of type %T does not implement gardensecurity.Object", obj)
	}
}
