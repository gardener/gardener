// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// GetExtensionClassOrDefault returns the given extension class or the default extension class if the given class is nil.
func GetExtensionClassOrDefault(class *extensionsv1alpha1.ExtensionClass) extensionsv1alpha1.ExtensionClass {
	return ptr.Deref(class, extensionsv1alpha1.ExtensionClassShoot)
}
