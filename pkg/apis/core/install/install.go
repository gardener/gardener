// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var (
	localSchemeBuilder = runtime.SchemeBuilder{
		gardencorev1.AddToScheme,
		v1beta1.AddToScheme,
	}
	// AddToScheme adds all versioned API types to the given scheme.
	AddToScheme = localSchemeBuilder.AddToScheme
)

// Install registers the API group and adds types to a scheme.
func Install(scheme *runtime.Scheme) {
	utilruntime.Must(core.AddToScheme(scheme))
	utilruntime.Must(gardencorev1.AddToScheme(scheme))
	utilruntime.Must(v1beta1.AddToScheme(scheme))

	utilruntime.Must(scheme.SetVersionPriority(v1beta1.SchemeGroupVersion))
}
