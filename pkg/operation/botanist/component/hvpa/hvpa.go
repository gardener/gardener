// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package hvpa

import (
	"context"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	managedResourceName = "hvpa"
	deploymentName      = "hvpa-controller"
	containerName       = "hvpa-controller"
	serviceName         = "hvpa-controller"

	portNameMetrics = "metrics"
)

// New creates a new instance of DeployWaiter for the HVPA controller.
func New(client client.Client, namespace string, values Values) component.DeployWaiter {
	return &hvpa{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type hvpa struct {
	client    client.Client
	namespace string
	values    Values
}

// Values is a set of configuration values for the HVPA component.
type Values struct {
	// Image is the container image.
	Image string
}

func (h *hvpa) Deploy(ctx context.Context) error {
	var (
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hvpa-controller",
				Namespace: h.namespace,
				Labels:    getLabels(),
			},
			AutomountServiceAccountToken: pointer.Bool(false),
		}
	)

	resources, err := registry.AddAllAndSerialize(
		serviceAccount,
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, h.client, h.namespace, managedResourceName, false, resources)
}

func (h *hvpa) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, h.client, h.namespace, managedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (h *hvpa) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, h.client, h.namespace, managedResourceName)
}

func (h *hvpa) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, h.client, h.namespace, managedResourceName)
}

func getLabels() map[string]string {
	return map[string]string{v1beta1constants.GardenRole: "hvpa"}
}
