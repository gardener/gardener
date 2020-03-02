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
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/pkg/utils/secrets"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// InfrastructureDefaultTimeout is the default timeout and defines how long Gardener should wait
// for a successful reconciliation of an infrastructure resource.
const InfrastructureDefaultTimeout = 5 * time.Minute

// DeployInfrastructure creates the `Infrastructure` extension resource in the shoot namespace in the seed
// cluster. Gardener waits until an external controller did reconcile the cluster successfully.
func (b *Botanist) DeployInfrastructure(ctx context.Context) error {
	var (
		lastOperation                       = b.Shoot.Info.Status.LastOperation
		creationPhase                       = lastOperation != nil && lastOperation.Type == gardencorev1beta1.LastOperationTypeCreate
		requestInfrastructureReconciliation = creationPhase || controllerutils.HasTask(b.Shoot.Info.Annotations, common.ShootTaskDeployInfrastructure)

		infrastructure = &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Name:      b.Shoot.Info.Name,
				Namespace: b.Shoot.SeedNamespace,
			},
		}
		providerConfig *runtime.RawExtension
	)

	if cfg := b.Shoot.Info.Spec.Provider.InfrastructureConfig; cfg != nil {
		providerConfig = &runtime.RawExtension{
			Raw: cfg.Raw,
		}
	}

	_, err := controllerutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), infrastructure, func() error {
		if requestInfrastructureReconciliation {
			metav1.SetMetaDataAnnotation(&infrastructure.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		}

		infrastructure.Spec = extensionsv1alpha1.InfrastructureSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type:           b.Shoot.Info.Spec.Provider.Type,
				ProviderConfig: providerConfig,
			},
			Region:       b.Shoot.Info.Spec.Region,
			SSHPublicKey: b.Secrets[v1beta1constants.SecretNameSSHKeyPair].Data[secrets.DataKeySSHAuthorizedKeys],
			SecretRef: corev1.SecretReference{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: infrastructure.Namespace,
			},
		}
		return nil
	})
	return err
}

// DestroyInfrastructure deletes the `Infrastructure` extension resource in the shoot namespace in the seed cluster,
// and it waits for a maximum of 10m until it is deleted.
func (b *Botanist) DestroyInfrastructure(ctx context.Context) error {
	if err := b.K8sSeedClient.Client().Delete(ctx, &extensionsv1alpha1.Infrastructure{ObjectMeta: metav1.ObjectMeta{Namespace: b.Shoot.SeedNamespace, Name: b.Shoot.Info.Name}}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// WaitUntilInfrastructureReady waits until the infrastructure resource has been reconciled successfully.
func (b *Botanist) WaitUntilInfrastructureReady(ctx context.Context) error {
	if err := retry.UntilTimeout(ctx, DefaultInterval, InfrastructureDefaultTimeout, func(ctx context.Context) (bool, error) {
		infrastructure := &extensionsv1alpha1.Infrastructure{}
		if err := b.K8sSeedClient.Client().Get(ctx, client.ObjectKey{Name: b.Shoot.Info.Name, Namespace: b.Shoot.SeedNamespace}, infrastructure); err != nil {
			return retry.SevereError(err)
		}

		if err := health.CheckExtensionObject(infrastructure); err != nil {
			b.Logger.WithError(err).Error("Infrastructure did not get ready yet")
			return retry.MinorError(err)
		}

		if infrastructure.Status.ProviderStatus != nil {
			b.Shoot.InfrastructureStatus = infrastructure.Status.ProviderStatus.Raw
		}
		if infrastructure.Status.NodesCIDR != nil {
			shootCopy := b.Shoot.Info.DeepCopy()
			if _, err := controllerutil.CreateOrUpdate(ctx, b.K8sGardenClient.Client(), shootCopy, func() error {
				shootCopy.Spec.Networking.Nodes = infrastructure.Status.NodesCIDR
				return nil
			}); err != nil {
				return retry.SevereError(err)
			}
			b.Shoot.Info = shootCopy
		}

		return retry.Ok()
	}); err != nil {
		return gardencorev1beta1helper.DetermineError(fmt.Sprintf("failed to create infrastructure: %v", err))
	}
	return nil
}

// WaitUntilInfrastructureDeleted waits until the infrastructure resource has been deleted.
func (b *Botanist) WaitUntilInfrastructureDeleted(ctx context.Context) error {
	var lastError *gardencorev1beta1.LastError

	if err := retry.UntilTimeout(ctx, DefaultInterval, InfrastructureDefaultTimeout, func(ctx context.Context) (bool, error) {
		infrastructure := &extensionsv1alpha1.Infrastructure{}
		if err := b.K8sSeedClient.Client().Get(ctx, client.ObjectKey{Name: b.Shoot.Info.Name, Namespace: b.Shoot.SeedNamespace}, infrastructure); err != nil {
			if apierrors.IsNotFound(err) {
				return retry.Ok()
			}
			return retry.SevereError(err)
		}

		if lastErr := infrastructure.Status.LastError; lastErr != nil {
			b.Logger.Errorf("Infrastructure did not get deleted yet, lastError is: %s", lastErr.Description)
			lastError = lastErr
		}

		b.Logger.Infof("Waiting for infrastructure to be deleted...")
		return retry.MinorError(gardencorev1beta1helper.WrapWithLastError(fmt.Errorf("infrastructure is still present"), lastError))
	}); err != nil {
		message := fmt.Sprintf("Failed to delete infrastructure")
		if lastError != nil {
			return gardencorev1beta1helper.DetermineError(fmt.Sprintf("%s: %s", message, lastError.Description))
		}
		return gardencorev1beta1helper.DetermineError(fmt.Sprintf("%s: %s", message, err.Error()))
	}

	return nil
}
