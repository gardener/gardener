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

	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
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
	)

	if err := registry.Add(
		namespaceGarden,
		clusterRoleSeedBootstrapper,
		clusterRoleBindingSeedBootstrapper,
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
