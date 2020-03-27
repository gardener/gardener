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

// WorkerDefaultTimeout is the default timeout and defines how long Gardener should wait
// for a successful reconciliation of a worker resource.
const WorkerDefaultTimeout = 10 * time.Minute

// DeployWorker creates the `Worker` extension resource in the shoot namespace in the seed
// cluster. Gardener waits until an external controller did reconcile the resource successfully.
func (b *Botanist) DeployWorker(ctx context.Context) error {
	var (
		worker = &extensionsv1alpha1.Worker{
			ObjectMeta: metav1.ObjectMeta{
				Name:      b.Shoot.Info.Name,
				Namespace: b.Shoot.SeedNamespace,
			},
		}

		pools []extensionsv1alpha1.WorkerPool
	)

	for _, worker := range b.Shoot.Info.Spec.Provider.Workers {
		var volume *extensionsv1alpha1.Volume
		if worker.Volume != nil {
			volume = &extensionsv1alpha1.Volume{
				Name:      worker.Volume.Name,
				Type:      worker.Volume.Type,
				Size:      worker.Volume.Size,
				Encrypted: worker.Volume.Encrypted,
			}
		}

		var dataVolumes []extensionsv1alpha1.Volume
		if len(worker.DataVolumes) > 0 {
			for _, dataVolume := range worker.DataVolumes {
				dataVolumes = append(dataVolumes, extensionsv1alpha1.Volume{
					Name:      dataVolume.Name,
					Type:      dataVolume.Type,
					Size:      dataVolume.Size,
					Encrypted: dataVolume.Encrypted,
				})
			}
		}

		var pConfig *runtime.RawExtension
		if worker.ProviderConfig != nil {
			pConfig = &runtime.RawExtension{
				Raw: worker.ProviderConfig.Raw,
			}
		}

		pools = append(pools, extensionsv1alpha1.WorkerPool{
			Name:           worker.Name,
			Minimum:        worker.Minimum,
			Maximum:        worker.Maximum,
			MaxSurge:       *worker.MaxSurge,
			MaxUnavailable: *worker.MaxUnavailable,
			Annotations:    worker.Annotations,
			Labels:         worker.Labels,
			Taints:         worker.Taints,
			MachineType:    worker.Machine.Type,
			MachineImage: extensionsv1alpha1.MachineImage{
				Name:    worker.Machine.Image.Name,
				Version: worker.Machine.Image.Version,
			},
			ProviderConfig:        pConfig,
			UserData:              []byte(b.Shoot.OperatingSystemConfigsMap[worker.Name].Downloader.Data.Content),
			Volume:                volume,
			DataVolumes:           dataVolumes,
			KubeletDataVolumeName: worker.KubeletDataVolumeName,
			Zones:                 worker.Zones,
		})
	}

	_, err := controllerutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), worker, func() error {
		metav1.SetMetaDataAnnotation(&worker.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)

		worker.Spec = extensionsv1alpha1.WorkerSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: string(b.Shoot.Info.Spec.Provider.Type),
			},
			Region: b.Shoot.Info.Spec.Region,
			SecretRef: corev1.SecretReference{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: worker.Namespace,
			},
			SSHPublicKey: b.Secrets[v1beta1constants.SecretNameSSHKeyPair].Data[secrets.DataKeySSHAuthorizedKeys],
			InfrastructureProviderStatus: &runtime.RawExtension{
				Raw: b.Shoot.InfrastructureStatus,
			},
			Pools: pools,
		}
		return nil
	})
	return err
}

// DestroyWorker deletes the `Worker` extension resource in the shoot namespace in the seed cluster,
// and it waits for a maximum of 5m until it is deleted.
func (b *Botanist) DestroyWorker(ctx context.Context) error {
	worker := &extensionsv1alpha1.Worker{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: b.Shoot.SeedNamespace,
			Name:      b.Shoot.Info.Name,
		},
	}

	return client.IgnoreNotFound(b.K8sSeedClient.Client().Delete(ctx, worker))
}

// WaitUntilWorkerReady waits until the worker extension resource has been successfully reconciled.
func (b *Botanist) WaitUntilWorkerReady(ctx context.Context) error {
	if err := retry.UntilTimeout(ctx, DefaultInterval, WorkerDefaultTimeout, func(ctx context.Context) (bool, error) {
		worker := &extensionsv1alpha1.Worker{}
		if err := b.K8sSeedClient.Client().Get(ctx, client.ObjectKey{Name: b.Shoot.Info.Name, Namespace: b.Shoot.SeedNamespace}, worker); err != nil {
			return retry.SevereError(err)
		}

		if err := health.CheckExtensionObject(worker); err != nil {
			b.Logger.WithError(err).Error("Worker did not get ready yet")
			return retry.MinorError(err)
		}

		b.Shoot.MachineDeployments = worker.Status.MachineDeployments
		return retry.Ok()
	}); err != nil {
		return gardencorev1beta1helper.DetermineError(err, fmt.Sprintf("Error while waiting for worker object to become ready: %v", err))
	}
	return nil
}

// WaitUntilWorkerDeleted waits until the worker extension resource has been deleted.
func (b *Botanist) WaitUntilWorkerDeleted(ctx context.Context) error {
	var lastError *gardencorev1beta1.LastError

	if err := retry.UntilTimeout(ctx, DefaultInterval, WorkerDefaultTimeout, func(ctx context.Context) (bool, error) {
		worker := &extensionsv1alpha1.Worker{}
		if err := b.K8sSeedClient.Client().Get(ctx, client.ObjectKey{Name: b.Shoot.Info.Name, Namespace: b.Shoot.SeedNamespace}, worker); err != nil {
			if apierrors.IsNotFound(err) {
				return retry.Ok()
			}
			return retry.SevereError(err)
		}

		if lastErr := worker.Status.LastError; lastErr != nil {
			b.Logger.Errorf("Worker did not get deleted yet, lastError is: %s", lastErr.Description)
			lastError = lastErr
		}

		b.Logger.Infof("Waiting for worker to be deleted...")
		return retry.MinorError(gardencorev1beta1helper.WrapWithLastError(fmt.Errorf("worker is still present"), lastError))
	}); err != nil {
		message := fmt.Sprintf("Error while waiting for worker object to be deleted")
		if lastError != nil {
			return gardencorev1beta1helper.DetermineError(errors.New(lastError.Description), fmt.Sprintf("%s: %s", message, lastError.Description))
		}
		return gardencorev1beta1helper.DetermineError(err, fmt.Sprintf("%s: %s", message, err.Error()))
	}

	return nil
}
