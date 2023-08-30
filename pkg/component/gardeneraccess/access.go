// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardeneraccess

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// ManagedResourceName is the name of the ManagedResource containing the resource specifications.
const ManagedResourceName = "shoot-core-gardeneraccess"

// New creates a new instance of the deployer for GardenerAccess.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values Values,
) component.DeployWaiter {
	return &gardener{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type gardener struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

// Values contains configurations for the component.
type Values struct {
	// ServerOutOfCluster is the out-of-cluster address of a kube-apiserver.
	ServerOutOfCluster string
	// ServerInCluster is the in-cluster address of a kube-apiserver.
	ServerInCluster string
}

type accessNameToServer struct {
	name   string
	server string
}

func (g *gardener) Deploy(ctx context.Context) error {
	var (
		accessNamesToServers = []accessNameToServer{
			{v1beta1constants.SecretNameGardener, g.values.ServerOutOfCluster},
			{v1beta1constants.SecretNameGardenerInternal, g.values.ServerInCluster},
		}
		serviceAccountNames = make([]string, 0, len(accessNamesToServers))
	)

	caSecret, found := g.secretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}

	for _, v := range accessNamesToServers {
		var (
			shootAccessSecret = gardenerutils.NewShootAccessSecret(v.name, g.namespace).WithNameOverride(v.name)
			kubeconfig        = kubernetesutils.NewKubeconfig(
				g.namespace,
				clientcmdv1.Cluster{Server: v.server, CertificateAuthorityData: caSecret.Data[secretsutils.DataKeyCertificateBundle]},
				clientcmdv1.AuthInfo{Token: ""},
			)
		)

		serviceAccountNames = append(serviceAccountNames, shootAccessSecret.ServiceAccountName)

		if err := shootAccessSecret.WithKubeconfig(kubeconfig).Reconcile(ctx, g.client); err != nil {
			return err
		}
	}

	data, err := g.computeResourcesData(serviceAccountNames...)
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, g.client, g.namespace, ManagedResourceName, managedresources.LabelValueGardener, true, data)
}

func (g *gardener) Destroy(ctx context.Context) error {
	for _, v := range []string{v1beta1constants.SecretNameGardener, v1beta1constants.SecretNameGardenerInternal} {
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: v, Namespace: g.namespace}}
		if err := g.client.Delete(ctx, secret); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed deleting secret %s: %w", client.ObjectKeyFromObject(secret), err)
		}
	}

	return managedresources.DeleteForShoot(ctx, g.client, g.namespace, ManagedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (g *gardener) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, g.client, g.namespace, ManagedResourceName)
}

func (g *gardener) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, g.client, g.namespace, ManagedResourceName)
}

func (g *gardener) computeResourcesData(serviceAccountNames ...string) (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:gardener",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     "cluster-admin",
			},
		}
	)

	for _, name := range serviceAccountNames {
		clusterRoleBinding.Subjects = append(clusterRoleBinding.Subjects, rbacv1.Subject{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      name,
			Namespace: metav1.NamespaceSystem,
		})
	}

	return registry.AddAllAndSerialize(clusterRoleBinding)
}
