// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package secret

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// ControllerName is the name of this controller.
const ControllerName = "shootstate-secret"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, gardenCluster, seedCluster cluster.Cluster) error {
	if r.GardenClient == nil {
		r.GardenClient = gardenCluster.GetClient()
	}
	if r.SeedClient == nil {
		r.SeedClient = seedCluster.GetClient()
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&corev1.Secret{}, builder.WithPredicates(r.SecretPredicate())).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: *r.Config.ConcurrentSyncs,
			RateLimiter:             workqueue.DefaultControllerRateLimiter(),
		}).
		Complete(r)
}

// SecretPredicate returns the predicates for the secret watch.
func (r *Reconciler) SecretPredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(object client.Object) bool {
		labels := object.GetLabels()
		return labels[secretsmanager.LabelKeyManagedBy] == secretsmanager.LabelValueSecretsManager &&
			labels[secretsmanager.LabelKeyPersist] == secretsmanager.LabelValueTrue
	})
}
