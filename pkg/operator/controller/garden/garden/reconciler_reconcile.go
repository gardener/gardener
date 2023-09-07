// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package garden

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	podsecurityadmissionapi "k8s.io/pod-security-admission/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/operator/v1alpha1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/etcd"
	"github.com/gardener/gardener/pkg/component/gardenerapiserver"
	"github.com/gardener/gardener/pkg/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/component/resourcemanager"
	"github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenletconfig "github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/gardener/secretsrotation"
	"github.com/gardener/gardener/pkg/utils/gardener/tokenrequest"
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
	if !controllerutil.ContainsFinalizer(garden, finalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.RuntimeClientSet.Client(), garden, finalizerName); err != nil {
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

	// create + label namespace
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

	wildcardCert, err := gardenerutils.GetWildcardCertificate(ctx, r.RuntimeClientSet.Client())
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

	var (
		allowBackup          = garden.Spec.VirtualCluster.ETCD != nil && garden.Spec.VirtualCluster.ETCD.Main != nil && garden.Spec.VirtualCluster.ETCD.Main.Backup != nil
		virtualClusterClient client.Client
		encryptedGVKs        = []schema.GroupVersionKind{
			corev1.SchemeGroupVersion.WithKind("SecretList"),
			gardencorev1beta1.SchemeGroupVersion.WithKind("ControllerDeploymentList"),
			gardencorev1beta1.SchemeGroupVersion.WithKind("ControllerRegistrationList"),
			gardencorev1beta1.SchemeGroupVersion.WithKind("InternalSecretList"),
			gardencorev1beta1.SchemeGroupVersion.WithKind("ShootStateList"),
		}

		g                              = flow.NewGraph("Garden reconciliation")
		generateGenericTokenKubeconfig = g.Add(flow.Task{
			Name: "Generating generic token kubeconfig",
			Fn: func(ctx context.Context) error {
				return r.generateGenericTokenKubeconfig(ctx, secretsManager)
			},
		})
		deployEtcdCRD = g.Add(flow.Task{
			Name: "Deploying ETCD-related custom resource definitions",
			Fn:   c.etcdCRD.Deploy,
		})
		deployVPACRD = g.Add(flow.Task{
			Name: "Deploying custom resource definitions for VPA",
			Fn:   flow.TaskFn(c.vpaCRD.Deploy).DoIf(vpaEnabled(garden.Spec.RuntimeCluster.Settings)),
		})
		reconcileHVPACRD = g.Add(flow.Task{
			Name: "Reconciling custom resource definitions for HVPA",
			Fn:   c.hvpaCRD.Deploy,
		})
		deployIstioCRD = g.Add(flow.Task{
			Name: "Deploying custom resource definitions for Istio",
			Fn:   c.istioCRD.Deploy,
		})
		deployFluentCRD = g.Add(flow.Task{
			Name: "Deploying custom resource definitions for fluent-operator",
			Fn:   c.fluentCRD.Deploy,
		})
		deployGardenerResourceManager = g.Add(flow.Task{
			Name:         "Deploying and waiting for gardener-resource-manager to be healthy",
			Fn:           component.OpWait(c.gardenerResourceManager).Deploy,
			Dependencies: flow.NewTaskIDs(deployEtcdCRD, deployVPACRD, reconcileHVPACRD, deployIstioCRD),
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
		deployHVPA = g.Add(flow.Task{
			Name:         "Deploying HVPA controller",
			Fn:           c.hvpaController.Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager),
		})
		deployEtcdDruid = g.Add(flow.Task{
			Name:         "Deploying ETCD Druid",
			Fn:           c.etcdDruid.Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager),
		})
		deployIstio = g.Add(flow.Task{
			Name:         "Deploying Istio",
			Fn:           component.OpWait(c.istio).Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager),
		})
		deployFluentOperator = g.Add(flow.Task{
			Name:         "Deploying fluent-operator",
			Fn:           c.fluentOperator.Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager, deployFluentCRD),
		})
		deployFluentOperatorCustomResources = g.Add(flow.Task{
			Name:         "Deploying fluent-operator CustomResources",
			Fn:           c.fluentOperatorCustomResources.Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager, deployFluentCRD),
		})
		deployFluentBit = g.Add(flow.Task{
			Name:         "Deploying fluent-bit",
			Fn:           c.fluentBit.Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager, deployFluentCRD),
		})
		deployVali = g.Add(flow.Task{
			Name:         "Deploying Vali",
			Fn:           c.vali.Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager),
		})
		syncPointSystemComponents = flow.NewTaskIDs(
			generateGenericTokenKubeconfig,
			deployRuntimeSystemResources,
			deployFluentCRD,
			deployVPA,
			deployHVPA,
			deployEtcdDruid,
			deployIstio,
			deployNginxIngressController,
			deployFluentOperator,
			deployFluentOperatorCustomResources,
			deployFluentBit,
			deployVali,
		)

		deployEtcds = g.Add(flow.Task{
			Name:         "Deploying main and events ETCDs of virtual garden",
			Fn:           r.deployEtcdsFunc(garden, c.etcdMain, c.etcdEvents),
			Dependencies: flow.NewTaskIDs(syncPointSystemComponents),
		})
		waitUntilEtcdsReady = g.Add(flow.Task{
			Name:         "Waiting until main and event ETCDs report readiness",
			Fn:           flow.Parallel(c.etcdMain.Wait, c.etcdEvents.Wait),
			Dependencies: flow.NewTaskIDs(deployEtcds),
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
			Fn:           r.deployKubeAPIServerFunc(ctx, garden, c.kubeAPIServer),
			Dependencies: flow.NewTaskIDs(waitUntilEtcdsReady),
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
				return component.OpWait(c.kubeControllerManager).Deploy(ctx)
			},
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerIsReady),
		})
		deployVirtualGardenGardenerResourceManager = g.Add(flow.Task{
			Name: "Deploying gardener-resource-manager for virtual garden",
			Fn: func(ctx context.Context) error {
				return r.deployVirtualGardenGardenerResourceManager(ctx, secretsManager, c.virtualGardenGardenerResourceManager)
			},
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerIsReady),
		})
		waitUntilVirtualGardenGardenerResourceManagerIsReady = g.Add(flow.Task{
			Name:         "Waiting until gardener-resource-manager for virtual garden rolled out",
			Fn:           c.virtualGardenGardenerResourceManager.Wait,
			Dependencies: flow.NewTaskIDs(deployVirtualGardenGardenerResourceManager),
		})

		deployGardenerAPIServer = g.Add(flow.Task{
			Name:         "Deploying Gardener API Server",
			Fn:           r.deployGardenerAPIServerFunc(ctx, garden, c.gardenerAPIServer),
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
					// TODO(rfranzke): Uncomment the next line after v1.85 has been released
					//  (together with restricting the garden's/shoot's tokenrequestor to the shoot class).
					// client.MatchingLabels{resourcesv1alpha1.ResourceManagerClass: resourcesv1alpha1.ResourceManagerClassShoot},
				)
			}).
				RetryUntilTimeout(5*time.Second, 30*time.Second).
				DoIf(helper.GetServiceAccountKeyRotationPhase(garden.Status.Credentials) == gardencorev1beta1.RotationPreparing),
			Dependencies: flow.NewTaskIDs(deployKubeControllerManager, deployVirtualGardenGardenerAccess, deployGardenerAPIServer, deployGardenerAdmissionController, deployGardenerControllerManager, deployGardenerScheduler),
		})
		initializeVirtualClusterClient = g.Add(flow.Task{
			Name: "Initializing connection to virtual garden cluster",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				virtualClusterClientSet, err := r.GardenClientMap.GetClient(ctx, keys.ForGarden(garden))
				if err != nil {
					return err
				}
				virtualClusterClient = virtualClusterClientSet.Client()
				return nil
			}).
				RetryUntilTimeout(time.Second, 30*time.Second),
			Dependencies: flow.NewTaskIDs(deployKubeAPIServerService, deployVirtualGardenGardenerAccess, renewVirtualClusterAccess),
		})
		rewriteSecretsAddLabel = g.Add(flow.Task{
			Name: "Labeling encrypted resources to re-encrypt them with new ETCD encryption key",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return secretsrotation.RewriteEncryptedDataAddLabel(ctx, log, virtualClusterClient, secretsManager, encryptedGVKs...)
			}).
				RetryUntilTimeout(30*time.Second, 10*time.Minute).
				DoIf(helper.GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials) == gardencorev1beta1.RotationPreparing),
			Dependencies: flow.NewTaskIDs(initializeVirtualClusterClient, waitUntilGardenerAPIServerReady),
		})
		_ = g.Add(flow.Task{
			Name: "Snapshotting ETCD after encrypted resources were re-encrypted with new ETCD encryption key",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return secretsrotation.SnapshotETCDAfterRewritingEncryptedData(ctx, r.RuntimeClientSet.Client(), r.snapshotETCDFunc(secretsManager, c.etcdMain), r.GardenNamespace, namePrefix+v1beta1constants.DeploymentNameKubeAPIServer)
			}).
				DoIf(allowBackup && helper.GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials) == gardencorev1beta1.RotationPreparing),
			Dependencies: flow.NewTaskIDs(rewriteSecretsAddLabel),
		})
		_ = g.Add(flow.Task{
			Name: "Removing label from re-encrypted resources after rotation of ETCD encryption key",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return secretsrotation.RewriteEncryptedDataRemoveLabel(ctx, log, r.RuntimeClientSet.Client(), virtualClusterClient, r.GardenNamespace, namePrefix+v1beta1constants.DeploymentNameKubeAPIServer, encryptedGVKs...)
			}).
				RetryUntilTimeout(30*time.Second, 10*time.Minute).
				DoIf(helper.GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials) == gardencorev1beta1.RotationCompleting),
			Dependencies: flow.NewTaskIDs(initializeVirtualClusterClient, waitUntilGardenerAPIServerReady),
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

	return reconcile.Result{}, secretsManager.Cleanup(ctx)
}

func (r *Reconciler) deployEtcdsFunc(garden *operatorv1alpha1.Garden, etcdMain, etcdEvents etcd.Interface) func(context.Context) error {
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

			var backupLeaderElection *gardenletconfig.ETCDBackupLeaderElection
			if r.Config.Controllers.Garden.ETCDConfig != nil {
				backupLeaderElection = r.Config.Controllers.Garden.ETCDConfig.BackupLeaderElection
			}

			container, prefix := etcdConfig.Main.Backup.BucketName, "virtual-garden-etcd-main"
			if idx := strings.Index(etcdConfig.Main.Backup.BucketName, "/"); idx != -1 {
				container = etcdConfig.Main.Backup.BucketName[:idx]
				prefix = fmt.Sprintf("%s/%s", strings.TrimSuffix(etcdConfig.Main.Backup.BucketName[idx+1:], "/"), prefix)
			}

			etcdMain.SetBackupConfig(&etcd.BackupConfig{
				Provider:             etcdConfig.Main.Backup.Provider,
				SecretRefName:        etcdConfig.Main.Backup.SecretRef.Name,
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
			if err := flow.Sequential(
				flow.Parallel(etcdMain.RolloutPeerCA, etcdEvents.RolloutPeerCA),
				flow.Parallel(etcdMain.Wait, etcdEvents.Wait),
			)(ctx); err != nil {
				return err
			}
		}

		return flow.Parallel(etcdMain.Deploy, etcdEvents.Deploy)(ctx)
	}
}

func (r *Reconciler) deployKubeAPIServerFunc(ctx context.Context, garden *operatorv1alpha1.Garden, kubeAPIServer kubeapiserver.Interface) flow.TaskFn {
	return func(context.Context) error {
		var (
			apiServerConfig *gardencorev1beta1.KubeAPIServerConfig
			sniConfig       = kubeapiserver.SNIConfig{Enabled: false}
		)

		if apiServer := garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer; apiServer != nil {
			apiServerConfig = apiServer.KubeAPIServerConfig

			if apiServer.SNI != nil {
				sniConfig.TLS = append(sniConfig.TLS, kubeapiserver.TLSSNIConfig{
					SecretName:     &apiServer.SNI.SecretName,
					DomainPatterns: apiServer.SNI.DomainPatterns,
				})
			}
		}

		var domainNames []string
		if domain := garden.Spec.VirtualCluster.DNS.Domain; domain != nil {
			domainNames = append(domainNames, *domain)
		}
		domainNames = append(domainNames, garden.Spec.VirtualCluster.DNS.Domains...)

		return shared.DeployKubeAPIServer(
			ctx,
			r.RuntimeClientSet.Client(),
			r.GardenNamespace,
			kubeAPIServer,
			apiServerConfig,
			kubeapiserver.ServerCertificateConfig{
				ExtraDNSNames: getAPIServerDomains(domainNames),
			},
			sniConfig,
			gardenerutils.GetAPIServerDomain(domainNames[0]),
			gardenerutils.GetAPIServerDomain(domainNames[0]),
			helper.GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials),
			helper.GetServiceAccountKeyRotationPhase(garden.Status.Credentials),
			false,
		)
	}
}

func (r *Reconciler) snapshotETCDFunc(secretsManager secretsmanager.Interface, etcdMain etcd.Interface) func(context.Context) error {
	return func(ctx context.Context) error {
		return shared.SnapshotEtcd(ctx, secretsManager, etcdMain)
	}
}

func (r *Reconciler) deployVirtualGardenGardenerResourceManager(ctx context.Context, secretsManager secretsmanager.Interface, resourceManager resourcemanager.Interface) error {
	return shared.DeployGardenerResourceManager(
		ctx,
		r.RuntimeClientSet.Client(),
		secretsManager,
		resourceManager,
		r.GardenNamespace,
		func(ctx context.Context) (int32, error) {
			return 2, nil
		},
		func() string { return namePrefix + v1beta1constants.DeploymentNameKubeAPIServer })
}

func (r *Reconciler) deployGardenerAPIServerFunc(ctx context.Context, garden *operatorv1alpha1.Garden, gardenerAPIServer gardenerapiserver.Interface) flow.TaskFn {
	return func(context.Context) error {
		return shared.DeployGardenerAPIServer(
			ctx,
			r.RuntimeClientSet.Client(),
			r.GardenNamespace,
			gardenerAPIServer,
			helper.GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials),
		)
	}
}
