// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
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

// Interface is an interface for managing ControlPlanes.
type Interface interface {
	component.DeployMigrateWaiter
	SetInfrastructureProviderStatus(*runtime.RawExtension)
	ProviderStatus() *runtime.RawExtension
}

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

// New creates a new instance of Interface.
func New(
	log logr.Logger,
	client client.Client,
	values *Values,
	waitInterval time.Duration,
	waitSevereThreshold time.Duration,
	waitTimeout time.Duration,
) Interface {
	name := values.Name
	if values.Purpose == extensionsv1alpha1.Exposure {
		name += "-exposure"
	}

	return &controlPlane{
		log:                 log,
		client:              client,
		values:              values,
		waitInterval:        waitInterval,
		waitSevereThreshold: waitSevereThreshold,
		waitTimeout:         waitTimeout,

		controlPlane: &extensionsv1alpha1.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: values.Namespace,
			},
		},
	}
}

type controlPlane struct {
	values              *Values
	log                 logr.Logger
	client              client.Client
	waitInterval        time.Duration
	waitSevereThreshold time.Duration
	waitTimeout         time.Duration

	controlPlane   *extensionsv1alpha1.ControlPlane
	providerStatus *runtime.RawExtension
}

// Deploy uses the seed client to create or update the ControlPlane resource.
func (c *controlPlane) Deploy(ctx context.Context) error {
	_, err := c.deploy(ctx, v1beta1constants.GardenerOperationReconcile)
	return err
}

func (c *controlPlane) deploy(ctx context.Context, operation string) (extensionsv1alpha1.Object, error) {
	var providerConfig *runtime.RawExtension
	if cfg := c.values.ProviderConfig; cfg != nil {
		providerConfig = &runtime.RawExtension{Raw: cfg.Raw}
	}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, c.client, c.controlPlane, func() error {
		metav1.SetMetaDataAnnotation(&c.controlPlane.ObjectMeta, v1beta1constants.GardenerOperation, operation)
		metav1.SetMetaDataAnnotation(&c.controlPlane.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().Format(time.RFC3339Nano))

		c.controlPlane.Spec = extensionsv1alpha1.ControlPlaneSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type:           c.values.Type,
				ProviderConfig: providerConfig,
			},
			Region:  c.values.Region,
			Purpose: &c.values.Purpose,
			SecretRef: corev1.SecretReference{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: c.controlPlane.Namespace,
			},
			InfrastructureProviderStatus: c.values.InfrastructureProviderStatus,
		}

		return nil
	})

	return c.controlPlane, err
}

// Restore uses the seed client and the ShootState to create the ControlPlane resources and restore their state.
func (c *controlPlane) Restore(ctx context.Context, shootState *gardencorev1beta1.ShootState) error {
	return extensions.RestoreExtensionWithDeployFunction(
		ctx,
		c.client,
		shootState,
		extensionsv1alpha1.ControlPlaneResource,
		c.deploy,
	)
}

// Migrate migrates the ControlPlane resources.
func (c *controlPlane) Migrate(ctx context.Context) error {
	return extensions.MigrateExtensionObjects(
		ctx,
		c.client,
		&extensionsv1alpha1.ControlPlaneList{},
		c.values.Namespace,
		nil,
	)
}

// Destroy deletes the ControlPlane resource.
func (c *controlPlane) Destroy(ctx context.Context) error {
	return extensions.DeleteExtensionObject(
		ctx,
		c.client,
		c.controlPlane,
	)
}

// Wait waits until the ControlPlane resource is ready.
func (c *controlPlane) Wait(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectReady(
		ctx,
		c.client,
		c.log,
		c.controlPlane,
		extensionsv1alpha1.ControlPlaneResource,
		c.waitInterval,
		c.waitSevereThreshold,
		c.waitTimeout,
		func() error {
			c.providerStatus = c.controlPlane.Status.ProviderStatus
			return nil
		},
	)
}

// WaitMigrate waits until the ControlPlane resources are migrated successfully.
func (c *controlPlane) WaitMigrate(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectMigrated(
		ctx,
		c.client,
		c.controlPlane,
		extensionsv1alpha1.ControlPlaneResource,
		c.waitInterval,
		c.waitTimeout,
	)
}

// WaitCleanup waits until the ControlPlane resource is deleted.
func (c *controlPlane) WaitCleanup(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectDeleted(
		ctx,
		c.client,
		c.log,
		c.controlPlane,
		extensionsv1alpha1.ControlPlaneResource,
		c.waitInterval,
		c.waitTimeout,
	)
}

// SetInfrastructureProviderStatus sets the infrastructure provider status in the values.
func (c *controlPlane) SetInfrastructureProviderStatus(status *runtime.RawExtension) {
	c.values.InfrastructureProviderStatus = status
}

// ProviderStatus returns the generated status of the provider.
func (c *controlPlane) ProviderStatus() *runtime.RawExtension {
	return c.providerStatus
}
