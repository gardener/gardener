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

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	podsecurityadmissionapi "k8s.io/pod-security-admission/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/operator/v1alpha1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	gardenerapiserver "github.com/gardener/gardener/pkg/component/gardener/apiserver"
	gardenerdashboard "github.com/gardener/gardener/pkg/component/gardener/dashboard"
	"github.com/gardener/gardener/pkg/component/gardener/resourcemanager"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus"
	gardenprometheus "github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/garden"
	"github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenletconfig "github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/utils"
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
		deployPrometheusCRD = g.Add(flow.Task{
			Name: "Deploying custom resource definitions for prometheus-operator",
			Fn:   c.prometheusCRD.Deploy,
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
			Fn:           component.OpWait(c.etcdDruid).Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager),
		})
		deployIstio = g.Add(flow.Task{
			Name:         "Deploying Istio",
			Fn:           component.OpWait(c.istio).Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager),
		})
		syncPointSystemComponents = flow.NewTaskIDs(
			generateGenericTokenKubeconfig,
			generateObservabilityIngressPassword,
			deployRuntimeSystemResources,
			deployFluentCRD,
			deployPrometheusCRD,
			deployVPA,
			deployHVPA,
			deployEtcdDruid,
			deployIstio,
			deployNginxIngressController,
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
			Fn:           r.deployKubeAPIServerFunc(garden, c.kubeAPIServer),
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
				c.kubeControllerManager.SetRuntimeConfig(c.kubeAPIServer.GetValues().RuntimeConfig)
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
			}).RetryUntilTimeout(5*time.Second, 30*time.Second),
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
				RetryUntilTimeout(time.Second, 30*time.Second),
			Dependencies: flow.NewTaskIDs(deployKubeAPIServerService, deployVirtualGardenGardenerAccess, renewVirtualClusterAccess),
		})
		_ = g.Add(flow.Task{
			Name: "Reconciling Gardener Dashboard",
			Fn: func(ctx context.Context) error {
				return r.deployGardenerDashboard(ctx, c.gardenerDashboard, garden.Spec.VirtualCluster.Gardener.Dashboard != nil, virtualClusterClient)
			},
			Dependencies: flow.NewTaskIDs(waitUntilGardenerAPIServerReady, initializeVirtualClusterClient),
		})

		// Renew seed secrets tasks must run sequentially. They all use "gardener.cloud/operation" annotation of the seeds and there can be only one annotation at the same time.
		renewGardenAccessSecretsInAllSeeds = g.Add(flow.Task{
			Name: "Label seeds to trigger renewal of their garden access secrets",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return secretsrotation.RenewGardenSecretsInAllSeeds(ctx, log, virtualClusterClient, v1beta1constants.SeedOperationRenewGardenAccessSecrets)
			}).RetryUntilTimeout(5*time.Second, 30*time.Second),
			SkipIf:       helper.GetServiceAccountKeyRotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPreparing,
			Dependencies: flow.NewTaskIDs(initializeVirtualClusterClient),
		})
		checkIfGardenAccessSecretsRenewalCompletedInAllSeeds = g.Add(flow.Task{
			Name: "Check if all seeds finished the renewal of their garden access secrets",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return secretsrotation.CheckIfGardenSecretsRenewalCompletedInAllSeeds(ctx, virtualClusterClient, v1beta1constants.SeedOperationRenewGardenAccessSecrets)
			}).RetryUntilTimeout(5*time.Second, 2*time.Minute),
			SkipIf:       helper.GetServiceAccountKeyRotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPreparing,
			Dependencies: flow.NewTaskIDs(renewGardenAccessSecretsInAllSeeds),
		})
		renewGardenletKubeconfigInAllSeeds = g.Add(flow.Task{
			Name: "Label seeds to trigger renewal of their gardenlet kubeconfig",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return secretsrotation.RenewGardenSecretsInAllSeeds(ctx, log, virtualClusterClient, v1beta1constants.GardenerOperationRenewKubeconfig)
			}).RetryUntilTimeout(5*time.Second, 30*time.Second),
			SkipIf:       helper.GetCARotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPreparing,
			Dependencies: flow.NewTaskIDs(checkIfGardenAccessSecretsRenewalCompletedInAllSeeds),
		})
		_ = g.Add(flow.Task{
			Name: "Check if all seeds finished the renewal of their gardenlet kubeconfig",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return secretsrotation.CheckIfGardenSecretsRenewalCompletedInAllSeeds(ctx, virtualClusterClient, v1beta1constants.GardenerOperationRenewKubeconfig)
			}).RetryUntilTimeout(5*time.Second, 2*time.Minute),
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
		deployPrometheus = g.Add(flow.Task{
			Name: "Deploying Prometheus",
			Fn: func(ctx context.Context) error {
				return r.deployGardenPrometheus(ctx, log, secretsManager, c.prometheus, virtualClusterClient)
			},
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager, deployPrometheusCRD, waitUntilGardenerAPIServerReady, initializeVirtualClusterClient),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying blackbox-exporter",
			Fn:           c.blackboxExporter.Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager, waitUntilKubeAPIServerIsReady, deployPrometheus),
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

		externalHostname := gardenerutils.GetAPIServerDomain(garden.Spec.VirtualCluster.DNS.Domains[0])
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
				ExtraDNSNames: getAPIServerDomains(garden.Spec.VirtualCluster.DNS.Domains),
			},
			sniConfig,
			externalHostname,
			externalHostname,
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

	// fetch ingress urls for prometheus-aggregate scrape config
	seedList := &gardencorev1beta1.SeedList{}
	if err := virtualGardenClient.List(ctx, seedList); err != nil {
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

func (r *Reconciler) deployGardenerDashboard(ctx context.Context, dashboard gardenerdashboard.Interface, enabled bool, virtualGardenClient client.Client) error {
	if !enabled {
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

	return component.OpWait(dashboard).Deploy(ctx)
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
