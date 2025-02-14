// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/component-base/version"
	podsecurityadmissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	"github.com/gardener/gardener/imagevector"
	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/operator/v1alpha1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	"github.com/gardener/gardener/pkg/component/extensions/dnsrecord"
	gardenerapiserver "github.com/gardener/gardener/pkg/component/gardener/apiserver"
	gardenerdashboard "github.com/gardener/gardener/pkg/component/gardener/dashboard"
	"github.com/gardener/gardener/pkg/component/gardener/resourcemanager"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus"
	gardenprometheus "github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/garden"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/gardener/secretsrotation"
	"github.com/gardener/gardener/pkg/utils/gardener/tokenrequest"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/timewindow"
)

func (r *Reconciler) reconcile(
	ctx context.Context,
	log logr.Logger,
	garden *operatorv1alpha1.Garden,
	secretsManager secretsmanager.Interface,
	targetVersion *semver.Version,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(garden, operatorv1alpha1.FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.RuntimeClientSet.Client(), garden, operatorv1alpha1.FinalizerName); err != nil {
			return reconcile.Result{}, err
		}
	}

	// VPA is a prerequisite. If it's enabled then we deploy the CRD (and later also the related components) as part of
	// the flow. However, when it's disabled then we check whether it is indeed available (and fail, otherwise).
	if !vpaEnabled(garden.Spec.RuntimeCluster.Settings) {
		if _, err := r.RuntimeClientSet.Client().RESTMapper().RESTMapping(schema.GroupKind{Group: "autoscaling.k8s.io", Kind: "VerticalPodAutoscaler"}); err != nil {
			return reconcile.Result{}, fmt.Errorf("VPA is required for runtime cluster but CRD is not installed: %s", err)
		}
	}

	// create + label garden namespace
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: r.GardenNamespace}}
	log.Info("Labeling and annotating namespace", "namespaceName", namespace.Name)
	if _, err := controllerutils.CreateOrGetAndMergePatch(ctx, r.RuntimeClientSet.Client(), namespace, func() error {
		metav1.SetMetaDataLabel(&namespace.ObjectMeta, podsecurityadmissionapi.EnforceLevelLabel, string(podsecurityadmissionapi.LevelPrivileged))
		metav1.SetMetaDataLabel(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigConsider, "true")
		metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigZones, strings.Join(garden.Spec.RuntimeCluster.Provider.Zones, ","))
		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Generating CA certificates for runtime and virtual clusters")
	for _, config := range caCertConfigurations() {
		if _, err := secretsManager.Generate(ctx, config, caCertGenerateOptionsFor(config.GetName(), helper.GetCARotationPhase(garden.Status.Credentials))...); err != nil {
			return reconcile.Result{}, err
		}
	}

	wildcardCert, err := gardenerutils.GetGardenWildcardCertificate(ctx, r.RuntimeClientSet.Client())
	if err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Instantiating component deployers")
	enableSeedAuthorizer, err := r.enableSeedAuthorizer(ctx)
	if err != nil {
		return reconcile.Result{}, err
	}

	c, err := r.instantiateComponents(ctx, log, garden, secretsManager, targetVersion, kubernetes.NewApplier(r.RuntimeClientSet.Client(), r.RuntimeClientSet.Client().RESTMapper()), wildcardCert, enableSeedAuthorizer)
	if err != nil {
		return reconcile.Result{}, err
	}

	// TODO (martinweindel) Temporary flag. Remove it again, when all provider extensions supporting BackupBucket are deployable on Garden runtime cluster.
	hasExtensionForBackupBucket, err := r.hasExtensionForBackupBucket(ctx, garden)
	if err != nil {
		return reconcile.Result{}, err
	}

	const (
		secretsTypeKey                 = "secretsType"
		secretsTypeGardenAccess        = "garden access"
		secretsTypeWorkloadIdentity    = "workload identity"
		secretsTypeGardenletKubeconfig = "gardenlet kubeconfig" // #nosec G101 -- No credential.

		defaultTimeout  = 30 * time.Second
		defaultInterval = 5 * time.Second
	)

	var (
		allowBackup             = garden.Spec.VirtualCluster.ETCD != nil && garden.Spec.VirtualCluster.ETCD.Main != nil && garden.Spec.VirtualCluster.ETCD.Main.Backup != nil
		virtualClusterClientSet kubernetes.Interface
		virtualClusterClient    client.Client
		defaultEncryptedGVKs    = append(gardenerutils.DefaultGardenerGVKsForEncryption(), gardenerutils.DefaultGVKsForEncryption()...)
		resourcesToEncrypt      = append(shared.NormalizeResources(getKubernetesResourcesForEncryption(garden)), getGardenerResourcesForEncryption(garden)...)
		encryptedResources      = shared.NormalizeResources(garden.Status.EncryptedResources)

		g                              = flow.NewGraph("Garden reconciliation")
		generateGenericTokenKubeconfig = g.Add(flow.Task{
			Name: "Generating generic token kubeconfig",
			Fn: func(ctx context.Context) error {
				return r.generateGenericTokenKubeconfig(ctx, garden, secretsManager)
			},
		})
		generateObservabilityIngressPassword = g.Add(flow.Task{
			Name: "Generating observability ingress password",
			Fn: func(ctx context.Context) error {
				return r.generateObservabilityIngressPassword(ctx, secretsManager)
			},
		})

		deployEtcdCRD = g.Add(flow.Task{
			Name: "Deploying ETCD-related custom resource definitions",
			Fn:   c.etcdCRD.Deploy,
		})
		deployVPACRD = g.Add(flow.Task{
			Name:   "Deploying custom resource definitions for VPA",
			Fn:     c.vpaCRD.Deploy,
			SkipIf: !vpaEnabled(garden.Spec.RuntimeCluster.Settings),
		})
		deployIstioCRD = g.Add(flow.Task{
			Name: "Deploying custom resource definitions for Istio",
			Fn:   c.istioCRD.Deploy,
		})
		deployFluentCRD = g.Add(flow.Task{
			Name: "Deploying custom resource definitions for fluent-operator",
			Fn:   c.fluentCRD.Deploy,
		})
		deployPrometheusCRD = g.Add(flow.Task{
			Name: "Deploying custom resource definitions for prometheus-operator",
			Fn:   component.OpWait(c.prometheusCRD).Deploy,
		})
		deployExtensionCRD = g.Add(flow.Task{
			Name: "Deploying custom resource definitions for extensions",
			Fn:   c.extensionCRD.Deploy,
		})

		_ = g.Add(flow.Task{
			Name: "Deploying VPA for gardener-operator",
			Fn: func(ctx context.Context) error {
				return gardenerutils.ReconcileVPAForGardenerComponent(ctx, r.RuntimeClientSet.Client(), v1beta1constants.DeploymentNameGardenerOperator, r.GardenNamespace)
			},
			Dependencies: flow.NewTaskIDs(deployVPACRD),
		})
		_ = g.Add(flow.Task{
			Name: "Deploying ServiceMonitor for gardener-operator",
			Fn: func(ctx context.Context) error {
				return r.deployOperatorServiceMonitor(ctx)
			},
			Dependencies: flow.NewTaskIDs(deployPrometheusCRD),
		})
		deployGardenerResourceManager = g.Add(flow.Task{
			Name:         "Deploying and waiting for gardener-resource-manager to be healthy",
			Fn:           component.OpWait(c.gardenerResourceManager).Deploy,
			Dependencies: flow.NewTaskIDs(deployEtcdCRD, deployVPACRD, deployIstioCRD),
		})
		deployNginxIngressController = g.Add(flow.Task{
			Name:         "Deploying nginx-ingress controller",
			Fn:           c.nginxIngressController.Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager),
		})
		deployRuntimeSystemResources = g.Add(flow.Task{
			Name:         "Deploying runtime system resources",
			Fn:           c.runtimeSystem.Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager),
		})
		deployVPA = g.Add(flow.Task{
			Name:         "Deploying Kubernetes vertical pod autoscaler",
			Fn:           c.verticalPodAutoscaler.Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager),
		})
		deployEtcdDruid = g.Add(flow.Task{
			Name:         "Deploying ETCD Druid",
			Fn:           component.OpWait(c.etcdDruid).Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager),
		})
		deployIstio = g.Add(flow.Task{
			Name:         "Deploying Istio",
			Fn:           component.OpWait(c.istio).Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager),
		})
		_ = g.Add(flow.Task{
			Name:         "Reconciling DNSRecords for virtual garden cluster and ingress controller",
			Fn:           func(ctx context.Context) error { return r.reconcileDNSRecords(ctx, log, garden) },
			SkipIf:       garden.Spec.DNS == nil,
			Dependencies: flow.NewTaskIDs(deployIstio),
		})
		syncPointSystemComponents = flow.NewTaskIDs(
			generateGenericTokenKubeconfig,
			generateObservabilityIngressPassword,
			deployRuntimeSystemResources,
			deployFluentCRD,
			deployPrometheusCRD,
			deployExtensionCRD,
			deployVPA,
			deployEtcdDruid,
			deployIstio,
			deployNginxIngressController,
		)

		backupBucket = etcdMainBackupBucket(garden)

		deployEtcdBackupBucket = g.Add(flow.Task{
			Name: "Reconciling main ETCD backup bucket",
			Fn: func(ctx context.Context) error {
				if err := r.deployEtcdMainBackupBucket(ctx, garden, backupBucket); err != nil {
					return err
				}

				return extensions.WaitUntilExtensionObjectReady(
					ctx,
					r.RuntimeClientSet.Client(),
					log,
					backupBucket,
					extensionsv1alpha1.BackupBucketResource,
					2*time.Second,
					30*time.Second,
					time.Minute,
					nil,
				)
			},
			SkipIf: garden.Spec.VirtualCluster.ETCD == nil ||
				garden.Spec.VirtualCluster.ETCD.Main == nil ||
				garden.Spec.VirtualCluster.ETCD.Main.Backup == nil ||
				!hasExtensionForBackupBucket,
			Dependencies: flow.NewTaskIDs(deployExtensionCRD),
		})
		deployEtcds = g.Add(flow.Task{
			Name:         "Deploying main and events ETCDs of virtual garden",
			Fn:           r.deployEtcdsFunc(garden, c.etcdMain, c.etcdEvents, backupBucket),
			Dependencies: flow.NewTaskIDs(syncPointSystemComponents, deployEtcdBackupBucket),
		})
		waitUntilEtcdsReady = g.Add(flow.Task{
			Name:         "Waiting until main and event ETCDs report readiness",
			Fn:           flow.Parallel(c.etcdMain.Wait, c.etcdEvents.Wait),
			Dependencies: flow.NewTaskIDs(deployEtcds),
		})
		deployExtensionResourcesBeforeKAPI = g.Add(flow.Task{
			Name:         "Deploying extension resources before kube-apiserver",
			Fn:           flow.TaskFn(c.extensions.DeployBeforeKubeAPIServer).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployExtensionCRD),
		})
		waitUntilExtensionResourcesBeforeKAPIReady = g.Add(flow.Task{
			Name:         "Waiting until extension resources handled before kube-apiserver are ready",
			Fn:           c.extensions.WaitBeforeKubeAPIServer,
			Dependencies: flow.NewTaskIDs(deployExtensionResourcesBeforeKAPI),
		})
		deployKubeAPIServerService = g.Add(flow.Task{
			Name:         "Deploying and waiting for kube-apiserver service in the runtime cluster",
			Fn:           component.OpWait(c.kubeAPIServerService).Deploy,
			Dependencies: flow.NewTaskIDs(syncPointSystemComponents),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying Kubernetes API server service SNI",
			Fn:           c.kubeAPIServerSNI.Deploy,
			Dependencies: flow.NewTaskIDs(deployKubeAPIServerService),
		})
		deployKubeAPIServer = g.Add(flow.Task{
			Name:         "Deploying Kubernetes API Server",
			Fn:           r.deployKubeAPIServerFunc(garden, c.kubeAPIServer),
			Dependencies: flow.NewTaskIDs(waitUntilEtcdsReady, waitUntilExtensionResourcesBeforeKAPIReady),
		})
		waitUntilKubeAPIServerIsReady = g.Add(flow.Task{
			Name:         "Waiting until Kubernetes API server rolled out",
			Fn:           c.kubeAPIServer.Wait,
			Dependencies: flow.NewTaskIDs(deployKubeAPIServer),
		})
		deployKubeControllerManager = g.Add(flow.Task{
			Name: "Deploying Kubernetes Controller Manager",
			Fn: func(ctx context.Context) error {
				c.kubeControllerManager.SetReplicaCount(1)
				c.kubeControllerManager.SetRuntimeConfig(c.kubeAPIServer.GetValues().RuntimeConfig)
				var services []net.IPNet
				for _, svc := range garden.Spec.VirtualCluster.Networking.Services {
					_, cidr, err := net.ParseCIDR(svc)
					if err != nil {
						return fmt.Errorf("cannot parse service network CIDR '%s': %w", svc, err)
					}
					services = append(services, *cidr)
				}
				c.kubeControllerManager.SetServiceNetworks(services)
				return component.OpWait(c.kubeControllerManager).Deploy(ctx)
			},
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerIsReady),
		})
		deployVirtualGardenGardenerResourceManager = g.Add(flow.Task{
			Name:         "Deploying gardener-resource-manager for virtual garden",
			Fn:           r.deployVirtualGardenGardenerResourceManager(secretsManager, c.virtualGardenGardenerResourceManager),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerIsReady),
		})
		waitUntilVirtualGardenGardenerResourceManagerIsReady = g.Add(flow.Task{
			Name:         "Waiting until gardener-resource-manager for virtual garden rolled out",
			Fn:           c.virtualGardenGardenerResourceManager.Wait,
			Dependencies: flow.NewTaskIDs(deployVirtualGardenGardenerResourceManager),
		})

		deployGardenerAPIServer = g.Add(flow.Task{
			Name:         "Deploying Gardener API Server",
			Fn:           r.deployGardenerAPIServerFunc(garden, c.gardenerAPIServer),
			Dependencies: flow.NewTaskIDs(waitUntilEtcdsReady, waitUntilKubeAPIServerIsReady, waitUntilVirtualGardenGardenerResourceManagerIsReady),
		})
		waitUntilGardenerAPIServerReady = g.Add(flow.Task{
			Name:         "Waiting until Gardener API server rolled out",
			Fn:           c.gardenerAPIServer.Wait,
			Dependencies: flow.NewTaskIDs(deployGardenerAPIServer),
		})
		deployGardenerAdmissionController = g.Add(flow.Task{
			Name:         "Deploying Gardener Admission Controller",
			Fn:           component.OpWait(c.gardenerAdmissionController).Deploy,
			Dependencies: flow.NewTaskIDs(waitUntilGardenerAPIServerReady),
		})
		deployGardenerControllerManager = g.Add(flow.Task{
			Name:         "Deploying Gardener Controller Manager",
			Fn:           component.OpWait(c.gardenerControllerManager).Deploy,
			Dependencies: flow.NewTaskIDs(waitUntilGardenerAPIServerReady),
		})
		deployGardenerScheduler = g.Add(flow.Task{
			Name:         "Deploying Gardener Scheduler",
			Fn:           component.OpWait(c.gardenerScheduler).Deploy,
			Dependencies: flow.NewTaskIDs(waitUntilGardenerAPIServerReady),
		})
		_ = g.Add(flow.Task{
			Name: "Deploying Gardener Discovery Server",
			Fn: func(ctx context.Context) error {
				if garden.Spec.VirtualCluster.Gardener.DiscoveryServer == nil {
					return component.OpDestroyAndWait(c.gardenerDiscoveryServer).Destroy(ctx)
				}
				return component.OpWait(c.gardenerDiscoveryServer).Deploy(ctx)
			},
			Dependencies: flow.NewTaskIDs(waitUntilGardenerAPIServerReady),
		})

		_ = g.Add(flow.Task{
			Name:         "Deploying virtual system resources",
			Fn:           c.virtualSystem.Deploy,
			Dependencies: flow.NewTaskIDs(deployVirtualGardenGardenerResourceManager),
		})
		deployVirtualGardenGardenerAccess = g.Add(flow.Task{
			Name:         "Deploying resources for gardener-operator access to virtual garden",
			Fn:           component.OpWait(c.virtualGardenGardenerAccess).Deploy,
			Dependencies: flow.NewTaskIDs(waitUntilVirtualGardenGardenerResourceManagerIsReady),
		})
		renewVirtualClusterAccess = g.Add(flow.Task{
			Name: "Renewing virtual garden access secrets after creation of new ServiceAccount signing key",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return tokenrequest.RenewAccessSecrets(ctx, r.RuntimeClientSet.Client(),
					client.InNamespace(r.GardenNamespace),
					client.MatchingLabels{resourcesv1alpha1.ResourceManagerClass: resourcesv1alpha1.ResourceManagerClassShoot},
				)
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       helper.GetServiceAccountKeyRotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPreparing,
			Dependencies: flow.NewTaskIDs(deployKubeControllerManager, deployVirtualGardenGardenerAccess, deployGardenerAPIServer, deployGardenerAdmissionController, deployGardenerControllerManager, deployGardenerScheduler),
		})
		initializeVirtualClusterClient = g.Add(flow.Task{
			Name: "Initializing connection to virtual garden cluster",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				virtualClusterClientSet, err = r.GardenClientMap.GetClient(ctx, keys.ForGarden(garden))
				if err != nil {
					return err
				}
				virtualClusterClient = virtualClusterClientSet.Client()
				return nil
			}).
				RetryUntilTimeout(time.Second, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployKubeAPIServerService, deployVirtualGardenGardenerAccess, renewVirtualClusterAccess),
		})
		_ = g.Add(flow.Task{
			Name: "Deploy gardener-info ConfigMap",
			Fn: func(ctx context.Context) error {
				return reconcileGardenerInfoConfigMap(ctx, log, virtualClusterClient, secretsManager, workloadIdentityTokenIssuerURL(garden))
			},
			Dependencies: flow.NewTaskIDs(waitUntilGardenerAPIServerReady, initializeVirtualClusterClient),
		})
		deployExtensionResources = g.Add(flow.Task{
			Name:         "Deploying extension resources",
			Fn:           flow.Parallel(c.extensions.DeployAfterKubeAPIServer, c.extensions.DeployAfterWorker).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeVirtualClusterClient),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until extension resources are ready",
			Fn:           flow.Parallel(c.extensions.WaitAfterKubeAPIServer, c.extensions.WaitAfterWorker),
			Dependencies: flow.NewTaskIDs(deployExtensionResources),
		})
		deleteStaleExtensionResources = g.Add(flow.Task{
			Name:         "Deleting stale extension resources",
			Fn:           flow.TaskFn(c.extensions.DeleteStaleResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(initializeVirtualClusterClient),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until stale extension resources are deleted",
			Fn:           c.extensions.WaitCleanupStaleResources,
			Dependencies: flow.NewTaskIDs(deleteStaleExtensionResources),
		})
		_ = g.Add(flow.Task{
			Name: "Reconciling Gardener Dashboard",
			Fn: func(ctx context.Context) error {
				return r.deployGardenerDashboard(ctx, c.gardenerDashboard, garden, secretsManager, virtualClusterClient)
			},
			Dependencies: flow.NewTaskIDs(waitUntilGardenerAPIServerReady, initializeVirtualClusterClient),
		})
		_ = g.Add(flow.Task{
			Name:         "Reconciling Gardener Dashboard web terminal controller manager",
			Fn:           c.terminalControllerManager.Deploy,
			Dependencies: flow.NewTaskIDs(waitUntilGardenerAPIServerReady),
		})

		// Renew seed secrets tasks must run sequentially. They all use "gardener.cloud/operation" annotation of the seeds and there can be only one annotation at the same time.
		renewGardenAccessSecretsInAllSeeds = g.Add(flow.Task{
			Name: "Label seeds to trigger renewal of their garden access secrets",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return secretsrotation.RenewGardenSecretsInAllSeeds(ctx, log.WithValues(secretsTypeKey, secretsTypeGardenAccess), virtualClusterClient, v1beta1constants.SeedOperationRenewGardenAccessSecrets)
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       helper.GetServiceAccountKeyRotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPreparing,
			Dependencies: flow.NewTaskIDs(initializeVirtualClusterClient),
		})
		checkIfGardenAccessSecretsRenewalCompletedInAllSeeds = g.Add(flow.Task{
			Name: "Check if all seeds finished the renewal of their garden access secrets",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return secretsrotation.CheckIfGardenSecretsRenewalCompletedInAllSeeds(ctx, virtualClusterClient, v1beta1constants.SeedOperationRenewGardenAccessSecrets, secretsTypeGardenAccess)
			}).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			SkipIf:       helper.GetServiceAccountKeyRotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPreparing,
			Dependencies: flow.NewTaskIDs(renewGardenAccessSecretsInAllSeeds),
		})
		renewWorkloadIdentityTokensInAllSeeds = g.Add(flow.Task{
			Name: "Annotate seeds to trigger renewal of workload identity tokens",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return secretsrotation.RenewGardenSecretsInAllSeeds(ctx, log.WithValues(secretsTypeKey, secretsTypeWorkloadIdentity), virtualClusterClient, v1beta1constants.SeedOperationRenewWorkloadIdentityTokens)
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       helper.GetWorkloadIdentityKeyRotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPreparing,
			Dependencies: flow.NewTaskIDs(initializeVirtualClusterClient, waitUntilGardenerAPIServerReady),
		})
		_ = g.Add(flow.Task{
			Name: "Check if all seeds finished the renewal of their workload identity tokens",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return secretsrotation.CheckIfGardenSecretsRenewalCompletedInAllSeeds(ctx, virtualClusterClient, v1beta1constants.SeedOperationRenewWorkloadIdentityTokens, secretsTypeWorkloadIdentity)
			}).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			SkipIf:       helper.GetWorkloadIdentityKeyRotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPreparing,
			Dependencies: flow.NewTaskIDs(renewWorkloadIdentityTokensInAllSeeds),
		})
		renewGardenletKubeconfigInAllSeeds = g.Add(flow.Task{
			Name: "Label seeds to trigger renewal of their gardenlet kubeconfig",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return secretsrotation.RenewGardenSecretsInAllSeeds(ctx, log.WithValues(secretsTypeKey, secretsTypeGardenletKubeconfig), virtualClusterClient, v1beta1constants.GardenerOperationRenewKubeconfig)
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       helper.GetCARotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPreparing,
			Dependencies: flow.NewTaskIDs(checkIfGardenAccessSecretsRenewalCompletedInAllSeeds),
		})
		_ = g.Add(flow.Task{
			Name: "Check if all seeds finished the renewal of their gardenlet kubeconfig",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return secretsrotation.CheckIfGardenSecretsRenewalCompletedInAllSeeds(ctx, virtualClusterClient, v1beta1constants.GardenerOperationRenewKubeconfig, secretsTypeGardenletKubeconfig)
			}).RetryUntilTimeout(defaultInterval, 2*time.Minute),
			SkipIf:       helper.GetCARotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPreparing,
			Dependencies: flow.NewTaskIDs(renewGardenletKubeconfigInAllSeeds),
		})
		rewriteResourcesAddLabel = g.Add(flow.Task{
			Name: "Labeling encrypted resources after modification of encryption config or to re-encrypt them with new ETCD encryption key",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return secretsrotation.RewriteEncryptedDataAddLabel(ctx, log, r.RuntimeClientSet.Client(), virtualClusterClientSet, secretsManager, r.GardenNamespace, namePrefix+v1beta1constants.DeploymentNameKubeAPIServer, resourcesToEncrypt, encryptedResources, defaultEncryptedGVKs)
			}).RetryUntilTimeout(30*time.Second, 10*time.Minute),
			SkipIf: helper.GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPreparing &&
				apiequality.Semantic.DeepEqual(resourcesToEncrypt, encryptedResources),
			Dependencies: flow.NewTaskIDs(initializeVirtualClusterClient, waitUntilGardenerAPIServerReady),
		})
		snapshotETCD = g.Add(flow.Task{
			Name: "Snapshotting ETCD after modification of encryption config or resources are re-encrypted with new ETCD encryption key",
			Fn: func(ctx context.Context) error {
				return secretsrotation.SnapshotETCDAfterRewritingEncryptedData(ctx, r.RuntimeClientSet.Client(), r.snapshotETCDFunc(secretsManager, c.etcdMain), r.GardenNamespace, namePrefix+v1beta1constants.DeploymentNameKubeAPIServer)
			},
			SkipIf: !allowBackup ||
				(helper.GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPreparing &&
					apiequality.Semantic.DeepEqual(resourcesToEncrypt, encryptedResources)),
			Dependencies: flow.NewTaskIDs(rewriteResourcesAddLabel),
		})
		_ = g.Add(flow.Task{
			Name: "Removing label from re-encrypted resources after modification of encryption config or rotation of ETCD encryption key",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				if err := secretsrotation.RewriteEncryptedDataRemoveLabel(ctx, log, r.RuntimeClientSet.Client(), virtualClusterClientSet, r.GardenNamespace, namePrefix+v1beta1constants.DeploymentNameKubeAPIServer, resourcesToEncrypt, encryptedResources, defaultEncryptedGVKs); err != nil {
					return err
				}

				if !apiequality.Semantic.DeepEqual(resourcesToEncrypt, encryptedResources) {
					encryptedResources := append(getKubernetesResourcesForEncryption(garden), getGardenerResourcesForEncryption(garden)...)

					patch := client.MergeFrom(garden.DeepCopy())
					garden.Status.EncryptedResources = encryptedResources
					if err := r.RuntimeClientSet.Client().Status().Patch(ctx, garden, patch); err != nil {
						return fmt.Errorf("error patching Garden status after snapshotting ETCD: %w", err)
					}
				}

				return nil
			}).RetryUntilTimeout(30*time.Second, 10*time.Minute),
			SkipIf: helper.GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationCompleting &&
				apiequality.Semantic.DeepEqual(resourcesToEncrypt, encryptedResources),
			Dependencies: flow.NewTaskIDs(initializeVirtualClusterClient, waitUntilGardenerAPIServerReady, snapshotETCD),
		})

		_ = g.Add(flow.Task{
			Name:         "Deploying fluent-operator",
			Fn:           c.fluentOperator.Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager, deployFluentCRD),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying fluent-operator CustomResources",
			Fn:           c.fluentOperatorCustomResources.Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager, deployFluentCRD),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying fluent-bit",
			Fn:           c.fluentBit.Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager, deployFluentCRD),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying Vali",
			Fn:           c.vali.Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager),
		})

		_ = g.Add(flow.Task{
			Name:         "Deploying prometheus-operator",
			Fn:           c.prometheusOperator.Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager, deployPrometheusCRD),
		})
		_ = g.Add(flow.Task{
			Name: "Deploying Alertmanager",
			Fn: func(ctx context.Context) error {
				credentialsSecret, found := secretsManager.Get(v1beta1constants.SecretNameObservabilityIngress)
				if !found {
					return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameObservabilityIngress)
				}

				c.alertManager.SetIngressAuthSecret(credentialsSecret)
				return c.alertManager.Deploy(ctx)
			},
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager, deployPrometheusCRD),
		})
		deployPrometheusGarden = g.Add(flow.Task{
			Name: "Deploying Garden Prometheus",
			Fn: func(ctx context.Context) error {
				return r.deployGardenPrometheus(ctx, log, secretsManager, c.prometheusGarden, virtualClusterClient)
			},
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager, deployPrometheusCRD, waitUntilGardenerAPIServerReady, initializeVirtualClusterClient),
		})
		_ = g.Add(flow.Task{
			Name: "Deploying long-term Prometheus",
			Fn: func(ctx context.Context) error {
				return r.deployLongTermPrometheus(ctx, secretsManager, c.prometheusLongTerm)
			},
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager, deployPrometheusCRD, deployPrometheusGarden),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying blackbox-exporter",
			Fn:           c.blackboxExporter.Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager, waitUntilKubeAPIServerIsReady, deployPrometheusGarden),
		})

		_ = g.Add(flow.Task{
			Name:         "Deploying Kube State Metrics",
			Fn:           c.kubeStateMetrics.Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager, waitUntilKubeAPIServerIsReady),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying Gardener Metrics Exporter",
			Fn:           c.gardenerMetricsExporter.Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager, waitUntilKubeAPIServerIsReady, waitUntilGardenerAPIServerReady),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying Plutono",
			Fn:           c.plutono.Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager),
		})
	)

	gardenCopy := garden.DeepCopy()
	if err := g.Compile().Run(ctx, flow.Opts{
		Log:              log,
		ProgressReporter: r.reportProgress(log, gardenCopy),
	}); err != nil {
		return reconcile.Result{}, flow.Errors(err)
	}
	*garden = *gardenCopy

	if !enableSeedAuthorizer {
		log.Info("Triggering a second reconciliation to enable seed authorizer feature")
		return reconcile.Result{Requeue: true}, nil
	}

	if err := r.updateHelmChartRefForGardenlets(ctx, log, virtualClusterClient); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed updating the Helm chart references in Gardenlet resources: %w", err)
	}

	return reconcile.Result{}, secretsManager.Cleanup(ctx)
}

func (r *Reconciler) deployEtcdMainBackupBucket(ctx context.Context, garden *operatorv1alpha1.Garden, backupBucket *extensionsv1alpha1.BackupBucket) error {
	if etcdConfig := garden.Spec.VirtualCluster.ETCD; etcdConfig == nil || etcdConfig.Main == nil || etcdConfig.Main.Backup == nil {
		return fmt.Errorf("no ETCD main backup configuration found in Garden resource")
	}
	if garden.Spec.RuntimeCluster.Provider.Region == nil {
		return fmt.Errorf("no region found in spec.runtimeCluster.provider.region in Garden resource")
	}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.RuntimeClientSet.Client(), backupBucket, func() error {
		metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, v1beta1constants.GardenerTimestamp, time.Now().UTC().Format(time.RFC3339Nano))

		backupBucket.Spec = extensionsv1alpha1.BackupBucketSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type:           garden.Spec.VirtualCluster.ETCD.Main.Backup.Provider,
				ProviderConfig: garden.Spec.VirtualCluster.ETCD.Main.Backup.ProviderConfig,
				Class:          ptr.To(extensionsv1alpha1.ExtensionClassGarden),
			},
			Region: *garden.Spec.RuntimeCluster.Provider.Region,
			SecretRef: corev1.SecretReference{
				Name:      garden.Spec.VirtualCluster.ETCD.Main.Backup.SecretRef.Name,
				Namespace: r.GardenNamespace,
			},
		}
		return nil
	})
	return err
}

func etcdMainBackupBucket(garden *operatorv1alpha1.Garden) *extensionsv1alpha1.BackupBucket {
	name, _ := etcdMainBackupBucketNameAndPrefix(garden)
	return &extensionsv1alpha1.BackupBucket{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func etcdMainBackupBucketNameAndPrefix(garden *operatorv1alpha1.Garden) (string, string) {
	prefix := "virtual-garden-etcd-main"
	if etcdConfig := garden.Spec.VirtualCluster.ETCD; etcdConfig != nil && etcdConfig.Main != nil && etcdConfig.Main.Backup != nil && etcdConfig.Main.Backup.BucketName != nil {
		name := *etcdConfig.Main.Backup.BucketName
		if idx := strings.Index(name, "/"); idx != -1 {
			prefix = fmt.Sprintf("%s/%s", strings.TrimSuffix(name[idx+1:], "/"), prefix)
			name = name[:idx]
		}
		return name, prefix
	}
	return "garden-" + string(garden.UID), prefix
}

func (r *Reconciler) deployEtcdsFunc(garden *operatorv1alpha1.Garden, etcdMain, etcdEvents etcd.Interface, backupBucket *extensionsv1alpha1.BackupBucket) func(context.Context) error {
	return func(ctx context.Context) error {
		if etcdConfig := garden.Spec.VirtualCluster.ETCD; etcdConfig != nil && etcdConfig.Main != nil && etcdConfig.Main.Backup != nil {
			snapshotSchedule, err := timewindow.DetermineSchedule(
				"%d %d * * *",
				garden.Spec.VirtualCluster.Maintenance.TimeWindow.Begin,
				garden.Spec.VirtualCluster.Maintenance.TimeWindow.End,
				garden.UID,
				garden.CreationTimestamp,
				timewindow.RandomizeWithinFirstHourOfTimeWindow,
			)
			if err != nil {
				return err
			}

			var backupLeaderElection *gardenletconfigv1alpha1.ETCDBackupLeaderElection
			if r.Config.Controllers.Garden.ETCDConfig != nil {
				backupLeaderElection = r.Config.Controllers.Garden.ETCDConfig.BackupLeaderElection
			}

			secretRefName := etcdConfig.Main.Backup.SecretRef.Name
			if backupBucket.Status.GeneratedSecretRef != nil {
				secretRefName = backupBucket.Status.GeneratedSecretRef.Name
			}

			container, prefix := etcdMainBackupBucketNameAndPrefix(garden)

			etcdMain.SetBackupConfig(&etcd.BackupConfig{
				Provider:             etcdConfig.Main.Backup.Provider,
				SecretRefName:        secretRefName,
				Container:            container,
				Prefix:               prefix,
				FullSnapshotSchedule: snapshotSchedule,
				LeaderElection:       backupLeaderElection,
			})
		}

		// Roll out the new peer CA first so that every member in the cluster trusts the old and the new CA.
		// This is required because peer certificates which are used for client and server authentication at the same time,
		// are re-created with the new CA in the `Deploy` step.
		if helper.GetCARotationPhase(garden.Status.Credentials) == gardencorev1beta1.RotationPreparing {
			if err := flow.Parallel(
				etcdMain.RolloutPeerCA,
				etcdEvents.RolloutPeerCA,
			)(ctx); err != nil {
				return err
			}
		}

		return flow.Parallel(etcdMain.Deploy, etcdEvents.Deploy)(ctx)
	}
}

func (r *Reconciler) deployKubeAPIServerFunc(garden *operatorv1alpha1.Garden, kubeAPIServer kubeapiserver.Interface) flow.TaskFn {
	return func(ctx context.Context) error {
		var (
			serviceAccountConfig *gardencorev1beta1.ServiceAccountConfig
			sniConfig            = kubeapiserver.SNIConfig{Enabled: false}
			services             []net.IPNet
		)

		if apiServer := garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer; apiServer != nil {
			if apiServer.ServiceAccountConfig != nil {
				serviceAccountConfig = apiServer.KubeAPIServerConfig.ServiceAccountConfig
			}

			if apiServer.SNI != nil {
				sniConfig.TLS = append(sniConfig.TLS, kubeapiserver.TLSSNIConfig{
					SecretName:     &apiServer.SNI.SecretName,
					DomainPatterns: apiServer.SNI.DomainPatterns,
				})
			}
		}

		for _, svc := range garden.Spec.VirtualCluster.Networking.Services {
			_, cidr, err := net.ParseCIDR(svc)
			if err != nil {
				return fmt.Errorf("failed to parse services CIDR '%s': %w", svc, err)
			}
			services = append(services, *cidr)
		}

		domains := toDomainNames(getAPIServerDomains(garden.Spec.VirtualCluster.DNS.Domains))
		externalHostname := domains[0]
		return shared.DeployKubeAPIServer(
			ctx,
			r.RuntimeClientSet.Client(),
			r.GardenNamespace,
			kubeAPIServer,
			kubeapiserver.ComputeKubeAPIServerServiceAccountConfig(
				serviceAccountConfig,
				externalHostname,
				helper.GetServiceAccountKeyRotationPhase(garden.Status.Credentials),
			),
			kubeapiserver.ServerCertificateConfig{
				ExtraDNSNames: domains,
			},
			sniConfig,
			externalHostname,
			externalHostname,
			nil,
			services,
			nil,
			shared.NormalizeResources(getKubernetesResourcesForEncryption(garden)),
			utils.FilterEntriesByFilterFn(shared.NormalizeResources(garden.Status.EncryptedResources), gardenerutils.IsServedByKubeAPIServer),
			helper.GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials),
			false,
		)
	}
}

func (r *Reconciler) snapshotETCDFunc(secretsManager secretsmanager.Interface, etcdMain etcd.Interface) func(context.Context) error {
	return func(ctx context.Context) error {
		return shared.SnapshotEtcd(ctx, secretsManager, etcdMain)
	}
}

func (r *Reconciler) deployVirtualGardenGardenerResourceManager(secretsManager secretsmanager.Interface, resourceManager resourcemanager.Interface) flow.TaskFn {
	return func(ctx context.Context) error {
		return shared.DeployGardenerResourceManager(
			ctx,
			r.RuntimeClientSet.Client(),
			secretsManager,
			resourceManager,
			r.GardenNamespace,
			func(_ context.Context) (int32, error) {
				return 2, nil
			},
			func() string { return namePrefix + v1beta1constants.DeploymentNameKubeAPIServer },
		)
	}
}

func (r *Reconciler) deployGardenerAPIServerFunc(garden *operatorv1alpha1.Garden, gardenerAPIServer gardenerapiserver.Interface) flow.TaskFn {
	return func(ctx context.Context) error {
		return shared.DeployGardenerAPIServer(
			ctx,
			r.RuntimeClientSet.Client(),
			r.GardenNamespace,
			gardenerAPIServer,
			getGardenerResourcesForEncryption(garden),
			utils.FilterEntriesByFilterFn(garden.Status.EncryptedResources, gardenerutils.IsServedByGardenerAPIServer),
			helper.GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials),
			helper.GetWorkloadIdentityKeyRotationPhase(garden.Status.Credentials),
		)
	}
}

func (r *Reconciler) deployGardenPrometheus(ctx context.Context, log logr.Logger, secretsManager secretsmanager.Interface, prometheus prometheus.Interface, virtualGardenClient client.Client) error {
	if err := gardenerutils.NewShootAccessSecret(gardenprometheus.AccessSecretName, r.GardenNamespace).Reconcile(ctx, r.RuntimeClientSet.Client()); err != nil {
		return fmt.Errorf("failed reconciling access secret for garden prometheus: %w", err)
	}

	// fetch auth secret for ingress
	credentialsSecret, found := secretsManager.Get(v1beta1constants.SecretNameObservabilityIngress)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameObservabilityIngress)
	}
	prometheus.SetIngressAuthSecret(credentialsSecret)

	// fetch global monitoring secret for prometheus-aggregate scrape config
	secretList := &corev1.SecretList{}
	if err := virtualGardenClient.List(ctx, secretList, client.InNamespace(r.GardenNamespace), client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleGlobalMonitoring}); err != nil {
		return fmt.Errorf("failed listing secrets in virtual garden for looking up global monitoring secret: %w", err)
	}

	var (
		globalMonitoringSecretRuntime *corev1.Secret
		err                           error
	)

	if len(secretList.Items) > 0 {
		globalMonitoringSecret := &secretList.Items[0]

		log.Info("Replicating global monitoring secret to garden namespace in runtime cluster", "secret", client.ObjectKeyFromObject(globalMonitoringSecret))
		globalMonitoringSecretRuntime, err = gardenerutils.ReplicateGlobalMonitoringSecret(ctx, r.RuntimeClientSet.Client(), "global-", r.GardenNamespace, globalMonitoringSecret)
		if err != nil {
			return err
		}
	}

	// fetch ingress urls of reachable seeds for prometheus-aggregate scrape config
	seedList := &gardencorev1beta1.SeedList{}
	if err := virtualGardenClient.List(ctx, seedList, client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(utils.MustNewRequirement(v1beta1constants.LabelSeedNetwork, selection.NotEquals, v1beta1constants.LabelSeedNetworkPrivate))}); err != nil {
		return fmt.Errorf("failed listing secrets in virtual garden: %w", err)
	}

	var prometheusAggregateTargets []monitoringv1alpha1.Target
	for _, seed := range seedList.Items {
		if seed.Spec.Ingress != nil {
			prometheusAggregateTargets = append(prometheusAggregateTargets, monitoringv1alpha1.Target(v1beta1constants.IngressDomainPrefixPrometheusAggregate+"."+seed.Spec.Ingress.Domain))
		}
	}

	prometheus.SetCentralScrapeConfigs(gardenprometheus.CentralScrapeConfigs(prometheusAggregateTargets, globalMonitoringSecretRuntime))
	return prometheus.Deploy(ctx)
}

func (r *Reconciler) deployOperatorServiceMonitor(ctx context.Context) error {
	sm := &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{
		Name:      gardenprometheus.Label + "-" + v1beta1constants.DeploymentNameGardenerOperator,
		Namespace: r.GardenNamespace,
	}}

	_, err := controllerutils.CreateOrGetAndMergePatch(ctx, r.RuntimeClientSet.Client(), sm, func() error {
		sm.Labels = utils.MergeStringMaps(sm.Labels, monitoringutils.Labels(gardenprometheus.Label))

		sm.Spec.Endpoints = []monitoringv1.Endpoint{{
			Port: "metrics",
			MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
				"rest_client_.+",
				"controller_runtime_.+",
				"workqueue_.+",
				"go_.+",
			)},
		}
		sm.Spec.Selector = metav1.LabelSelector{MatchLabels: map[string]string{
			v1beta1constants.LabelApp:  v1beta1constants.LabelGardener,
			v1beta1constants.LabelRole: "operator",
		}}
		return nil
	})
	return err
}

func (r *Reconciler) deployGardenerDashboard(ctx context.Context, dashboard gardenerdashboard.Interface, garden *operatorv1alpha1.Garden, secretsManager secretsmanager.Interface, virtualGardenClient client.Client) error {
	if garden.Spec.VirtualCluster.Gardener.Dashboard == nil {
		return component.OpDestroyAndWait(dashboard).Destroy(ctx)
	}

	// fetch the first managed seed that is not labeled with seed.gardener.cloud/network=private
	// TODO(rfranzke): Remove this once https://github.com/gardener/dashboard/issues/1789 has been fixed.
	seedList := &gardencorev1beta1.SeedList{}
	if err := virtualGardenClient.List(ctx, seedList, client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(utils.MustNewRequirement(v1beta1constants.LabelSeedNetwork, selection.NotEquals, v1beta1constants.LabelSeedNetworkPrivate))}); err != nil {
		return fmt.Errorf("failed listing secrets in virtual garden: %w", err)
	}

	for _, seed := range seedList.Items {
		// The dashboard can only handle managed seeds for terminals (otherwise, it has no chance to acquire a
		// kubeconfig) - for managed seeds, it can use the shoots/adminkubeconfig subresource. Hence, let's find the
		// first managed seed here.
		if err := virtualGardenClient.Get(ctx, client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: seed.Name}, &metav1.PartialObjectMetadata{TypeMeta: metav1.TypeMeta{APIVersion: seedmanagementv1alpha1.SchemeGroupVersion.String(), Kind: "ManagedSeed"}}); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed checking whether seed %s is a managed seed: %w", seed.Name, err)
			}
			continue
		}

		dashboard.SetGardenTerminalSeedHost(seed.Name)
		break
	}

	if garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer == nil || garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.SNI == nil {
		caSecret, found := secretsManager.Get(v1beta1constants.SecretNameCACluster)
		if !found {
			return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
		}
		dashboard.SetAPIServerCABundle(ptr.To(utils.EncodeBase64(caSecret.Data[secretsutils.DataKeyCertificateBundle])))
	}

	return component.OpWait(dashboard).Deploy(ctx)
}

func (r *Reconciler) deployLongTermPrometheus(ctx context.Context, secretsManager secretsmanager.Interface, prometheus prometheus.Interface) error {
	// fetch auth secret for ingress
	credentialsSecret, found := secretsManager.Get(v1beta1constants.SecretNameObservabilityIngress)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameObservabilityIngress)
	}
	prometheus.SetIngressAuthSecret(credentialsSecret)
	return prometheus.Deploy(ctx)
}

func (r *Reconciler) updateHelmChartRefForGardenlets(ctx context.Context, log logr.Logger, virtualClusterClient client.Client) error {
	gardenletChartImage, err := imagevector.Charts().FindImage(imagevector.ChartImageNameGardenlet)
	if err != nil {
		return err
	}
	gardenletChartImage.WithOptionalTag(version.Get().GitVersion)

	gardenletList := &seedmanagementv1alpha1.GardenletList{}
	if err := virtualClusterClient.List(ctx, gardenletList, client.MatchingLabels{operatorv1alpha1.LabelKeyGardenletAutoUpdates: "true"}); err != nil {
		return fmt.Errorf("failed listing Gardenlets with label %s: %w", operatorv1alpha1.LabelKeyGardenletAutoUpdates, err)
	}

	for _, gardenlet := range gardenletList.Items {
		if ptr.Deref(gardenlet.Spec.Deployment.Helm.OCIRepository.Ref, "") == gardenletChartImage.String() {
			continue
		}

		log.Info("Updating Helm chart reference of Gardenlet resource", "gardenlet", client.ObjectKeyFromObject(&gardenlet), "ref", gardenletChartImage.String())

		patch := client.MergeFrom(gardenlet.DeepCopy())
		gardenlet.Spec.Deployment.Helm.OCIRepository = gardencorev1.OCIRepository{Ref: ptr.To(gardenletChartImage.String())}
		if err := virtualClusterClient.Patch(ctx, &gardenlet, patch); err != nil {
			return fmt.Errorf("failed updating Helm chart reference of Gardenlet resource: %w", err)
		}
	}

	return nil
}

func (r *Reconciler) reconcileDNSRecords(ctx context.Context, log logr.Logger, garden *operatorv1alpha1.Garden) error {
	dnsRecordList := &extensionsv1alpha1.DNSRecordList{}
	if err := r.listManagedDNSRecords(ctx, dnsRecordList); err != nil {
		return fmt.Errorf("failed listing DNS records: %w", err)
	}

	staleDNSRecordNames := sets.New[string]()
	for _, dnsRecord := range dnsRecordList.Items {
		staleDNSRecordNames.Insert(dnsRecord.Name)
	}

	istioIngressGatewayLoadBalancerAddress, err := kubernetesutils.WaitUntilLoadBalancerIsReady(ctx, log, r.RuntimeClientSet.Client(), namePrefix+v1beta1constants.DefaultSNIIngressNamespace, v1beta1constants.DefaultSNIIngressServiceName, time.Minute)
	if err != nil {
		return fmt.Errorf("failed waiting until %s/%s is ready: %w", namePrefix+v1beta1constants.DefaultSNIIngressNamespace, v1beta1constants.DefaultSNIIngressServiceName, err)
	}

	var taskFns []flow.TaskFn

	apiDomains := getAPIServerDomains(garden.Spec.VirtualCluster.DNS.Domains)
	ingressDomains := getIngressWildcardDomains(garden.Spec.RuntimeCluster.Ingress.Domains)

	for _, domain := range append(apiDomains, ingressDomains...) {
		dnsName := domain.Name
		provider := getDNSProvider(*garden.Spec.DNS, domain.Provider)
		if provider == nil {
			return fmt.Errorf("provider %q not found in DNS providers", *domain.Provider)
		}

		recordName := strings.ReplaceAll(strings.ReplaceAll(dnsName, ".", "-"), "*", "wildcard") + "-" + provider.Type
		staleDNSRecordNames.Delete(recordName)

		taskFns = append(taskFns, func(ctx context.Context) error {
			return component.OpWait(dnsrecord.New(
				log,
				r.RuntimeClientSet.Client(),
				&dnsrecord.Values{
					Name:                         recordName,
					Namespace:                    r.GardenNamespace,
					Labels:                       map[string]string{labelKeyOrigin: labelValueOperator},
					DNSName:                      dnsName,
					Values:                       []string{istioIngressGatewayLoadBalancerAddress},
					RecordType:                   extensionsv1alpha1helper.GetDNSRecordType(istioIngressGatewayLoadBalancerAddress),
					Type:                         provider.Type,
					Class:                        ptr.To(extensionsv1alpha1.ExtensionClassGarden),
					SecretName:                   provider.SecretRef.Name,
					ReconcileOnlyOnChangeOrError: true,
				},
				dnsrecord.DefaultInterval,
				dnsrecord.DefaultSevereThreshold,
				dnsrecord.DefaultTimeout,
			)).Deploy(ctx)
		})
	}

	// cleanup no longer needed DNS records
	for _, staleDNSRecordName := range staleDNSRecordNames.UnsortedList() {
		taskFns = append(taskFns, func(ctx context.Context) error {
			return component.OpDestroyAndWait(dnsrecord.New(
				log,
				r.RuntimeClientSet.Client(),
				&dnsrecord.Values{
					Name:      staleDNSRecordName,
					Namespace: r.GardenNamespace,
				},
				dnsrecord.DefaultInterval,
				dnsrecord.DefaultSevereThreshold,
				dnsrecord.DefaultTimeout,
			)).Destroy(ctx)
		})
	}

	return flow.Parallel(taskFns...)(ctx)
}

func (r *Reconciler) listManagedDNSRecords(ctx context.Context, dnsRecordList *extensionsv1alpha1.DNSRecordList) error {
	if err := r.RuntimeClientSet.Client().List(ctx, dnsRecordList, client.InNamespace(r.GardenNamespace), client.MatchingLabels{labelKeyOrigin: labelValueOperator}); err != nil {
		return fmt.Errorf("failed listing DNS records: %w", err)
	}
	return nil
}

func (r *Reconciler) hasExtensionForBackupBucket(ctx context.Context, garden *operatorv1alpha1.Garden) (bool, error) {
	if garden.Spec.VirtualCluster.ETCD == nil ||
		garden.Spec.VirtualCluster.ETCD.Main == nil ||
		garden.Spec.VirtualCluster.ETCD.Main.Backup == nil {
		return false, nil
	}

	list := &operatorv1alpha1.ExtensionList{}
	if err := r.RuntimeClientSet.Client().List(ctx, list); err != nil {
		return false, fmt.Errorf("failed listing extensions: %w", err)
	}
	for _, ext := range list.Items {
		for _, res := range ext.Spec.Resources {
			if res.Kind == extensionsv1alpha1.BackupBucketResource && res.Type == garden.Spec.VirtualCluster.ETCD.Main.Backup.Provider {
				return true, nil
			}
		}
	}
	return false, nil
}

func getKubernetesResourcesForEncryption(garden *operatorv1alpha1.Garden) []string {
	var encryptionConfig *gardencorev1beta1.EncryptionConfig

	if apiServer := garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer; apiServer != nil && apiServer.KubeAPIServerConfig != nil {
		encryptionConfig = apiServer.KubeAPIServerConfig.EncryptionConfig
	}

	return shared.GetResourcesForEncryptionFromConfig(encryptionConfig)
}

func getGardenerResourcesForEncryption(garden *operatorv1alpha1.Garden) []string {
	var encryptionConfig *gardencorev1beta1.EncryptionConfig

	if garden.Spec.VirtualCluster.Gardener.APIServer != nil {
		encryptionConfig = garden.Spec.VirtualCluster.Gardener.APIServer.EncryptionConfig
	}

	return shared.GetResourcesForEncryptionFromConfig(encryptionConfig)
}

func getDNSProvider(dns operatorv1alpha1.DNSManagement, providerName *string) *operatorv1alpha1.DNSProvider {
	if len(dns.Providers) == 0 {
		return nil
	}
	if providerName == nil {
		return &dns.Providers[0]
	}

	for _, provider := range dns.Providers {
		if provider.Name == *providerName {
			return &provider
		}
	}
	return nil
}

func reconcileGardenerInfoConfigMap(ctx context.Context, log logr.Logger, virtualGardenClient client.Client, secretsManager secretsmanager.Interface, workloadIdentityIssuerURL string) error {
	const (
		configMapName            = "gardener-info"
		gardenerAPIServerDataKey = "gardenerAPIServer"
	)

	var gardenerAPIServerVersion string
	if ver := getGardenerAPIServerVersion(log, secretsManager); ver != nil {
		gardenerAPIServerVersion = *ver
		log.Info("Successfully retrieved actual Gardener API Server version", "version", gardenerAPIServerVersion)
	} else {
		gardenerAPIServerVersion = version.Get().GitVersion
		log.Info("Failed to retrieve actual Gardener API Server version, will use the version of the Gardener Operator", "version", gardenerAPIServerVersion)
	}

	gardenerInfo := struct {
		Version                   string `json:"version" yaml:"version"`
		WorkloadIdentityIssuerURL string `json:"workloadIdentityIssuerURL" yaml:"workloadIdentityIssuerURL"`
	}{
		Version:                   gardenerAPIServerVersion,
		WorkloadIdentityIssuerURL: workloadIdentityIssuerURL,
	}

	marshalled, err := yaml.Marshal(gardenerInfo)
	if err != nil {
		return err
	}

	configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: gardencorev1beta1.GardenerSystemPublicNamespace, Name: configMapName}}
	log.Info("Reconciling gardener-info ConfigMap", "configMap", configMap)
	_, err = controllerutils.CreateOrGetAndMergePatch(ctx, virtualGardenClient, configMap, func() error {
		if configMap.Data == nil {
			configMap.Data = make(map[string]string)
		}
		configMap.Data[gardenerAPIServerDataKey] = string(marshalled)
		return nil
	})
	return err
}

func getGardenerAPIServerVersion(log logr.Logger, secretsManager secretsmanager.Interface) *string {
	caGardener, ok := secretsManager.Get(operatorv1alpha1.SecretNameCAGardener, secretsmanager.Bundle)
	if !ok {
		return nil
	}
	caBundle, ok := caGardener.Data[secretsutils.DataKeyCertificateBundle]
	if !ok {
		return nil
	}

	rawKubeconfig, err := runtime.Encode(clientcmdlatest.Codec, kubernetesutils.NewKubeconfig(gardenerapiserver.DeploymentName,
		clientcmdv1.Cluster{
			Server:                   gardenerapiserver.DeploymentName + "." + v1beta1constants.GardenNamespace + ".svc",
			CertificateAuthorityData: caBundle,
		},
		clientcmdv1.AuthInfo{},
	))
	if err != nil {
		log.Error(err, "Failed to encode kubeconfig")
		return nil
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(rawKubeconfig)
	if err != nil {
		log.Error(err, "Failed to create restConfig")
		return nil
	}

	c, err := kubernetesclientset.NewForConfig(restConfig)
	if err != nil {
		log.Error(err, "Failed to create client for gardener-apiserver service")
		return nil
	}

	v, err := c.Discovery().ServerVersion()
	if err != nil {
		log.Error(err, "Failed to get Gardener API Server version")
		return nil
	}
	return &v.GitVersion
}
