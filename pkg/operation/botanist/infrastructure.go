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
	"github.com/gardener/gardener/pkg/utils/secrets"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// InfrastructureDefaultTimeout is the default timeout and defines how long Gardener should wait
// for a successful reconciliation of an infrastructure resource.
const InfrastructureDefaultTimeout = 5 * time.Minute

// DeployInfrastructure creates the `Infrastructure` extension resource in the shoot namespace in the seed
// cluster. Gardener waits until an external controller did reconcile the cluster successfully.
func (b *Botanist) DeployInfrastructure(ctx context.Context) error {
	var (
		operation                      = v1beta1constants.GardenerOperationReconcile
		lastOperation                  = b.Shoot.Info.Status.LastOperation
		creationPhase                  = lastOperation != nil && lastOperation.Type == gardencorev1beta1.LastOperationTypeCreate
		shootIsWakingUp                = !gardencorev1beta1helper.HibernationIsEnabled(b.Shoot.Info) && b.Shoot.Info.Status.IsHibernated
		restorePhase                   = b.isRestorePhase()
		requestInfrastructureOperation = creationPhase || shootIsWakingUp || restorePhase || controllerutils.HasTask(b.Shoot.Info.Annotations, common.ShootTaskDeployInfrastructure)
		infrastructure                 = &extensionsv1alpha1.Infrastructure{
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

	if restorePhase {
		operation = v1beta1constants.GardenerOperationWaitForState
	}

	_, err := controllerutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), infrastructure, func() error {
		if requestInfrastructureOperation {
			metav1.SetMetaDataAnnotation(&infrastructure.ObjectMeta, v1beta1constants.GardenerOperation, operation)
			metav1.SetMetaDataAnnotation(&infrastructure.ObjectMeta, v1beta1constants.GardenerTimestamp, time.Now().UTC().String())
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
	if err != nil {
		return err
	}

	if restorePhase {
		return b.restoreExtensionObject(ctx, b.K8sSeedClient.Client(), infrastructure, extensionsv1alpha1.InfrastructureResource)
	}

	return nil
}

// DestroyInfrastructure deletes the `Infrastructure` extension resource in the shoot namespace in the seed cluster,
// and it waits for a maximum of 10m until it is deleted.
func (b *Botanist) DestroyInfrastructure(ctx context.Context) error {
	return common.DeleteExtensionCR(
		ctx,
		b.K8sSeedClient.Client(),
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Infrastructure{} },
		b.Shoot.SeedNamespace,
		b.Shoot.Info.Name,
	)
}

// WaitUntilInfrastructureReady waits until the infrastructure resource has been reconciled successfully.
func (b *Botanist) WaitUntilInfrastructureReady(ctx context.Context) error {
	return common.WaitUntilExtensionCRReady(
		ctx,
		b.K8sSeedClient.Client(),
		b.Logger,
		func() runtime.Object { return &extensionsv1alpha1.Infrastructure{} },
		"Infrastructure",
		b.Shoot.SeedNamespace,
		b.Shoot.Info.Name,
		DefaultInterval,
		DefaultSevereThreshold,
		InfrastructureDefaultTimeout,
		func(obj runtime.Object) error {
			infrastructure, ok := obj.(*extensionsv1alpha1.Infrastructure)
			if !ok {
				return fmt.Errorf("expected extensionsv1alpha1.Infrastructure but got %T", infrastructure)
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
					return err
				}
				b.Shoot.Info = shootCopy
			}
			return nil
		},
	)
}

// WaitUntilInfrastructureDeleted waits until the infrastructure resource has been deleted.
func (b *Botanist) WaitUntilInfrastructureDeleted(ctx context.Context) error {
	return common.WaitUntilExtensionCRDeleted(
		ctx,
		b.K8sSeedClient.Client(),
		b.Logger,
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Infrastructure{} },
		"Infrastructure",
		b.Shoot.SeedNamespace,
		b.Shoot.Info.Name,
		DefaultInterval,
		InfrastructureDefaultTimeout,
	)
}
