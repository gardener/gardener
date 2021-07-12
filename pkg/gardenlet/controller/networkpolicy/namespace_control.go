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
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// namespaceReconciler implements the reconcile.Reconcile interface for namespace reconciliation.
type namespaceReconciler struct {
	log                    *logrus.Logger
	seedClient             client.Client
	seedName               string
	shootNamespaceSelector labels.Selector
	resolver               hostnameresolver.HostResolver
}

// newNamespaceReconciler returns the new namespace reconciler.
func newNamespaceReconciler(
	seedLogger *logrus.Logger,
	seedClient client.Client,
	seedName string,
	shootNamespaceSelector labels.Selector,
	resolver hostnameresolver.HostResolver,
) reconcile.Reconciler {
	return &namespaceReconciler{
		seedClient:             seedClient,
		log:                    seedLogger,
		seedName:               seedName,
		shootNamespaceSelector: shootNamespaceSelector,
		resolver:               resolver,
	}
}

// Reconcile reconciles namespace in order to create the "allowed-to-seed-apiserver" Network Policy
func (r *namespaceReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	namespace := &corev1.Namespace{}
	if err := r.seedClient.Get(ctx, request.NamespacedName, namespace); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return reconcile.Result{}, err
		}
		r.log.Debugf("namespace %q is not found, not trying to reconcile", request.Name)
		return reconcile.Result{}, nil
	}

	// if the namespace is not the Garden or a Shoot namespace - delete the existing NetworkPolicy
	if !(namespace.Name == v1beta1constants.GardenNamespace || r.shootNamespaceSelector.Matches(labels.Set(namespace.Labels))) {
		policy := &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      helper.AllowToSeedAPIServer,
				Namespace: request.Name,
			},
		}
		err := r.seedClient.Delete(ctx, policy)
		if client.IgnoreNotFound(err) != nil {
			return reconcile.Result{}, fmt.Errorf("unable to delete NetworkPolicy %q from namespace %q: %w", policy.Name, namespace.Name, err)
		} else if err == nil {
			r.log.Infof("Deleting NetworkPolicy %q from namespace %q", policy.Name, namespace.Name)
		}
		return reconcile.Result{}, nil
	}

	if namespace.DeletionTimestamp != nil {
		r.log.Debugf("Do not update NetworkPolicy %q in namespace %q - namespace has a deletion timestamp", helper.AllowToSeedAPIServer, namespace.Name)
		return reconcile.Result{}, nil
	}

	if namespace.Status.Phase != corev1.NamespaceActive {
		r.log.Debugf("Do not update NetworkPolicy %q in namespace %q - namespace is not active", helper.AllowToSeedAPIServer, namespace.Name)
		return reconcile.Result{}, nil
	}

	r.log.Debugf("Reconciling NetworkPolicy %q in namespace %q", helper.AllowToSeedAPIServer, request.Name)

	kubernetesEndpoints := corev1.Endpoints{}
	kubernetesEndpointsKey := types.NamespacedName{Namespace: corev1.NamespaceDefault, Name: "kubernetes"}
	if err := r.seedClient.Get(ctx, kubernetesEndpointsKey, &kubernetesEndpoints); err != nil {
		return reconcile.Result{}, err
	}

	egressRules := helper.GetEgressRules(append(kubernetesEndpoints.Subsets, r.resolver.Subset()...)...)
	// avoid duplicate NetworkPolicy updates
	policy := &networkingv1.NetworkPolicy{}
	if err := r.seedClient.Get(ctx, kutil.Key(request.Name, helper.AllowToSeedAPIServer), policy); client.IgnoreNotFound(err) != nil {
		return reconcile.Result{}, err
	}
	if apiequality.Semantic.DeepEqual(policy.Spec.Egress, egressRules) {
		r.log.Debugf("NetworkPolicy %q in namespace %q already up-to date", helper.AllowToSeedAPIServer, request.Name)
		return reconcile.Result{}, nil
	}

	if err := helper.EnsureNetworkPolicy(ctx, r.seedClient, request.Name, egressRules); err != nil {
		return reconcile.Result{}, err
	}

	r.log.Infof("Successfully updated NetworkPolicy %q in namespace %q", helper.AllowToSeedAPIServer, request.Name)
	return reconcile.Result{}, nil
}
