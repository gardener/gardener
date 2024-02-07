// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package alertmanager

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	port = 9093
)

// Values contains configuration values for the AlertManager resources.
type Values struct {
	// Name is the name of the AlertManager. It will be used for the resource names of AlertManager and ManagedResource.
	Name string
	// Image defines the container image of AlertManager.
	Image string
	// Version is the version of AlertManager.
	Version string
	// PriorityClassName is the name of the priority class for the StatefulSet.
	PriorityClassName string
	// StorageCapacity is the storage capacity of AlertManager.
	StorageCapacity resource.Quantity
	// AlertingSMTPSecret is the alerting SMTP secret.
	AlertingSMTPSecret *corev1.Secret
}

// New creates a new instance of DeployWaiter for the AlertManager.
func New(client client.Client, namespace string, values Values) component.DeployWaiter {
	return &alertManager{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type alertManager struct {
	client    client.Client
	namespace string
	values    Values
}

func (a *alertManager) Deploy(ctx context.Context) error {
	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

	resources, err := registry.AddAllAndSerialize(
		a.service(),
		a.alertManager(),
		a.vpa(),
		a.config(),
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, a.client, a.namespace, a.name(), false, resources)
}

func (a *alertManager) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, a.client, a.namespace, a.name())
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy or
// deleted.
var TimeoutWaitForManagedResource = 5 * time.Minute

func (a *alertManager) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, a.client, a.namespace, a.name())
}

func (a *alertManager) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, a.client, a.namespace, a.name())
}

func (a *alertManager) name() string {
	return "alertmanager-" + a.values.Name
}

func (a *alertManager) getLabels() map[string]string {
	return map[string]string{
		"component":                "alertmanager",
		v1beta1constants.LabelRole: v1beta1constants.LabelMonitoring,
	}
}
