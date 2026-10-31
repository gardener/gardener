// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package access

import (
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// ControllerName is the name of this controller.
const ControllerName = "virtual-cluster-access"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, namespace, secretName string) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	if r.FS == nil {
		r.FS = afero.NewOsFs()
	}

	if r.GardenNamespace == "" {
		r.GardenNamespace = namespace
	}

	if len(r.APIAudiences) == 0 {
		r.APIAudiences = []string{v1beta1constants.GardenerAudience}
	}

	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&corev1.Secret{}, builder.WithPredicates(IsGardenerInternalSecretPredicate(secretName, namespace))).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}

// IsGardenerInternalSecretPredicate is a predicate that returns true if the object matches the given name and namespace.
func IsGardenerInternalSecretPredicate(name, namespace string) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(o client.Object) bool {
		return o.GetNamespace() == namespace && o.GetName() == name
	})
}
