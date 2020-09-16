// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package install

import (
	"github.com/gardener/gardener/pkg/apis/settings"
	"github.com/gardener/gardener/pkg/apis/settings/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

// Install registers the API group and adds types to a scheme.
func Install(scheme *runtime.Scheme) {
	utilruntime.Must(settings.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))

	utilruntime.Must(scheme.SetVersionPriority(v1alpha1.SchemeGroupVersion))
}
