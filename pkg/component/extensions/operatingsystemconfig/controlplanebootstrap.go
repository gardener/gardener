// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/nodeinit"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
)

type controlPlaneBootstrap struct {
	// This type only implements a subset of Interface. Embed this to spare the panic stubs.
	Interface

	log    logr.Logger
	client client.Client
	values *ControlPlaneBootstrapValues

	waitInterval        time.Duration
	waitSevereThreshold time.Duration
	waitTimeout         time.Duration

	osc *extensionsv1alpha1.OperatingSystemConfig
}

// ControlPlaneBootstrapValues contains the values used to create an OperatingSystemConfig resource for gardenadm bootstrap.
type ControlPlaneBootstrapValues struct {
	// Namespace is the namespace for the OperatingSystemConfig resource.
	Namespace string
	// Worker is the control plane worker pool.
	Worker *gardencorev1beta1.Worker
	// GardenadmImage is the gardenadm image reference that should be pulled.
	GardenadmImage string
}

// NewControlPlaneBootstrap creates a new instance of Interface for deploying OperatingSystemConfigs during gardenadm bootstrap.
func NewControlPlaneBootstrap(
	log logr.Logger,
	client client.Client,
	values *ControlPlaneBootstrapValues,
	waitInterval, waitSevereThreshold, waitTimeout time.Duration,
) Interface {
	return &controlPlaneBootstrap{
		log:    log,
		client: client,
		values: values,

		waitInterval:        waitInterval,
		waitSevereThreshold: waitSevereThreshold,
		waitTimeout:         waitTimeout,

		osc: &extensionsv1alpha1.OperatingSystemConfig{ObjectMeta: metav1.ObjectMeta{
			Name:      "gardenadm-" + values.Worker.Name,
			Namespace: values.Namespace,
		}},
	}
}

func (c *controlPlaneBootstrap) Deploy(ctx context.Context) error {
	units, files, err := nodeinit.GardenadmConfig(c.values.GardenadmImage)
	if err != nil {
		return err
	}

	_, err = controllerutils.GetAndCreateOrMergePatch(ctx, c.client, c.osc, func() error {
		metav1.SetMetaDataAnnotation(&c.osc.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		metav1.SetMetaDataAnnotation(&c.osc.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().Format(time.RFC3339Nano))

		c.osc.Spec = extensionsv1alpha1.OperatingSystemConfigSpec{
			Purpose: extensionsv1alpha1.OperatingSystemConfigPurposeProvision,
			Units:   units,
			Files:   files,
		}

		// TODO(timebertt): ensure Worker.Machine.Image is set in Shoot validation for `gardenadm bootstrap`
		c.osc.Spec.Type = c.values.Worker.Machine.Image.Name
		c.osc.Spec.ProviderConfig = c.values.Worker.Machine.Image.ProviderConfig

		return nil
	})
	return err
}

func (c *controlPlaneBootstrap) Wait(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectReady(
		ctx,
		c.client,
		c.log,
		c.osc,
		extensionsv1alpha1.OperatingSystemConfigResource,
		c.waitInterval,
		c.waitSevereThreshold,
		c.waitTimeout,
		func() error {
			if c.osc.Status.CloudConfig == nil {
				return fmt.Errorf("no cloud config information provided in status")
			}

			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Namespace: c.osc.Status.CloudConfig.SecretRef.Namespace,
				Name:      c.osc.Status.CloudConfig.SecretRef.Name,
			}}
			if err := c.client.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
				return fmt.Errorf("failed getting cloud config secret %q: %w", client.ObjectKeyFromObject(secret), err)
			}

			return nil
		},
	)
}

func (c *controlPlaneBootstrap) WorkerPoolNameToOperatingSystemConfigsMap() map[string]*OperatingSystemConfigs {
	return map[string]*OperatingSystemConfigs{
		c.values.Worker.Name: {
			Init: Data{
				SecretName: ptr.To(c.osc.Status.CloudConfig.SecretRef.Name),
			},
		},
	}
}
