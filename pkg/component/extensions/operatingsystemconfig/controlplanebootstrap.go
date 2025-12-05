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
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

type controlPlaneBootstrap struct {
	// This type only implements a subset of Interface. Embed this to spare the panic stubs.
	Interface

	log            logr.Logger
	client         client.Client
	secretsManager secretsmanager.Interface
	values         *ControlPlaneBootstrapValues

	waitInterval        time.Duration
	waitSevereThreshold time.Duration
	waitTimeout         time.Duration

	osc Data
}

// ControlPlaneBootstrapValues contains the values used to create an OperatingSystemConfig resource for gardenadm bootstrap.
type ControlPlaneBootstrapValues struct {
	*Values

	// Worker is the control plane worker pool.
	Worker *gardencorev1beta1.Worker
	// GardenadmImage is the gardenadm image reference that should be pulled.
	GardenadmImage string
}

// NewControlPlaneBootstrap creates a new instance of Interface for deploying OperatingSystemConfigs during gardenadm bootstrap.
func NewControlPlaneBootstrap(
	log logr.Logger,
	client client.Client,
	secretsManager secretsmanager.Interface,
	values *ControlPlaneBootstrapValues,
	waitInterval, waitSevereThreshold, waitTimeout time.Duration,
) Interface {
	return &controlPlaneBootstrap{
		log:            log,
		client:         client,
		secretsManager: secretsManager,
		values:         values,

		waitInterval:        waitInterval,
		waitSevereThreshold: waitSevereThreshold,
		waitTimeout:         waitTimeout,

		osc: Data{
			Object: &extensionsv1alpha1.OperatingSystemConfig{ObjectMeta: metav1.ObjectMeta{
				Name:      "gardenadm-" + values.Worker.Name,
				Namespace: values.Namespace,
			}},
			// Ensure that the secret name is included in the worker pool mapping, so that the worker pool hash in the
			// self-hosted shoot will result in the same hash.
			// Without the NodeAgentSecretName, the WorkerPoolHash func will always use hash version v1, although we used v2
			// in the bootstrap cluster.
			IncludeSecretNameInWorkerPool: true,
		},
	}
}

func (c *controlPlaneBootstrap) Deploy(ctx context.Context) error {
	oscKey, err := calculateKeyForValues(LatestHashVersion(), c.values.Values, c.values.Worker)
	if err != nil {
		return err
	}
	c.osc.GardenerNodeAgentSecretName = oscKey

	sshKeypairSecret, found := c.secretsManager.Get(v1beta1constants.SecretNameSSHKeyPair)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameSSHKeyPair)
	}

	units, files, err := nodeinit.GardenadmConfig(c.values.GardenadmImage, string(sshKeypairSecret.Data[secretsutils.DataKeySSHAuthorizedKeys]))
	if err != nil {
		return err
	}

	_, err = controllerutils.GetAndCreateOrMergePatch(ctx, c.client, c.osc.Object, func() error {
		metav1.SetMetaDataAnnotation(&c.osc.Object.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		metav1.SetMetaDataAnnotation(&c.osc.Object.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().Format(time.RFC3339Nano))

		c.osc.Object.Spec = extensionsv1alpha1.OperatingSystemConfigSpec{
			Purpose: extensionsv1alpha1.OperatingSystemConfigPurposeProvision,
			Units:   units,
			Files:   files,
		}

		// TODO(timebertt): ensure Worker.Machine.Image is set in Shoot validation for `gardenadm bootstrap`
		c.osc.Object.Spec.Type = c.values.Worker.Machine.Image.Name
		c.osc.Object.Spec.ProviderConfig = c.values.Worker.Machine.Image.ProviderConfig

		return nil
	})
	return err
}

func (c *controlPlaneBootstrap) Wait(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectReady(
		ctx,
		c.client,
		c.log,
		c.osc.Object,
		extensionsv1alpha1.OperatingSystemConfigResource,
		c.waitInterval,
		c.waitSevereThreshold,
		c.waitTimeout,
		func(ctx context.Context) error {
			if c.osc.Object.Status.CloudConfig == nil {
				return fmt.Errorf("no cloud config information provided in status")
			}

			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Namespace: c.osc.Object.Status.CloudConfig.SecretRef.Namespace,
				Name:      c.osc.Object.Status.CloudConfig.SecretRef.Name,
			}}
			if err := c.client.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
				return fmt.Errorf("failed getting cloud config secret %q: %w", client.ObjectKeyFromObject(secret), err)
			}

			c.osc.SecretName = ptr.To(c.osc.Object.Status.CloudConfig.SecretRef.Name)
			return nil
		},
	)
}

func (c *controlPlaneBootstrap) WorkerPoolNameToOperatingSystemConfigsMap() map[string]*OperatingSystemConfigs {
	return map[string]*OperatingSystemConfigs{
		c.values.Worker.Name: {
			Init: c.osc,
		},
	}
}
