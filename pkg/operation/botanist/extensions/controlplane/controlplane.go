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

package controlplane

import (
	"context"
	"fmt"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/shoot"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// DefaultInterval is the default interval for retry operations.
	DefaultInterval = 5 * time.Second
	// DefaultSevereThreshold is the default threshold until an error reported by another component is treated as
	// 'severe'.
	DefaultSevereThreshold = 30 * time.Second
	// DefaultTimeout is the default timeout and defines how long Gardener should wait for a successful reconciliation
	// of a ControlPlane resource.
	DefaultTimeout = 3 * time.Minute
)

// TimeNow returns the current time. Exposed for testing.
var TimeNow = time.Now

// Values contains the values used to create an ControlPlane resources.
type Values struct {
	// Namespace is the Shoot namespace in the seed.
	Namespace string
	// Name is the name of the ControlPlane resource. Commonly the Shoot's name.
	Name string
	// Type is the type of the ControlPlane provider.
	Type string
	// ProviderConfig contains the provider config for the ControlPlane provider.
	ProviderConfig *runtime.RawExtension
	// Purpose is the purpose of the ControlPlane resource (normal/exposure).
	Purpose extensionsv1alpha1.Purpose
	// Region is the region of the shoot.
	Region string
	// InfrastructureProviderStatus is the provider status of the Infrastructure resource which might be relevant for
	// the ControlPlane reconciliation.
	InfrastructureProviderStatus *runtime.RawExtension
}

// New creates a new instance of an ControlPlane deployer.
func New(
	logger logrus.FieldLogger,
	client client.Client,
	values *Values,
	waitInterval time.Duration,
	waitSevereThreshold time.Duration,
	waitTimeout time.Duration,
) shoot.ExtensionControlPlane {
	return &controlPlane{
		client:              client,
		logger:              logger,
		values:              values,
		waitInterval:        waitInterval,
		waitSevereThreshold: waitSevereThreshold,
		waitTimeout:         waitTimeout,
	}
}

type controlPlane struct {
	values              *Values
	logger              logrus.FieldLogger
	client              client.Client
	waitInterval        time.Duration
	waitSevereThreshold time.Duration
	waitTimeout         time.Duration

	providerStatus *runtime.RawExtension
}

func (i *controlPlane) name() string {
	if i.values.Purpose == extensionsv1alpha1.Exposure {
		return i.values.Name + "-exposure"
	}
	return i.values.Name
}

// Deploy uses the seed client to create or update the ControlPlane resource.
func (i *controlPlane) Deploy(ctx context.Context) error {
	_, err := i.deploy(ctx, v1beta1constants.GardenerOperationReconcile)
	return err
}

func (i *controlPlane) deploy(ctx context.Context, operation string) (extensionsv1alpha1.Object, error) {
	var (
		controlPlane = &extensionsv1alpha1.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      i.name(),
				Namespace: i.values.Namespace,
			},
		}
		providerConfig *runtime.RawExtension
	)

	if cfg := i.values.ProviderConfig; cfg != nil {
		providerConfig = &runtime.RawExtension{Raw: cfg.Raw}
	}

	_, err := controllerutil.CreateOrUpdate(ctx, i.client, controlPlane, func() error {
		metav1.SetMetaDataAnnotation(&controlPlane.ObjectMeta, v1beta1constants.GardenerOperation, operation)
		metav1.SetMetaDataAnnotation(&controlPlane.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().String())

		controlPlane.Spec = extensionsv1alpha1.ControlPlaneSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type:           i.values.Type,
				ProviderConfig: providerConfig,
			},
			Region:  i.values.Region,
			Purpose: &i.values.Purpose,
			SecretRef: corev1.SecretReference{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: controlPlane.Namespace,
			},
			InfrastructureProviderStatus: i.values.InfrastructureProviderStatus,
		}

		return nil
	})

	return controlPlane, err
}

// Restore uses the seed client and the ShootState to create the ControlPlane resources and restore their state.
func (i *controlPlane) Restore(ctx context.Context, shootState *gardencorev1alpha1.ShootState) error {
	return common.RestoreExtensionWithDeployFunction(
		ctx,
		shootState,
		i.client,
		extensionsv1alpha1.ControlPlaneResource,
		i.values.Namespace,
		i.deploy,
	)
}

// Migrate migrates the ControlPlane resources.
func (i *controlPlane) Migrate(ctx context.Context) error {
	return common.MigrateExtensionCRs(
		ctx,
		i.client,
		&extensionsv1alpha1.ControlPlaneList{},
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.ControlPlane{} },
		i.values.Namespace,
	)
}

// Destroy deletes the ControlPlane resource.
func (i *controlPlane) Destroy(ctx context.Context) error {
	return common.DeleteExtensionCR(
		ctx,
		i.client,
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.ControlPlane{} },
		i.values.Namespace,
		i.name(),
	)
}

// Wait waits until the ControlPlane resource is ready.
func (i *controlPlane) Wait(ctx context.Context) error {
	return common.WaitUntilExtensionCRReady(
		ctx,
		i.client,
		i.logger,
		func() runtime.Object { return &extensionsv1alpha1.ControlPlane{} },
		extensionsv1alpha1.ControlPlaneResource,
		i.values.Namespace,
		i.name(),
		i.waitInterval,
		i.waitSevereThreshold,
		i.waitTimeout,
		func(obj runtime.Object) error {
			controlPlane, ok := obj.(*extensionsv1alpha1.ControlPlane)
			if !ok {
				return fmt.Errorf("expected extensionsv1alpha1.ControlPlane but got %T", controlPlane)
			}

			i.providerStatus = controlPlane.Status.ProviderStatus
			return nil
		},
	)
}

// WaitMigrate waits until the ControlPlane resources are migrated successfully.
func (i *controlPlane) WaitMigrate(ctx context.Context) error {
	return common.WaitUntilExtensionCRMigrated(
		ctx,
		i.client,
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.ControlPlane{} },
		i.values.Namespace,
		i.name(),
		i.waitInterval,
		i.waitTimeout,
	)
}

// WaitCleanup waits until the ControlPlane resource is deleted.
func (i *controlPlane) WaitCleanup(ctx context.Context) error {
	return common.WaitUntilExtensionCRDeleted(
		ctx,
		i.client,
		i.logger,
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.ControlPlane{} },
		extensionsv1alpha1.ControlPlaneResource,
		i.values.Namespace,
		i.name(),
		i.waitInterval,
		i.waitTimeout,
	)
}

// SetInfrastructureProviderStatus sets the infrastructure provider status in the values.
func (i *controlPlane) SetInfrastructureProviderStatus(status *runtime.RawExtension) {
	i.values.InfrastructureProviderStatus = status
}

// ProviderStatus returns the generated status of the provider.
func (i *controlPlane) ProviderStatus() *runtime.RawExtension {
	return i.providerStatus
}
