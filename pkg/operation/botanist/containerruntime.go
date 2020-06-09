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

package botanist

import (
	"context"
	"fmt"
	"time"

	gardencorev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/flow"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// DeployContainerRuntimeResources creates a `Container runtime` resource in the shoot namespace in the seed
// cluster. Deploys one resource per CRI per Worker.
// Gardener waits until an external controller has reconciled the resources successfully.
func (b *Botanist) DeployContainerRuntimeResources(ctx context.Context) error {
	var fns []flow.TaskFn

	for _, worker := range b.Shoot.Info.Spec.Provider.Workers {
		if worker.CRI == nil {
			continue
		}

		for _, containerRuntime := range worker.CRI.ContainerRuntimes {
			cr := containerRuntime
			workerName := worker.Name

			toApply := extensionsv1alpha1.ContainerRuntime{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getContainerRuntimeKey(cr.Type, workerName),
					Namespace: b.Shoot.SeedNamespace,
				},
			}

			fns = append(fns, func(ctx context.Context) error {
				_, err := controllerutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), &toApply, func() error {
					metav1.SetMetaDataAnnotation(&toApply.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
					metav1.SetMetaDataAnnotation(&toApply.ObjectMeta, v1beta1constants.GardenerTimestamp, time.Now().UTC().String())
					toApply.Spec.BinaryPath = extensionsv1alpha1.ContainerDRuntimeContainersBinFolder
					toApply.Spec.Type = cr.Type
					toApply.Spec.ProviderConfig = cr.ProviderConfig
					toApply.Spec.WorkerPool.Name = workerName
					toApply.Spec.WorkerPool.Selector.MatchLabels = map[string]string{gardencorev1beta1constants.LabelWorkerPool: workerName, gardencorev1beta1constants.LabelWorkerPoolDeprecated: workerName}
					return nil
				})
				return err
			})
		}
	}

	return flow.Parallel(fns...)(ctx)
}

func getContainerRuntimeKey(criType, workerName string) string {
	return fmt.Sprintf("%s-%s", criType, workerName)
}

// WaitUntilContainerRuntimeResourcesReady waits until all container runtime resources report `Succeeded` in their last operation state.
// The state must be reported before the passed context is cancelled or a container runtime's timeout has been reached.
// As soon as one timeout has been overstepped the function returns an error, further waits on container runtime will be aborted.
func (b *Botanist) WaitUntilContainerRuntimeResourcesReady(ctx context.Context) error {
	var fns []flow.TaskFn
	for _, worker := range b.Shoot.Info.Spec.Provider.Workers {
		if worker.CRI == nil {
			continue
		}

		for _, containerRuntime := range worker.CRI.ContainerRuntimes {
			fns = append(fns, func(ctx context.Context) error {
				return common.WaitUntilExtensionCRReady(
					ctx,
					b.K8sSeedClient.Client(),
					b.Logger,
					func() runtime.Object { return &extensionsv1alpha1.ContainerRuntime{} },
					"ContainerRuntime",
					b.Shoot.SeedNamespace,
					getContainerRuntimeKey(containerRuntime.Type, worker.Name),
					DefaultInterval,
					DefaultSevereThreshold,
					shoot.ExtensionDefaultTimeout,
					nil,
				)
			})
		}
	}

	return flow.ParallelExitOnError(fns...)(ctx)
}

// DeleteStaleContainerRuntimeResources deletes unused container runtime resources from the shoot namespace in the seed.
func (b *Botanist) DeleteStaleContainerRuntimeResources(ctx context.Context) error {
	wantedContainerRuntimeTypes := sets.NewString()
	for _, worker := range b.Shoot.Info.Spec.Provider.Workers {
		if worker.CRI != nil {
			for _, containerRuntime := range worker.CRI.ContainerRuntimes {
				key := getContainerRuntimeKey(containerRuntime.Type, worker.Name)
				wantedContainerRuntimeTypes.Insert(key)
			}
		}
	}
	return b.deleteContainerRuntimeResources(ctx, wantedContainerRuntimeTypes)
}

// DeleteAllContainerRuntimeResources deletes all container runtime resources from the Shoot namespace in the Seed.
func (b *Botanist) DeleteAllContainerRuntimeResources(ctx context.Context) error {
	return b.deleteContainerRuntimeResources(ctx, sets.NewString())
}

func (b *Botanist) deleteContainerRuntimeResources(ctx context.Context, wantedContainerRuntimeTypes sets.String) error {
	return common.DeleteExtensionCRs(
		ctx,
		b.K8sSeedClient.Client(),
		&extensionsv1alpha1.ContainerRuntimeList{},
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.ContainerRuntime{} },
		b.Shoot.SeedNamespace,
		func(obj extensionsv1alpha1.Object) bool {
			cr, ok := obj.(*extensionsv1alpha1.ContainerRuntime)
			if !ok {
				return false
			}
			return !wantedContainerRuntimeTypes.Has(getContainerRuntimeKey(cr.Spec.Type, cr.Spec.WorkerPool.Name))
		},
	)
}

// WaitUntilContainerRuntimeResourcesDeleted waits until all container runtime resources are gone or the context is cancelled.
func (b *Botanist) WaitUntilContainerRuntimeResourcesDeleted(ctx context.Context) error {
	return common.WaitUntilExtensionCRsDeleted(
		ctx,
		b.K8sSeedClient.Client(),
		b.Logger,
		&extensionsv1alpha1.ContainerRuntimeList{},
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.ContainerRuntime{} },
		"ContainerRuntime",
		b.Shoot.SeedNamespace,
		DefaultInterval,
		shoot.ExtensionDefaultTimeout,
		nil,
	)
}
