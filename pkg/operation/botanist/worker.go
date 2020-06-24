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
	"strconv"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/secrets"
	versionutils "github.com/gardener/gardener/pkg/utils/version"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// WorkerDefaultTimeout is the default timeout and defines how long Gardener should wait
// for a successful reconciliation of a worker resource.
const WorkerDefaultTimeout = 10 * time.Minute

// DeployWorker creates the `Worker` extension resource in the shoot namespace in the seed
// cluster. Gardener waits until an external controller did reconcile the resource successfully.
func (b *Botanist) DeployWorker(ctx context.Context) error {
	var (
		operation    = v1beta1constants.GardenerOperationReconcile
		restorePhase = b.isRestorePhase()
		worker       = &extensionsv1alpha1.Worker{
			ObjectMeta: metav1.ObjectMeta{
				Name:      b.Shoot.Info.Name,
				Namespace: b.Shoot.SeedNamespace,
			},
		}

		pools []extensionsv1alpha1.WorkerPool
	)

	k8sVersionLessThan115, err := versionutils.CompareVersions(b.Shoot.Info.Spec.Kubernetes.Version, "<", "1.15")
	if err != nil {
		return err
	}

	for _, workerPool := range b.Shoot.Info.Spec.Provider.Workers {
		var volume *extensionsv1alpha1.Volume
		if workerPool.Volume != nil {
			volume = &extensionsv1alpha1.Volume{
				Name:      workerPool.Volume.Name,
				Type:      workerPool.Volume.Type,
				Size:      workerPool.Volume.VolumeSize,
				Encrypted: workerPool.Volume.Encrypted,
			}
		}

		var dataVolumes []extensionsv1alpha1.DataVolume
		if len(workerPool.DataVolumes) > 0 {
			for _, dataVolume := range workerPool.DataVolumes {
				dataVolumes = append(dataVolumes, extensionsv1alpha1.DataVolume{
					Name:      dataVolume.Name,
					Type:      dataVolume.Type,
					Size:      dataVolume.VolumeSize,
					Encrypted: dataVolume.Encrypted,
				})
			}
		}

		if workerPool.Labels == nil {
			workerPool.Labels = map[string]string{}
		}

		// k8s node role labels
		if k8sVersionLessThan115 {
			workerPool.Labels["kubernetes.io/role"] = "node"
			workerPool.Labels["node-role.kubernetes.io/node"] = ""
		} else {
			workerPool.Labels["node.kubernetes.io/role"] = "node"
		}

		if v1beta1helper.SystemComponentsAllowed(&workerPool) {
			workerPool.Labels[v1beta1constants.LabelWorkerPoolSystemComponents] = strconv.FormatBool(workerPool.SystemComponents.Allow)
		}

		// worker pool name labels
		workerPool.Labels[v1beta1constants.LabelWorkerPool] = workerPool.Name
		workerPool.Labels[v1beta1constants.LabelWorkerPoolDeprecated] = workerPool.Name

		// add CRI labels selected by the RuntimeClass
		if workerPool.CRI != nil {
			workerPool.Labels[extensionsv1alpha1.CRINameWorkerLabel] = string(workerPool.CRI.Name)
			if len(workerPool.CRI.ContainerRuntimes) > 0 {
				for _, cr := range workerPool.CRI.ContainerRuntimes {
					key := fmt.Sprintf(extensionsv1alpha1.ContainerRuntimeNameWorkerLabel, cr.Type)
					workerPool.Labels[key] = "true"
				}
			}
		}

		var pConfig *runtime.RawExtension
		if workerPool.ProviderConfig != nil {
			pConfig = &runtime.RawExtension{
				Raw: workerPool.ProviderConfig.Raw,
			}
		}

		pools = append(pools, extensionsv1alpha1.WorkerPool{
			Name:           workerPool.Name,
			Minimum:        workerPool.Minimum,
			Maximum:        workerPool.Maximum,
			MaxSurge:       *workerPool.MaxSurge,
			MaxUnavailable: *workerPool.MaxUnavailable,
			Annotations:    workerPool.Annotations,
			Labels:         workerPool.Labels,
			Taints:         workerPool.Taints,
			MachineType:    workerPool.Machine.Type,
			MachineImage: extensionsv1alpha1.MachineImage{
				Name:    workerPool.Machine.Image.Name,
				Version: *workerPool.Machine.Image.Version,
			},
			ProviderConfig:        pConfig,
			UserData:              []byte(b.Shoot.OperatingSystemConfigsMap[workerPool.Name].Downloader.Data.Content),
			Volume:                volume,
			DataVolumes:           dataVolumes,
			KubeletDataVolumeName: workerPool.KubeletDataVolumeName,
			Zones:                 workerPool.Zones,
		})
	}

	if restorePhase {
		operation = v1beta1constants.GardenerOperationWaitForState
	}

	_, err = controllerutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), worker, func() error {
		metav1.SetMetaDataAnnotation(&worker.ObjectMeta, v1beta1constants.GardenerOperation, operation)
		metav1.SetMetaDataAnnotation(&worker.ObjectMeta, v1beta1constants.GardenerTimestamp, time.Now().UTC().String())
		worker.Spec = extensionsv1alpha1.WorkerSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: b.Shoot.Info.Spec.Provider.Type,
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
	if err != nil {
		return err
	}

	if restorePhase {
		return b.restoreExtensionObject(ctx, b.K8sSeedClient.Client(), worker, extensionsv1alpha1.WorkerResource)
	}

	return nil
}

// DestroyWorker deletes the `Worker` extension resource in the shoot namespace in the seed cluster,
// and it waits for a maximum of 5m until it is deleted.
func (b *Botanist) DestroyWorker(ctx context.Context) error {
	return common.DeleteExtensionCR(
		ctx,
		b.K8sSeedClient.Client(),
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Worker{} },
		b.Shoot.SeedNamespace,
		b.Shoot.Info.Name,
	)
}

// WaitUntilWorkerReady waits until the worker extension resource has been successfully reconciled.
func (b *Botanist) WaitUntilWorkerReady(ctx context.Context) error {
	return common.WaitUntilExtensionCRReady(
		ctx,
		b.K8sSeedClient.DirectClient(),
		b.Logger,
		func() runtime.Object { return &extensionsv1alpha1.Worker{} },
		"Worker",
		b.Shoot.SeedNamespace,
		b.Shoot.Info.Name,
		DefaultInterval,
		DefaultSevereThreshold,
		WorkerDefaultTimeout,
		func(obj runtime.Object) error {
			worker, ok := obj.(*extensionsv1alpha1.Worker)
			if !ok {
				return fmt.Errorf("expected extensionsv1alpha1.Worker but got %T", obj)
			}

			b.Shoot.MachineDeployments = worker.Status.MachineDeployments
			return nil
		},
	)
}

// WaitUntilWorkerDeleted waits until the worker extension resource has been deleted.
func (b *Botanist) WaitUntilWorkerDeleted(ctx context.Context) error {
	return common.WaitUntilExtensionCRDeleted(
		ctx,
		b.K8sSeedClient.Client(),
		b.Logger,
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Worker{} },
		"Worker",
		b.Shoot.SeedNamespace,
		b.Shoot.Info.Name,
		DefaultInterval,
		WorkerDefaultTimeout,
	)
}
