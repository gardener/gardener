// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package machinecontrollermanager

import (
	"context"

	"github.com/Masterminds/semver"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

// New creates a new instance of DeployWaiter for the machine-controller-manager.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values Values,
) component.DeployWaiter {
	return &machineControllerManager{
		client:                        client,
		namespace:                     namespace,
		secretsManager:                secretsManager,
		values:                        values,
		runtimeVersionGreaterEqual123: versionutils.ConstraintK8sGreaterEqual123.Check(values.RuntimeKubernetesVersion),
	}
}

type machineControllerManager struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values

	runtimeVersionGreaterEqual123 bool
}

// Values is a set of configuration values for the machine-controller-manager component.
type Values struct {
	// Image is the container image used for machine-controller-manager.
	Image string
	// Replicas is the number of replicas for the deployment.
	Replicas int32
	// RuntimeKubernetesVersion is the Kubernetes version of the runtime cluster.
	RuntimeKubernetesVersion *semver.Version
}

func (m *machineControllerManager) Deploy(ctx context.Context) error {
	var (
		serviceAccount = m.emptyServiceAccount()
	)

	if _, err := controllerutils.GetAndCreateOrStrategicMergePatch(ctx, m.client, serviceAccount, func() error {
		serviceAccount.AutomountServiceAccountToken = pointer.Bool(false)
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (m *machineControllerManager) Destroy(ctx context.Context) error {
	return kubernetesutils.DeleteObjects(ctx, m.client,
		m.emptyServiceAccount(),
	)
}

func (m *machineControllerManager) Wait(_ context.Context) error        { return nil }
func (m *machineControllerManager) WaitCleanup(_ context.Context) error { return nil }

func (m *machineControllerManager) emptyServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "machine-controller-manager", Namespace: m.namespace}}
}
