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

package gardeneradmissioncontroller

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	admissioncontrollerv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	// DeploymentName is the name of the admission controller deployment.
	DeploymentName = "gardener-admission-controller"
	// ServiceName is the name of the admission controller service.
	ServiceName = "gardener-admission-controller"

	serverPort  = 2719
	healthzPort = 2722
	metricsPort = 2723

	managedResourceNameRuntime = "gardener-admission-controller-runtime"
	managedResourceNameVirtual = "gardener-admission-controller-virtual"
)

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy or
// deleted.
var TimeoutWaitForManagedResource = 5 * time.Minute

// Values contains configuration values for the gardener-admission-controller resources.
type Values struct {
	ClientConnection               ClientConnection
	LogLevel                       string
	ResourceAdmissionConfiguration *admissioncontrollerv1alpha1.ResourceAdmissionConfiguration
	ReplicaCount                   int32
}

// ClientConnection holds values for the client connection.
type ClientConnection struct {
	QPS   float32
	Burst int32
}

// New creates a new instance of DeployWaiter for the gardener-admission-controller.
func New(client client.Client, namespace string, secretsManager secretsmanager.Interface, values Values) component.DeployWaiter {
	return &admissioncontroller{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type admissioncontroller struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

func (a admissioncontroller) Deploy(ctx context.Context) error {
	var (
		runtimeRegistry           = managedresources.NewRegistry(operatorclient.RuntimeScheme, operatorclient.RuntimeCodec, operatorclient.RuntimeSerializer)
		virtualGardenAccessSecret = a.newVirtualGardenAccessSecret()
	)

	secretServerCert, err := a.reconcileSecretServerCert(ctx)
	if err != nil {
		return err
	}

	if err := virtualGardenAccessSecret.Reconcile(ctx, a.client); err != nil {
		return err
	}

	admissonConfigMap, err := a.admissionConfigConfigMap()
	if err != nil {
		return err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	runtimeResources, err := runtimeRegistry.AddAllAndSerialize(
		a.podDisruptionBudget(),
		admissonConfigMap,
	)
	if err != nil {
		return err
	}

	if err := managedresources.CreateForSeed(ctx, a.client, a.namespace, managedResourceNameRuntime, false, runtimeResources); err != nil {
		return err
	}
	if err := managedresources.WaitUntilHealthy(timeoutCtx, a.client, a.namespace, managedResourceNameRuntime); err != nil {
		return err
	}

	var (
		virtualRegistry = managedresources.NewRegistry(operatorclient.VirtualScheme, operatorclient.VirtualCodec, operatorclient.VirtualSerializer)
	)

	virtualResources, err := virtualRegistry.AddAllAndSerialize()
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, a.client, a.namespace, managedResourceNameVirtual, managedresources.LabelValueGardener, false, virtualResources)
}

func (a admissioncontroller) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return flow.Parallel(
		func(ctx context.Context) error {
			return managedresources.WaitUntilHealthy(ctx, a.client, a.namespace, managedResourceNameRuntime)
		},
		func(ctx context.Context) error {
			return managedresources.WaitUntilHealthy(ctx, a.client, a.namespace, managedResourceNameVirtual)
		},
	)(timeoutCtx)
}

func (a admissioncontroller) Destroy(ctx context.Context) error {
	if err := managedresources.DeleteForShoot(ctx, a.client, a.namespace, managedResourceNameVirtual); err != nil {
		return err
	}

	if err := managedresources.DeleteForSeed(ctx, a.client, a.namespace, managedResourceNameRuntime); err != nil {
		return err
	}

	return kubernetesutils.DeleteObjects(ctx, a.client, a.newVirtualGardenAccessSecret().Secret)
}

func (a admissioncontroller) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return flow.Parallel(
		func(ctx context.Context) error {
			return managedresources.WaitUntilDeleted(ctx, a.client, a.namespace, managedResourceNameRuntime)
		},
		func(ctx context.Context) error {
			return managedresources.WaitUntilDeleted(ctx, a.client, a.namespace, managedResourceNameVirtual)
		},
	)(timeoutCtx)
}

// GetLabels returns the labels for the gardener-admission-controller.
func GetLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelGardener,
		v1beta1constants.LabelRole: "admission-controller",
	}
}
