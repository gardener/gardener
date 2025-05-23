// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package workloadidentity

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	securityv1alpha1constants "github.com/gardener/gardener/pkg/apis/security/v1alpha1/constants"
	securityclientset "github.com/gardener/gardener/pkg/client/security/clientset/versioned"
)

// ControllerName is the name of the controller.
const ControllerName = "token-requestor-workload-identity"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, sourceCluster, targetCluster cluster.Cluster) error {
	if r.SeedClient == nil {
		r.SeedClient = sourceCluster.GetClient()
	}
	if r.GardenClient == nil {
		r.GardenClient = targetCluster.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}
	if r.JitterFunc == nil {
		r.JitterFunc = wait.Jitter
	}

	if r.GardenSecurityClient == nil {
		var err error

		r.GardenSecurityClient, err = securityclientset.NewForConfig(targetCluster.GetConfig())
		if err != nil {
			return fmt.Errorf("could not create securityV1Alpha1Client: %w", err)
		}
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&corev1.Secret{}, builder.WithPredicates(r.SecretPredicate())).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: r.ConcurrentSyncs,
		}).
		Complete(r)
}

// SecretPredicate is the predicate for secrets.
func (r *Reconciler) SecretPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return r.isRelevantSecret(e.Object) },
		UpdateFunc:  func(e event.UpdateEvent) bool { return r.isRelevantSecretUpdate(e.ObjectOld, e.ObjectNew) },
		DeleteFunc:  func(e event.DeleteEvent) bool { return r.isRelevantSecret(e.Object) },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

func (r *Reconciler) isRelevantSecret(obj client.Object) bool {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return false
	}
	return secret.Labels != nil &&
		secret.Labels[securityv1alpha1constants.LabelPurpose] == securityv1alpha1constants.LabelPurposeWorkloadIdentityTokenRequestor
}

func (r *Reconciler) isRelevantSecretUpdate(oldObj, newObj client.Object) bool {
	return r.isRelevantSecret(newObj) || r.isRelevantSecret(oldObj)
}
