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

package infrastructure

import (
	"context"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/operation/botanist/component"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// DefaultInterval is the default interval for retry operations.
	DefaultInterval = 10 * time.Second
	// DefaultSevereThreshold is the default threshold until an error reported by another component is treated as 'severe'.
	DefaultSevereThreshold = 3 * time.Minute
	// DefaultTimeout is the default timeout and defines how long Gardener should wait
	// for a successful reconciliation of an infrastructure resource.
	DefaultTimeout = 10 * time.Minute
)

// TimeNow returns the current time. Exposed for testing.
var TimeNow = time.Now

// Interface is an interface for managing Infrastructures.
type Interface interface {
	component.DeployMigrateWaiter
	// Get retrieves and returns the Infrastructure resources based on the configured values.
	Get(context.Context) (*extensionsv1alpha1.Infrastructure, error)
	// SetSSHPublicKey sets the SSH public key in the values.
	SetSSHPublicKey([]byte)
	// ProviderStatus returns the generated status of the provider.
	ProviderStatus() *runtime.RawExtension
	// NodesCIDR returns the generated nodes CIDR of the provider.
	NodesCIDR() *string
}

// Values contains the values used to create an Infrastructure resources.
type Values struct {
	// Namespace is the Shoot namespace in the seed.
	Namespace string
	// Name is the name of the Infrastructure resource. Commonly the Shoot's name.
	Name string
	// Type is the type of infrastructure provider.
	Type string
	// ProviderConfig contains the provider config for the Infrastructure provider.
	ProviderConfig *runtime.RawExtension
	// Region is the region of the shoot.
	Region string
	// SSHPublicKey is the to-be-used SSH public key of the shoot.
	SSHPublicKey []byte
	// AnnotateOperation indicates if the Infrastructure resource shall be annotated with the
	// respective "gardener.cloud/operation" (forcing a reconciliation or restoration). If this is false
	// then the Infrastructure object will be created/updated but the extension controller will not
	// act upon it.
	AnnotateOperation bool
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
	return &infrastructure{
		log:                 log,
		client:              client,
		values:              values,
		waitInterval:        waitInterval,
		waitSevereThreshold: waitSevereThreshold,
		waitTimeout:         waitTimeout,

		infrastructure: &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Name:      values.Name,
				Namespace: values.Namespace,
			},
		},
	}
}

type infrastructure struct {
	values              *Values
	log                 logr.Logger
	client              client.Client
	waitInterval        time.Duration
	waitSevereThreshold time.Duration
	waitTimeout         time.Duration

	infrastructure *extensionsv1alpha1.Infrastructure
	providerStatus *runtime.RawExtension
	nodesCIDR      *string
}

// Deploy uses the seed client to create or update the Infrastructure resource.
func (i *infrastructure) Deploy(ctx context.Context) error {
	_, err := i.deploy(ctx, v1beta1constants.GardenerOperationReconcile)
	return err
}

func (i *infrastructure) deploy(ctx context.Context, operation string) (extensionsv1alpha1.Object, error) {
	var providerConfig *runtime.RawExtension
	if cfg := i.values.ProviderConfig; cfg != nil {
		providerConfig = &runtime.RawExtension{
			Raw: cfg.Raw,
		}
	}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, i.client, i.infrastructure, func() error {
		if i.values.AnnotateOperation {
			metav1.SetMetaDataAnnotation(&i.infrastructure.ObjectMeta, v1beta1constants.GardenerOperation, operation)
		}

		metav1.SetMetaDataAnnotation(&i.infrastructure.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().String())

		i.infrastructure.Spec = extensionsv1alpha1.InfrastructureSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type:           i.values.Type,
				ProviderConfig: providerConfig,
			},
			Region:       i.values.Region,
			SSHPublicKey: i.values.SSHPublicKey,
			SecretRef: corev1.SecretReference{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: i.infrastructure.Namespace,
			},
		}
		return nil
	})

	return i.infrastructure, err
}

// Restore uses the seed client and the ShootState to create the Infrastructure resources and restore their state.
func (i *infrastructure) Restore(ctx context.Context, shootState *gardencorev1alpha1.ShootState) error {
	return extensions.RestoreExtensionWithDeployFunction(
		ctx,
		i.client,
		shootState,
		extensionsv1alpha1.InfrastructureResource,
		i.deploy,
	)
}

// Migrate migrates the Infrastructure resources.
func (i *infrastructure) Migrate(ctx context.Context) error {
	return extensions.MigrateExtensionObject(
		ctx,
		i.client,
		i.infrastructure,
	)
}

// Destroy deletes the Infrastructure resource.
func (i *infrastructure) Destroy(ctx context.Context) error {
	return extensions.DeleteExtensionObject(
		ctx,
		i.client,
		i.infrastructure,
	)
}

// Wait waits until the Infrastructure resource is ready.
func (i *infrastructure) Wait(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectReady(
		ctx,
		i.client,
		i.log,
		i.infrastructure,
		extensionsv1alpha1.InfrastructureResource,
		i.waitInterval,
		i.waitSevereThreshold,
		i.waitTimeout,
		func() error {
			i.extractStatus(i.infrastructure.Status)
			return nil
		})
}

// WaitMigrate waits until the Infrastructure resources are migrated successfully.
func (i *infrastructure) WaitMigrate(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectMigrated(
		ctx,
		i.client,
		i.infrastructure,
		extensionsv1alpha1.InfrastructureResource,
		i.waitInterval,
		i.waitTimeout,
	)
}

// WaitCleanup waits until the Infrastructure resource is deleted.
func (i *infrastructure) WaitCleanup(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectDeleted(
		ctx,
		i.client,
		i.log,
		i.infrastructure,
		extensionsv1alpha1.InfrastructureResource,
		i.waitInterval,
		i.waitTimeout,
	)
}

// Get retrieves and returns the Infrastructure resources based on the configured values.
func (i *infrastructure) Get(ctx context.Context) (*extensionsv1alpha1.Infrastructure, error) {
	if err := i.client.Get(ctx, client.ObjectKeyFromObject(i.infrastructure), i.infrastructure); err != nil {
		return nil, err
	}

	i.extractStatus(i.infrastructure.Status)
	return i.infrastructure, nil
}

// SetSSHPublicKey sets the SSH public key in the values.
func (i *infrastructure) SetSSHPublicKey(key []byte) {
	i.values.SSHPublicKey = key
}

// ProviderStatus returns the generated status of the provider.
func (i *infrastructure) ProviderStatus() *runtime.RawExtension {
	return i.providerStatus
}

// NodesCIDR returns the generated nodes CIDR of the provider.
func (i *infrastructure) NodesCIDR() *string {
	return i.nodesCIDR
}

func (i *infrastructure) extractStatus(status extensionsv1alpha1.InfrastructureStatus) {
	i.providerStatus = status.ProviderStatus
	i.nodesCIDR = status.NodesCIDR
}
