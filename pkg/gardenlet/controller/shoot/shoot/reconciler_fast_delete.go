// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/utils/errors"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/gardener/shootstate"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	utilclient "github.com/gardener/gardener/pkg/utils/kubernetes/client"
	retryutils "github.com/gardener/gardener/pkg/utils/retry"
)

// runFastDeleteShootFlow abandons resources inside the Shoot without restoring its control plane. Workers,
// infrastructure, extensions, and Seed-side resources are still deleted using their normal deletion operations.
func (r *Reconciler) runFastDeleteShootFlow(ctx context.Context, o *operation.Operation) *v1beta1helper.WrappedLastErrors {
	if v1beta1helper.IsShootSelfHosted(o.Shoot.GetInfo().Spec.Provider.Workers) {
		return nil
	}

	var (
		botanist        *botanistpkg.Botanist
		tasksWithErrors []string
		err             error
	)
	for _, lastError := range o.Shoot.GetInfo().Status.LastErrors {
		if lastError.TaskID != nil {
			tasksWithErrors = append(tasksWithErrors, *lastError.TaskID)
		}
	}
	errorContext := errors.NewErrorContext("Shoot cluster fast deletion", tasksWithErrors)
	if err := errors.HandleErrors(errorContext,
		func(errorID string) error {
			o.CleanShootTaskError(ctx, errorID)
			return nil
		},
		nil,
		errors.ToExecute("Create botanist", func() error {
			return retryutils.UntilTimeout(ctx, 10*time.Second, 10*time.Minute, func(context.Context) (bool, error) {
				botanist, err = botanistpkg.New(ctx, o)
				if err != nil {
					return retryutils.MinorError(err)
				}
				return retryutils.Ok()
			})
		}),
		errors.ToExecute("Check required extensions exist", func() error {
			return botanist.WaitUntilRequiredExtensionsReady(ctx)
		}),
		errors.ToExecute("Retrieve the Shoot namespace in the Seed cluster", func() error {
			return checkIfSeedNamespaceExists(ctx, o, botanist)
		}),
	); err != nil {
		return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), err)
	}

	const (
		defaultInterval = 5 * time.Second
		defaultTimeout  = 30 * time.Second
	)
	retry := func(fn flow.TaskFn) flow.TaskFn { return fn.RetryUntilTimeout(defaultInterval, defaultTimeout) }
	var (
		nonTerminatingNamespace = botanist.SeedNamespaceObject.UID != "" && botanist.SeedNamespaceObject.Status.Phase != corev1.NamespaceTerminating
		workerless              = o.Shoot.IsWorkerless
		cleaner                 = NewCleaner(o.Logger, botanist.SeedClientSet.Client(), r.GardenClient, o.Shoot.ControlPlaneNamespace)
		g                       = flow.NewGraph("Shoot cluster fast deletion")

		// Only prepare Seed-side prerequisites for extension and machine deletion. No Shoot client is initialized.
		prepareSeedResources = g.Add(flow.Task{
			Name: "Preparing Seed resources for deletion",
			Fn: flow.Sequential(
				retry(botanist.DeployControlPlaneNamespace),
				retry(botanist.EnsureShootClusterIdentity),
				retry(botanist.ReconcileIstioInternalLoadBalancingConfigMap),
				retry(botanist.InitializeSecretsManagement),
				retry(botanist.DeployReferencedResources),
				func(ctx context.Context) error {
					if workerless {
						return nil
					}
					return botanist.DeployCloudProviderSecret(ctx)
				},
			),
			SkipIf: !nonTerminatingNamespace,
		})
		stopShootResourceManager = g.Add(flow.Task{
			Name: "Stopping Shoot gardener-resource-manager",
			Fn:   flow.TaskFn(cleaner.StopShootResourceManager).Timeout(5 * time.Minute),
		})
		deleteShootManagedResources = g.Add(flow.Task{
			Name:         "Finalizing Shoot managed resources without in-Shoot cleanup",
			Fn:           retry(cleaner.DeleteShootManagedResources),
			Dependencies: flow.NewTaskIDs(stopShootResourceManager),
		})
		deleteClusterAutoscaler = g.Add(flow.Task{
			Name: "Deleting cluster autoscaler",
			Fn: retry(func(ctx context.Context) error {
				return botanist.Shoot.Components.ControlPlane.ClusterAutoscaler.Destroy(ctx)
			}),
			SkipIf:       workerless,
			Dependencies: flow.NewTaskIDs(deleteShootManagedResources),
		})
		destroyWorker = g.Add(flow.Task{
			Name: "Deleting Shoot workers",
			Fn: func(ctx context.Context) error {
				return flow.Sequential(
					func(ctx context.Context) error {
						if !nonTerminatingNamespace {
							return nil
						}
						return botanist.DeployMachineControllerManager(ctx)
					},
					retry(botanist.Shoot.Components.Extensions.Worker.Destroy),
					botanist.Shoot.Components.Extensions.Worker.WaitCleanup,
				)(ctx)
			},
			SkipIf:       workerless,
			Dependencies: flow.NewTaskIDs(prepareSeedResources, deleteClusterAutoscaler),
		})
		deleteWorkerComponents = g.Add(flow.Task{
			Name: "Deleting machine-controller-manager and operating system configs",
			Fn: func(ctx context.Context) error {
				return flow.Parallel(
					botanist.Shoot.Components.ControlPlane.MachineControllerManager.Destroy,
					flow.Sequential(
						retry(botanist.Shoot.Components.Extensions.OperatingSystemConfig.Destroy),
						func(ctx context.Context) error {
							if botanist.Shoot.HibernationEnabled {
								return nil
							}
							return botanist.Shoot.Components.Extensions.OperatingSystemConfig.WaitCleanup(ctx)
						},
					),
				)(ctx)
			},
			SkipIf:       workerless,
			Dependencies: flow.NewTaskIDs(destroyWorker),
		})
		deleteNetworkAndContainerRuntime = g.Add(flow.Task{
			Name: "Deleting network and container runtime extensions",
			Fn: func(ctx context.Context) error {
				return flow.Parallel(
					flow.Sequential(retry(botanist.Shoot.Components.Extensions.Network.Destroy), botanist.Shoot.Components.Extensions.Network.WaitCleanup),
					flow.Sequential(retry(botanist.Shoot.Components.Extensions.ContainerRuntime.Destroy), botanist.Shoot.Components.Extensions.ContainerRuntime.WaitCleanup),
				)(ctx)
			},
			SkipIf:       workerless,
			Dependencies: flow.NewTaskIDs(prepareSeedResources, deleteShootManagedResources),
		})
		deleteManagedResources = g.Add(flow.Task{
			Name: "Deleting managed resources and dependency watchdog access",
			Fn: flow.Sequential(
				retry(cleaner.DeleteShootManagedResources),
				retry(botanist.DeleteManagedResources),
				func(ctx context.Context) error {
					if workerless {
						return nil
					}
					return retry(botanist.Shoot.Components.DependencyWatchdogAccess.Destroy)(ctx)
				},
				flow.TaskFn(botanist.WaitUntilManagedResourcesDeleted).Timeout(10*time.Minute),
			),
			Dependencies: flow.NewTaskIDs(prepareSeedResources, deleteShootManagedResources, destroyWorker),
		})
		deleteExtensionsBeforeKubeAPIServer = g.Add(flow.Task{
			Name: "Deleting extensions before kube-apiserver and stale extensions",
			Fn: flow.Parallel(
				flow.Sequential(retry(botanist.Shoot.Components.Extensions.Extension.DestroyBeforeKubeAPIServer), botanist.Shoot.Components.Extensions.Extension.WaitCleanupBeforeKubeAPIServer),
				flow.Sequential(retry(botanist.Shoot.Components.Extensions.Extension.DeleteStaleResources), botanist.Shoot.Components.Extensions.Extension.WaitCleanupStaleResources),
			),
			Dependencies: flow.NewTaskIDs(deleteWorkerComponents, deleteManagedResources),
		})
		destroyControlPlane = g.Add(flow.Task{
			Name: "Deleting control plane extension",
			Fn: func(ctx context.Context) error {
				return flow.Sequential(retry(botanist.Shoot.Components.Extensions.ControlPlane.Destroy), botanist.Shoot.Components.Extensions.ControlPlane.WaitCleanup)(ctx)
			},
			SkipIf:       workerless,
			Dependencies: flow.NewTaskIDs(deleteNetworkAndContainerRuntime, deleteExtensionsBeforeKubeAPIServer),
		})
		deleteKubeAPIServer = g.Add(flow.Task{
			Name: "Deleting Kubernetes API server",
			Fn: flow.Sequential(
				retry(cleaner.DeleteShootManagedResources),
				flow.TaskFn(botanist.WaitUntilShootManagedResourcesDeleted).Timeout(defaultTimeout),
				retry(botanist.DeleteKubeAPIServer),
				botanist.Shoot.Components.ControlPlane.KubeAPIServer.WaitCleanup,
			),
			Dependencies: flow.NewTaskIDs(destroyControlPlane, deleteExtensionsBeforeKubeAPIServer),
		})
		deleteExtensionsAfterKubeAPIServer = g.Add(flow.Task{
			Name: "Deleting extensions after kube-apiserver",
			Fn: flow.Sequential(
				retry(botanist.Shoot.Components.Extensions.Extension.DestroyAfterKubeAPIServer),
				botanist.Shoot.Components.Extensions.Extension.WaitCleanupAfterKubeAPIServer,
				botanist.Shoot.Components.Extensions.Extension.WaitCleanup,
				retry(cleaner.DeleteShootManagedResources),
			),
			Dependencies: flow.NewTaskIDs(deleteKubeAPIServer),
		})
		deleteControlPlaneResources = g.Add(flow.Task{
			Name: "Deleting API server exposure, resource-manager, and public service account keys",
			Fn: flow.Parallel(
				flow.Sequential(retry(botanist.Shoot.Components.ControlPlane.KubeAPIServerSNI.Destroy), retry(botanist.Shoot.Components.ControlPlane.KubeAPIServerService.Destroy)),
				botanist.Shoot.Components.ControlPlane.ResourceManager.Destroy,
				botanist.DeletePublicServiceAccountKeys,
			),
			Dependencies: flow.NewTaskIDs(deleteKubeAPIServer),
		})
		destroyInfrastructure = g.Add(flow.Task{
			Name: "Deleting Shoot infrastructure",
			Fn: func(ctx context.Context) error {
				return flow.Sequential(retry(botanist.Shoot.Components.Extensions.Infrastructure.Destroy), botanist.Shoot.Components.Extensions.Infrastructure.WaitCleanup)(ctx)
			},
			SkipIf:       workerless,
			Dependencies: flow.NewTaskIDs(destroyControlPlane, deleteExtensionsBeforeKubeAPIServer),
		})
		deleteDNSRecords = g.Add(flow.Task{
			Name: "Deleting ingress and external DNS records",
			Fn: flow.Parallel(
				botanist.DestroyExternalDNSRecord,
				func(ctx context.Context) error {
					if workerless {
						return nil
					}
					return botanist.DestroyIngressDNSRecord(ctx)
				},
			),
			SkipIf:       !nonTerminatingNamespace,
			Dependencies: flow.NewTaskIDs(deleteKubeAPIServer),
		})
		deleteObservability = g.Add(flow.Task{
			Name: "Deleting Shoot observability components in Seed",
			Fn: flow.Sequential(
				flow.Parallel(
					retry(botanist.Shoot.Components.ControlPlane.Alertmanager.Destroy),
					retry(botanist.DestroyPrometheus),
					retry(botanist.Shoot.Components.ControlPlane.BlackboxExporter.Destroy),
					retry(botanist.Shoot.Components.ControlPlane.Plutono.Destroy),
					retry(botanist.DestroySeedLogging),
				),
				retry(botanist.Shoot.Components.ControlPlane.IstioBasicAuthServer.Destroy),
			),
			Dependencies: flow.NewTaskIDs(deleteExtensionsAfterKubeAPIServer, deleteControlPlaneResources, destroyInfrastructure, deleteDNSRecords),
		})
		deleteEtcdAndReferencedResources = g.Add(flow.Task{
			Name: "Deleting etcds, referenced resources, and internal DNS record",
			Fn: flow.Parallel(
				flow.Sequential(retry(botanist.DestroyEtcd), retry(botanist.WaitUntilEtcdsDeleted)),
				retry(botanist.DestroyReferencedResources),
				func(ctx context.Context) error {
					if !nonTerminatingNamespace {
						return nil
					}
					return botanist.DestroyInternalDNSRecord(ctx)
				},
			),
			Dependencies: flow.NewTaskIDs(deleteObservability),
		})
		_ = g.Add(flow.Task{
			Name: "Deleting Shoot namespace in Seed and Shoot State",
			Fn: flow.Sequential(
				retry(botanist.DeleteSeedNamespace),
				flow.Parallel(botanist.WaitUntilSeedNamespaceDeleted, func(ctx context.Context) error {
					return shootstate.Delete(ctx, botanist.GardenClient, botanist.Shoot.GetInfo())
				}),
			),
			Dependencies: flow.NewTaskIDs(deleteEtcdAndReferencedResources),
		})
	)

	if err := g.Compile().Run(ctx, flow.Opts{
		Log:              o.Logger,
		ProgressReporter: r.newProgressReporter(o.ReportShootProgress),
		ErrorCleaner:     o.CleanShootTaskError,
		ErrorContext:     errorContext,
	}); err != nil {
		return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), flow.Errors(err))
	}
	if err := o.ShootClientMap.InvalidateClient(keys.ForShoot(o.Shoot.GetInfo())); err != nil {
		err = fmt.Errorf("failed to invalidate shoot client: %w", err)
		return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), err)
	}
	o.Logger.Info("Successfully fast-deleted Shoot cluster; external workload resources may remain")
	return nil
}

// StopShootResourceManager prevents in-Shoot cleanup while ManagedResources are being deleted. Waiting for Pods to
// disappear (rather than for readiness) also works when the Shoot API is unavailable.
func (c *cleaner) StopShootResourceManager(ctx context.Context) error {
	deployment := &appsv1.Deployment{}
	key := client.ObjectKey{Namespace: c.seedNamespace, Name: v1beta1constants.DeploymentNameGardenerResourceManager}
	if err := c.seedClient.Get(ctx, key, deployment); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	} else {
		patch := client.MergeFrom(deployment.DeepCopy())
		deployment.Spec.Replicas = new(int32(0))
		if err := client.IgnoreNotFound(c.seedClient.Patch(ctx, deployment, patch)); err != nil {
			return err
		}
	}

	return kubernetesutils.WaitUntilResourcesDeleted(ctx, c.seedClient, &corev1.PodList{}, 5*time.Second,
		client.InNamespace(c.seedNamespace),
		client.MatchingLabels{v1beta1constants.LabelApp: v1beta1constants.DeploymentNameGardenerResourceManager},
	)
}

// DeleteShootManagedResources follows force deletion's explicit finalization approach, but only deletes Shoot-targeted
// ManagedResources. Seed-targeted ManagedResources retain their finalizers and are deleted by the normal flow.
func (c *cleaner) DeleteShootManagedResources(ctx context.Context) error {
	mrList := &resourcesv1alpha1.ManagedResourceList{}
	if err := c.seedClient.List(ctx, mrList, client.InNamespace(c.seedNamespace)); err != nil {
		return err
	}

	shootMRList := &resourcesv1alpha1.ManagedResourceList{}
	for _, mr := range mrList.Items {
		if mr.Spec.Class != nil && *mr.Spec.Class != "" {
			continue
		}
		if err := client.IgnoreNotFound(c.seedClient.Delete(ctx, &mr)); err != nil {
			return err
		}
		shootMRList.Items = append(shootMRList.Items, mr)
	}

	// Finalizers can be removed without talking to the Shoot or requiring its resource-manager to run.
	return utilclient.ApplyToObjects(ctx, shootMRList, func(ctx context.Context, object client.Object) error {
		if len(object.GetFinalizers()) == 0 {
			return nil
		}
		c.log.Info("Removing finalizers from Shoot ManagedResource", "managedResource", client.ObjectKeyFromObject(object))
		return client.IgnoreNotFound(controllerutils.RemoveAllFinalizers(ctx, c.seedClient, object))
	})
}
