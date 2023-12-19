// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package tokenrequestor

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	corev1clientset "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

// ControllerName is the name of the controller.
const ControllerName = "token-requestor"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, sourceCluster, targetCluster cluster.Cluster) error {
	if r.SourceClient == nil {
		r.SourceClient = sourceCluster.GetClient()
	}
	if r.TargetClient == nil {
		r.TargetClient = targetCluster.GetClient()
	}
	if r.TargetCoreV1Client == nil {
		var err error
		r.TargetCoreV1Client, err = corev1clientset.NewForConfig(targetCluster.GetConfig())
		if err != nil {
			return fmt.Errorf("could not create coreV1Client: %w", err)
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
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
}

func (r *Reconciler) isRelevantSecret(obj client.Object) bool {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return false
	}
	return secret.Labels != nil &&
		secret.Labels[resourcesv1alpha1.ResourceManagerPurpose] == resourcesv1alpha1.LabelPurposeTokenRequest &&
		(r.Class == nil || secret.Labels[resourcesv1alpha1.ResourceManagerClass] == *r.Class)
}

func (r *Reconciler) isRelevantSecretUpdate(oldObj, newObj client.Object) bool {
	return r.isRelevantSecret(newObj) || r.isRelevantSecret(oldObj)
}
