// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package networkpolicy

import (
	"context"
	"fmt"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/gardenlet/controller/networkpolicy/helper"
	"github.com/gardener/gardener/pkg/gardenlet/controller/networkpolicy/hostnameresolver"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// namespaceReconciler implements the reconcile.Reconcile interface for namespace reconciliation.
type namespaceReconciler struct {
	seedClient             client.Client
	seedName               string
	shootNamespaceSelector labels.Selector
	resolver               hostnameresolver.HostResolver
}

// newNamespaceReconciler returns the new namespace reconciler.
func newNamespaceReconciler(
	seedClient client.Client,
	seedName string,
	shootNamespaceSelector labels.Selector,
	resolver hostnameresolver.HostResolver,
) reconcile.Reconciler {
	return &namespaceReconciler{
		seedClient:             seedClient,
		seedName:               seedName,
		shootNamespaceSelector: shootNamespaceSelector,
		resolver:               resolver,
	}
}

// Reconcile reconciles namespace in order to create the "allowed-to-seed-apiserver" Network Policy
func (r *namespaceReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	namespace := &corev1.Namespace{}
	if err := r.seedClient.Get(ctx, request.NamespacedName, namespace); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	networkPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      helper.AllowToSeedAPIServer,
			Namespace: request.Name,
		},
	}
	log = log.WithValues("networkPolicy", client.ObjectKeyFromObject(networkPolicy))

	// if the namespace is not the Garden, IstioSystem or a Shoot namespace - delete the existing NetworkPolicy
	if !(namespace.Name == v1beta1constants.GardenNamespace || namespace.Name == v1beta1constants.IstioSystemNamespace || r.shootNamespaceSelector.Matches(labels.Set(namespace.Labels))) {
		if err := r.seedClient.Delete(ctx, networkPolicy); client.IgnoreNotFound(err) != nil {
			return reconcile.Result{}, fmt.Errorf("unable to delete NetworkPolicy %q from namespace %q: %w", networkPolicy.Name, namespace.Name, err)
		}

		log.Info("Deleted NetworkPolicy")
		return reconcile.Result{}, nil
	}

	if namespace.DeletionTimestamp != nil {
		log.V(1).Info("Do not update NetworkPolicy because namespace has a deletion timestamp")
		return reconcile.Result{}, nil
	}

	if namespace.Status.Phase != corev1.NamespaceActive {
		log.V(1).Info("Do not update NetworkPolicy because namespace is not in 'Active' phase")
		return reconcile.Result{}, nil
	}

	log.V(1).Info("Reconciling NetworkPolicy")

	kubernetesEndpoints := corev1.Endpoints{}
	kubernetesEndpointsKey := types.NamespacedName{Namespace: corev1.NamespaceDefault, Name: "kubernetes"}
	if err := r.seedClient.Get(ctx, kubernetesEndpointsKey, &kubernetesEndpoints); err != nil {
		return reconcile.Result{}, err
	}

	egressRules := helper.GetEgressRules(append(kubernetesEndpoints.Subsets, r.resolver.Subset()...)...)
	// avoid duplicate NetworkPolicy updates
	if err := r.seedClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy); client.IgnoreNotFound(err) != nil {
		return reconcile.Result{}, err
	}
	if apiequality.Semantic.DeepEqual(networkPolicy.Spec.Egress, egressRules) {
		log.V(1).Info("Do not update NetworkPolicy because it already is up-to-date")
		return reconcile.Result{}, nil
	}

	if err := helper.EnsureNetworkPolicy(ctx, r.seedClient, request.Name, egressRules); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Successfully updated NetworkPolicy")
	return reconcile.Result{}, nil
}
