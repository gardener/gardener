// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package access

import (
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

// ControllerName is the name of this controller.
const ControllerName = "garden-access"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, namespace, secretName string) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	if r.Manager == nil {
		r.Manager = mgr
	}

	if r.FS == nil {
		r.FS = afero.NewOsFs()
	}

	accessSecretPredicates := predicate.NewPredicateFuncs(func(o client.Object) bool {
		_, hasRenewAnnotation := o.GetAnnotations()[resourcesv1alpha1.ServiceAccountTokenRenewTimestamp]

		return o.GetNamespace() == namespace &&
			o.GetName() == secretName &&
			hasRenewAnnotation
	})

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&corev1.Secret{}, builder.WithPredicates(accessSecretPredicates)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}
