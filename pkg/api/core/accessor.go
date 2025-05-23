// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
)

// Accessor tries to create a `gardencore.Object` from the given runtime.Object.
//
// If the given object already implements Object, it is returned as-is.
// Otherwise, an error with the type of the object is returned.
func Accessor(obj runtime.Object) (gardencore.Object, error) {
	switch v := obj.(type) {
	case gardencore.Object:
		return v, nil
	default:
		return nil, fmt.Errorf("value of type %T does not implement gardencore.Object", obj)
	}
}
