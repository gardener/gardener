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

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	controllerutils "github.com/gardener/gardener/pkg/controllermanager/controller/utils"
	"github.com/gardener/gardener/pkg/migration"
	"github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// InfrastructureDefaultTimeout is the default timeout and defines how long Gardener should wait
// for a successful reconciliation of an infrastructure resource.
const InfrastructureDefaultTimeout = 10 * time.Minute

// DeployInfrastructure creates the `Infrastructure` extension resource in the shoot namespace in the seed
// cluster. Gardener waits until an external controller did reconcile the cluster successfully.
func (b *Botanist) DeployInfrastructure(ctx context.Context) error {
	var (
		lastOperation                       = b.Shoot.Info.Status.LastOperation
		creationPhase                       = lastOperation != nil && lastOperation.Type == gardencorev1alpha1.LastOperationTypeCreate
		requestInfrastructureReconciliation = creationPhase || controllerutils.HasTask(b.Shoot.Info.Annotations, common.ShootTaskDeployInfrastructure)

		infrastructure = &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Name:      b.Shoot.Info.Name,
				Namespace: b.Shoot.SeedNamespace,
			},
		}
	)

	// In the future the providerConfig will be blindly copied from the core.gardener.cloud/v1alpha1.Shoot
	// resource. However, until we have completely moved to this resource, we have to compute the needed
	// configuration ourselves from garden.sapcloud.io/v1beta1.Shoot.
	providerConfig, err := migration.ShootToInfrastructureConfig(b.Shoot.Info)
	if err != nil {
		return err
	}

	return kutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), infrastructure, func() error {
		if requestInfrastructureReconciliation {
			metav1.SetMetaDataAnnotation(&infrastructure.ObjectMeta, gardencorev1alpha1.GardenerOperation, gardencorev1alpha1.GardenerOperationReconcile)
		}

		infrastructure.Spec = extensionsv1alpha1.InfrastructureSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: string(b.Shoot.CloudProvider),
			},
			Region:       b.Shoot.Info.Spec.Cloud.Region,
			SSHPublicKey: b.Secrets[gardencorev1alpha1.SecretNameSSHKeyPair].Data[secrets.DataKeySSHAuthorizedKeys],
			SecretRef: corev1.SecretReference{
				Name:      gardencorev1alpha1.SecretNameCloudProvider,
				Namespace: infrastructure.Namespace,
			},
			ProviderConfig: &runtime.RawExtension{
				Object: providerConfig,
			},
		}
		return nil
	})
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
	var (
		timedContext, cancel = context.WithTimeout(ctx, InfrastructureDefaultTimeout)
		lastError            *gardencorev1alpha1.LastError
		infrastructureStatus []byte
	)

	defer cancel()

	if err := wait.PollUntil(5*time.Second, func() (bool, error) {
		infrastructure := &extensionsv1alpha1.Infrastructure{}
		if err := b.K8sSeedClient.Client().Get(ctx, client.ObjectKey{Name: b.Shoot.Info.Name, Namespace: b.Shoot.SeedNamespace}, infrastructure); err != nil {
			return false, err
		}

		if lastErr := infrastructure.Status.LastError; lastErr != nil {
			b.Logger.Errorf("Infrastructure did not get ready yet, lastError is: %s", lastErr.Description)
			lastError = lastErr
		}

		if lastOperation := infrastructure.Status.LastOperation; lastOperation != nil &&
			lastOperation.State == gardencorev1alpha1.LastOperationStateSucceeded &&
			infrastructure.Status.ObservedGeneration == infrastructure.Generation &&
			!metav1.HasAnnotation(infrastructure.ObjectMeta, gardencorev1alpha1.GardenerOperation) {

			if providerStatus := infrastructure.Status.ProviderStatus; providerStatus != nil {
				infrastructureStatus = providerStatus.Raw
			}
			return true, nil
		}

		b.Logger.Infof("Waiting for infrastructure to be ready...")
		return false, nil
	}, timedContext.Done()); err != nil {
		message := fmt.Sprintf("Failed to create infrastructure")
		if lastError != nil {
			return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("%s: %s", message, lastError.Description))
		}
		return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("%s: %s", message, err.Error()))
	}

	b.Shoot.InfrastructureStatus = infrastructureStatus
	return nil
}

// WaitUntilInfrastructureDeleted waits until the infrastructure resource has been deleted.
func (b *Botanist) WaitUntilInfrastructureDeleted(ctx context.Context) error {
	var (
		timedContext, cancel = context.WithTimeout(ctx, InfrastructureDefaultTimeout)
		lastError            *gardencorev1alpha1.LastError
	)

	defer cancel()

	if err := wait.PollUntil(5*time.Second, func() (bool, error) {
		infrastructure := &extensionsv1alpha1.Infrastructure{}
		if err := b.K8sSeedClient.Client().Get(ctx, client.ObjectKey{Name: b.Shoot.Info.Name, Namespace: b.Shoot.SeedNamespace}, infrastructure); err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}

		if lastErr := infrastructure.Status.LastError; lastErr != nil {
			b.Logger.Errorf("Infrastructure did not get deleted yet, lastError is: %s", lastErr.Description)
			lastError = lastErr
		}

		b.Logger.Infof("Waiting for infrastructure to be deleted...")
		return false, nil
	}, timedContext.Done()); err != nil {
		message := fmt.Sprintf("Failed to delete infrastructure")
		if lastError != nil {
			return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("%s: %s", message, lastError.Description))
		}
		return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("%s: %s", message, err.Error()))
	}

	return nil
}
