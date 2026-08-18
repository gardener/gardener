// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	"github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/shoot/helper"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/errors"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	gardenletutils "github.com/gardener/gardener/pkg/utils/gardener/gardenlet"
	"github.com/gardener/gardener/pkg/utils/gardener/secretsrotation"
	"github.com/gardener/gardener/pkg/utils/gardener/shootstate"
	"github.com/gardener/gardener/pkg/utils/gardener/tokenrequest"
	retryutils "github.com/gardener/gardener/pkg/utils/retry"
)

const (
	defaultTimeout  = 30 * time.Second
	defaultInterval = 5 * time.Second
)

type flowContext struct {
	operationType                  gardencorev1beta1.LastOperationType
	allowBackup                    bool
	hasNodesCIDR                   bool
	requestControlPlanePodsRestart bool
	kubeProxyEnabled               bool
	deployKubeAPIServerTaskTimeout time.Duration
	shootSSHAccessEnabled          bool
	skipReadiness                  bool
	isCopyOfBackupsRequired        bool
	isRestoring                    bool
	isRestoringHAControlPlane      bool
	isHibernatingShootWithWorkers  bool
}

// runReconcileShootFlow reconciles the Shoot cluster.
// It receives an Operation object <o> which stores the Shoot object.
func (r *Reconciler) runReconcileShootFlow(ctx context.Context, o *operation.Operation, operationType gardencorev1beta1.LastOperationType) *v1beta1helper.WrappedLastErrors {
	var tasksWithErrors []string
	for _, lastError := range o.Shoot.GetInfo().Status.LastErrors {
		if lastError.TaskID != nil {
			tasksWithErrors = append(tasksWithErrors, *lastError.TaskID)
		}
	}

	var (
		skipReadiness = metav1.HasAnnotation(o.Shoot.GetInfo().ObjectMeta, v1beta1constants.AnnotationShootSkipReadiness)
		flowCtx       = flowContext{
			operationType:                  operationType,
			allowBackup:                    v1beta1helper.GetBackupConfigForShoot(o.Shoot.GetInfo(), o.GetSeed()) != nil,
			hasNodesCIDR:                   o.Shoot.GetInfo().Spec.Networking != nil && o.Shoot.GetInfo().Spec.Networking.Nodes != nil && (o.Shoot.GetInfo().Status.Networking != nil || skipReadiness),
			requestControlPlanePodsRestart: controllerutils.HasTask(o.Shoot.GetInfo().Annotations, v1beta1constants.ShootTaskRestartControlPlanePods),
			kubeProxyEnabled:               v1beta1helper.KubeProxyEnabled(o.Shoot.GetInfo().Spec.Kubernetes.KubeProxy),
			deployKubeAPIServerTaskTimeout: defaultTimeout,
			shootSSHAccessEnabled:          v1beta1helper.ShootEnablesSSHAccess(o.Shoot.GetInfo()),
			skipReadiness:                  skipReadiness,
			isRestoring:                    operationType == gardencorev1beta1.LastOperationTypeRestore,
			isRestoringHAControlPlane:      o.Shoot.IsRestorePhase() && v1beta1helper.IsHAControlPlaneConfigured(o.Shoot.GetInfo()),
			isHibernatingShootWithWorkers:  o.Shoot.HibernationEnabled && !o.Shoot.GetInfo().Status.IsHibernated && !o.Shoot.IsWorkerless,
		}

		b            *botanistpkg.Botanist
		worker       *extensionsv1alpha1.Worker
		errorContext = errors.NewErrorContext(fmt.Sprintf("Shoot cluster %s", utils.IifString(flowCtx.isRestoring, "restoration", "reconciliation")), tasksWithErrors)
	)

	if err := errors.HandleErrors(errorContext,
		func(errorID string) error {
			o.CleanShootTaskError(ctx, errorID)
			return nil
		},
		nil,
		errors.ToExecute("Create botanist", func() error {
			return retryutils.UntilTimeout(ctx, 10*time.Second, 10*time.Minute, func(context.Context) (done bool, err error) {
				b, err = botanistpkg.New(ctx, o)
				if err != nil {
					return retryutils.MinorError(err)
				}
				return retryutils.Ok()
			})
		}),
		errors.ToExecute("Check required extensions", func() error {
			return b.WaitUntilRequiredExtensionsReady(ctx)
		}),
		errors.ToExecute("Check if copy of backups is required", func() error {
			var err error
			flowCtx.isCopyOfBackupsRequired, err = b.IsCopyOfBackupsRequired(ctx)
			return err
		}),
		errors.ToExecute("Retrieve the Worker resource", func() error {
			if b.Shoot.IsWorkerless {
				return nil
			}
			obj, err := b.Shoot.Components.Extensions.Worker.Get(ctx)
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			}
			worker = obj
			return nil
		}),
	); err != nil {
		return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), err)
	}

	// During the 'Preparing' phase of different rotation operations, components are deployed twice. Also, the
	// different deployment functions call the `Wait` method after the first deployment. Hence, we should use
	// the respective timeout in this case instead of the (too short) default timeout to prevent undesired and confusing
	// errors in the reconciliation flow.
	if v1beta1helper.GetShootETCDEncryptionKeyRotationPhase(b.Shoot.GetInfo().Status.Credentials) == gardencorev1beta1.RotationPreparing {
		flowCtx.deployKubeAPIServerTaskTimeout = kubeapiserver.TimeoutWaitForDeployment
	}

	if flowCtx.hasNodesCIDR {
		if err := b.UpdateDualStackMigrationConditionIfNeeded(ctx); err != nil {
			return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), err)
		}
		networks, err := shoot.ToNetworks(b.Shoot.GetInfo(), b.Shoot.IsWorkerless)
		if err != nil {
			return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), err)
		}
		b.Shoot.Networks = networks
	}

	if err := b.SetInPlaceUpdatePendingWorkers(ctx, worker); err != nil {
		return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), err)
	}

	graph := flow.NewGraph(fmt.Sprintf("Shoot cluster %s", utils.IifString(flowCtx.isRestoring, "restoration", "reconciliation")))
	if err := r.setupShootReconciliationFlow(ctx, b, flowCtx, graph); err != nil {
		err = fmt.Errorf("setting up reconciliation flow failed: %w", err)
		return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), err)
	}
	f := graph.Compile()

	if err := f.Run(ctx, flow.Opts{
		Log:              b.Logger,
		ProgressReporter: r.newProgressReporter(b.ReportShootProgress),
		ErrorContext:     errorContext,
		ErrorCleaner:     b.CleanShootTaskError,
	}); err != nil {
		return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), flow.Errors(err))
	}

	// TODO(rfranzke): Remove this if-condition once the Shoot controller flow has progressed.
	if !b.Shoot.IsSelfHosted() {
		b.Logger.Info("Cleaning no longer required secrets")
		if err := b.SecretsManager.Cleanup(ctx); err != nil {
			err = fmt.Errorf("failed to clean no longer required secrets: %w", err)
			return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), err)
		}
	}

	if !r.ShootStateControllerEnabled && b.Shoot.IsRestorePhase() {
		b.Logger.Info("Deleting Shoot State after successful restoration")
		if err := shootstate.Delete(ctx, b.GardenClient, b.Shoot.GetInfo()); err != nil {
			err = fmt.Errorf("failed to delete shoot state: %w", err)
			return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), err)
		}
	}

	// ensure that shoot client is invalidated after it has been hibernated
	if b.Shoot.HibernationEnabled {
		if err := b.ShootClientMap.InvalidateClient(keys.ForShoot(b.Shoot.GetInfo())); err != nil {
			err = fmt.Errorf("failed to invalidate shoot client: %w", err)
			return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), err)
		}
	}

	if _, ok := b.Shoot.GetInfo().Annotations[v1beta1constants.AnnotationShootSkipReadiness]; ok {
		b.Logger.Info("Removing skip-readiness annotation")

		if err := b.Shoot.UpdateInfo(ctx, b.GardenClient, false, func(shoot *gardencorev1beta1.Shoot) error {
			delete(shoot.Annotations, v1beta1constants.AnnotationShootSkipReadiness)
			return nil
		}); err != nil {
			return nil
		}
	}

	b.Logger.Info("Successfully reconciled Shoot cluster", "operation", utils.IifString(flowCtx.isRestoring, "restored", "reconciled"))
	return nil
}

func (r *Reconciler) setupShootReconciliationFlow(ctx context.Context, b *botanistpkg.Botanist, flowCtx flowContext, g *flow.Graph) error {
	// If the Shoot is a self-hosted shoot, we have to check if it also is the garden runtime cluster. In this case,
	// gardener-operator is taking over responsibility of some components (e.g., etcd-druid). Detect this by checking whether a Garden resource exists.
	shootIsGarden := false
	if b.Shoot.IsSelfHosted() {
		var err error
		shootIsGarden, err = gardenletutils.ClusterIsGarden(ctx, b.SeedClientSet.Client())
		if err != nil {
			return fmt.Errorf("failed checking whether shoot is garden: %w", err)
		}
	}

	var (
		deployExtensionAfterKAPIMsg = "Deploying extension resources after kube-apiserver"
		waitExtensionAfterKAPIMsg   = "Waiting until extension resources handled after kube-apiserver are ready"

		rotationPreparingPhases = sets.New(gardencorev1beta1.RotationPreparing, gardencorev1beta1.RotationPreparingWithoutWorkersRollout)
		caRotationPreparing     = rotationPreparingPhases.Has(v1beta1helper.GetShootCARotationPhase(b.Shoot.GetInfo().Status.Credentials))
		saKeyRotationPreparing  = rotationPreparingPhases.Has(v1beta1helper.GetShootServiceAccountKeyRotationPhase(b.Shoot.GetInfo().Status.Credentials))
	)
	if b.Shoot.HibernationEnabled {
		deployExtensionAfterKAPIMsg = "Hibernating extension resources before kube-apiserver hibernation"
		waitExtensionAfterKAPIMsg = "Waiting until extension resources hibernated before kube-apiserver hibernation are ready"
	}

	var (
		deployNamespace            = g.AddGroup(b.DeployNamespacesTaskGroup())
		ensureShootClusterIdentity = g.Add(flow.Task{
			Name:         "Ensuring Shoot cluster identity",
			Fn:           flow.TaskFn(b.EnsureShootClusterIdentity).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deployNamespace),
		})
		deployCloudProviderSecret                    = g.AddGroup(b.DeployCloudProviderSecretTaskGroup())
		reconcileIstioInternalLoadbalancingConfigMap = g.Add(flow.Task{
			Name:         "Reconcile Istio internal load balancing ConfigMap",
			Fn:           flow.TaskFn(b.ReconcileIstioInternalLoadBalancingConfigMap).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deployNamespace),
		})
		_                           = g.AddGroup(b.ReconcileCustomResourceDefinitionsTaskGroup())
		_                           = g.AddGroup(b.ReconcileClusterResourceTaskGroup())
		initializeSecretsManagement = g.AddGroup(
			b.InitializeSecretsManagementTaskGroup().
				WithDependencies(reconcileIstioInternalLoadbalancingConfigMap),
		)
		_                     = g.AddGroup(b.ReconcileRuntimeGardenerResourceManagerTaskGroup(true, shootIsGarden, flowCtx.skipReadiness))
		initialValiDeployment = g.Add(flow.Task{
			Name:         "Deploying initial shoot logging stack in Seed",
			Fn:           flow.TaskFn(b.DeployLogging).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       flowCtx.isHibernatingShootWithWorkers || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deployNamespace, initializeSecretsManagement),
		})
		deployReferencedResources    = g.AddGroup(b.ReconcileReferencedResourcesTaskGroup())
		waitUntilInfrastructureReady = g.AddGroup(b.ReconcileInfrastructureTaskGroup(flowCtx.skipReadiness))
		deployKubeAPIServerService   = g.Add(flow.Task{
			Name:         "Deploying Kubernetes API server service in the Seed cluster",
			Fn:           flow.TaskFn(b.Shoot.Components.ControlPlane.KubeAPIServerService.Deploy).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deployNamespace, ensureShootClusterIdentity, initializeSecretsManagement).InsertIf(!flowCtx.hasNodesCIDR, waitUntilInfrastructureReady),
		})
		waitUntilKubeAPIServerServiceIsReady = g.Add(flow.Task{
			Name:         "Waiting until Kubernetes API server service in the Seed cluster has reported readiness",
			Fn:           b.Shoot.Components.ControlPlane.KubeAPIServerService.Wait,
			SkipIf:       b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deployKubeAPIServerService),
		})
		_ = g.Add(flow.Task{
			Name:         "Ensuring advertised addresses for the Shoot",
			Fn:           b.UpdateAdvertisedAddresses,
			SkipIf:       b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement, waitUntilKubeAPIServerServiceIsReady),
		})
		deployInternalDomainDNSRecord = g.Add(flow.Task{
			Name: "Deploying internal domain DNS record",
			Fn: func(ctx context.Context) error {
				if err := b.DeployOrDestroyInternalDNSRecord(ctx); err != nil {
					return err
				}
				return b.RemoveTaskAnnotation(ctx, v1beta1constants.ShootTaskDeployDNSRecordInternal)
			},
			SkipIf:       b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deployReferencedResources, waitUntilKubeAPIServerServiceIsReady),
		})
		_ = g.Add(flow.Task{
			Name: "Deploying external domain DNS record",
			Fn: func(ctx context.Context) error {
				if err := b.DeployOrDestroyExternalDNSRecord(ctx); err != nil {
					return err
				}
				return b.RemoveTaskAnnotation(ctx, v1beta1constants.ShootTaskDeployDNSRecordExternal)
			},
			SkipIf:       b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deployReferencedResources, waitUntilKubeAPIServerServiceIsReady),
		})
		deploySourceBackupEntry = g.Add(flow.Task{
			Name:         "Deploying source backup entry",
			Fn:           b.DeploySourceBackupEntry,
			SkipIf:       !flowCtx.isCopyOfBackupsRequired || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deployNamespace),
		})
		waitUntilSourceBackupEntryInGardenReconciled = g.Add(flow.Task{
			Name:         "Waiting until the source backup entry has been reconciled",
			Fn:           b.Shoot.Components.SourceBackupEntry.Wait,
			SkipIf:       flowCtx.skipReadiness || !flowCtx.isCopyOfBackupsRequired || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deploySourceBackupEntry),
		})
		deployBackupBucketInGarden = g.Add(flow.Task{
			Name: "Deploying BackupBucket for ETCD data",
			Fn: func(ctx context.Context) error {
				return b.Shoot.Components.BackupBucket.Deploy(ctx)
			},
			SkipIf:       !flowCtx.allowBackup || !b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deployNamespace),
		})
		waitUntilBackupBucketInGardenReconciled = g.Add(flow.Task{
			Name: "Waiting until the BackupBucket for ETCD data has been reconciled",
			Fn: func(ctx context.Context) error {
				return b.Shoot.Components.BackupBucket.Wait(ctx)
			},
			SkipIf:       flowCtx.skipReadiness || !flowCtx.allowBackup || !b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deployBackupBucketInGarden),
		})
		deployBackupEntryInGarden = g.Add(flow.Task{
			Name:         "Deploying BackupEntry for ETCD data",
			Fn:           b.DeployBackupEntry,
			SkipIf:       !flowCtx.allowBackup,
			Dependencies: flow.NewTaskIDs(deployNamespace, waitUntilSourceBackupEntryInGardenReconciled, waitUntilBackupBucketInGardenReconciled),
		})
		waitUntilBackupEntryInGardenReconciled = g.Add(flow.Task{
			Name:         "Waiting until the BackupEntry for ETCD data has been reconciled",
			Fn:           b.Shoot.Components.BackupEntry.Wait,
			SkipIf:       flowCtx.skipReadiness || !flowCtx.allowBackup,
			Dependencies: flow.NewTaskIDs(deployBackupEntryInGarden),
		})
		copyEtcdBackups = g.Add(flow.Task{
			Name:         "Copying etcd backups to new seed's backup bucket",
			Fn:           b.DeployEtcdCopyBackupsTask,
			SkipIf:       !flowCtx.isCopyOfBackupsRequired,
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement, deployCloudProviderSecret, waitUntilBackupEntryInGardenReconciled, waitUntilSourceBackupEntryInGardenReconciled),
		})
		waitUntilEtcdBackupsCopied = g.Add(flow.Task{
			Name:         "Waiting until etcd backups are copied",
			Fn:           b.Shoot.Components.ControlPlane.EtcdCopyBackupsTask.Wait,
			SkipIf:       flowCtx.skipReadiness || !flowCtx.isCopyOfBackupsRequired,
			Dependencies: flow.NewTaskIDs(copyEtcdBackups),
		})
		_ = g.Add(flow.Task{
			Name:         "Destroying copy etcd backups task resource",
			Fn:           b.Shoot.Components.ControlPlane.EtcdCopyBackupsTask.Destroy,
			SkipIf:       !flowCtx.isCopyOfBackupsRequired,
			Dependencies: flow.NewTaskIDs(waitUntilEtcdBackupsCopied),
		})
		waitUntilEtcdReady = g.AddGroup(b.ReconcileETCDsTaskGroup(shootIsGarden, flowCtx.isRestoringHAControlPlane, flowCtx.skipReadiness).WithDependencies(
			waitUntilBackupEntryInGardenReconciled,
			waitUntilEtcdBackupsCopied,
		))
		destroySourceBackupEntry = g.Add(flow.Task{
			Name:         "Destroying source backup entry",
			Fn:           b.DestroySourceBackupEntry,
			SkipIf:       !flowCtx.allowBackup || !b.Shoot.IsRestorePhase() || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(waitUntilEtcdReady),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until source backup entry has been deleted",
			Fn:           b.Shoot.Components.SourceBackupEntry.WaitCleanup,
			SkipIf:       !flowCtx.allowBackup || flowCtx.skipReadiness || !b.Shoot.IsRestorePhase() || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(destroySourceBackupEntry),
		})
		deployExtensionResourcesBeforeKAPI = g.Add(flow.Task{
			Name:         "Deploying extension resources before kube-apiserver",
			Fn:           flow.TaskFn(b.DeployExtensionsBeforeKubeAPIServer).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement, deployCloudProviderSecret, deployReferencedResources, waitUntilInfrastructureReady),
		})
		waitUntilExtensionResourcesBeforeKAPIReady = g.Add(flow.Task{
			Name:         "Waiting until extension resources handled before kube-apiserver are ready",
			Fn:           b.Shoot.Components.Extensions.Extension.WaitBeforeKubeAPIServer,
			SkipIf:       b.Shoot.HibernationEnabled || flowCtx.skipReadiness || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deployExtensionResourcesBeforeKAPI),
		})
		deployKubeAPIServer = g.Add(flow.Task{
			Name: "Deploying Kubernetes API server",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return b.DeployKubeAPIServer(ctx)
			}).RetryUntilTimeout(defaultInterval, flowCtx.deployKubeAPIServerTaskTimeout),
			SkipIf: b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(
				initializeSecretsManagement,
				waitUntilEtcdReady,
				waitUntilKubeAPIServerServiceIsReady,
				waitUntilExtensionResourcesBeforeKAPIReady,
			).InsertIf(!flowCtx.hasNodesCIDR, waitUntilInfrastructureReady),
		})
		waitUntilKubeAPIServerIsReady = g.Add(flow.Task{
			Name:         "Waiting until Kubernetes API server rolled out",
			Fn:           b.Shoot.Components.ControlPlane.KubeAPIServer.Wait,
			SkipIf:       b.Shoot.HibernationEnabled || flowCtx.skipReadiness || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deployKubeAPIServer),
		})
		deployKubeAPIServerServiceSNISettings = g.Add(flow.Task{
			Name:         "Deploying and waiting for Kubernetes API server service SNI settings in the Seed cluster",
			Fn:           flow.TaskFn(b.DeployKubeAPIServerSNI).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerIsReady),
		})
		_ = g.Add(flow.Task{
			Name:         "Cleaning up stale Kubernetes API server services in the Seed cluster",
			Fn:           flow.TaskFn(b.CleanupKubeAPIServerLoadBalancingServices).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       b.ShootUsesIstioTLSTermination(),
			Dependencies: flow.NewTaskIDs(deployKubeAPIServerServiceSNISettings),
		})
		scaleEtcdAfterRestore = g.Add(flow.Task{
			Name:         "Scaling main and events etcd after kube-apiserver is ready",
			Fn:           flow.TaskFn(b.ScaleUpETCD).RetryUntilTimeout(defaultInterval, helper.GetEtcdDeployTimeout(b.Shoot, defaultTimeout)),
			SkipIf:       !flowCtx.isRestoringHAControlPlane || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(waitUntilEtcdReady, waitUntilKubeAPIServerIsReady),
		})
		waitUntilEtcdScaledAfterRestore = g.Add(flow.Task{
			Name:         "Waiting until main and events etcd scaled up after kube-apiserver is ready",
			Fn:           flow.TaskFn(b.WaitUntilEtcdsReady),
			SkipIf:       !flowCtx.isRestoringHAControlPlane || flowCtx.skipReadiness || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(scaleEtcdAfterRestore),
		})
		waitUntilGardenerResourceManagerReady = g.AddGroup(
			b.ReconcileGardenerResourceManagerTaskGroup(true, flowCtx.skipReadiness).
				WithDependencies(waitUntilKubeAPIServerIsReady),
		)
		_ = g.Add(flow.Task{
			Name: "Renewing shoot access secrets after creation of new ServiceAccount signing key",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return tokenrequest.RenewAccessSecrets(ctx, b.SeedClientSet.Client(),
					client.InNamespace(b.Shoot.ControlPlaneNamespace),
					client.MatchingLabels{resourcesv1alpha1.ResourceManagerClass: resourcesv1alpha1.ResourceManagerClassShoot},
				)
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf: !sets.New(
				gardencorev1beta1.RotationPreparing,
				gardencorev1beta1.RotationPreparingWithoutWorkersRollout,
			).Has(v1beta1helper.GetShootServiceAccountKeyRotationPhase(b.Shoot.GetInfo().Status.Credentials)) || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(waitUntilGardenerResourceManagerReady),
		})
		waitUntilControlPlaneReady    = g.AddGroup(b.ReconcileControlPlaneTaskGroup(flowCtx.skipReadiness))
		waitUntilShootNamespacesReady = g.AddGroup(b.ReconcileShootNamespacesTaskGroup(flowCtx.skipReadiness))
		deployVPNSeedServer           = g.Add(flow.Task{
			Name:         "Deploying vpn-seed-server",
			Fn:           flow.TaskFn(b.DeployVPNServer).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       b.Shoot.IsWorkerless || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement, deployNamespace, waitUntilGardenerResourceManagerReady),
		})
		deployGardenerAccess = g.Add(flow.Task{
			Name:         "Deploying Gardener shoot access resources",
			Fn:           flow.TaskFn(b.Shoot.Components.GardenerAccess.Deploy).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement, waitUntilGardenerResourceManagerReady),
		})
		initializeShootClients = g.Add(flow.Task{
			Name:         "Initializing connection to Shoot",
			Fn:           flow.TaskFn(b.InitializeDesiredShootClients).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			SkipIf:       b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deployInternalDomainDNSRecord, deployGardenerAccess),
		})
		_ = g.Add(flow.Task{
			Name:         "Sync public service account signing keys to Garden cluster",
			Fn:           b.SyncPublicServiceAccountKeys,
			SkipIf:       b.Shoot.HibernationEnabled || !v1beta1helper.HasManagedIssuer(b.Shoot.GetInfo()) || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(initializeShootClients),
		})
		rewriteResourcesAddLabel = g.Add(flow.Task{
			Name: "Labeling resources after modification of encryption config or to encrypt them with new ETCD encryption key",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return secretsrotation.RewriteEncryptedDataAddLabel(ctx, b.Logger, b.SeedClientSet.Client(), b.ShootClientSet, b.SecretsManager, b.Shoot.ControlPlaneNamespace, v1beta1constants.DeploymentNameKubeAPIServer, b.Shoot.ResourcesToEncrypt, b.Shoot.EncryptedResources, gardenerutils.DefaultGVKsForEncryption())
			}).RetryUntilTimeout(30*time.Second, 10*time.Minute),
			SkipIf: b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted() ||
				(v1beta1helper.GetShootETCDEncryptionKeyRotationPhase(b.Shoot.GetInfo().Status.Credentials) != gardencorev1beta1.RotationPreparing &&
					sets.New(b.Shoot.ResourcesToEncrypt...).Equal(sets.New(b.Shoot.EncryptedResources...)) && (b.Shoot.EncryptionProviderToUse == b.Shoot.UsedEncryptionProvider ||
					v1beta1helper.GetShootETCDEncryptionKeyRotationPhase(b.Shoot.GetInfo().Status.Credentials) == gardencorev1beta1.RotationCompleting)),
			Dependencies: flow.NewTaskIDs(initializeShootClients),
		})
		snapshotETCD = g.Add(flow.Task{
			Name: "Snapshotting ETCD after modification of encryption config or resources are re-encrypted with new ETCD encryption key",
			Fn: func(ctx context.Context) error {
				return secretsrotation.SnapshotETCDAfterRewritingEncryptedData(ctx, b.SeedClientSet.Client(), b.SnapshotEtcd, b.Shoot.ControlPlaneNamespace, v1beta1constants.DeploymentNameKubeAPIServer)
			},
			SkipIf: !flowCtx.allowBackup || b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted() ||
				(v1beta1helper.GetShootETCDEncryptionKeyRotationPhase(b.Shoot.GetInfo().Status.Credentials) != gardencorev1beta1.RotationPreparing &&
					sets.New(b.Shoot.ResourcesToEncrypt...).Equal(sets.New(b.Shoot.EncryptedResources...)) && (b.Shoot.EncryptionProviderToUse == b.Shoot.UsedEncryptionProvider ||
					v1beta1helper.GetShootETCDEncryptionKeyRotationPhase(b.Shoot.GetInfo().Status.Credentials) == gardencorev1beta1.RotationCompleting)),
			Dependencies: flow.NewTaskIDs(rewriteResourcesAddLabel),
		})
		_ = g.Add(flow.Task{
			Name: "Removing label from resources after modification of encryption config or rotation of ETCD encryption key",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				if err := secretsrotation.RewriteEncryptedDataRemoveLabel(ctx, b.Logger, b.SeedClientSet.Client(), b.ShootClientSet, b.Shoot.ControlPlaneNamespace, v1beta1constants.DeploymentNameKubeAPIServer, b.Shoot.ResourcesToEncrypt, b.Shoot.EncryptedResources, gardenerutils.DefaultGVKsForEncryption()); err != nil {
					return err
				}

				if !sets.New(b.Shoot.ResourcesToEncrypt...).Equal(sets.New(b.Shoot.EncryptedResources...)) ||
					b.Shoot.EncryptionProviderToUse != b.Shoot.UsedEncryptionProvider {
					if err := b.Shoot.UpdateInfoStatus(ctx, b.GardenClient, true, false, func(shoot *gardencorev1beta1.Shoot) error {
						var (
							encryptedResources     []string
							encryptionProviderType gardencorev1beta1.EncryptionProviderType
						)
						if b.Shoot.GetInfo().Spec.Kubernetes.KubeAPIServer != nil {
							encryptedResources = shared.StringifyGroupResources(shared.GetResourcesForEncryptionFromConfig(b.Shoot.GetInfo().Spec.Kubernetes.KubeAPIServer.EncryptionConfig))
							encryptionProviderType = v1beta1helper.GetEncryptionProviderType(b.Shoot.GetInfo().Spec.Kubernetes.KubeAPIServer)
						}

						if shoot.Status.Credentials == nil {
							shoot.Status.Credentials = &gardencorev1beta1.ShootCredentials{}
						}
						if shoot.Status.Credentials.EncryptionAtRest == nil {
							shoot.Status.Credentials.EncryptionAtRest = &gardencorev1beta1.EncryptionAtRest{}
						}

						shoot.Status.Credentials.EncryptionAtRest.Provider.Type = encryptionProviderType

						if len(encryptedResources) > 0 {
							shoot.Status.Credentials.EncryptionAtRest.Resources = encryptedResources
						} else {
							shoot.Status.Credentials.EncryptionAtRest.Resources = nil
						}

						return nil
					}); err != nil {
						return err
					}
				}

				return nil
			}).RetryUntilTimeout(30*time.Second, 10*time.Minute),
			SkipIf: b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted() ||
				(v1beta1helper.GetShootETCDEncryptionKeyRotationPhase(b.Shoot.GetInfo().Status.Credentials) != gardencorev1beta1.RotationCompleting &&
					sets.New(b.Shoot.ResourcesToEncrypt...).Equal(sets.New(b.Shoot.EncryptedResources...)) && (b.Shoot.EncryptionProviderToUse == b.Shoot.UsedEncryptionProvider ||
					v1beta1helper.GetShootETCDEncryptionKeyRotationPhase(b.Shoot.GetInfo().Status.Credentials) == gardencorev1beta1.RotationPreparing ||
					v1beta1helper.GetShootETCDEncryptionKeyRotationPhase(b.Shoot.GetInfo().Status.Credentials) == gardencorev1beta1.RotationPrepared)),
			Dependencies: flow.NewTaskIDs(initializeShootClients, snapshotETCD),
		})
		deployKubeScheduler = g.Add(flow.Task{
			Name: "Deploying Kubernetes scheduler",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return b.Shoot.Components.ControlPlane.KubeScheduler.Deploy(ctx)
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       b.Shoot.IsWorkerless || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement, waitUntilGardenerResourceManagerReady),
		})
		_ = g.Add(flow.Task{
			Name:         "Reconciling Kubernetes vertical pod autoscaler",
			Fn:           flow.TaskFn(b.DeployVerticalPodAutoscaler).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       b.Shoot.IsWorkerless || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement, waitUntilGardenerResourceManagerReady),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying dependency-watchdog shoot access resources",
			Fn:           flow.TaskFn(b.DeployDependencyWatchdogAccess).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       b.Shoot.IsWorkerless || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement, waitUntilGardenerResourceManagerReady),
		})
		deployKubeControllerManager = g.Add(flow.Task{
			Name:         "Deploying Kubernetes controller manager",
			Fn:           flow.TaskFn(b.DeployKubeControllerManager).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement, deployCloudProviderSecret, waitUntilGardenerResourceManagerReady),
		})

		waitUntilKubeControllerManagerReady = g.Add(flow.Task{
			Name:         "Waiting until kube-controller-manager reports readiness",
			Fn:           b.Shoot.Components.ControlPlane.KubeControllerManager.Wait,
			SkipIf:       flowCtx.skipReadiness || (!saKeyRotationPreparing && !caRotationPreparing) || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deployKubeControllerManager),
		})
		createNewServiceAccountSecrets = g.Add(flow.Task{
			Name: "Creating new ServiceAccount secrets after creation of new signing key",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return secretsrotation.CreateNewServiceAccountSecrets(ctx, b.Logger, b.ShootClientSet.Client(), b.SecretsManager)
			}).RetryUntilTimeout(30*time.Second, 10*time.Minute),
			SkipIf:       !saKeyRotationPreparing || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(initializeShootClients, waitUntilKubeControllerManagerReady),
		})
		_ = g.Add(flow.Task{
			Name: "Deleting old ServiceAccount secrets after rotation of signing key",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return secretsrotation.DeleteOldServiceAccountSecrets(ctx, b.Logger, b.ShootClientSet.Client(), b.Shoot.GetInfo().Status.Credentials.Rotation.ServiceAccountKey.LastInitiationFinishedTime.Time)
			}).RetryUntilTimeout(30*time.Second, 10*time.Minute),
			SkipIf:       v1beta1helper.GetShootServiceAccountKeyRotationPhase(b.Shoot.GetInfo().Status.Credentials) != gardencorev1beta1.RotationCompleting || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(initializeShootClients, waitUntilKubeControllerManagerReady),
		})
		waitUntilKubeRootCAConfigMapsUpdated = g.Add(flow.Task{
			Name:         "Waiting until kube-root-ca.crt ConfigMaps have been updated with new CA bundle",
			Fn:           flow.TaskFn(b.WaitUntilKubeRootCAConfigMapsUpdated).RetryUntilTimeout(30*time.Second, 5*time.Minute),
			SkipIf:       b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled || flowCtx.skipReadiness || !caRotationPreparing,
			Dependencies: flow.NewTaskIDs(initializeShootClients, waitUntilKubeControllerManagerReady),
		})
		deleteBastions = g.Add(flow.Task{
			Name:         "Deleting Bastions",
			Fn:           b.DeleteBastions,
			SkipIf:       flowCtx.shootSSHAccessEnabled || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deployReferencedResources, waitUntilInfrastructureReady, waitUntilControlPlaneReady),
		})
		deployExtensionResourcesAfterKAPI = g.Add(flow.Task{
			Name:         deployExtensionAfterKAPIMsg,
			Fn:           flow.TaskFn(b.DeployExtensionsAfterKubeAPIServer).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deployReferencedResources, initializeShootClients),
		})
		waitUntilExtensionResourcesAfterKAPIReady = g.Add(flow.Task{
			Name:         waitExtensionAfterKAPIMsg,
			Fn:           b.Shoot.Components.Extensions.Extension.WaitAfterKubeAPIServer,
			SkipIf:       flowCtx.skipReadiness || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deployExtensionResourcesAfterKAPI),
		})
		_                                   = g.AddGroup(b.ReconcileContainerRuntimeTaskGroup(flowCtx.skipReadiness))
		waitUntilOperatingSystemConfigReady = g.AddGroup(b.ReconcileOperatingSystemConfigTaskGroup(flowCtx.skipReadiness).WithDependencies(
			waitUntilInfrastructureReady,
			waitUntilControlPlaneReady,
			deleteBastions,
			waitUntilExtensionResourcesAfterKAPIReady,
		))
		deployShootSystemResources = g.AddGroup(b.ReconcileSystemResourcesTaskGroup().WithDependencies(
			waitUntilOperatingSystemConfigReady,
			waitUntilControlPlaneReady,
			waitUntilExtensionResourcesAfterKAPIReady, // Extensions might deploy webhooks for system components
			waitUntilGardenerResourceManagerReady,
			initializeShootClients,
			waitUntilOperatingSystemConfigReady,
		))
		_ = g.Add(flow.Task{
			Name: "Deploying shoot cluster identity",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return b.DeployClusterIdentity(ctx)
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(waitUntilGardenerResourceManagerReady, ensureShootClusterIdentity, waitUntilOperatingSystemConfigReady),
		})
		_ = g.Add(flow.Task{
			Name:         "Populating static manifests from seed to shoot",
			Fn:           flow.TaskFn(b.PopulateStaticManifestsFromSeedToShoot).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(waitUntilGardenerResourceManagerReady, waitUntilShootNamespacesReady),
		})
		deployMetricsServer = g.Add(flow.Task{
			Name: "Deploying metrics-server system component",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return b.Shoot.Components.SystemComponents.MetricsServer.Deploy(ctx)
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf: b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(waitUntilControlPlaneReady, waitUntilExtensionResourcesAfterKAPIReady, // Extensions might deploy webhooks for system components
				waitUntilGardenerResourceManagerReady, waitUntilOperatingSystemConfigReady, deployKubeScheduler, waitUntilShootNamespacesReady),
		})
		deployVPNShoot = g.Add(flow.Task{
			Name:   "Deploying vpn-shoot system component",
			Fn:     flow.TaskFn(b.DeployVPNShoot).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf: b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(waitUntilControlPlaneReady, waitUntilExtensionResourcesAfterKAPIReady, // Extensions might deploy webhooks for system components
				waitUntilGardenerResourceManagerReady, waitUntilGardenerResourceManagerReady, deployKubeScheduler, deployVPNSeedServer, waitUntilShootNamespacesReady),
		})
		waitUntilVPNShootReady = g.Add(flow.Task{
			Name: "Waiting until vpn-shoot system component is ready",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return b.Shoot.Components.SystemComponents.VPNShoot.Wait(ctx)
			}),
			SkipIf:       b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted() || flowCtx.skipReadiness,
			Dependencies: flow.NewTaskIDs(deployVPNShoot),
		})
		deployNodeProblemDetector = g.Add(flow.Task{
			Name: "Deploying node-problem-detector system component",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return b.Shoot.Components.SystemComponents.NodeProblemDetector.Deploy(ctx)
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf: b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(waitUntilControlPlaneReady, waitUntilExtensionResourcesAfterKAPIReady, // Extensions might deploy webhooks for system components
				waitUntilGardenerResourceManagerReady, waitUntilOperatingSystemConfigReady, waitUntilShootNamespacesReady),
		})
		reconcileSystemComponents = g.AddGroup(b.ReconcileSystemComponentsTaskGroup(flowCtx.kubeProxyEnabled, flowCtx.skipReadiness).WithDependencies(
			waitUntilControlPlaneReady,
			waitUntilExtensionResourcesAfterKAPIReady, // Extensions might deploy webhooks for system components
			initializeShootClients,
			ensureShootClusterIdentity,
			deployKubeScheduler,
		))
		deployAPIServerProxy = g.Add(flow.Task{
			Name:   "Deploying apiserver-proxy",
			Fn:     flow.TaskFn(b.DeployAPIServerProxy).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf: b.Shoot.IsWorkerless || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(waitUntilControlPlaneReady, waitUntilExtensionResourcesAfterKAPIReady, // Extensions might deploy webhooks for system components
				waitUntilGardenerResourceManagerReady, initializeShootClients, ensureShootClusterIdentity, deployKubeScheduler, waitUntilShootNamespacesReady),
		})
		deployBlackboxExporter = g.Add(flow.Task{
			Name:   "Deploying blackbox-exporter",
			Fn:     flow.TaskFn(b.ReconcileBlackboxExporterCluster).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf: b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(waitUntilControlPlaneReady, waitUntilExtensionResourcesAfterKAPIReady, // Extensions might deploy webhooks for system components
				waitUntilGardenerResourceManagerReady, initializeShootClients, ensureShootClusterIdentity, deployKubeScheduler, waitUntilShootNamespacesReady),
		})
		deployNodeExporter = g.Add(flow.Task{
			Name:   "Deploying node-exporter",
			Fn:     flow.TaskFn(b.ReconcileNodeExporter).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf: b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(waitUntilControlPlaneReady, waitUntilExtensionResourcesAfterKAPIReady, // Extensions might deploy webhooks for system components
				waitUntilGardenerResourceManagerReady, initializeShootClients, ensureShootClusterIdentity, deployKubeScheduler, waitUntilShootNamespacesReady),
		})
		deployKubernetesDashboard = g.Add(flow.Task{
			Name:   "Deploying addon Kubernetes Dashboard",
			Fn:     flow.TaskFn(b.DeployKubernetesDashboard).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf: b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(waitUntilControlPlaneReady, waitUntilExtensionResourcesAfterKAPIReady, // Extensions might deploy webhooks for system components
				waitUntilGardenerResourceManagerReady, initializeShootClients, ensureShootClusterIdentity, deployKubeScheduler, waitUntilShootNamespacesReady),
		})
		deployNginxIngressAddon = g.Add(flow.Task{
			Name:   "Deploying addon Nginx Ingress Controller",
			Fn:     flow.TaskFn(b.DeployNginxIngressAddon).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf: b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(waitUntilControlPlaneReady, waitUntilExtensionResourcesAfterKAPIReady, // Extensions might deploy webhooks for system components
				waitUntilGardenerResourceManagerReady, initializeShootClients, ensureShootClusterIdentity, deployKubeScheduler, waitUntilShootNamespacesReady),
		})
		deployManagedResourceForGardenerNodeAgent = g.Add(flow.Task{
			Name:         "Deploying managed resources for the gardener-node-agent",
			Fn:           flow.TaskFn(b.DeployManagedResourceForGardenerNodeAgent).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(waitUntilGardenerResourceManagerReady, ensureShootClusterIdentity, waitUntilOperatingSystemConfigReady),
		})

		syncPointAllSystemComponentsDeployed = flow.NewTaskIDs(
			reconcileSystemComponents,
			deployAPIServerProxy,
			deployShootSystemResources,
			deployNodeExporter,
			deployMetricsServer,
			waitUntilVPNShootReady,
			deployNodeProblemDetector,
			deployBlackboxExporter,
			deployKubernetesDashboard,
			deployNginxIngressAddon,
		)
		reconcileStaticControlPlanePods = g.AddGroup(b.ReconcileStaticControlPlanePodsTaskGroup(false))

		scaleClusterAutoscalerToZero = g.Add(flow.Task{
			Name:         "Scaling down cluster autoscaler",
			Fn:           flow.TaskFn(b.ScaleClusterAutoscalerToZero).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       b.Shoot.IsWorkerless || !b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deployManagedResourceForGardenerNodeAgent),
		})
		_ = g.AddGroup(b.ReconcileMachineControllerManagerTaskGroup().WithDependencies(
			waitUntilInfrastructureReady,
			initializeShootClients,
			waitUntilOperatingSystemConfigReady,
			createNewServiceAccountSecrets,
			scaleClusterAutoscalerToZero,
			reconcileStaticControlPlanePods,
		))
		waitUntilWorkerReady = g.AddGroup(b.ReconcileWorkerTaskGroup(flowCtx.skipReadiness).WithDependencies(
			deployManagedResourceForGardenerNodeAgent,
			waitUntilKubeRootCAConfigMapsUpdated,
			waitUntilOperatingSystemConfigReady,
		))
		_ = g.Add(flow.Task{
			Name:         "Checking if we have dual-stack pod CIDRs in nodes",
			Fn:           b.CheckPodCIDRsInNodes,
			SkipIf:       b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(waitUntilWorkerReady),
		})
		_ = g.Add(flow.Task{
			Name:         "Scaling down machine-controller-manager",
			Fn:           flow.TaskFn(b.ScaleMachineControllerManagerToZero).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       b.Shoot.IsWorkerless || !b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(waitUntilWorkerReady),
		})

		deploySeedLogging = g.Add(flow.Task{
			Name:         "Deploying shoot logging stack in Seed",
			Fn:           flow.TaskFn(b.DeployLogging).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(initialValiDeployment, waitUntilGardenerResourceManagerReady).InsertIf(flowCtx.isHibernatingShootWithWorkers, waitUntilWorkerReady),
		})
		deployPlutonoForLogging = g.Add(flow.Task{
			Name:         "Reconciling Plutono for Shoot in Seed for the logging stack",
			Fn:           flow.TaskFn(b.DeployPlutono).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			SkipIf:       b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deploySeedLogging),
		})
		nginxLBReady = g.Add(flow.Task{
			Name:         "Waiting until nginx ingress LoadBalancer is ready",
			Fn:           b.WaitUntilNginxIngressServiceIsReady,
			SkipIf:       b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled || !v1beta1helper.NginxIngressEnabled(b.Shoot.GetInfo().Spec.Addons) || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(initializeShootClients, waitUntilWorkerReady, ensureShootClusterIdentity),
		})
		_ = g.Add(flow.Task{
			Name: "Deploying nginx ingress DNS record",
			Fn: func(ctx context.Context) error {
				if err := b.DeployOrDestroyIngressDNSRecord(ctx); err != nil {
					return err
				}
				return b.RemoveTaskAnnotation(ctx, v1beta1constants.ShootTaskDeployDNSRecordIngress)
			},
			SkipIf:       b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(nginxLBReady),
		})
		waitUntilTunnelConnectionExists = g.Add(flow.Task{
			Name:         "Waiting until the Kubernetes API server can connect to the Shoot workers",
			Fn:           b.WaitUntilTunnelConnectionExists,
			SkipIf:       b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled || flowCtx.skipReadiness || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(syncPointAllSystemComponentsDeployed, reconcileSystemComponents, waitUntilWorkerReady),
		})
		_ = g.Add(flow.Task{
			Name: "Waiting until all shoot worker nodes have updated the operating system config",
			Fn: func(ctx context.Context) error {
				return b.WaitUntilOperatingSystemConfigUpdatedForAllWorkerPools(ctx, false)
			},
			SkipIf:       b.Shoot.IsWorkerless || b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(waitUntilWorkerReady, waitUntilTunnelConnectionExists),
		})
		deployAlertmanager = g.Add(flow.Task{
			Name:         "Reconciling Shoot Alertmanager",
			Fn:           flow.TaskFn(b.DeployAlertManager).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			SkipIf:       b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(initializeShootClients, waitUntilTunnelConnectionExists, waitUntilWorkerReady).InsertIf(!flowCtx.hasNodesCIDR, waitUntilInfrastructureReady),
		})
		waitUntilAlertmanagerReconciled = g.Add(flow.Task{
			Name:         "Waiting until Shoot Alertmanager is reconciled",
			Fn:           b.WaitForAlertManager,
			SkipIf:       b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deployAlertmanager),
		})
		deployPrometheus = g.Add(flow.Task{
			Name:         "Reconciling Shoot Prometheus",
			Fn:           flow.TaskFn(b.DeployPrometheus).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			SkipIf:       b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(initializeShootClients, waitUntilTunnelConnectionExists, waitUntilWorkerReady).InsertIf(!flowCtx.hasNodesCIDR, waitUntilInfrastructureReady),
		})
		waitUntilPrometheusReconciled = g.Add(flow.Task{
			Name:         "Waiting until Shoot Prometheus is reconciled",
			Fn:           b.WaitForPrometheus,
			SkipIf:       b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deployPrometheus),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying control plane blackbox-exporter",
			Fn:           flow.TaskFn(b.ReconcileBlackboxExporterControlPlane).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			SkipIf:       b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(initializeShootClients, waitUntilTunnelConnectionExists, waitUntilWorkerReady).InsertIf(!flowCtx.hasNodesCIDR, waitUntilInfrastructureReady),
		})
		_ = g.Add(flow.Task{
			Name:         "Reconciling kube-state-metrics for Shoot in Seed for the monitoring stack",
			Fn:           flow.TaskFn(b.DeployKubeStateMetrics).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			SkipIf:       b.Shoot.IsWorkerless || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deployPrometheus, deployAlertmanager),
		})
		deployPlutonoForMonitoring = g.Add(flow.Task{
			Name:         "Reconciling Plutono for Shoot in Seed for the monitoring stack",
			Fn:           flow.TaskFn(b.DeployPlutono).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			SkipIf:       b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deployPrometheus, deployAlertmanager),
		})
		waitUntilPlutonoReconciled = g.Add(flow.Task{
			Name:         "Waiting until Plutono for Shoot in Seed is reconciled",
			Fn:           b.WaitForPlutono,
			SkipIf:       b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deployPlutonoForLogging, deployPlutonoForMonitoring),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying istio-basic-auth-server",
			Fn:           flow.TaskFn(b.Shoot.Components.ControlPlane.IstioBasicAuthServer.Deploy).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			SkipIf:       b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(waitUntilAlertmanagerReconciled, waitUntilPrometheusReconciled, waitUntilPlutonoReconciled),
		})

		hibernateControlPlane = g.Add(flow.Task{
			Name:         "Hibernating control plane",
			Fn:           flow.TaskFn(b.HibernateControlPlane).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			SkipIf:       !b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(initializeShootClients, deployPrometheus, deployAlertmanager, deploySeedLogging, waitUntilWorkerReady, waitUntilExtensionResourcesAfterKAPIReady, waitUntilEtcdScaledAfterRestore),
		})

		// logic is inverted here
		// extensions that are deployed before the kube-apiserver are hibernated after it
		hibernateExtensionResourcesAfterKAPIHibernation = g.Add(flow.Task{
			Name:         "Hibernating extension resources after kube-apiserver hibernation",
			Fn:           flow.TaskFn(b.DeployExtensionsBeforeKubeAPIServer).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       !b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(hibernateControlPlane),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until extension resources hibernated after kube-apiserver hibernation are ready",
			Fn:           b.Shoot.Components.Extensions.Extension.WaitBeforeKubeAPIServer,
			SkipIf:       flowCtx.skipReadiness || !b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(hibernateExtensionResourcesAfterKAPIHibernation),
		})
		_ = g.Add(flow.Task{
			Name:         "Destroying ingress domain DNS record if hibernated",
			Fn:           b.DestroyIngressDNSRecord,
			SkipIf:       !b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(hibernateControlPlane),
		})
		_ = g.Add(flow.Task{
			Name:         "Destroying external domain DNS record if hibernated",
			Fn:           b.DestroyExternalDNSRecord,
			SkipIf:       !b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(hibernateControlPlane),
		})
		_ = g.Add(flow.Task{
			Name:         "Destroying internal domain DNS record if hibernated",
			Fn:           b.DestroyInternalDNSRecord,
			SkipIf:       !b.Shoot.HibernationEnabled || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(hibernateControlPlane),
		})
		deleteStaleExtensionResources = g.Add(flow.Task{
			Name:         "Deleting stale extension resources",
			Fn:           flow.TaskFn(b.Shoot.Components.Extensions.Extension.DeleteStaleResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(initializeShootClients),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until stale extension resources are deleted",
			Fn:           b.Shoot.Components.Extensions.Extension.WaitCleanupStaleResources,
			SkipIf:       b.Shoot.HibernationEnabled || flowCtx.skipReadiness || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deleteStaleExtensionResources),
		})
		_ = g.Add(flow.Task{
			Name: "Restarting control plane pods",
			Fn: func(ctx context.Context) error {
				if err := b.RestartControlPlanePods(ctx); err != nil {
					return err
				}
				return b.RemoveTaskAnnotation(ctx, v1beta1constants.ShootTaskRestartControlPlanePods)
			},
			SkipIf:       !flowCtx.requestControlPlanePodsRestart || b.Shoot.IsSelfHosted(),
			Dependencies: flow.NewTaskIDs(deployKubeControllerManager, waitUntilControlPlaneReady),
		})
	)

	return nil
}
