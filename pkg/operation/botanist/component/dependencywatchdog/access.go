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

package dependencywatchdog

import (
	"context"

	"github.com/gardener/gardener/pkg/operation/botanist/component"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ExternalProbeSecretName is the name of the kubecfg secret with internal DNS for external access.
	ExternalProbeSecretName = gutil.SecretNamePrefixShootAccess + "dependency-watchdog-external-probe"
	// InternalProbeSecretName is the name of the kubecfg secret with cluster IP access.
	InternalProbeSecretName = gutil.SecretNamePrefixShootAccess + "dependency-watchdog-internal-probe"
)

// NewAccess creates a new instance of the deployer for shoot cluster access for the dependency-watchdog.
func NewAccess(
	client client.Client,
	namespace string,
	values AccessValues,
) AccessInterface {
	return &dependencyWatchdogAccess{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

// AccessInterface contains functions for deploying access credentials for shoot clusters.
type AccessInterface interface {
	component.Deployer
	SetCACertificate([]byte)
}

type dependencyWatchdogAccess struct {
	client    client.Client
	namespace string
	values    AccessValues
}

// AccessValues contains configurations for the component.
type AccessValues struct {
	// ServerOutOfCluster is the out-of-cluster address of a kube-apiserver.
	ServerOutOfCluster string
	// ServerInCluster is the in-cluster address of a kube-apiserver.
	ServerInCluster string

	caCertificate []byte
}

func (d *dependencyWatchdogAccess) Deploy(ctx context.Context) error {
	for name, server := range map[string]string{
		InternalProbeSecretName: d.values.ServerInCluster,
		ExternalProbeSecretName: d.values.ServerOutOfCluster,
	} {
		var (
			shootAccessSecret = gutil.NewShootAccessSecret(name, d.namespace).WithNameOverride(name)
			kubeconfig        = kutil.NewKubeconfig(d.namespace, server, d.values.caCertificate, clientcmdv1.AuthInfo{Token: ""})
		)

		if err := shootAccessSecret.WithKubeconfig(kubeconfig).Reconcile(ctx, d.client); err != nil {
			return err
		}
	}

	// TODO(rfranzke): Remove in a future release.
	return kutil.DeleteObjects(ctx, d.client,
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "dependency-watchdog-internal-probe", Namespace: d.namespace}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "dependency-watchdog-external-probe", Namespace: d.namespace}},
	)
}

func (d *dependencyWatchdogAccess) Destroy(_ context.Context) error { return nil }

func (d *dependencyWatchdogAccess) SetCACertificate(caCert []byte) { d.values.caCertificate = caCert }
