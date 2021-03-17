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

package extension

import (
	"context"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/flow"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	// for a successful reconciliation of an Extension resource.
	DefaultTimeout = 3 * time.Minute
)

// TimeNow returns the current time. Exposed for testing.
var TimeNow = time.Now

// Interface contains references to an Extension deployer.
type Interface interface {
	component.DeployMigrateWaiter
	// DeleteStaleResources deletes unused Extension resources from the shoot namespace in the seed.
	DeleteStaleResources(context.Context) error
	// Extensions returns the map of extensions where the key is the type and the value is an Extension structure.
	Extensions() map[string]Extension
}

// Extension contains information about the desired Extension resources as well as configuration information.
type Extension struct {
	extensionsv1alpha1.Extension
	// Timeout is the maximum waiting time for the Extension status to report readiness.
	Timeout time.Duration
}

// Values contains the values used to create an Extension resources.
type Values struct {
	// Namespace is the namespace into which the Extension resources should be deployed.
	Namespace string
	// Extensions is the map of extensions where the key is the type and the value is an Extension structure.
	Extensions map[string]Extension
}

type extension struct {
	values              *Values
	client              client.Client
	logger              logrus.FieldLogger
	waitInterval        time.Duration
	waitSevereThreshold time.Duration
	waitTimeout         time.Duration
}

// New creates a new instance of Extension deployer.
func New(
	logger logrus.FieldLogger,
	client client.Client,
	values *Values,
	waitInterval time.Duration,
	waitSevereThreshold time.Duration,
	waitTimeout time.Duration,
) Interface {
	return &extension{
		values:              values,
		client:              client,
		logger:              logger,
		waitInterval:        waitInterval,
		waitSevereThreshold: waitSevereThreshold,
		waitTimeout:         waitTimeout,
	}
}

// Deploy uses the seed client to create or update the Extension resources.
func (e *extension) Deploy(ctx context.Context) error {
	fns := e.forEach(func(ctx context.Context, extension extensionsv1alpha1.Extension, _ time.Duration) error {
		deployer := &deployer{e.client, extension}

		_, err := deployer.deploy(ctx, v1beta1constants.GardenerOperationReconcile)
		return err
	})

	return flow.Parallel(fns...)(ctx)
}

// Destroy deletes all the Extension resources.
func (e *extension) Destroy(ctx context.Context) error {
	return e.deleteExtensionResources(ctx, sets.NewString())
}

// Wait waits until the Extension resources are ready.
func (e *extension) Wait(ctx context.Context) error {
	fns := e.forEach(func(ctx context.Context, extension extensionsv1alpha1.Extension, timeout time.Duration) error {
		return common.WaitUntilExtensionCRReady(
			ctx,
			e.client,
			e.logger,
			func() client.Object { return &extensionsv1alpha1.Extension{} },
			extensionsv1alpha1.ExtensionResource,
			extension.Namespace,
			extension.Name,
			e.waitInterval,
			e.waitSevereThreshold,
			timeout,
			nil,
		)
	})

	return flow.ParallelExitOnError(fns...)(ctx)
}

// WaitCleanup waits until the Extension resources are cleaned up.
func (e *extension) WaitCleanup(ctx context.Context) error {
	return common.WaitUntilExtensionCRsDeleted(
		ctx,
		e.client,
		e.logger,
		&extensionsv1alpha1.ExtensionList{},
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Extension{} },
		extensionsv1alpha1.ExtensionResource,
		e.values.Namespace,
		e.waitInterval,
		e.waitTimeout,
		nil,
	)
}

// Restore uses the seed client and the ShootState to create the Extension resources and restore their state.
func (e *extension) Restore(ctx context.Context, shootState *gardencorev1alpha1.ShootState) error {
	fns := e.forEach(func(ctx context.Context, extension extensionsv1alpha1.Extension, _ time.Duration) error {
		deployer := &deployer{e.client, extension}

		return common.RestoreExtensionWithDeployFunction(
			ctx,
			e.client,
			shootState,
			extensionsv1alpha1.ExtensionResource,
			e.values.Namespace,
			deployer.deploy,
		)
	})

	return flow.Parallel(fns...)(ctx)
}

// Migrate migrates the Extension resources.
func (e *extension) Migrate(ctx context.Context) error {
	return common.MigrateExtensionCRs(
		ctx,
		e.client,
		&extensionsv1alpha1.ExtensionList{},
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Extension{} },
		e.values.Namespace,
	)
}

// WaitMigrate waits until the Extension resources are migrated successfully.
func (e *extension) WaitMigrate(ctx context.Context) error {
	return common.WaitUntilExtensionCRsMigrated(
		ctx,
		e.client,
		&extensionsv1alpha1.ExtensionList{},
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Extension{} },
		e.values.Namespace,
		e.waitInterval,
		e.waitTimeout,
	)
}

// DeleteStaleResources deletes unused Extension resources from the shoot namespace in the seed.
func (e *extension) DeleteStaleResources(ctx context.Context) error {
	wantedExtensionTypes := sets.NewString()
	for _, extension := range e.values.Extensions {
		wantedExtensionTypes.Insert(extension.Spec.Type)
	}
	return e.deleteExtensionResources(ctx, wantedExtensionTypes)
}

func (e *extension) deleteExtensionResources(ctx context.Context, wantedExtensionTypes sets.String) error {
	return common.DeleteExtensionCRs(
		ctx,
		e.client,
		&extensionsv1alpha1.ExtensionList{},
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.Extension{} },
		e.values.Namespace,
		func(obj extensionsv1alpha1.Object) bool {
			return !wantedExtensionTypes.Has(obj.GetExtensionSpec().GetExtensionType())
		},
	)
}

func (e *extension) forEach(fn func(context.Context, extensionsv1alpha1.Extension, time.Duration) error) []flow.TaskFn {
	fns := make([]flow.TaskFn, 0, len(e.values.Extensions))

	for _, ext := range e.values.Extensions {
		obj := ext.Extension
		timeout := ext.Timeout

		fns = append(fns, func(ctx context.Context) error {
			return fn(ctx, obj, timeout)
		})
	}

	return fns
}

// Extensions returns the map of extensions where the key is the type and the value is an Extension structure.
func (e *extension) Extensions() map[string]Extension {
	return e.values.Extensions
}

type deployer struct {
	client client.Client
	obj    extensionsv1alpha1.Extension
}

func (d *deployer) deploy(ctx context.Context, operation string) (extensionsv1alpha1.Object, error) {
	obj := &extensionsv1alpha1.Extension{
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.obj.Name,
			Namespace: d.obj.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, d.client, obj, func() error {
		metav1.SetMetaDataAnnotation(&obj.ObjectMeta, v1beta1constants.GardenerOperation, operation)
		metav1.SetMetaDataAnnotation(&obj.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().String())
		obj.Spec.Type = d.obj.Spec.Type
		obj.Spec.ProviderConfig = d.obj.Spec.ProviderConfig
		return nil
	})

	return obj, err
}
