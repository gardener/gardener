// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/common"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// NetworkDefaultTimeout is the default timeout and defines how long Gardener should wait
// for a successful reconciliation of a network resource.
const NetworkDefaultTimeout = 3 * time.Minute

// DeployNetwork creates the `Network` extension resource in the shoot namespace in the seed
// cluster. Gardener waits until an external controller did reconcile the cluster successfully.
func (b *Botanist) DeployNetwork(ctx context.Context) error {
	var (
		restorePhase = b.isRestorePhase()
		operation    = v1beta1constants.GardenerOperationReconcile
		network      = &extensionsv1alpha1.Network{
			ObjectMeta: metav1.ObjectMeta{
				Name:      b.Shoot.Info.Name,
				Namespace: b.Shoot.SeedNamespace,
			},
		}
	)

	if restorePhase {
		operation = v1beta1constants.GardenerOperationWaitForState
	}

	_, err := controllerutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), network, func() error {
		metav1.SetMetaDataAnnotation(&network.ObjectMeta, v1beta1constants.GardenerOperation, operation)
		metav1.SetMetaDataAnnotation(&network.ObjectMeta, v1beta1constants.GardenerTimestamp, time.Now().UTC().String())

		network.Spec = extensionsv1alpha1.NetworkSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: string(b.Shoot.Info.Spec.Networking.Type),
			},
			PodCIDR:     b.Shoot.Networks.Pods.String(),
			ServiceCIDR: b.Shoot.Networks.Services.String(),
		}

		if b.Shoot.Info.Spec.Networking.ProviderConfig != nil {
			network.Spec.ProviderConfig = &b.Shoot.Info.Spec.Networking.ProviderConfig.RawExtension
		}

		return nil
	})
	if err != nil {
		return err
	}

	if restorePhase {
		return b.restoreExtensionObject(ctx, b.K8sSeedClient.Client(), network, &network.ObjectMeta, &network.Status.DefaultStatus, extensionsv1alpha1.NetworkResource, network.Name, network.GetExtensionSpec().GetExtensionPurpose())
	}

	return nil
}

// DestroyNetwork deletes the `Network` extension resource in the shoot namespace in the seed cluster,
// and it waits for a maximum of 10m until it is deleted.
func (b *Botanist) DestroyNetwork(ctx context.Context) error {
	return common.DeleteExtensionCR(
		ctx,
		b.K8sSeedClient.Client(),
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Network{} },
		b.Shoot.SeedNamespace,
		b.Shoot.Info.Name,
	)
}

// WaitUntilNetworkIsReady waits until the network resource has been reconciled successfully.
func (b *Botanist) WaitUntilNetworkIsReady(ctx context.Context) error {
	return common.WaitUntilExtensionCRReady(
		ctx,
		b.K8sSeedClient.Client(),
		b.Logger,
		func() runtime.Object { return &extensionsv1alpha1.Network{} },
		"Network",
		b.Shoot.SeedNamespace,
		b.Shoot.Info.Name,
		DefaultInterval,
		DefaultSevereThreshold,
		NetworkDefaultTimeout,
		nil,
	)
}

// WaitUntilNetworkIsDeleted waits until the Network resource has been deleted.
func (b *Botanist) WaitUntilNetworkIsDeleted(ctx context.Context) error {
	return common.WaitUntilExtensionCRDeleted(
		ctx,
		b.K8sSeedClient.Client(),
		b.Logger,
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Network{} },
		"Network",
		b.Shoot.SeedNamespace,
		b.Shoot.Info.Name,
		DefaultInterval,
		NetworkDefaultTimeout,
	)
}
