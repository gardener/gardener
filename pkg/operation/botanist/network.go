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
	"errors"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// NetworkDefaultTimeout is the default timeout and defines how long Gardener should wait
// for a successful reconciliation of a network resource.
const NetworkDefaultTimeout = 3 * time.Minute

// DeployNetwork creates the `Network` extension resource in the shoot namespace in the seed
// cluster. Gardener waits until an external controller did reconcile the cluster successfully.
func (b *Botanist) DeployNetwork(ctx context.Context) error {
	network := &extensionsv1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.Shoot.Info.Name,
			Namespace: b.Shoot.SeedNamespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), network, func() error {
		metav1.SetMetaDataAnnotation(&network.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
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
	return err
}

// DestroyNetwork deletes the `Network` extension resource in the shoot namespace in the seed cluster,
// and it waits for a maximum of 10m until it is deleted.
func (b *Botanist) DestroyNetwork(ctx context.Context) error {
	obj := &extensionsv1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: b.Shoot.SeedNamespace,
			Name:      b.Shoot.Info.Name,
		},
	}

	if err := common.ConfirmDeletion(ctx, b.K8sSeedClient.Client(), obj); err != nil {
		return err
	}

	return client.IgnoreNotFound(b.K8sSeedClient.Client().Delete(ctx, obj))
}

// WaitUntilNetworkIsReady waits until the network resource has been reconciled successfully.
func (b *Botanist) WaitUntilNetworkIsReady(ctx context.Context) error {
	if err := retry.UntilTimeout(ctx, DefaultInterval, NetworkDefaultTimeout, func(ctx context.Context) (bool, error) {
		network := &extensionsv1alpha1.Network{}
		if err := b.K8sSeedClient.Client().Get(ctx, client.ObjectKey{Name: b.Shoot.Info.Name, Namespace: b.Shoot.SeedNamespace}, network); err != nil {
			return retry.SevereError(err)
		}
		if err := health.CheckExtensionObject(network); err != nil {
			b.Logger.WithError(err).Error("Network did not get ready yet")
			return retry.MinorError(err)
		}
		return retry.Ok()
	}); err != nil {
		return gardencorev1beta1helper.DetermineError(err, fmt.Sprintf("failed to create network: %v", err))
	}
	return nil
}

// WaitUntilNetworkIsDeleted waits until the Network resource has been deleted.
func (b *Botanist) WaitUntilNetworkIsDeleted(ctx context.Context) error {
	var lastError *gardencorev1beta1.LastError

	if err := retry.UntilTimeout(ctx, DefaultInterval, NetworkDefaultTimeout, func(ctx context.Context) (bool, error) {
		network := &extensionsv1alpha1.Network{}
		if err := b.K8sSeedClient.Client().Get(ctx, client.ObjectKey{Name: b.Shoot.Info.Name, Namespace: b.Shoot.SeedNamespace}, network); err != nil {
			if apierrors.IsNotFound(err) {
				return retry.Ok()
			}
			return retry.SevereError(err)
		}

		if lastErr := network.Status.LastError; lastErr != nil {
			b.Logger.Errorf("Network did not get deleted yet, lastError is: %s", lastErr.Description)
			lastError = lastErr
		}

		b.Logger.Infof("Waiting for Network to be deleted...")
		return retry.MinorError(gardencorev1beta1helper.WrapWithLastError(fmt.Errorf("network is still present"), lastError))
	}); err != nil {
		message := "Failed to delete Network"
		if lastError != nil {
			return gardencorev1beta1helper.DetermineError(errors.New(lastError.Description), fmt.Sprintf("%s: %s", message, lastError.Description))
		}
		return gardencorev1beta1helper.DetermineError(err, fmt.Sprintf("%s: %s", message, err.Error()))
	}

	return nil
}
