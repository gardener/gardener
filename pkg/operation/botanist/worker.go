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
	"github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/pkg/utils/secrets"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	for _, worker := range b.Shoot.GetWorkers() {
		var volume *extensionsv1alpha1.Volume
		ok, volumeType, volumeSize, err := b.Shoot.GetWorkerVolumesByName(worker.Name)
		if err != nil {
			return fmt.Errorf("could not find worker volume information for pool %q: %+v", worker.Name, err)
		}
		if ok {
			volume = &extensionsv1alpha1.Volume{
				Type: volumeType,
				Size: volumeSize,
			}
		}

		machineImage := worker.MachineImage
		if machineImage == nil {
			machineImage = b.Shoot.GetDefaultMachineImage()
		}

		pools = append(pools, extensionsv1alpha1.WorkerPool{
			Name:           worker.Name,
			Minimum:        worker.AutoScalerMin,
			Maximum:        worker.AutoScalerMax,
			MaxSurge:       *worker.MaxSurge,
			MaxUnavailable: *worker.MaxUnavailable,
			Annotations:    worker.Annotations,
			Labels:         worker.Labels,
			Taints:         worker.Taints,
			MachineType:    worker.MachineType,
			MachineImage: extensionsv1alpha1.MachineImage{
				Name:    string(machineImage.Name),
				Version: machineImage.Version,
			},
			UserData: []byte(b.Shoot.OperatingSystemConfigsMap[worker.Name].Downloader.Data.Content),
			Volume:   volume,
			Zones:    b.Shoot.GetZones(),
		})
	}

	return kutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), worker, func() error {
		metav1.SetMetaDataAnnotation(&worker.ObjectMeta, gardencorev1alpha1.GardenerOperation, gardencorev1alpha1.GardenerOperationReconcile)

		worker.Spec = extensionsv1alpha1.WorkerSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: string(b.Shoot.CloudProvider),
			},
			Region: b.Shoot.Info.Spec.Cloud.Region,
			SecretRef: corev1.SecretReference{
				Name:      gardencorev1alpha1.SecretNameCloudProvider,
				Namespace: worker.Namespace,
			},
			SSHPublicKey: b.Secrets[gardencorev1alpha1.SecretNameSSHKeyPair].Data[secrets.DataKeySSHAuthorizedKeys],
			InfrastructureProviderStatus: &runtime.RawExtension{
				Raw: b.Shoot.InfrastructureStatus,
			},
			Pools: pools,
		}
		return nil
	})
}

// DestroyWorker deletes the `Worker` extension resource in the shoot namespace in the seed cluster,
// and it waits for a maximum of 5m until it is deleted.
func (b *Botanist) DestroyWorker(ctx context.Context) error {
	if err := b.K8sSeedClient.Client().Delete(ctx, &extensionsv1alpha1.Worker{ObjectMeta: metav1.ObjectMeta{Namespace: b.Shoot.SeedNamespace, Name: b.Shoot.Info.Name}}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
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
		return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("Error while waiting for worker object to become ready: %v", err))
	}
	return nil
}

// WaitUntilWorkerDeleted waits until the worker extension resource has been deleted.
func (b *Botanist) WaitUntilWorkerDeleted(ctx context.Context) error {
	var lastError *gardencorev1alpha1.LastError

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
		return retry.MinorError(common.WrapWithLastError(fmt.Errorf("worker is still present"), lastError))
	}); err != nil {
		message := fmt.Sprintf("Error while waiting for worker object to be deleted")
		if lastError != nil {
			return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("%s: %s", message, lastError.Description))
		}
		return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("%s: %s", message, err.Error()))
	}

	return nil
}
