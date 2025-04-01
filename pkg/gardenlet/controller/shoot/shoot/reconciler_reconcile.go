// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	"github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/shoot/helper"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/errors"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/gardener/secretsrotation"
	"github.com/gardener/gardener/pkg/utils/gardener/shootstate"
	"github.com/gardener/gardener/pkg/utils/gardener/tokenrequest"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	retryutils "github.com/gardener/gardener/pkg/utils/retry"
)

// runReconcileShootFlow reconciles the Shoot cluster.
// It receives an Operation object <o> which stores the Shoot object.
func (r *Reconciler) runReconcileShootFlow(ctx context.Context, o *operation.Operation, operationType gardencorev1beta1.LastOperationType) *v1beta1helper.WrappedLastErrors {
	// We create the botanists (which will do the actual work).
	var (
		botanist                *botanistpkg.Botanist
		err                     error
		isCopyOfBackupsRequired bool
		tasksWithErrors         []string

		isRestoring   = operationType == gardencorev1beta1.LastOperationTypeRestore
		skipReadiness = metav1.HasAnnotation(o.Shoot.GetInfo().ObjectMeta, v1beta1constants.AnnotationShootSkipReadiness)
	)

	for _, lastError := range o.Shoot.GetInfo().Status.LastErrors {
		if lastError.TaskID != nil {
			tasksWithErrors = append(tasksWithErrors, *lastError.TaskID)
		}
	}

	errorContext := errors.NewErrorContext(fmt.Sprintf("Shoot cluster %s", utils.IifString(isRestoring, "restoration", "reconciliation")), tasksWithErrors)

	err = errors.HandleErrors(errorContext,
		func(errorID string) error {
			o.CleanShootTaskError(ctx, errorID)
			return nil
		},
		nil,
		errors.ToExecute("Create botanist", func() error {
			return retryutils.UntilTimeout(ctx, 10*time.Second, 10*time.Minute, func(context.Context) (done bool, err error) {
				botanist, err = botanistpkg.New(ctx, o)
				if err != nil {
					return retryutils.MinorError(err)
				}
				return retryutils.Ok()
			})
		}),
		errors.ToExecute("Check required extensions", func() error {
			return botanist.WaitUntilRequiredExtensionsReady(ctx)
		}),
		errors.ToExecute("Check if copy of backups is required", func() error {
			isCopyOfBackupsRequired, err = botanist.IsCopyOfBackupsRequired(ctx)
			return err
		}),
	)
	if err != nil {
		return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), err)
	}

	const (
		defaultTimeout  = 30 * time.Second
		defaultInterval = 5 * time.Second
	)

	var (
		allowBackup                    = o.Seed.GetInfo().Spec.Backup != nil
		hasNodesCIDR                   = o.Shoot.GetInfo().Spec.Networking != nil && o.Shoot.GetInfo().Spec.Networking.Nodes != nil && (o.Shoot.GetInfo().Status.Networking != nil || skipReadiness)
		useDNS                         = botanist.ShootUsesDNS()
		generation                     = o.Shoot.GetInfo().Generation
		requestControlPlanePodsRestart = controllerutils.HasTask(o.Shoot.GetInfo().Annotations, v1beta1constants.ShootTaskRestartControlPlanePods)
		kubeProxyEnabled               = v1beta1helper.KubeProxyEnabled(o.Shoot.GetInfo().Spec.Kubernetes.KubeProxy)
		deployKubeAPIServerTaskTimeout = defaultTimeout
		shootSSHAccessEnabled          = v1beta1helper.ShootEnablesSSHAccess(o.Shoot.GetInfo())
		isRestoringHAControlPlane      = botanist.IsRestorePhase() && v1beta1helper.IsHAControlPlaneConfigured(o.Shoot.GetInfo())
	)

	// During the 'Preparing' phase of different rotation operations, components are deployed twice. Also, the
	// different deployment functions call the `Wait` method after the first deployment. Hence, we should use
	// the respective timeout in this case instead of the (too short) default timeout to prevent undesired and confusing
	// errors in the reconciliation flow.
	if v1beta1helper.GetShootETCDEncryptionKeyRotationPhase(o.Shoot.GetInfo().Status.Credentials) == gardencorev1beta1.RotationPreparing {
		deployKubeAPIServerTaskTimeout = kubeapiserver.TimeoutWaitForDeployment
	}

	var (
		deployExtensionAfterKAPIMsg = "Deploying extension resources after kube-apiserver"
		waitExtensionAfterKAPIMsg   = "Waiting until extension resources handled after kube-apiserver are ready"
	)
	if o.Shoot.HibernationEnabled {
		deployExtensionAfterKAPIMsg = "Hibernating extension resources before kube-apiserver hibernation"
		waitExtensionAfterKAPIMsg = "Waiting until extension resources hibernated before kube-apiserver hibernation are ready"
	}

	if hasNodesCIDR {
		networks, err := shoot.ToNetworks(o.Shoot.GetInfo(), o.Shoot.IsWorkerless)
		if err != nil {
			return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), err)
		}
		o.Shoot.Networks = networks
	}

	nodeAgentAuthorizerWebhookReady, err := botanist.IsGardenerResourceManagerReady(ctx)
	if err != nil {
		return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), err)
	}

	var (
		g = flow.NewGraph(fmt.Sprintf("Shoot cluster %s", utils.IifString(isRestoring, "restoration", "reconciliation")))

		deployNamespace = g.Add(flow.Task{
			Name: "Deploying Shoot namespace in Seed",
			Fn:   flow.TaskFn(botanist.DeploySeedNamespace).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})
		ensureShootClusterIdentity = g.Add(flow.Task{
			Name:         "Ensuring Shoot cluster identity",
			Fn:           flow.TaskFn(botanist.EnsureShootClusterIdentity).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployNamespace),
		})
		deployCloudProviderSecret = g.Add(flow.Task{
			Name:         "Deploying cloud provider account secret",
			Fn:           flow.TaskFn(botanist.DeployCloudProviderSecret).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(deployNamespace),
		})
		initializeSecretsManagement = g.Add(flow.Task{
			Name:         "Initializing secrets management",
			Fn:           flow.TaskFn(botanist.InitializeSecretsManagement).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployNamespace),
		})
		initialValiDeployment = g.Add(flow.Task{
			Name:         "Deploying initial shoot logging stack in Seed",
			Fn:           flow.TaskFn(botanist.DeployLogging).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployNamespace, initializeSecretsManagement),
		})
		// TODO(oliver-goetz): Remove this step when Gardener v1.115.0 is released.
		_ = g.Add(flow.Task{
			Name:         "Deploying Kubernetes API server ingress with trusted certificate in the Seed cluster",
			Fn:           botanist.DeployKubeAPIServerIngress,
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement),
		})
		deployReferencedResources = g.Add(flow.Task{
			Name:         "Deploying referenced resources",
			Fn:           flow.TaskFn(botanist.DeployReferencedResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployNamespace),
		})
		deployInfrastructure = g.Add(flow.Task{
			Name:         "Deploying Shoot infrastructure",
			Fn:           flow.TaskFn(botanist.DeployInfrastructure).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement, deployCloudProviderSecret, deployReferencedResources),
		})
		waitUntilInfrastructureReady = g.Add(flow.Task{
			Name: "Waiting until shoot infrastructure has been reconciled",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				if !skipReadiness {
					if err := botanist.WaitForInfrastructure(ctx); err != nil {
						return err
					}
				}
				return removeTaskAnnotation(ctx, o, generation, v1beta1constants.ShootTaskDeployInfrastructure)
			}),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(deployInfrastructure),
		})
		deployKubeAPIServerService = g.Add(flow.Task{
			Name:         "Deploying Kubernetes API server service in the Seed cluster",
			Fn:           flow.TaskFn(botanist.Shoot.Components.ControlPlane.KubeAPIServerService.Deploy).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployNamespace, ensureShootClusterIdentity).InsertIf(!hasNodesCIDR, waitUntilInfrastructureReady),
		})
		waitUntilKubeAPIServerServiceIsReady = g.Add(flow.Task{
			Name:         "Waiting until Kubernetes API server service in the Seed cluster has reported readiness",
			Fn:           botanist.Shoot.Components.ControlPlane.KubeAPIServerService.Wait,
			SkipIf:       o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(deployKubeAPIServerService),
		})
		_ = g.Add(flow.Task{
			Name:         "Ensuring advertised addresses for the Shoot",
			Fn:           botanist.UpdateAdvertisedAddresses,
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerServiceIsReady),
		})
		deployInternalDomainDNSRecord = g.Add(flow.Task{
			Name: "Deploying internal domain DNS record",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				if err := botanist.DeployOrDestroyInternalDNSRecord(ctx); err != nil {
					return err
				}
				return removeTaskAnnotation(ctx, o, generation, v1beta1constants.ShootTaskDeployDNSRecordInternal)
			}),
			SkipIf:       o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(deployReferencedResources, waitUntilKubeAPIServerServiceIsReady),
		})
		_ = g.Add(flow.Task{
			Name: "Deploying external domain DNS record",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				if err := botanist.DeployOrDestroyExternalDNSRecord(ctx); err != nil {
					return err
				}
				return removeTaskAnnotation(ctx, o, generation, v1beta1constants.ShootTaskDeployDNSRecordExternal)
			}),
			SkipIf:       o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(deployReferencedResources, waitUntilKubeAPIServerServiceIsReady),
		})
		deploySourceBackupEntry = g.Add(flow.Task{
			Name:   "Deploying source backup entry",
			Fn:     botanist.DeploySourceBackupEntry,
			SkipIf: !isCopyOfBackupsRequired,
		})
		waitUntilSourceBackupEntryInGardenReconciled = g.Add(flow.Task{
			Name:         "Waiting until the source backup entry has been reconciled",
			Fn:           botanist.Shoot.Components.SourceBackupEntry.Wait,
			SkipIf:       skipReadiness || !isCopyOfBackupsRequired,
			Dependencies: flow.NewTaskIDs(deploySourceBackupEntry),
		})
		deployBackupEntryInGarden = g.Add(flow.Task{
			Name:         "Deploying backup entry",
			Fn:           botanist.DeployBackupEntry,
			SkipIf:       !allowBackup,
			Dependencies: flow.NewTaskIDs(deployNamespace, waitUntilSourceBackupEntryInGardenReconciled),
		})
		waitUntilBackupEntryInGardenReconciled = g.Add(flow.Task{
			Name:         "Waiting until the backup entry has been reconciled",
			Fn:           botanist.Shoot.Components.BackupEntry.Wait,
			SkipIf:       skipReadiness || !allowBackup,
			Dependencies: flow.NewTaskIDs(deployBackupEntryInGarden),
		})
		copyEtcdBackups = g.Add(flow.Task{
			Name:         "Copying etcd backups to new seed's backup bucket",
			Fn:           botanist.DeployEtcdCopyBackupsTask,
			SkipIf:       !isCopyOfBackupsRequired,
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement, deployCloudProviderSecret, waitUntilBackupEntryInGardenReconciled, waitUntilSourceBackupEntryInGardenReconciled),
		})
		waitUntilEtcdBackupsCopied = g.Add(flow.Task{
			Name:         "Waiting until etcd backups are copied",
			Fn:           botanist.Shoot.Components.ControlPlane.EtcdCopyBackupsTask.Wait,
			SkipIf:       skipReadiness || !isCopyOfBackupsRequired,
			Dependencies: flow.NewTaskIDs(copyEtcdBackups),
		})
		_ = g.Add(flow.Task{
			Name:         "Destroying copy etcd backups task resource",
			Fn:           botanist.Shoot.Components.ControlPlane.EtcdCopyBackupsTask.Destroy,
			SkipIf:       !isCopyOfBackupsRequired,
			Dependencies: flow.NewTaskIDs(waitUntilEtcdBackupsCopied),
		})
		deployETCD = g.Add(flow.Task{
			Name:         "Deploying main and events etcd",
			Fn:           flow.TaskFn(botanist.DeployEtcd).RetryUntilTimeout(defaultInterval, helper.GetEtcdDeployTimeout(o.Shoot, defaultTimeout)),
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement, deployCloudProviderSecret, waitUntilBackupEntryInGardenReconciled, waitUntilEtcdBackupsCopied),
		})
		destroySourceBackupEntry = g.Add(flow.Task{
			Name:         "Destroying source backup entry",
			Fn:           botanist.DestroySourceBackupEntry,
			SkipIf:       !allowBackup || !botanist.IsRestorePhase(),
			Dependencies: flow.NewTaskIDs(deployETCD),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until source backup entry has been deleted",
			Fn:           botanist.Shoot.Components.SourceBackupEntry.WaitCleanup,
			SkipIf:       !allowBackup || skipReadiness || !botanist.IsRestorePhase(),
			Dependencies: flow.NewTaskIDs(destroySourceBackupEntry),
		})
		waitUntilEtcdReady = g.Add(flow.Task{
			Name:         "Waiting until main and event etcd report readiness",
			Fn:           botanist.WaitUntilEtcdsReady,
			SkipIf:       (!isRestoringHAControlPlane && o.Shoot.HibernationEnabled) || skipReadiness,
			Dependencies: flow.NewTaskIDs(deployETCD),
		})
		deployExtensionResourcesBeforeKAPI = g.Add(flow.Task{
			Name:         "Deploying extension resources before kube-apiserver",
			Fn:           flow.TaskFn(botanist.DeployExtensionsBeforeKubeAPIServer).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement, deployCloudProviderSecret, deployReferencedResources, waitUntilInfrastructureReady),
		})
		waitUntilExtensionResourcesBeforeKAPIReady = g.Add(flow.Task{
			Name:         "Waiting until extension resources handled before kube-apiserver are ready",
			Fn:           botanist.Shoot.Components.Extensions.Extension.WaitBeforeKubeAPIServer,
			SkipIf:       o.Shoot.HibernationEnabled || skipReadiness,
			Dependencies: flow.NewTaskIDs(deployExtensionResourcesBeforeKAPI),
		})
		deployKubeAPIServer = g.Add(flow.Task{
			Name: "Deploying Kubernetes API server",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.DeployKubeAPIServer(ctx, nodeAgentAuthorizerWebhookReady && features.DefaultFeatureGate.Enabled(features.NodeAgentAuthorizer))
			}).RetryUntilTimeout(defaultInterval, deployKubeAPIServerTaskTimeout),
			Dependencies: flow.NewTaskIDs(
				initializeSecretsManagement,
				deployETCD,
				waitUntilEtcdReady,
				waitUntilKubeAPIServerServiceIsReady,
				waitUntilExtensionResourcesBeforeKAPIReady,
			).InsertIf(!hasNodesCIDR, waitUntilInfrastructureReady),
		})
		waitUntilKubeAPIServerIsReady = g.Add(flow.Task{
			Name:         "Waiting until Kubernetes API server rolled out",
			Fn:           botanist.Shoot.Components.ControlPlane.KubeAPIServer.Wait,
			SkipIf:       o.Shoot.HibernationEnabled || skipReadiness,
			Dependencies: flow.NewTaskIDs(deployKubeAPIServer),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying Kubernetes API server service SNI settings in the Seed cluster",
			Fn:           flow.TaskFn(botanist.DeployKubeAPIServerSNI).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerIsReady),
		})
		scaleEtcdAfterRestore = g.Add(flow.Task{
			Name:         "Scaling main and events etcd after kube-apiserver is ready",
			Fn:           flow.TaskFn(botanist.ScaleUpETCD).RetryUntilTimeout(defaultInterval, helper.GetEtcdDeployTimeout(o.Shoot, defaultTimeout)),
			SkipIf:       !isRestoringHAControlPlane,
			Dependencies: flow.NewTaskIDs(waitUntilEtcdReady, waitUntilKubeAPIServerIsReady),
		})
		waitUntilEtcdScaledAfterRestore = g.Add(flow.Task{
			Name:         "Waiting until main and events etcd scaled up after kube-apiserver is ready",
			Fn:           flow.TaskFn(botanist.WaitUntilEtcdsReady),
			SkipIf:       !isRestoringHAControlPlane || skipReadiness,
			Dependencies: flow.NewTaskIDs(scaleEtcdAfterRestore),
		})
		deployGardenerResourceManager = g.Add(flow.Task{
			Name:         "Deploying gardener-resource-manager",
			Fn:           flow.TaskFn(botanist.DeployGardenerResourceManager).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerIsReady),
		})
		waitUntilGardenerResourceManagerReady = g.Add(flow.Task{
			Name:         "Waiting until gardener-resource-manager reports readiness",
			Fn:           botanist.Shoot.Components.ControlPlane.ResourceManager.Wait,
			SkipIf:       o.Shoot.HibernationEnabled || skipReadiness,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager),
		})
		// TODO(oliver-goetz): Consider removing this two-step deployment once we only support Kubernetes 1.32+ (in this
		//  version, the structured authorization feature has been promoted to GA). We already use structured authz for
		//  1.30+ clusters. This is similar to kube-apiserver deployment in gardener-operator.
		//  See https://github.com/gardener/gardener/pull/10682#discussion_r1816324389 for more information.
		deployKubeAPIServerWithNodeAgentAuthorizer = g.Add(flow.Task{
			Name: "Deploying Kubernetes API server with node-agent-authorizer",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.DeployKubeAPIServer(ctx, true)
			}).RetryUntilTimeout(defaultInterval, deployKubeAPIServerTaskTimeout),
			SkipIf:       !features.DefaultFeatureGate.Enabled(features.NodeAgentAuthorizer) || nodeAgentAuthorizerWebhookReady,
			Dependencies: flow.NewTaskIDs(waitUntilGardenerResourceManagerReady),
		})
		waitUntilKubeAPIServerWithNodeAgentAuthorizerIsReady = g.Add(flow.Task{
			Name:         "Waiting until Kubernetes API server with node-agent-authorizer rolled out",
			Fn:           botanist.Shoot.Components.ControlPlane.KubeAPIServer.Wait,
			SkipIf:       !features.DefaultFeatureGate.Enabled(features.NodeAgentAuthorizer) || o.Shoot.HibernationEnabled || nodeAgentAuthorizerWebhookReady || skipReadiness,
			Dependencies: flow.NewTaskIDs(deployKubeAPIServerWithNodeAgentAuthorizer),
		})
		_ = g.Add(flow.Task{
			Name: "Renewing shoot access secrets after creation of new ServiceAccount signing key",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return tokenrequest.RenewAccessSecrets(ctx, o.SeedClientSet.Client(),
					client.InNamespace(o.Shoot.ControlPlaneNamespace),
					client.MatchingLabels{resourcesv1alpha1.ResourceManagerClass: resourcesv1alpha1.ResourceManagerClassShoot},
				)
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf: !sets.New(
				gardencorev1beta1.RotationPreparing,
				gardencorev1beta1.RotationPreparingWithoutWorkersRollout,
			).Has(v1beta1helper.GetShootServiceAccountKeyRotationPhase(o.Shoot.GetInfo().Status.Credentials)),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerWithNodeAgentAuthorizerIsReady, waitUntilGardenerResourceManagerReady),
		})
		deployControlPlane = g.Add(flow.Task{
			Name:         "Deploying shoot control plane components",
			Fn:           flow.TaskFn(botanist.DeployControlPlane).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerWithNodeAgentAuthorizerIsReady, waitUntilGardenerResourceManagerReady),
		})
		waitUntilControlPlaneReady = g.Add(flow.Task{
			Name: "Waiting until shoot control plane has been reconciled",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.ControlPlane.Wait(ctx)
			}),
			SkipIf:       o.Shoot.IsWorkerless || skipReadiness,
			Dependencies: flow.NewTaskIDs(deployControlPlane),
		})
		deploySeedLogging = g.Add(flow.Task{
			Name:         "Deploying shoot logging stack in Seed",
			Fn:           flow.TaskFn(botanist.DeployLogging).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initialValiDeployment, waitUntilGardenerResourceManagerReady),
		})
		deployShootNamespaces = g.Add(flow.Task{
			Name:         "Deploying shoot namespaces system component",
			Fn:           flow.TaskFn(botanist.Shoot.Components.SystemComponents.Namespaces.Deploy).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager),
		})
		waitUntilShootNamespacesReady = g.Add(flow.Task{
			Name:         "Waiting until shoot namespaces have been reconciled",
			Fn:           botanist.Shoot.Components.SystemComponents.Namespaces.Wait,
			SkipIf:       o.Shoot.HibernationEnabled || skipReadiness,
			Dependencies: flow.NewTaskIDs(waitUntilGardenerResourceManagerReady, deployShootNamespaces),
		})
		deployVPNSeedServer = g.Add(flow.Task{
			Name:         "Deploying vpn-seed-server",
			Fn:           flow.TaskFn(botanist.DeployVPNServer).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement, deployNamespace, waitUntilKubeAPIServerWithNodeAgentAuthorizerIsReady),
		})
		deployControlPlaneExposure = g.Add(flow.Task{
			Name:         "Deploying shoot control plane exposure components",
			Fn:           flow.TaskFn(botanist.DeployControlPlaneExposure).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless || useDNS,
			Dependencies: flow.NewTaskIDs(deployReferencedResources, waitUntilKubeAPIServerWithNodeAgentAuthorizerIsReady),
		})
		waitUntilControlPlaneExposureReady = g.Add(flow.Task{
			Name: "Waiting until Shoot control plane exposure has been reconciled",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.ControlPlaneExposure.Wait(ctx)
			}),
			SkipIf:       o.Shoot.IsWorkerless || useDNS || skipReadiness,
			Dependencies: flow.NewTaskIDs(deployControlPlaneExposure),
		})
		destroyControlPlaneExposure = g.Add(flow.Task{
			Name: "Destroying shoot control plane exposure",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.ControlPlaneExposure.Destroy(ctx)
			}),
			SkipIf:       o.Shoot.IsWorkerless || !useDNS,
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerWithNodeAgentAuthorizerIsReady),
		})
		waitUntilControlPlaneExposureDeleted = g.Add(flow.Task{
			Name: "Waiting until shoot control plane exposure has been destroyed",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.ControlPlaneExposure.WaitCleanup(ctx)
			}),
			SkipIf:       o.Shoot.IsWorkerless || !useDNS,
			Dependencies: flow.NewTaskIDs(destroyControlPlaneExposure),
		})
		deployGardenerAccess = g.Add(flow.Task{
			Name:         "Deploying Gardener shoot access resources",
			Fn:           flow.TaskFn(botanist.Shoot.Components.GardenerAccess.Deploy).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement, waitUntilGardenerResourceManagerReady),
		})
		initializeShootClients = g.Add(flow.Task{
			Name:         "Initializing connection to Shoot",
			Fn:           flow.TaskFn(botanist.InitializeDesiredShootClients).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerWithNodeAgentAuthorizerIsReady, waitUntilControlPlaneExposureReady, waitUntilControlPlaneExposureDeleted, deployInternalDomainDNSRecord, deployGardenerAccess),
		})
		_ = g.Add(flow.Task{
			Name: "Sync public service account signing keys to Garden cluster",
			Fn:   botanist.SyncPublicServiceAccountKeys,
			SkipIf: o.Shoot.HibernationEnabled ||
				!v1beta1helper.HasManagedIssuer(botanist.Shoot.GetInfo()),
			Dependencies: flow.NewTaskIDs(initializeShootClients),
		})
		rewriteResourcesAddLabel = g.Add(flow.Task{
			Name: "Labeling resources after modification of encryption config or to encrypt them with new ETCD encryption key",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return secretsrotation.RewriteEncryptedDataAddLabel(ctx, o.Logger, o.SeedClientSet.Client(), o.ShootClientSet, o.SecretsManager, o.Shoot.ControlPlaneNamespace, v1beta1constants.DeploymentNameKubeAPIServer, o.Shoot.ResourcesToEncrypt, o.Shoot.EncryptedResources, gardenerutils.DefaultGVKsForEncryption())
			}).RetryUntilTimeout(30*time.Second, 10*time.Minute),
			SkipIf: v1beta1helper.GetShootETCDEncryptionKeyRotationPhase(o.Shoot.GetInfo().Status.Credentials) != gardencorev1beta1.RotationPreparing &&
				apiequality.Semantic.DeepEqual(o.Shoot.ResourcesToEncrypt, o.Shoot.EncryptedResources),
			Dependencies: flow.NewTaskIDs(initializeShootClients),
		})
		snapshotETCD = g.Add(flow.Task{
			Name: "Snapshotting ETCD after modification of encryption config or resources are re-encrypted with new ETCD encryption key",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return secretsrotation.SnapshotETCDAfterRewritingEncryptedData(ctx, o.SeedClientSet.Client(), botanist.SnapshotEtcd, o.Shoot.ControlPlaneNamespace, v1beta1constants.DeploymentNameKubeAPIServer)
			}),
			SkipIf: !allowBackup ||
				(v1beta1helper.GetShootETCDEncryptionKeyRotationPhase(o.Shoot.GetInfo().Status.Credentials) != gardencorev1beta1.RotationPreparing &&
					apiequality.Semantic.DeepEqual(o.Shoot.ResourcesToEncrypt, o.Shoot.EncryptedResources)),
			Dependencies: flow.NewTaskIDs(rewriteResourcesAddLabel),
		})
		_ = g.Add(flow.Task{
			Name: "Removing label from resources after modification of encryption config or rotation of ETCD encryption key",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				if err := secretsrotation.RewriteEncryptedDataRemoveLabel(ctx, o.Logger, o.SeedClientSet.Client(), o.ShootClientSet, o.Shoot.ControlPlaneNamespace, v1beta1constants.DeploymentNameKubeAPIServer, o.Shoot.ResourcesToEncrypt, o.Shoot.EncryptedResources, gardenerutils.DefaultGVKsForEncryption()); err != nil {
					return err
				}

				if !apiequality.Semantic.DeepEqual(o.Shoot.ResourcesToEncrypt, o.Shoot.EncryptedResources) {
					if err := o.Shoot.UpdateInfoStatus(ctx, o.GardenClient, true, func(shoot *gardencorev1beta1.Shoot) error {
						var encryptedResources []string
						if o.Shoot.GetInfo().Spec.Kubernetes.KubeAPIServer != nil {
							encryptedResources = shared.GetResourcesForEncryptionFromConfig(o.Shoot.GetInfo().Spec.Kubernetes.KubeAPIServer.EncryptionConfig)
						}

						shoot.Status.EncryptedResources = encryptedResources
						return nil
					}); err != nil {
						return err
					}
				}

				return nil
			}).RetryUntilTimeout(30*time.Second, 10*time.Minute),
			SkipIf: v1beta1helper.GetShootETCDEncryptionKeyRotationPhase(o.Shoot.GetInfo().Status.Credentials) != gardencorev1beta1.RotationCompleting &&
				apiequality.Semantic.DeepEqual(o.Shoot.ResourcesToEncrypt, o.Shoot.EncryptedResources),
			Dependencies: flow.NewTaskIDs(initializeShootClients, snapshotETCD),
		})
		deployKubeScheduler = g.Add(flow.Task{
			Name: "Deploying Kubernetes scheduler",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.ControlPlane.KubeScheduler.Deploy(ctx)
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement, waitUntilGardenerResourceManagerReady),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying Kubernetes vertical pod autoscaler",
			Fn:           flow.TaskFn(botanist.DeployVerticalPodAutoscaler).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement, waitUntilGardenerResourceManagerReady),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying dependency-watchdog shoot access resources",
			Fn:           flow.TaskFn(botanist.DeployDependencyWatchdogAccess).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement, waitUntilGardenerResourceManagerReady),
		})
		deployKubeControllerManager = g.Add(flow.Task{
			Name:         "Deploying Kubernetes controller manager",
			Fn:           flow.TaskFn(botanist.DeployKubeControllerManager).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement, deployCloudProviderSecret, waitUntilKubeAPIServerWithNodeAgentAuthorizerIsReady),
		})
		waitUntilKubeControllerManagerReady = g.Add(flow.Task{
			Name: "Waiting until kube-controller-manager reports readiness",
			Fn:   botanist.Shoot.Components.ControlPlane.KubeControllerManager.Wait,
			SkipIf: skipReadiness || !sets.New(
				gardencorev1beta1.RotationPreparing,
				gardencorev1beta1.RotationPreparingWithoutWorkersRollout,
			).Has(v1beta1helper.GetShootServiceAccountKeyRotationPhase(o.Shoot.GetInfo().Status.Credentials)),
			Dependencies: flow.NewTaskIDs(deployKubeControllerManager),
		})
		createNewServiceAccountSecrets = g.Add(flow.Task{
			Name: "Creating new ServiceAccount secrets after creation of new signing key",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return secretsrotation.CreateNewServiceAccountSecrets(ctx, o.Logger, o.ShootClientSet.Client(), o.SecretsManager)
			}).RetryUntilTimeout(30*time.Second, 10*time.Minute),
			SkipIf: !sets.New(
				gardencorev1beta1.RotationPreparing,
				gardencorev1beta1.RotationPreparingWithoutWorkersRollout,
			).Has(v1beta1helper.GetShootServiceAccountKeyRotationPhase(o.Shoot.GetInfo().Status.Credentials)),
			Dependencies: flow.NewTaskIDs(initializeShootClients, waitUntilKubeControllerManagerReady),
		})
		_ = g.Add(flow.Task{
			Name: "Deleting old ServiceAccount secrets after rotation of signing key",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return secretsrotation.DeleteOldServiceAccountSecrets(ctx, o.Logger, o.ShootClientSet.Client(), o.Shoot.GetInfo().Status.Credentials.Rotation.ServiceAccountKey.LastInitiationFinishedTime.Time)
			}).RetryUntilTimeout(30*time.Second, 10*time.Minute),
			SkipIf:       v1beta1helper.GetShootServiceAccountKeyRotationPhase(o.Shoot.GetInfo().Status.Credentials) != gardencorev1beta1.RotationCompleting,
			Dependencies: flow.NewTaskIDs(initializeShootClients, waitUntilKubeControllerManagerReady),
		})
		deleteBastions = g.Add(flow.Task{
			Name:         "Deleting Bastions",
			Fn:           botanist.DeleteBastions,
			SkipIf:       shootSSHAccessEnabled,
			Dependencies: flow.NewTaskIDs(deployReferencedResources, waitUntilInfrastructureReady, waitUntilControlPlaneReady),
		})
		deployExtensionResourcesAfterKAPI = g.Add(flow.Task{
			Name:         deployExtensionAfterKAPIMsg,
			Fn:           flow.TaskFn(botanist.DeployExtensionsAfterKubeAPIServer).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployReferencedResources, initializeShootClients),
		})
		waitUntilExtensionResourcesAfterKAPIReady = g.Add(flow.Task{
			Name:         waitExtensionAfterKAPIMsg,
			Fn:           botanist.Shoot.Components.Extensions.Extension.WaitAfterKubeAPIServer,
			SkipIf:       skipReadiness,
			Dependencies: flow.NewTaskIDs(deployExtensionResourcesAfterKAPI),
		})
		deployOperatingSystemConfig = g.Add(flow.Task{
			Name:         "Deploying operating system specific configuration for shoot workers",
			Fn:           flow.TaskFn(botanist.DeployOperatingSystemConfig).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(deployReferencedResources, waitUntilInfrastructureReady, waitUntilControlPlaneReady, deleteBastions, waitUntilExtensionResourcesAfterKAPIReady),
		})
		waitUntilOperatingSystemConfigReady = g.Add(flow.Task{
			Name: "Waiting until operating system configurations for worker nodes have been reconciled",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.OperatingSystemConfig.Wait(ctx)
			}),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(deployOperatingSystemConfig),
		})
		deleteStaleOperatingSystemConfigResources = g.Add(flow.Task{
			Name: "Delete stale operating system config resources",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.OperatingSystemConfig.DeleteStaleResources(ctx)
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(deployOperatingSystemConfig),
		})
		_ = g.Add(flow.Task{
			Name: "Waiting until stale operating system config resources are deleted",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.OperatingSystemConfig.WaitCleanupStaleResources(ctx)
			}),
			SkipIf:       o.Shoot.IsWorkerless || o.Shoot.HibernationEnabled || skipReadiness,
			Dependencies: flow.NewTaskIDs(deleteStaleOperatingSystemConfigResources),
		})
		deployNetwork = g.Add(flow.Task{
			Name:         "Deploying shoot network plugin",
			Fn:           flow.TaskFn(botanist.DeployNetwork).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(deployReferencedResources, waitUntilGardenerResourceManagerReady, waitUntilOperatingSystemConfigReady, deployKubeScheduler, waitUntilShootNamespacesReady),
		})
		waitUntilNetworkIsReady = g.Add(flow.Task{
			Name: "Waiting until shoot network plugin has been reconciled",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.Network.Wait(ctx)
			}),
			SkipIf:       o.Shoot.IsWorkerless || skipReadiness,
			Dependencies: flow.NewTaskIDs(deployNetwork),
		})
		_ = g.Add(flow.Task{
			Name: "Deploying shoot cluster identity",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.DeployClusterIdentity(ctx)
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager, ensureShootClusterIdentity, waitUntilOperatingSystemConfigReady),
		})
		deployShootSystemResources = g.Add(flow.Task{
			Name:         "Deploying shoot system resources",
			Fn:           flow.TaskFn(botanist.DeployShootSystem).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(waitUntilGardenerResourceManagerReady, initializeShootClients, waitUntilOperatingSystemConfigReady, waitUntilShootNamespacesReady),
		})
		deployCoreDNS = g.Add(flow.Task{
			Name: "Deploying CoreDNS system component",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				if err := botanist.DeployCoreDNS(ctx); err != nil {
					return err
				}
				if controllerutils.HasTask(o.Shoot.GetInfo().Annotations, v1beta1constants.ShootTaskRestartCoreAddons) {
					return removeTaskAnnotation(ctx, o, generation, v1beta1constants.ShootTaskRestartCoreAddons)
				}
				return nil
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless || o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(waitUntilGardenerResourceManagerReady, initializeShootClients, waitUntilOperatingSystemConfigReady, deployKubeScheduler, waitUntilShootNamespacesReady),
		})
		deployNodeLocalDNS = g.Add(flow.Task{
			Name:         "Reconcile node-local-dns system component",
			Fn:           flow.TaskFn(botanist.ReconcileNodeLocalDNS),
			SkipIf:       o.Shoot.IsWorkerless || o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager, initializeShootClients, waitUntilOperatingSystemConfigReady, deployKubeScheduler, waitUntilShootNamespacesReady, waitUntilNetworkIsReady),
		})
		deployMetricsServer = g.Add(flow.Task{
			Name: "Deploying metrics-server system component",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.SystemComponents.MetricsServer.Deploy(ctx)
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless || o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(waitUntilGardenerResourceManagerReady, waitUntilOperatingSystemConfigReady, deployKubeScheduler, waitUntilShootNamespacesReady),
		})
		deployVPNShoot = g.Add(flow.Task{
			Name: "Deploying vpn-shoot system component",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.SystemComponents.VPNShoot.Deploy(ctx)
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless || o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(waitUntilGardenerResourceManagerReady, deployGardenerResourceManager, deployKubeScheduler, deployVPNSeedServer, waitUntilShootNamespacesReady),
		})
		deployNodeProblemDetector = g.Add(flow.Task{
			Name: "Deploying node-problem-detector system component",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.SystemComponents.NodeProblemDetector.Deploy(ctx)
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless || o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager, waitUntilOperatingSystemConfigReady, waitUntilShootNamespacesReady),
		})
		deployKubeProxy = g.Add(flow.Task{
			Name:         "Deploying kube-proxy system component",
			Fn:           flow.TaskFn(botanist.DeployKubeProxy).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless || o.Shoot.HibernationEnabled || !kubeProxyEnabled,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager, initializeShootClients, ensureShootClusterIdentity, deployKubeScheduler, waitUntilShootNamespacesReady),
		})
		_ = g.Add(flow.Task{
			Name: "Deleting stale kube-proxy DaemonSets",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.SystemComponents.KubeProxy.DeleteStaleResources(ctx)
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless || o.Shoot.HibernationEnabled || !kubeProxyEnabled,
			Dependencies: flow.NewTaskIDs(deployKubeProxy),
		})
		_ = g.Add(flow.Task{
			Name: "Deleting kube-proxy system component",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.SystemComponents.KubeProxy.Destroy(ctx)
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless || o.Shoot.HibernationEnabled || kubeProxyEnabled,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager, initializeShootClients, ensureShootClusterIdentity, deployKubeScheduler),
		})
		deployAPIServerProxy = g.Add(flow.Task{
			Name:         "Deploying apiserver-proxy",
			Fn:           flow.TaskFn(botanist.DeployAPIServerProxy).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(waitUntilGardenerResourceManagerReady, initializeShootClients, ensureShootClusterIdentity, deployKubeScheduler, waitUntilShootNamespacesReady),
		})
		deployBlackboxExporter = g.Add(flow.Task{
			Name:         "Deploying blackbox-exporter",
			Fn:           flow.TaskFn(botanist.ReconcileBlackboxExporterCluster).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless || o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(waitUntilGardenerResourceManagerReady, initializeShootClients, ensureShootClusterIdentity, deployKubeScheduler, waitUntilShootNamespacesReady),
		})
		deployNodeExporter = g.Add(flow.Task{
			Name: "Deploying node-exporter",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.ReconcileNodeExporter(ctx)
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless || o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(waitUntilGardenerResourceManagerReady, initializeShootClients, ensureShootClusterIdentity, deployKubeScheduler, waitUntilShootNamespacesReady),
		})
		deployKubernetesDashboard = g.Add(flow.Task{
			Name:         "Deploying addon Kubernetes Dashboard",
			Fn:           flow.TaskFn(botanist.DeployKubernetesDashboard).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless || o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(waitUntilGardenerResourceManagerReady, initializeShootClients, ensureShootClusterIdentity, deployKubeScheduler, waitUntilShootNamespacesReady),
		})
		deployNginxIngressAddon = g.Add(flow.Task{
			Name:         "Deploying addon Nginx Ingress Controller",
			Fn:           flow.TaskFn(botanist.DeployNginxIngressAddon).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless || o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(waitUntilGardenerResourceManagerReady, initializeShootClients, ensureShootClusterIdentity, deployKubeScheduler, waitUntilShootNamespacesReady),
		})
		deployManagedResourceForGardenerNodeAgent = g.Add(flow.Task{
			Name:         "Deploying managed resources for the gardener-node-agent",
			Fn:           flow.TaskFn(botanist.DeployManagedResourceForGardenerNodeAgent).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless || o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager, ensureShootClusterIdentity, waitUntilOperatingSystemConfigReady),
		})

		syncPointAllSystemComponentsDeployed = flow.NewTaskIDs(
			waitUntilNetworkIsReady,
			deployAPIServerProxy,
			deployShootSystemResources,
			deployCoreDNS,
			deployNodeExporter,
			deployNodeLocalDNS,
			deployMetricsServer,
			deployVPNShoot,
			deployNodeProblemDetector,
			deployKubeProxy,
			deployBlackboxExporter,
			deployKubernetesDashboard,
			deployNginxIngressAddon,
		)

		scaleClusterAutoscalerToZero = g.Add(flow.Task{
			Name:         "Scaling down cluster autoscaler",
			Fn:           flow.TaskFn(botanist.ScaleClusterAutoscalerToZero).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless || !o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(deployManagedResourceForGardenerNodeAgent),
		})
		deployMachineControllerManager = g.Add(flow.Task{
			Name:         "Deploying machine-controller-manager",
			Fn:           flow.TaskFn(botanist.DeployMachineControllerManager),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(deployCloudProviderSecret, deployReferencedResources, waitUntilInfrastructureReady, initializeShootClients, waitUntilOperatingSystemConfigReady, waitUntilNetworkIsReady, createNewServiceAccountSecrets, scaleClusterAutoscalerToZero),
		})
		deployWorker = g.Add(flow.Task{
			Name:         "Configuring shoot worker pools",
			Fn:           flow.TaskFn(botanist.DeployWorker).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(deployMachineControllerManager),
		})
		waitUntilWorkerStatusUpdate = g.Add(flow.Task{
			Name: "Waiting until worker resource status is updated with latest machine deployments",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.Worker.WaitUntilWorkerStatusMachineDeploymentsUpdated(ctx)
			}),
			SkipIf:       o.Shoot.IsWorkerless || o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(deployWorker),
		})
		deployExtensionResourcesAfterWorker = g.Add(flow.Task{
			Name:         "Deploying extension resources after workers",
			Fn:           flow.TaskFn(botanist.DeployExtensionsAfterWorker).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(waitUntilWorkerStatusUpdate),
		})
		deployClusterAutoscaler = g.Add(flow.Task{
			Name:         "Deploying cluster autoscaler",
			Fn:           flow.TaskFn(botanist.DeployClusterAutoscaler).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless || o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(waitUntilWorkerStatusUpdate, deployManagedResourceForGardenerNodeAgent),
		})
		waitUntilWorkerReady = g.Add(flow.Task{
			Name: "Waiting until shoot worker nodes have been reconciled",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.Worker.Wait(ctx)
			}),
			SkipIf:       o.Shoot.IsWorkerless || skipReadiness,
			Dependencies: flow.NewTaskIDs(deployWorker, waitUntilWorkerStatusUpdate, deployManagedResourceForGardenerNodeAgent),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until extension resources handled after workers are ready",
			Fn:           botanist.Shoot.Components.Extensions.Extension.WaitAfterWorker,
			SkipIf:       o.Shoot.IsWorkerless || skipReadiness,
			Dependencies: flow.NewTaskIDs(deployExtensionResourcesAfterWorker),
		})
		_ = g.Add(flow.Task{
			Name:         "Scaling down machine-controller-manager",
			Fn:           flow.TaskFn(botanist.ScaleMachineControllerManagerToZero).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless || !o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(waitUntilWorkerReady),
		})

		_ = g.Add(flow.Task{
			Name:         "Reconciling Plutono for Shoot in Seed for the logging stack",
			Fn:           flow.TaskFn(botanist.DeployPlutono).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			Dependencies: flow.NewTaskIDs(deploySeedLogging),
		})
		nginxLBReady = g.Add(flow.Task{
			Name:         "Waiting until nginx ingress LoadBalancer is ready",
			Fn:           botanist.WaitUntilNginxIngressServiceIsReady,
			SkipIf:       o.Shoot.IsWorkerless || o.Shoot.HibernationEnabled || !v1beta1helper.NginxIngressEnabled(botanist.Shoot.GetInfo().Spec.Addons),
			Dependencies: flow.NewTaskIDs(initializeShootClients, waitUntilWorkerReady, ensureShootClusterIdentity),
		})
		_ = g.Add(flow.Task{
			Name: "Deploying nginx ingress DNS record",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				if err := botanist.DeployOrDestroyIngressDNSRecord(ctx); err != nil {
					return err
				}
				return removeTaskAnnotation(ctx, o, generation, v1beta1constants.ShootTaskDeployDNSRecordIngress)
			}),
			SkipIf:       o.Shoot.IsWorkerless || o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(nginxLBReady),
		})
		waitUntilTunnelConnectionExists = g.Add(flow.Task{
			Name:         "Waiting until the Kubernetes API server can connect to the Shoot workers",
			Fn:           botanist.WaitUntilTunnelConnectionExists,
			SkipIf:       o.Shoot.IsWorkerless || o.Shoot.HibernationEnabled || skipReadiness,
			Dependencies: flow.NewTaskIDs(syncPointAllSystemComponentsDeployed, waitUntilNetworkIsReady, waitUntilWorkerReady),
		})
		waitUntilOperatingSystemConfigUpdated = g.Add(flow.Task{
			Name:         "Waiting until all shoot worker nodes have updated the operating system config",
			Fn:           botanist.WaitUntilOperatingSystemConfigUpdatedForAllWorkerPools,
			SkipIf:       o.Shoot.IsWorkerless || o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(waitUntilWorkerReady, waitUntilTunnelConnectionExists),
		})
		// TODO(oliver-goetz): Remove this when removing NodeAgentAuthorizer feature gate.
		_ = g.Add(flow.Task{
			Name: "Delete gardener-node-agent shoot access secret because NodeAgentAuthorizer feature gate is enabled",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return deleteGardenerNodeAgentShootAccess(ctx, o)
			}),
			SkipIf:       !features.DefaultFeatureGate.Enabled(features.NodeAgentAuthorizer) || o.Shoot.IsWorkerless || o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(waitUntilOperatingSystemConfigUpdated),
		})
		deployAlertmanager = g.Add(flow.Task{
			Name:         "Reconciling Shoot Alertmanager",
			Fn:           flow.TaskFn(botanist.DeployAlertManager).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			Dependencies: flow.NewTaskIDs(initializeShootClients, waitUntilTunnelConnectionExists, waitUntilWorkerReady).InsertIf(!hasNodesCIDR, waitUntilInfrastructureReady),
		})
		deployPrometheus = g.Add(flow.Task{
			Name:         "Reconciling Shoot Prometheus",
			Fn:           flow.TaskFn(botanist.DeployPrometheus).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			Dependencies: flow.NewTaskIDs(initializeShootClients, waitUntilTunnelConnectionExists, waitUntilWorkerReady).InsertIf(!hasNodesCIDR, waitUntilInfrastructureReady),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying control plane blackbox-exporter",
			Fn:           flow.TaskFn(botanist.ReconcileBlackboxExporterControlPlane).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			Dependencies: flow.NewTaskIDs(initializeShootClients, waitUntilTunnelConnectionExists, waitUntilWorkerReady).InsertIf(!hasNodesCIDR, waitUntilInfrastructureReady),
		})
		_ = g.Add(flow.Task{
			Name:         "Reconciling kube-state-metrics for Shoot in Seed for the monitoring stack",
			Fn:           flow.TaskFn(botanist.DeployKubeStateMetrics).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(deployPrometheus, deployAlertmanager),
		})
		_ = g.Add(flow.Task{
			Name:         "Reconciling Plutono for Shoot in Seed for the monitoring stack",
			Fn:           flow.TaskFn(botanist.DeployPlutono).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			Dependencies: flow.NewTaskIDs(deployPrometheus, deployAlertmanager),
		})

		hibernateControlPlane = g.Add(flow.Task{
			Name:         "Hibernating control plane",
			Fn:           flow.TaskFn(botanist.HibernateControlPlane).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			SkipIf:       !o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(initializeShootClients, deployPrometheus, deployAlertmanager, deploySeedLogging, deployClusterAutoscaler, waitUntilWorkerReady, waitUntilExtensionResourcesAfterKAPIReady, waitUntilEtcdScaledAfterRestore),
		})

		// logic is inverted here
		// extensions that are deployed before the kube-apiserver are hibernated after it
		hibernateExtensionResourcesAfterKAPIHibernation = g.Add(flow.Task{
			Name:         "Hibernating extension resources after kube-apiserver hibernation",
			Fn:           flow.TaskFn(botanist.DeployExtensionsBeforeKubeAPIServer).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       !o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(hibernateControlPlane),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until extension resources hibernated after kube-apiserver hibernation are ready",
			Fn:           botanist.Shoot.Components.Extensions.Extension.WaitBeforeKubeAPIServer,
			SkipIf:       skipReadiness || !o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(hibernateExtensionResourcesAfterKAPIHibernation),
		})
		_ = g.Add(flow.Task{
			Name:         "Destroying ingress domain DNS record if hibernated",
			Fn:           botanist.DestroyIngressDNSRecord,
			SkipIf:       !o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(hibernateControlPlane),
		})
		_ = g.Add(flow.Task{
			Name:         "Destroying external domain DNS record if hibernated",
			Fn:           botanist.DestroyExternalDNSRecord,
			SkipIf:       !o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(hibernateControlPlane),
		})
		_ = g.Add(flow.Task{
			Name:         "Destroying internal domain DNS record if hibernated",
			Fn:           botanist.DestroyInternalDNSRecord,
			SkipIf:       !o.Shoot.HibernationEnabled,
			Dependencies: flow.NewTaskIDs(hibernateControlPlane),
		})
		deleteStaleExtensionResources = g.Add(flow.Task{
			Name:         "Deleting stale extension resources",
			Fn:           flow.TaskFn(botanist.Shoot.Components.Extensions.Extension.DeleteStaleResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeShootClients),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until stale extension resources are deleted",
			Fn:           botanist.Shoot.Components.Extensions.Extension.WaitCleanupStaleResources,
			SkipIf:       o.Shoot.HibernationEnabled || skipReadiness,
			Dependencies: flow.NewTaskIDs(deleteStaleExtensionResources),
		})
		deployContainerRuntimeResources = g.Add(flow.Task{
			Name:         "Deploying container runtime resources",
			Fn:           flow.TaskFn(botanist.DeployContainerRuntime).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(deployReferencedResources, initializeShootClients),
		})
		_ = g.Add(flow.Task{
			Name: "Waiting until container runtime resources are ready",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.ContainerRuntime.Wait(ctx)
			}),
			SkipIf:       o.Shoot.IsWorkerless || skipReadiness,
			Dependencies: flow.NewTaskIDs(deployContainerRuntimeResources),
		})
		deleteStaleContainerRuntimeResources = g.Add(flow.Task{
			Name: "Deleting stale container runtime resources",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.ContainerRuntime.DeleteStaleResources(ctx)
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(initializeShootClients),
		})
		_ = g.Add(flow.Task{
			Name: "Waiting until stale container runtime resources are deleted",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.ContainerRuntime.WaitCleanupStaleResources(ctx)
			}),
			SkipIf:       o.Shoot.IsWorkerless || o.Shoot.HibernationEnabled || skipReadiness,
			Dependencies: flow.NewTaskIDs(deleteStaleContainerRuntimeResources),
		})
		_ = g.Add(flow.Task{
			Name: "Restarting control plane pods",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				if err := botanist.RestartControlPlanePods(ctx); err != nil {
					return err
				}
				return removeTaskAnnotation(ctx, o, generation, v1beta1constants.ShootTaskRestartControlPlanePods)
			}),
			SkipIf:       !requestControlPlanePodsRestart,
			Dependencies: flow.NewTaskIDs(deployKubeControllerManager, deployControlPlane, deployControlPlaneExposure),
		})
	)

	f := g.Compile()

	if err := f.Run(ctx, flow.Opts{
		Log:              o.Logger,
		ProgressReporter: r.newProgressReporter(o.ReportShootProgress),
		ErrorContext:     errorContext,
		ErrorCleaner:     o.CleanShootTaskError,
	}); err != nil {
		return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), flow.Errors(err))
	}

	o.Logger.Info("Cleaning no longer required secrets")
	if err := botanist.SecretsManager.Cleanup(ctx); err != nil {
		err = fmt.Errorf("failed to clean no longer required secrets: %w", err)
		return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), err)
	}

	if !r.ShootStateControllerEnabled && botanist.IsRestorePhase() {
		o.Logger.Info("Deleting Shoot State after successful restoration")
		if err := shootstate.Delete(ctx, botanist.GardenClient, botanist.Shoot.GetInfo()); err != nil {
			err = fmt.Errorf("failed to delete shoot state: %w", err)
			return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), err)
		}
	}

	// ensure that shoot client is invalidated after it has been hibernated
	if o.Shoot.HibernationEnabled {
		if err := o.ShootClientMap.InvalidateClient(keys.ForShoot(o.Shoot.GetInfo())); err != nil {
			err = fmt.Errorf("failed to invalidate shoot client: %w", err)
			return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), err)
		}
	}

	if _, ok := o.Shoot.GetInfo().Annotations[v1beta1constants.AnnotationShootSkipReadiness]; ok {
		o.Logger.Info("Removing skip-readiness annotation")

		if err := o.Shoot.UpdateInfo(ctx, o.GardenClient, false, func(shoot *gardencorev1beta1.Shoot) error {
			delete(shoot.Annotations, v1beta1constants.AnnotationShootSkipReadiness)
			return nil
		}); err != nil {
			return nil
		}
	}

	o.Logger.Info("Successfully reconciled Shoot cluster", "operation", utils.IifString(isRestoring, "restored", "reconciled"))
	return nil
}

func removeTaskAnnotation(ctx context.Context, o *operation.Operation, generation int64, tasksToRemove ...string) error {
	// Check if shoot generation was changed mid-air, i.e., whether we need to wait for the next reconciliation until we
	// can safely remove the task annotations to ensure all required tasks are executed.
	shoot := &gardencorev1beta1.Shoot{}
	if err := o.GardenClient.Get(ctx, client.ObjectKeyFromObject(o.Shoot.GetInfo()), shoot); err != nil {
		return err
	}

	if shoot.Generation != generation {
		return nil
	}

	return o.Shoot.UpdateInfo(ctx, o.GardenClient, false, func(shoot *gardencorev1beta1.Shoot) error {
		controllerutils.RemoveTasks(shoot.Annotations, tasksToRemove...)
		return nil
	})
}

// TODO(oliver-goetz): Remove this when removing NodeAgentAuthorizer feature gate.
func deleteGardenerNodeAgentShootAccess(ctx context.Context, o *operation.Operation) error {
	if err := kubernetesutils.DeleteObject(
		ctx,
		o.SeedClientSet.Client(),
		gardenerutils.NewShootAccessSecret(nodeagentconfigv1alpha1.AccessSecretName, o.Shoot.ControlPlaneNamespace).Secret,
	); err != nil {
		return fmt.Errorf("failed to delete gardener-node-agent shoot access secret in control plane namespace in seed: %w", err)
	}

	return kubernetesutils.DeleteObjects(
		ctx,
		o.ShootClientSet.Client(),
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodeagentconfigv1alpha1.AccessSecretName,
				Namespace: metav1.NamespaceSystem,
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodeagentconfigv1alpha1.AccessSecretName,
				Namespace: metav1.NamespaceSystem,
			},
		},
	)
}
