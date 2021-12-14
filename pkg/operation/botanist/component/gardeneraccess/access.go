// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagedResourceName is the name of the ManagedResource containing the resource specifications.
const ManagedResourceName = "shoot-core-gardeneraccess"

// New creates a new instance of the deployer for GardenerAccess.
func New(
	client client.Client,
	namespace string,
	values Values,
) Interface {
	return &gardener{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

// Interface contains functions for deploying access credentials for shoot clusters.
type Interface interface {
	component.Deployer
	SetCACertificate([]byte)
}

type gardener struct {
	client    client.Client
	namespace string
	values    Values
}

// Values contains configurations for the component.
type Values struct {
	// ServerOutOfCluster is the out-of-cluster address of a kube-apiserver.
	ServerOutOfCluster string
	// ServerInCluster is the in-cluster address of a kube-apiserver.
	ServerInCluster string

	caCertificate []byte
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

	for _, v := range accessNamesToServers {
		var (
			shootAccessSecret = gutil.NewShootAccessSecret(v.name, g.namespace).WithNameOverride(v.name)
			kubeconfig        = kutil.NewKubeconfig(g.namespace, v.server, g.values.caCertificate, clientcmdv1.AuthInfo{Token: ""})
		)

		serviceAccountNames = append(serviceAccountNames, shootAccessSecret.ServiceAccountName)

		// TODO(rfranzke): Remove in a future release.
		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, g.client, shootAccessSecret.Secret, func() error {
			if shootAccessSecret.Secret.Data["gardener.crt"] != nil || shootAccessSecret.Secret.Data["gardener-internal.crt"] != nil {
				shootAccessSecret.Secret.Data = nil
			}
			return nil
		}); err != nil {
			return err
		}

		if err := shootAccessSecret.WithKubeconfig(kubeconfig).Reconcile(ctx, g.client); err != nil {
			return err
		}
	}

	data, err := g.computeResourcesData(serviceAccountNames...)
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, g.client, g.namespace, ManagedResourceName, true, data)
}

func (g *gardener) Destroy(ctx context.Context) error {
	return managedresources.DeleteForShoot(ctx, g.client, g.namespace, ManagedResourceName)
}

func (g *gardener) SetCACertificate(caCert []byte) { g.values.caCertificate = caCert }

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
