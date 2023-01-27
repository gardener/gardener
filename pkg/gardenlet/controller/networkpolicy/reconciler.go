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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/gardenlet/controller/networkpolicy/helper"
	"github.com/gardener/gardener/pkg/gardenlet/controller/networkpolicy/hostnameresolver"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Reconciler implements the reconcile.Reconcile interface for namespace reconciliation.
type Reconciler struct {
	RuntimeClient  client.Client
	Config         config.NetworkPolicyControllerConfiguration
	Resolver       hostnameresolver.HostResolver
	ResolverUpdate <-chan event.GenericEvent
}

// Reconcile reconciles namespace in order to create some central network policies.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	namespace := &corev1.Namespace{}
	if err := r.RuntimeClient.Get(ctx, request.NamespacedName, namespace); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if namespace.DeletionTimestamp != nil {
		log.V(1).Info("Skip NetworkPolicy reconciliation because namespace has a deletion timestamp")
		return reconcile.Result{}, nil
	}

	if namespace.Status.Phase != corev1.NamespaceActive {
		log.V(1).Info("Skip NetworkPolicy reconciliation because namespace is not in 'Active' phase")
		return reconcile.Result{}, nil
	}

	for _, policyConfig := range r.networkPolicyConfigs() {
		networkPolicy := &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      policyConfig.name,
				Namespace: request.Name,
			},
		}
		log = log.WithValues("networkPolicy", client.ObjectKeyFromObject(networkPolicy))

		if !labelsMatchAnySelector(namespace.Labels, policyConfig.namespaceSelectors) {
			log.Info("Deleting NetworkPolicy")
			if err := kubernetesutils.DeleteObject(ctx, r.RuntimeClient, networkPolicy); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to delete NetworkPolicy %s: %w", client.ObjectKeyFromObject(networkPolicy), err)
			}
			continue
		}

		log.V(1).Info("Reconciling NetworkPolicy")
		if err := policyConfig.reconcileFunc(ctx, log, networkPolicy); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to reconcile NetworkPolicy %s: %w", client.ObjectKeyFromObject(networkPolicy), err)
		}
		log.Info("Successfully reconciled NetworkPolicy")
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) reconcileNetworkPolicy(ctx context.Context, log logr.Logger, networkPolicy *networkingv1.NetworkPolicy, mutateFunc func(*networkingv1.NetworkPolicy)) error {
	log = log.WithValues("networkPolicy", client.ObjectKeyFromObject(networkPolicy))

	if err := r.RuntimeClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy); client.IgnoreNotFound(err) != nil {
		return err
	}

	// avoid duplicative NetworkPolicy updates
	networkPolicyCopy := networkPolicy.DeepCopy()
	mutateFunc(networkPolicyCopy)
	if apiequality.Semantic.DeepEqual(networkPolicy, networkPolicyCopy) {
		log.V(1).Info("Skip NetworkPolicy reconciliation because it already is up-to-date")
		return nil
	}

	log.Info("Reconciling NetworkPolicy")

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.RuntimeClient, networkPolicy, func() error {
		mutateFunc(networkPolicy)
		return nil
	})
	return err
}

type networkPolicyConfig struct {
	name               string
	reconcileFunc      func(context.Context, logr.Logger, *networkingv1.NetworkPolicy) error
	namespaceSelectors []labels.Selector
}

func (r *Reconciler) networkPolicyConfigs() []networkPolicyConfig {
	configs := []networkPolicyConfig{
		// This network policy is deprecated and will be removed soon in favor of `allow-to-runtime-apiserver`.
		{
			name: "allow-to-seed-apiserver",
			reconcileFunc: func(ctx context.Context, log logr.Logger, networkPolicy *networkingv1.NetworkPolicy) error {
				return r.reconcileNetworkPolicyAllowToAPIServer(ctx, log, networkPolicy, v1beta1constants.LabelNetworkPolicyToSeedAPIServer)
			},
			namespaceSelectors: []labels.Selector{
				labels.SelectorFromSet(labels.Set{v1beta1constants.LabelRole: v1beta1constants.GardenNamespace}),
				labels.SelectorFromSet(labels.Set{v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioSystem}),
				labels.SelectorFromSet(labels.Set{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot}),
			},
		},
		{
			name: "allow-to-runtime-apiserver",
			reconcileFunc: func(ctx context.Context, log logr.Logger, networkPolicy *networkingv1.NetworkPolicy) error {
				return r.reconcileNetworkPolicyAllowToAPIServer(ctx, log, networkPolicy, v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer)
			},
			namespaceSelectors: []labels.Selector{
				labels.SelectorFromSet(labels.Set{v1beta1constants.LabelRole: v1beta1constants.GardenNamespace}),
				labels.SelectorFromSet(labels.Set{v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioSystem}),
				labels.SelectorFromSet(labels.Set{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot}),
			},
		},
	}

	return configs
}

func labelsMatchAnySelector(labelsToCheck map[string]string, selectors []labels.Selector) bool {
	for _, selector := range selectors {
		if selector.Matches(labels.Set(labelsToCheck)) {
			return true
		}
	}
	return false
}

func (r *Reconciler) reconcileNetworkPolicyAllowToAPIServer(ctx context.Context, log logr.Logger, networkPolicy *networkingv1.NetworkPolicy, labelKey string) error {
	kubernetesEndpoints := &corev1.Endpoints{}
	if err := r.RuntimeClient.Get(ctx, client.ObjectKey{Name: "kubernetes", Namespace: corev1.NamespaceDefault}, kubernetesEndpoints); err != nil {
		return err
	}

	return r.reconcileNetworkPolicy(ctx, log, networkPolicy, func(policy *networkingv1.NetworkPolicy) {
		metav1.SetMetaDataAnnotation(&policy.ObjectMeta, v1beta1constants.GardenerDescription, fmt.Sprintf("Allows "+
			"egress traffic from pods labeled with '%s=%s' to the endpoints in the default namespace of the kube-apiserver "+
			"of the runtime cluster.",
			labelKey, v1beta1constants.LabelNetworkPolicyAllowed))

		policy.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{labelKey: v1beta1constants.LabelNetworkPolicyAllowed}},
			Egress:      helper.GetEgressRules(append(kubernetesEndpoints.Subsets, r.Resolver.Subset()...)...),
			Ingress:     []networkingv1.NetworkPolicyIngressRule{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		}
	})
}
