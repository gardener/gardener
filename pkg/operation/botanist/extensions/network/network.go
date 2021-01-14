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

package network

import (
	"context"
	"net"
	"time"

	"github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/common"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// DefaultInterval is the default interval for retry operations.
	DefaultInterval = 5 * time.Second
	// DefaultSevereThreshold is the default threshold until an error reported by another component is treated as 'severe'.
	DefaultSevereThreshold = 30 * time.Second
	// DefaultTimeout is the default timeout and defines how long Gardener should wait
	// for a successful reconciliation of a network resource.
	DefaultTimeout = 3 * time.Minute
)

// TimeNow returns the current time. Exposed for testing.
var TimeNow = time.Now

// Values contains the values used to create a Network CRD
type Values struct {
	// Namespace is the namespace of the Shoot network in the Seed
	Namespace string
	// Name is the name of the Network extension. Commonly the Shoot's name.
	Name string
	// Type is the type of Network plugin/extension (e.g calico)
	Type string
	// ProviderConfig contains the provider config for the Network extension.
	ProviderConfig *runtime.RawExtension
	// PodCIDR is the Shoot's pod CIDR in the Shoot VPC
	PodCIDR *net.IPNet
	// ServiceCIDR is the Shoot's service CIDR in the Shoot VPC
	ServiceCIDR *net.IPNet
}

// New creates a new instance of DeployWaiter for a Network.
func New(
	logger logrus.FieldLogger,
	client client.Client,
	values *Values,
	waitInterval time.Duration,
	waitSevereThreshold time.Duration,
	waitTimeout time.Duration,
) component.DeployMigrateWaiter {
	return &network{
		client:              client,
		logger:              logger,
		values:              values,
		waitInterval:        waitInterval,
		waitSevereThreshold: waitSevereThreshold,
		waitTimeout:         waitTimeout,
	}
}

type network struct {
	values              *Values
	logger              logrus.FieldLogger
	client              client.Client
	waitInterval        time.Duration
	waitSevereThreshold time.Duration
	waitTimeout         time.Duration
}

// Deploy uses the seed client to create or update the Network custom resource in the Shoot namespace in the Seed
func (d *network) Deploy(ctx context.Context) error {
	_, err := d.internalDeploy(ctx, v1beta1constants.GardenerOperationReconcile)
	return err
}

// Restore uses the seed client and the ShootState to create the Network custom resource in the Shoot namespace in the Seed and restore its state
func (d *network) Restore(ctx context.Context, shootState *v1alpha1.ShootState) error {
	return common.RestoreExtensionWithDeployFunction(ctx, shootState, d.client, extensionsv1alpha1.NetworkResource, d.values.Namespace, d.internalDeploy)
}

// Migrate migrates the Network custom resource
func (d *network) Migrate(ctx context.Context) error {
	return common.MigrateExtensionCR(
		ctx,
		d.client,
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Network{} },
		d.values.Namespace,
		d.values.Name,
	)
}

// WaitMigrate waits until the Network custom resource has been successfully migrated.
func (d *network) WaitMigrate(ctx context.Context) error {
	return common.WaitUntilExtensionCRMigrated(
		ctx,
		d.client,
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Network{} },
		d.values.Namespace,
		d.values.Name,
		d.waitInterval,
		d.waitTimeout,
	)
}

// Destroy deletes the Network CRD
func (d *network) Destroy(ctx context.Context) error {
	return common.DeleteExtensionCR(
		ctx,
		d.client,
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Network{} },
		d.values.Namespace,
		d.values.Name,
	)
}

// Wait waits until the Network CRD is ready (deployed or restored)
func (d *network) Wait(ctx context.Context) error {
	return common.WaitUntilExtensionCRReady(
		ctx,
		d.client,
		d.logger,
		func() client.Object { return &extensionsv1alpha1.Network{} },
		extensionsv1alpha1.NetworkResource,
		d.values.Namespace,
		d.values.Name,
		d.waitInterval,
		d.waitSevereThreshold,
		d.waitTimeout,
		nil,
	)
}

// WaitCleanup waits until the Network CRD is deleted
func (d *network) WaitCleanup(ctx context.Context) error {
	return common.WaitUntilExtensionCRDeleted(
		ctx,
		d.client,
		d.logger,
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Network{} },
		extensionsv1alpha1.NetworkResource,
		d.values.Namespace,
		d.values.Name,
		d.waitInterval,
		d.waitTimeout,
	)
}

func (d *network) internalDeploy(ctx context.Context, operation string) (extensionsv1alpha1.Object, error) {
	var (
		network = &extensionsv1alpha1.Network{
			ObjectMeta: metav1.ObjectMeta{
				Name:      d.values.Name,
				Namespace: d.values.Namespace,
			},
		}
	)

	_, err := controllerutil.CreateOrUpdate(ctx, d.client, network, func() error {
		metav1.SetMetaDataAnnotation(&network.ObjectMeta, v1beta1constants.GardenerOperation, operation)
		metav1.SetMetaDataAnnotation(&network.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().String())

		network.Spec = extensionsv1alpha1.NetworkSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type:           d.values.Type,
				ProviderConfig: d.values.ProviderConfig,
			},
			PodCIDR:     d.values.PodCIDR.String(),
			ServiceCIDR: d.values.ServiceCIDR.String(),
		}

		return nil
	})

	return network, err
}
