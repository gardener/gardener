// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package containerruntime

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils/flow"
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

// Interface is an interface for managing ContainerRuntimes.
type Interface interface {
	component.DeployMigrateWaiter
	// DeleteStaleResources deletes unused container runtime resources from the shoot namespace in the seed.
	DeleteStaleResources(ctx context.Context) error
	// WaitCleanupStaleResources waits until all unused ContainerRuntime resources are cleaned up.
	WaitCleanupStaleResources(ctx context.Context) error
}

// Values contains the values used to create a ContainerRuntime resources.
type Values struct {
	// Namespace is the namespace for the ContainerRuntime resource.
	Namespace string
	// Workers is the list of worker pools.
	Workers []gardencorev1beta1.Worker
}

type containerRuntime struct {
	values              *Values
	log                 logr.Logger
	client              client.Client
	waitInterval        time.Duration
	waitSevereThreshold time.Duration
	waitTimeout         time.Duration

	containerRuntimes map[string]*extensionsv1alpha1.ContainerRuntime
}

// New creates a new instance of Interface.
func New(
	log logr.Logger,
	client client.Client,
	values *Values,
	waitInterval time.Duration,
	waitSevereThreshold time.Duration,
	waitTimeout time.Duration,
) Interface {
	return &containerRuntime{
		values:              values,
		log:                 log,
		client:              client,
		waitInterval:        waitInterval,
		waitSevereThreshold: waitSevereThreshold,
		waitTimeout:         waitTimeout,

		containerRuntimes: make(map[string]*extensionsv1alpha1.ContainerRuntime),
	}
}

// Deploy uses the seed client to create or update the ContainerRuntime resources.
func (c *containerRuntime) Deploy(ctx context.Context) error {
	fns := c.forEachContainerRuntime(func(ctx context.Context, cr *extensionsv1alpha1.ContainerRuntime, coreCR gardencorev1beta1.ContainerRuntime, workerName string) error {
		_, err := c.deploy(ctx, cr, coreCR, workerName, v1beta1constants.GardenerOperationReconcile)
		return err
	})

	return flow.Parallel(fns...)(ctx)
}

func (c *containerRuntime) deploy(ctx context.Context, cr *extensionsv1alpha1.ContainerRuntime, coreCR gardencorev1beta1.ContainerRuntime, workerName, operation string) (extensionsv1alpha1.Object, error) {
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, c.client, cr, func() error {
		metav1.SetMetaDataAnnotation(&cr.ObjectMeta, v1beta1constants.GardenerOperation, operation)
		metav1.SetMetaDataAnnotation(&cr.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().Format(time.RFC3339Nano))

		cr.Spec.BinaryPath = extensionsv1alpha1.ContainerDRuntimeContainersBinFolder
		cr.Spec.Type = coreCR.Type
		cr.Spec.ProviderConfig = coreCR.ProviderConfig
		cr.Spec.WorkerPool.Name = workerName
		cr.Spec.WorkerPool.Selector.MatchLabels = map[string]string{v1beta1constants.LabelWorkerPool: workerName, v1beta1constants.LabelWorkerPoolDeprecated: workerName}
		return nil
	})
	return cr, err
}

// Destroy deletes the ContainerRuntime resources.
func (c *containerRuntime) Destroy(ctx context.Context) error {
	return c.deleteContainerRuntimeResources(ctx, sets.New[string]())
}

// Wait waits until the ContainerRuntime resources are ready.
func (c *containerRuntime) Wait(ctx context.Context) error {
	fns := c.forEachContainerRuntime(func(ctx context.Context, cr *extensionsv1alpha1.ContainerRuntime, _ gardencorev1beta1.ContainerRuntime, _ string) error {
		return extensions.WaitUntilExtensionObjectReady(
			ctx,
			c.client,
			c.log,
			cr,
			extensionsv1alpha1.ContainerRuntimeResource,
			c.waitInterval,
			c.waitSevereThreshold,
			c.waitTimeout,
			nil,
		)
	})

	return flow.ParallelExitOnError(fns...)(ctx)
}

// WaitCleanup waits until all ContainerRuntime resources are cleaned up.
func (c *containerRuntime) WaitCleanup(ctx context.Context) error {
	return c.waitCleanup(ctx, sets.New[string]())
}

// Restore uses the seed client and the ShootState to create the ContainerRuntime resources and restore their state.
func (c *containerRuntime) Restore(ctx context.Context, shootState *gardencorev1beta1.ShootState) error {
	fns := c.forEachContainerRuntime(func(ctx context.Context, cr *extensionsv1alpha1.ContainerRuntime, coreCR gardencorev1beta1.ContainerRuntime, workerName string) error {
		return extensions.RestoreExtensionWithDeployFunction(ctx, c.client, shootState, extensionsv1alpha1.ContainerRuntimeResource, func(ctx context.Context, operationAnnotation string) (extensionsv1alpha1.Object, error) {
			return c.deploy(ctx, cr, coreCR, workerName, operationAnnotation)
		})
	})

	return flow.Parallel(fns...)(ctx)
}

// Migrate migrates the ContainerRuntime resources.
func (c *containerRuntime) Migrate(ctx context.Context) error {
	return extensions.MigrateExtensionObjects(
		ctx,
		c.client,
		&extensionsv1alpha1.ContainerRuntimeList{},
		c.values.Namespace,
		nil,
	)
}

// WaitMigrate waits until the ContainerRuntime resources are migrated successfully.
func (c *containerRuntime) WaitMigrate(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectsMigrated(
		ctx,
		c.client,
		&extensionsv1alpha1.ContainerRuntimeList{},
		extensionsv1alpha1.ContainerRuntimeResource,
		c.values.Namespace,
		c.waitInterval,
		c.waitTimeout,
		nil,
	)
}

// DeleteStaleResources deletes unused container runtime resources from the shoot namespace in the seed.
func (c *containerRuntime) DeleteStaleResources(ctx context.Context) error {
	return c.deleteContainerRuntimeResources(ctx, c.getWantedContainerRuntimeNames())
}

func (c *containerRuntime) deleteContainerRuntimeResources(ctx context.Context, wantedContainerRuntimeNames sets.Set[string]) error {
	return extensions.DeleteExtensionObjects(
		ctx,
		c.client,
		&extensionsv1alpha1.ContainerRuntimeList{},
		c.values.Namespace,
		func(obj extensionsv1alpha1.Object) bool {
			return !wantedContainerRuntimeNames.Has(obj.GetName())
		},
	)
}

// WaitCleanupStaleResources waits until all unused ContainerRuntime resources are cleaned up.
func (c *containerRuntime) WaitCleanupStaleResources(ctx context.Context) error {
	return c.waitCleanup(ctx, c.getWantedContainerRuntimeNames())
}

func (c *containerRuntime) waitCleanup(ctx context.Context, wantedContainerRuntimeNames sets.Set[string]) error {
	return extensions.WaitUntilExtensionObjectsDeleted(
		ctx,
		c.client,
		c.log,
		&extensionsv1alpha1.ContainerRuntimeList{},
		extensionsv1alpha1.ContainerRuntimeResource,
		c.values.Namespace,
		c.waitInterval,
		c.waitTimeout,
		func(obj extensionsv1alpha1.Object) bool {
			return !wantedContainerRuntimeNames.Has(obj.GetName())
		},
	)
}

// getWantedContainerRuntimeNames returns the names of all container runtime resources, that are currently needed based
// on the configured worker pools.
func (c *containerRuntime) getWantedContainerRuntimeNames() sets.Set[string] {
	wantedContainerRuntimeNames := sets.New[string]()
	for _, worker := range c.values.Workers {
		if worker.CRI != nil {
			for _, cr := range worker.CRI.ContainerRuntimes {
				wantedContainerRuntimeNames.Insert(getContainerRuntimeName(cr.Type, worker.Name))
			}
		}
	}
	return wantedContainerRuntimeNames
}

func (c *containerRuntime) forEachContainerRuntime(fn func(ctx context.Context, cr *extensionsv1alpha1.ContainerRuntime, coreCR gardencorev1beta1.ContainerRuntime, workerName string) error) []flow.TaskFn {
	var fns []flow.TaskFn
	for _, worker := range c.values.Workers {
		if worker.CRI == nil {
			continue
		}
		for _, cr := range worker.CRI.ContainerRuntimes {
			var (
				workerName = worker.Name
				coreCR     = cr
				crName     = getContainerRuntimeName(coreCR.Type, workerName)
			)

			extensionCR, ok := c.containerRuntimes[crName]
			if !ok {
				extensionCR = c.emptyContainerRuntimeExtension(crName)
				// store object for later usage (we want to pass a filled object to WaitUntil*)
				c.containerRuntimes[crName] = extensionCR
			}

			fns = append(fns, func(ctx context.Context) error {
				return fn(ctx, extensionCR, coreCR, workerName)
			})
		}
	}

	return fns
}

func (c *containerRuntime) emptyContainerRuntimeExtension(name string) *extensionsv1alpha1.ContainerRuntime {
	return &extensionsv1alpha1.ContainerRuntime{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: c.values.Namespace,
		},
	}
}

func getContainerRuntimeName(criType, workerName string) string {
	return fmt.Sprintf("%s-%s", criType, workerName)
}
