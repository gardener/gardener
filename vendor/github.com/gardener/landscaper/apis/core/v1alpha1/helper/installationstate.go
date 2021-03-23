// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package helper

import landscaperv1alpha1 "github.com/gardener/landscaper/apis/core/v1alpha1"

// NewInstallationReferenceState creates a new installation reference state from a given installation
func NewInstallationReferenceState(name string, inst *landscaperv1alpha1.Installation) landscaperv1alpha1.NamedObjectReference {
	return landscaperv1alpha1.NamedObjectReference{
		Name: name,
		Reference: landscaperv1alpha1.ObjectReference{
			Name:      inst.Name,
			Namespace: inst.Namespace,
		},
	}
}
