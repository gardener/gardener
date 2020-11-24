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

package containerruntime

import (
	"context"
	"fmt"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/flow"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// DefaultInterval is the default interval for retry operations.
	DefaultInterval = 5 * time.Second
	// DefaultSevereThreshold is the default threshold until an error reported by another component is treated as 'severe'.
	DefaultSevereThreshold = 30 * time.Second
	// DefaultTimeout is the default timeout and defines how long Gardener should wait
	// for a successful reconciliation of a containerruntime resource.
	DefaultTimeout = 3 * time.Minute
)

// TimeNow returns the current time. Exposed for testing.
var TimeNow = time.Now

// Values contains the values used to create a ContainerRuntime resources.
type Values struct {
	Namespace string
	Workers   []gardencorev1beta1.Worker
}

type containerruntime struct {
	values              *Values
	client              client.Client
	logger              logrus.FieldLogger
	waitInterval        time.Duration
	waitSevereThreshold time.Duration
	waitTimeout         time.Duration
}

// New creates a new instance of ExtensionContainerRuntime deployer.
func New(
	logger logrus.FieldLogger,
	client client.Client,
	values *Values,
	waitInterval time.Duration,
	waitSevereThreshold time.Duration,
	waitTimeout time.Duration,
) shoot.ExtensionContainerRuntime {
	return &containerruntime{
		values:              values,
		client:              client,
		logger:              logger,
		waitInterval:        waitInterval,
		waitSevereThreshold: waitSevereThreshold,
		waitTimeout:         waitTimeout,
	}
}

// Deploy uses the seed client to create or update the ContainerRuntime resources.
func (d *containerruntime) Deploy(ctx context.Context) error {
	fns := d.forEachContainerRuntime(func(ctx context.Context, workerName string, cr gardencorev1beta1.ContainerRuntime) error {
		rd := resourceDeployer{d.values.Namespace, workerName, cr, d.client}
		_, err := rd.deploy(ctx, v1beta1constants.GardenerOperationReconcile)
		return err
	})

	return flow.Parallel(fns...)(ctx)
}

// Destroy deletes the ContainerRuntime resources.
func (d *containerruntime) Destroy(ctx context.Context) error {
	return d.deleteContainerRuntimeResources(ctx, sets.NewString())
}

// Wait waits until the ContainerRuntime resources are ready.
func (d *containerruntime) Wait(ctx context.Context) error {
	fns := d.forEachContainerRuntime(func(ctx context.Context, workerName string, cr gardencorev1beta1.ContainerRuntime) error {
		return common.WaitUntilExtensionCRReady(
			ctx,
			d.client,
			d.logger,
			func() runtime.Object { return &extensionsv1alpha1.ContainerRuntime{} },
			extensionsv1alpha1.ContainerRuntimeResource,
			d.values.Namespace,
			getContainerRuntimeKey(cr.Type, workerName),
			d.waitInterval,
			d.waitSevereThreshold,
			d.waitTimeout,
			nil,
		)
	})

	return flow.ParallelExitOnError(fns...)(ctx)
}

// WaitCleanup waits until the ContainerRuntime resources are cleaned up.
func (d *containerruntime) WaitCleanup(ctx context.Context) error {
	return common.WaitUntilExtensionCRsDeleted(
		ctx,
		d.client,
		d.logger,
		&extensionsv1alpha1.ContainerRuntimeList{},
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.ContainerRuntime{} },
		extensionsv1alpha1.ContainerRuntimeResource,
		d.values.Namespace,
		d.waitInterval,
		d.waitTimeout,
		nil,
	)
}

// Restore uses the seed client and the ShootState to create the ContainerRuntime resources and restore their state.
func (d *containerruntime) Restore(ctx context.Context, shootState *gardencorev1alpha1.ShootState) error {
	fns := d.forEachContainerRuntime(func(ctx context.Context, workerName string, cr gardencorev1beta1.ContainerRuntime) error {
		rd := resourceDeployer{d.values.Namespace, workerName, cr, d.client}
		return common.RestoreExtensionWithDeployFunction(ctx, shootState, d.client, extensionsv1alpha1.ContainerRuntimeResource, d.values.Namespace, rd.deploy)
	})

	return flow.Parallel(fns...)(ctx)
}

// Migrate migrates the ContainerRuntime resources.
func (d *containerruntime) Migrate(ctx context.Context) error {
	return common.MigrateExtensionCRs(
		ctx,
		d.client,
		&extensionsv1alpha1.ContainerRuntimeList{},
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.ContainerRuntime{} },
		d.values.Namespace,
	)
}

// WaitMigrate waits until the ContainerRuntime resources are migrated successfully.
func (d *containerruntime) WaitMigrate(ctx context.Context) error {
	return common.WaitUntilExtensionCRsMigrated(
		ctx,
		d.client,
		&extensionsv1alpha1.ContainerRuntimeList{},
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.ContainerRuntime{} },
		d.values.Namespace,
		d.waitInterval,
		d.waitTimeout,
	)
}

// DeleteStaleResources deletes unused container runtime resources from the shoot namespace in the seed.
func (d *containerruntime) DeleteStaleResources(ctx context.Context) error {
	wantedContainerRuntimeTypes := sets.NewString()
	for _, worker := range d.values.Workers {
		if worker.CRI != nil {
			for _, containerRuntime := range worker.CRI.ContainerRuntimes {
				key := getContainerRuntimeKey(containerRuntime.Type, worker.Name)
				wantedContainerRuntimeTypes.Insert(key)
			}
		}
	}
	return d.deleteContainerRuntimeResources(ctx, wantedContainerRuntimeTypes)
}

func (d *containerruntime) deleteContainerRuntimeResources(ctx context.Context, wantedContainerRuntimeTypes sets.String) error {
	return common.DeleteExtensionCRs(
		ctx,
		d.client,
		&extensionsv1alpha1.ContainerRuntimeList{},
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.ContainerRuntime{} },
		d.values.Namespace,
		func(obj extensionsv1alpha1.Object) bool {
			cr, ok := obj.(*extensionsv1alpha1.ContainerRuntime)
			if !ok {
				return false
			}
			return !wantedContainerRuntimeTypes.Has(getContainerRuntimeKey(cr.Spec.Type, cr.Spec.WorkerPool.Name))
		},
	)
}

func (d *containerruntime) forEachContainerRuntime(fn func(ctx context.Context, workerName string, cr gardencorev1beta1.ContainerRuntime) error) []flow.TaskFn {
	var fns []flow.TaskFn
	for _, worker := range d.values.Workers {
		if worker.CRI == nil {
			continue
		}
		for _, containerRuntime := range worker.CRI.ContainerRuntimes {
			cr := containerRuntime
			workerName := worker.Name
			fns = append(fns, func(ctx context.Context) error {
				return fn(ctx, workerName, cr)
			})
		}
	}

	return fns
}

func getContainerRuntimeKey(criType, workerName string) string {
	return fmt.Sprintf("%s-%s", criType, workerName)
}

type resourceDeployer struct {
	namespace        string
	workerName       string
	containerRuntime gardencorev1beta1.ContainerRuntime
	client           client.Client
}

func (d *resourceDeployer) deploy(ctx context.Context, operation string) (extensionsv1alpha1.Object, error) {
	toApply := extensionsv1alpha1.ContainerRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getContainerRuntimeKey(d.containerRuntime.Type, d.workerName),
			Namespace: d.namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, d.client, &toApply, func() error {
		metav1.SetMetaDataAnnotation(&toApply.ObjectMeta, v1beta1constants.GardenerOperation, operation)
		metav1.SetMetaDataAnnotation(&toApply.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().String())
		toApply.Spec.BinaryPath = extensionsv1alpha1.ContainerDRuntimeContainersBinFolder
		toApply.Spec.Type = d.containerRuntime.Type
		toApply.Spec.ProviderConfig = d.containerRuntime.ProviderConfig
		toApply.Spec.WorkerPool.Name = d.workerName
		toApply.Spec.WorkerPool.Selector.MatchLabels = map[string]string{v1beta1constants.LabelWorkerPool: d.workerName, v1beta1constants.LabelWorkerPoolDeprecated: d.workerName}
		return nil
	})
	return &toApply, err
}
