// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardener/pkg/apis/operations"
	"github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
)

var (
	localSchemeBuilder = runtime.SchemeBuilder{
		v1alpha1.AddToScheme,
	}
	// AddToScheme adds all versioned API types to the given scheme.
	AddToScheme = localSchemeBuilder.AddToScheme
)

// Install registers the API group and adds types to a scheme.
func Install(scheme *runtime.Scheme) {
	utilruntime.Must(operations.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))

	utilruntime.Must(scheme.SetVersionPriority(v1alpha1.SchemeGroupVersion))
}
