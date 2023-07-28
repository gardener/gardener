// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package virtual

import (
	"context"
	"time"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	settingsv1alpha1 "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

// ManagedResourceName is the name of the ManagedResource containing the resource specifications.
const ManagedResourceName = "garden-system-virtual"

// New creates a new instance of DeployWaiter for virtual garden system resources.
func New(client client.Client, namespace string, values Values) component.DeployWaiter {
	return &gardenSystem{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type gardenSystem struct {
	client    client.Client
	namespace string
	values    Values
}

// Values contains values for the system resources.
type Values struct {
	// SeedAuthorizerEnabled determines whether the seed authorizer is enabled.
	SeedAuthorizerEnabled bool
}

func (g *gardenSystem) Deploy(ctx context.Context) error {
	data, err := g.computeResourcesData()
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, g.client, g.namespace, ManagedResourceName, false, data)
}

func (g *gardenSystem) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, g.client, g.namespace, ManagedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (g *gardenSystem) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, g.client, g.namespace, ManagedResourceName)
}

func (g *gardenSystem) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, g.client, g.namespace, ManagedResourceName)
}

func (g *gardenSystem) computeResourcesData() (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

		namespaceGarden = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   v1beta1constants.GardenNamespace,
				Labels: map[string]string{v1beta1constants.LabelApp: v1beta1constants.LabelGardener},
			},
		}
		clusterRoleSeedBootstrapper = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:seed-bootstrapper",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{certificatesv1.GroupName},
					Resources: []string{"certificatesigningrequests"},
					Verbs:     []string{"create", "get"},
				},
				{
					APIGroups: []string{certificatesv1.GroupName},
					Resources: []string{"certificatesigningrequests/seedclient"},
					Verbs:     []string{"create"},
				},
			},
		}
		clusterRoleBindingSeedBootstrapper = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterRoleSeedBootstrapper.Name,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRoleSeedBootstrapper.Name,
			},
			Subjects: []rbacv1.Subject{{
				APIGroup: rbacv1.GroupName,
				Kind:     "Group",
				Name:     bootstraptokenapi.BootstrapDefaultGroup,
			}},
		}
		clusterRoleGardenerAdmin = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "gardener.cloud:admin",
				Labels: map[string]string{v1beta1constants.GardenRole: "admin"},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{
						gardencorev1beta1.GroupName,
						seedmanagementv1alpha1.GroupName,
						"dashboard.gardener.cloud",
						settingsv1alpha1.GroupName,
						operationsv1alpha1.GroupName,
					},
					Resources: []string{"*"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{corev1.GroupName},
					Resources: []string{"events", "namespaces", "resourcequotas"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{eventsv1.GroupName},
					Resources: []string{"events"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{rbacv1.GroupName},
					Resources: []string{"clusterroles", "clusterrolebindings", "roles", "rolebindings"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{admissionregistrationv1.GroupName},
					Resources: []string{"mutatingwebhookconfigurations", "validatingwebhookconfigurations"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{apiregistrationv1.GroupName},
					Resources: []string{"apiservices"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{apiextensionsv1.GroupName},
					Resources: []string{"customresourcedefinitions"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{coordinationv1.GroupName},
					Resources: []string{"leases"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{certificatesv1.GroupName},
					Resources: []string{"certificatesigningrequests"},
					Verbs:     []string{"*"},
				},
			},
		}
		clusterRoleBindingGardenerAdmin = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterRoleGardenerAdmin.Name,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRoleGardenerAdmin.Name,
			},
			Subjects: []rbacv1.Subject{{
				APIGroup: rbacv1.GroupName,
				Kind:     "User",
				Name:     "system:kube-aggregator",
			}},
		}
	)

	if err := registry.Add(
		namespaceGarden,
		clusterRoleSeedBootstrapper,
		clusterRoleBindingSeedBootstrapper,
		clusterRoleGardenerAdmin,
		clusterRoleBindingGardenerAdmin,
	); err != nil {
		return nil, err
	}

	if !g.values.SeedAuthorizerEnabled {
		var (
			clusterRoleSeeds = &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "gardener.cloud:system:seeds",
				},
				Rules: []rbacv1.PolicyRule{{
					APIGroups: []string{"*"},
					Resources: []string{"*"},
					Verbs:     []string{"*"},
				}},
			}
			clusterRoleBindingSeeds = &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterRoleSeeds.Name,
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.GroupName,
					Kind:     "ClusterRole",
					Name:     clusterRoleSeeds.Name,
				},
				Subjects: []rbacv1.Subject{{
					APIGroup: rbacv1.GroupName,
					Kind:     "Group",
					Name:     v1beta1constants.SeedsGroup,
				}},
			}
		)

		if err := registry.Add(
			clusterRoleSeeds,
			clusterRoleBindingSeeds,
		); err != nil {
			return nil, err
		}
	}

	return registry.SerializedObjects(), nil
}
