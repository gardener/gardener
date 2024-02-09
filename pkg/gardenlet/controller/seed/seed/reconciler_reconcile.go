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

package seed

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	"github.com/go-logr/logr"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	podsecurityadmissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/clusterautoscaler"
	"github.com/gardener/gardener/pkg/component/clusteridentity"
	"github.com/gardener/gardener/pkg/component/etcd"
	extensioncrds "github.com/gardener/gardener/pkg/component/extensions/crds"
	"github.com/gardener/gardener/pkg/component/hvpa"
	"github.com/gardener/gardener/pkg/component/istio"
	"github.com/gardener/gardener/pkg/component/kubeapiserverexposure"
	"github.com/gardener/gardener/pkg/component/logging/fluentoperator"
	"github.com/gardener/gardener/pkg/component/machinecontrollermanager"
	"github.com/gardener/gardener/pkg/component/monitoring/prometheusoperator"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/component/vpa"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenlethelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/gardener/tokenrequest"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

func (r *Reconciler) reconcile(
	ctx context.Context,
	log logr.Logger,
	seedObj *seedpkg.Seed,
	seedIsGarden bool,
) error {
	seed := seedObj.GetInfo()

	if !controllerutil.ContainsFinalizer(seed, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.GardenClient, seed, gardencorev1beta1.GardenerName); err != nil {
			return err
		}
	}

	// Check whether the Kubernetes version of the Seed cluster fulfills the minimal requirements.
	if err := r.checkMinimumK8SVersion(r.SeedClientSet.Version()); err != nil {
		return err
	}

	if err := r.runReconcileSeedFlow(ctx, log, seedObj, seedIsGarden); err != nil {
		return err
	}

	if seed.Spec.Backup != nil {
		// This should be post updating the seed is available. Since, scheduler will then mostly use
		// same seed for deploying the backupBucket extension.
		if err := deployBackupBucketInGarden(ctx, r.GardenClient, seed); err != nil {
			return err
		}
	}

	return nil
}

func (r *Reconciler) checkMinimumK8SVersion(version string) error {
	const minKubernetesVersion = "1.24"

	seedVersionOK, err := versionutils.CompareVersions(version, ">=", minKubernetesVersion)
	if err != nil {
		return err
	}
	if !seedVersionOK {
		return fmt.Errorf("the Kubernetes version of the Seed cluster must be at least %s", minKubernetesVersion)
	}

	return nil
}

func (r *Reconciler) runReconcileSeedFlow(
	ctx context.Context,
	log logr.Logger,
	seed *seedpkg.Seed,
	seedIsGarden bool,
) error {
	var (
		applier       = r.SeedClientSet.Applier()
		seedClient    = r.SeedClientSet.Client()
		chartApplier  = r.SeedClientSet.ChartApplier()
		chartRenderer = r.SeedClientSet.ChartRenderer()
	)

	secrets, err := gardenerutils.ReadGardenSecrets(ctx, log, r.GardenClient, gardenerutils.ComputeGardenNamespace(seed.GetInfo().Name), true)
	if err != nil {
		return err
	}

	secretsManager, err := secretsmanager.New(
		ctx,
		log.WithName("secretsmanager"),
		clock.RealClock{},
		seedClient,
		r.GardenNamespace,
		v1beta1constants.SecretManagerIdentityGardenlet,
		secretsmanager.Config{CASecretAutoRotation: true},
	)
	if err != nil {
		return err
	}

	// Deploy dedicated CA certificate for seed cluster, auto-rotate it roughly once a month and drop the old CA 24 hours
	// after rotation.
	if _, err := secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
		Name:       v1beta1constants.SecretNameCASeed,
		CommonName: "kubernetes",
		CertType:   secretsutils.CACert,
		Validity:   ptr.To(30 * 24 * time.Hour),
	}, secretsmanager.Rotate(secretsmanager.KeepOld), secretsmanager.IgnoreOldSecretsAfter(24*time.Hour)); err != nil {
		return err
	}

	kubernetesVersion, err := semver.NewVersion(r.SeedClientSet.Version())
	if err != nil {
		return err
	}

	var (
		vpaEnabled     = seed.GetInfo().Spec.Settings == nil || seed.GetInfo().Spec.Settings.VerticalPodAutoscaler == nil || seed.GetInfo().Spec.Settings.VerticalPodAutoscaler.Enabled
		hvpaEnabled    = features.DefaultFeatureGate.Enabled(features.HVPA)
		loggingEnabled = gardenlethelper.IsLoggingEnabled(&r.Config)
	)

	if !vpaEnabled {
		// VPA is a prerequisite. If it's not enabled via the seed spec it must be provided through some other mechanism.
		if _, err := seedClient.RESTMapper().RESTMapping(schema.GroupKind{Group: "autoscaling.k8s.io", Kind: "VerticalPodAutoscaler"}); err != nil {
			return fmt.Errorf("VPA is required for seed cluster: %s", err)
		}
	}

	// create + label garden namespace
	gardenNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: r.GardenNamespace}}
	log.Info("Labeling and annotating namespace", "namespaceName", gardenNamespace.Name)
	if _, err := controllerutils.CreateOrGetAndMergePatch(ctx, seedClient, gardenNamespace, func() error {
		metav1.SetMetaDataLabel(&gardenNamespace.ObjectMeta, "role", v1beta1constants.GardenNamespace)

		// When the seed is the garden cluster then this information is managed by gardener-operator.
		if !seedIsGarden {
			metav1.SetMetaDataLabel(&gardenNamespace.ObjectMeta, podsecurityadmissionapi.EnforceLevelLabel, string(podsecurityadmissionapi.LevelPrivileged))
			metav1.SetMetaDataLabel(&gardenNamespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigConsider, "true")
			metav1.SetMetaDataAnnotation(&gardenNamespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigZones, strings.Join(seed.GetInfo().Spec.Provider.Zones, ","))
		}
		return nil
	}); err != nil {
		return err
	}

	// label kube-system namespace
	namespaceKubeSystem := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: metav1.NamespaceSystem}}
	log.Info("Labeling namespace", "namespaceName", namespaceKubeSystem.Name)
	patch := client.MergeFrom(namespaceKubeSystem.DeepCopy())
	metav1.SetMetaDataLabel(&namespaceKubeSystem.ObjectMeta, "role", metav1.NamespaceSystem)
	if err := seedClient.Patch(ctx, namespaceKubeSystem, patch); err != nil {
		return err
	}

	// replicate global monitoring secret (read from garden cluster) to the seed cluster's garden namespace
	globalMonitoringSecretGarden, ok := secrets[v1beta1constants.GardenRoleGlobalMonitoring]
	if !ok {
		return errors.New("global monitoring secret not found in seed namespace")
	}

	log.Info("Replicating global monitoring secret to garden namespace in seed", "secret", client.ObjectKeyFromObject(globalMonitoringSecretGarden))
	globalMonitoringSecretSeed := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "seed-" + globalMonitoringSecretGarden.Name, Namespace: r.GardenNamespace}}
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, seedClient, globalMonitoringSecretSeed, func() error {
		globalMonitoringSecretSeed.Type = globalMonitoringSecretGarden.Type
		globalMonitoringSecretSeed.Data = globalMonitoringSecretGarden.Data
		globalMonitoringSecretSeed.Immutable = globalMonitoringSecretGarden.Immutable

		if _, ok := globalMonitoringSecretSeed.Data[secretsutils.DataKeySHA1Auth]; !ok {
			globalMonitoringSecretSeed.Data[secretsutils.DataKeySHA1Auth] = utils.CreateSHA1Secret(globalMonitoringSecretGarden.Data[secretsutils.DataKeyUserName], globalMonitoringSecretGarden.Data[secretsutils.DataKeyPassword])
		}

		return nil
	}); err != nil {
		return err
	}

	var alertingSMTPSecret *corev1.Secret
	if secret, ok := secrets[v1beta1constants.GardenRoleAlerting]; ok && string(secret.Data["auth_type"]) == "smtp" {
		alertingSMTPSecret = secret
	}

	// Deploy the CRDs in the seed cluster.
	log.Info("Deploying custom resource definitions")
	if err := machinecontrollermanager.NewCRD(seedClient, applier).Deploy(ctx); err != nil {
		return err
	}

	if err := extensioncrds.NewCRD(applier).Deploy(ctx); err != nil {
		return err
	}

	if !seedIsGarden {
		if err := etcd.NewCRD(seedClient, applier).Deploy(ctx); err != nil {
			return err
		}

		if err := istio.NewCRD(chartApplier).Deploy(ctx); err != nil {
			return err
		}

		if vpaEnabled {
			if err := vpa.NewCRD(applier, nil).Deploy(ctx); err != nil {
				return err
			}
		}

		if hvpaEnabled {
			if err := hvpa.NewCRD(applier).Deploy(ctx); err != nil {
				return err
			}
		}

		if err := fluentoperator.NewCRDs(applier).Deploy(ctx); err != nil {
			return err
		}

		if err := prometheusoperator.NewCRDs(applier).Deploy(ctx); err != nil {
			return err
		}

		// When the seed is the garden cluster then gardener-resource-manager is reconciled by the gardener-operator.
		var defaultNotReadyTolerationSeconds, defaultUnreachableTolerationSeconds *int64
		if nodeToleration := r.Config.NodeToleration; nodeToleration != nil {
			defaultNotReadyTolerationSeconds = nodeToleration.DefaultNotReadyTolerationSeconds
			defaultUnreachableTolerationSeconds = nodeToleration.DefaultUnreachableTolerationSeconds
		}

		var additionalNetworkPolicyNamespaceSelectors []metav1.LabelSelector
		if config := r.Config.Controllers.NetworkPolicy; config != nil {
			additionalNetworkPolicyNamespaceSelectors = config.AdditionalNamespaceSelectors
		}

		// Deploy gardener-resource-manager first since it serves central functionality (e.g., projected token mount
		// webhook) which is required for all other components to start-up.
		gardenerResourceManager, err := sharedcomponent.NewRuntimeGardenerResourceManager(
			seedClient,
			r.GardenNamespace,
			kubernetesVersion,
			secretsManager,
			r.Config.LogLevel, r.Config.LogFormat,
			v1beta1constants.SecretNameCASeed,
			v1beta1constants.PriorityClassNameSeedSystemCritical,
			defaultNotReadyTolerationSeconds,
			defaultUnreachableTolerationSeconds,
			features.DefaultFeatureGate.Enabled(features.DefaultSeccompProfile),
			v1beta1helper.SeedSettingTopologyAwareRoutingEnabled(seed.GetInfo().Spec.Settings),
			additionalNetworkPolicyNamespaceSelectors,
			seed.GetInfo().Spec.Provider.Zones,
		)
		if err != nil {
			return err
		}

		log.Info("Deploying and waiting for gardener-resource-manager to be healthy")
		if err := component.OpWait(gardenerResourceManager).Deploy(ctx); err != nil {
			return err
		}
	}

	// Deploy System Resources
	systemResources, err := defaultSystem(seedClient, seed, r.GardenNamespace)
	if err != nil {
		return err
	}

	if err := systemResources.Deploy(ctx); err != nil {
		return err
	}

	// Wait until required extensions are ready because they might be needed by following deployments
	if err := WaitUntilRequiredExtensionsReady(ctx, r.GardenClient, seed.GetInfo(), 5*time.Second, 1*time.Minute); err != nil {
		return err
	}

	wildcardCert, err := gardenerutils.GetWildcardCertificate(ctx, seedClient)
	if err != nil {
		return err
	}

	var wildCardSecretName *string
	if wildcardCert != nil {
		wildCardSecretName = ptr.To(wildcardCert.GetName())
	}

	seedIsOriginOfClusterIdentity, err := clusteridentity.IsClusterIdentityEmptyOrFromOrigin(ctx, seedClient, v1beta1constants.ClusterIdentityOriginSeed)
	if err != nil {
		return err
	}

	if err := cleanupOrphanExposureClassHandlerResources(ctx, log, seedClient, r.Config.ExposureClassHandlers, seed.GetInfo().Spec.Provider.Zones); err != nil {
		return err
	}

	// setup for flow graph
	var dnsRecord component.DeployMigrateWaiter

	istio, istioDefaultLabels, istioDefaultNamespace, err := defaultIstio(ctx, seedClient, chartRenderer, seed, &r.Config, seedIsGarden)
	if err != nil {
		return err
	}
	dwdWeeder, dwdProber, err := defaultDependencyWatchdogs(seedClient, kubernetesVersion, seed.GetInfo().Spec.Settings, r.GardenNamespace)
	if err != nil {
		return err
	}
	vpnAuthzServer, err := defaultVPNAuthzServer(seedClient, kubernetesVersion, r.GardenNamespace)
	if err != nil {
		return err
	}
	monitoring, err := defaultMonitoring(
		seedClient,
		chartApplier,
		secretsManager,
		r.GardenNamespace,
		seed,
		alertingSMTPSecret,
		globalMonitoringSecretSeed,
		hvpaEnabled,
		seed.GetIngressFQDN("p-seed"),
		wildCardSecretName,
	)
	if err != nil {
		return err
	}
	cachePrometheus, err := defaultCachePrometheus(log, seedClient, r.GardenNamespace, seed)
	if err != nil {
		return err
	}

	var (
		g           = flow.NewGraph("Seed cluster creation")
		deployIstio = g.Add(flow.Task{
			Name: "Deploying Istio",
			Fn:   istio.Deploy,
		})
		istioLBReady = g.Add(flow.Task{
			Name: "Waiting until istio LoadBalancer is ready",
			Fn: func(ctx context.Context) error {
				dnsRecord, err = deployNginxIngressAndWaitForIstioServiceAndGetDNSComponent(
					ctx,
					log,
					seed,
					r.GardenClient,
					seedClient,
					kubernetesVersion,
					r.GardenNamespace,
					seedIsGarden,
					istioDefaultLabels,
					istioDefaultNamespace,
				)
				return err
			},
			Dependencies: flow.NewTaskIDs(deployIstio),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying managed ingress DNS record",
			Fn:           func(ctx context.Context) error { return deployDNSResources(ctx, dnsRecord) },
			Dependencies: flow.NewTaskIDs(istioLBReady),
		})
		_ = g.Add(flow.Task{
			Name: "Deploying cluster-autoscaler resources",
			Fn:   clusterautoscaler.NewBootstrapper(seedClient, r.GardenNamespace).Deploy,
		})
		_ = g.Add(flow.Task{
			Name: "Deploying machine-controller-manager resources",
			Fn:   machinecontrollermanager.NewBootstrapper(seedClient, r.GardenNamespace).Deploy,
		})
		_ = g.Add(flow.Task{
			Name: "Deploying dependency-watchdog-weeder",
			Fn:   dwdWeeder.Deploy,
		})
		_ = g.Add(flow.Task{
			Name: "Deploying dependency-watchdog-prober",
			Fn:   dwdProber.Deploy,
		})
		_ = g.Add(flow.Task{
			Name: "Deploying VPN authorization server",
			Fn:   vpnAuthzServer.Deploy,
		})
		_ = g.Add(flow.Task{
			Name: "Deploying monitoring components",
			Fn:   monitoring.Deploy,
		})
		_ = g.Add(flow.Task{
			Name: "Renewing garden access secrets",
			Fn: func(ctx context.Context) error {
				// renew access secrets in all namespaces with the resources.gardener.cloud/class=garden label
				if err := tokenrequest.RenewAccessSecrets(ctx, seedClient, client.MatchingLabels{resourcesv1alpha1.ResourceManagerClass: resourcesv1alpha1.ResourceManagerClassGarden}); err != nil {
					return err
				}

				// remove operation annotation from seed after successful operation
				return removeSeedOperationAnnotation(ctx, r.GardenClient, seed)
			},
			SkipIf: seed.GetInfo().Annotations[v1beta1constants.GardenerOperation] != v1beta1constants.SeedOperationRenewGardenAccessSecrets,
		})

		_ = g.Add(flow.Task{
			Name: "Renewing garden kubeconfig",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				if err := renewGardenKubeconfig(ctx, seedClient, r.Config.GardenClientConnection); err != nil {
					return err
				}

				// remove operation annotation from seed after successful operation
				return removeSeedOperationAnnotation(ctx, r.GardenClient, seed)
			}),
			SkipIf: seed.GetInfo().Annotations[v1beta1constants.GardenerOperation] != v1beta1constants.GardenerOperationRenewKubeconfig,
		})
	)

	// Use the managed resource for cluster-identity only if there is no cluster-identity config map in kube-system namespace from a different origin than seed.
	// This prevents gardenlet from deleting the config map accidentally on seed deletion when it was created by a different party (gardener-apiserver or shoot).
	if seedIsOriginOfClusterIdentity {
		_ = g.Add(flow.Task{
			Name: "Deploying cluster-identity",
			Fn:   clusteridentity.NewForSeed(seedClient, r.GardenNamespace, *seed.GetInfo().Status.ClusterIdentity).Deploy,
		})
	}

	// When the seed is the garden cluster then the following components are reconciled by the gardener-operator.
	if !seedIsGarden {
		vpa, err := sharedcomponent.NewVerticalPodAutoscaler(
			seedClient,
			r.GardenNamespace,
			kubernetesVersion,
			secretsManager,
			vpaEnabled,
			v1beta1constants.SecretNameCASeed,
			v1beta1constants.PriorityClassNameSeedSystem800,
			v1beta1constants.PriorityClassNameSeedSystem700,
			v1beta1constants.PriorityClassNameSeedSystem700,
		)
		if err != nil {
			return err
		}

		hvpa, err := sharedcomponent.NewHVPA(
			seedClient,
			r.GardenNamespace,
			hvpaEnabled,
			kubernetesVersion,
			v1beta1constants.PriorityClassNameSeedSystem700,
		)
		if err != nil {
			return err
		}

		etcdDruid, err := sharedcomponent.NewEtcdDruid(
			seedClient,
			r.GardenNamespace,
			kubernetesVersion,
			r.ComponentImageVectors,
			r.Config.ETCDConfig,
			v1beta1constants.PriorityClassNameSeedSystem800,
		)
		if err != nil {
			return err
		}

		fluentOperator, err := sharedcomponent.NewFluentOperator(
			seedClient,
			r.GardenNamespace,
			loggingEnabled,
			v1beta1constants.PriorityClassNameSeedSystem600,
		)
		if err != nil {
			return err
		}

		fluentBit, err := sharedcomponent.NewFluentBit(
			seedClient,
			r.GardenNamespace,
			loggingEnabled,
			v1beta1constants.PriorityClassNameSeedSystem600,
		)
		if err != nil {
			return err
		}

		fluentOperatorCustomResources, err := getFluentOperatorCustomResources(
			seedClient,
			r.GardenNamespace,
			loggingEnabled,
			seedIsGarden,
			gardenlethelper.IsEventLoggingEnabled(&r.Config),
		)
		if err != nil {
			return err
		}

		plutono, err := defaultPlutono(
			seedClient,
			r.GardenNamespace,
			secretsManager,
			seed.GetIngressFQDN("g-seed"),
			globalMonitoringSecretSeed.Name,
			wildCardSecretName,
		)
		if err != nil {
			return err
		}

		vali, err := defaultVali(
			ctx,
			seedClient,
			r.Config.Logging,
			r.GardenNamespace,
			loggingEnabled && gardenlethelper.IsValiEnabled(&r.Config),
			hvpaEnabled,
		)
		if err != nil {
			return err
		}

		kubeStateMetrics, err := sharedcomponent.NewKubeStateMetrics(
			seedClient,
			r.GardenNamespace,
			kubernetesVersion,
			v1beta1constants.PriorityClassNameSeedSystem600,
		)
		if err != nil {
			return err
		}

		prometheusOperator, err := sharedcomponent.NewPrometheusOperator(
			seedClient,
			r.GardenNamespace,
			v1beta1constants.PriorityClassNameSeedSystem600,
		)
		if err != nil {
			return err
		}

		var (
			_ = g.Add(flow.Task{
				Name: "Deploying Kubernetes vertical pod autoscaler",
				Fn:   vpa.Deploy,
			})
			_ = g.Add(flow.Task{
				Name: "Deploying HVPA controller",
				Fn:   hvpa.Deploy,
			})
			_ = g.Add(flow.Task{
				Name: "Deploying ETCD Druid",
				Fn:   etcdDruid.Deploy,
			})
			_ = g.Add(flow.Task{
				Name: "Deploying kube-state-metrics",
				Fn:   kubeStateMetrics.Deploy,
			})
			deployFluentOperator = g.Add(flow.Task{
				Name: "Deploying Fluent Operator",
				Fn:   fluentOperator.Deploy,
			})
			_ = g.Add(flow.Task{
				Name:         "Deploying Fluent Bit",
				Fn:           fluentBit.Deploy,
				Dependencies: flow.NewTaskIDs(deployFluentOperator),
			})
			_ = g.Add(flow.Task{
				Name:         "Deploying Fluent Operator custom resources",
				Fn:           fluentOperatorCustomResources.Deploy,
				Dependencies: flow.NewTaskIDs(deployFluentOperator),
			})
			_ = g.Add(flow.Task{
				Name: "Deploying Plutono",
				Fn:   plutono.Deploy,
			})
			_ = g.Add(flow.Task{
				Name: "Deploying Vali",
				Fn:   vali.Deploy,
			})
			_ = g.Add(flow.Task{
				Name: "Deploying Prometheus Operator",
				Fn:   prometheusOperator.Deploy,
			})
		)
	}

	kubeAPIServerService := kubeapiserverexposure.NewInternalNameService(seedClient, r.GardenNamespace)
	if wildcardCert != nil {
		kubeAPIServerIngress := kubeapiserverexposure.NewIngress(seedClient, r.GardenNamespace, kubeapiserverexposure.IngressValues{
			Host:             seed.GetIngressFQDN("api-seed"),
			IngressClassName: ptr.To(v1beta1constants.SeedNginxIngressClass),
			ServiceName:      v1beta1constants.DeploymentNameKubeAPIServer,
			TLSSecretName:    &wildcardCert.Name,
		})
		var (
			_ = g.Add(flow.Task{
				Name: "Deploying kube-apiserver service",
				Fn:   kubeAPIServerService.Deploy,
			})
			_ = g.Add(flow.Task{
				Name: "Deploying kube-apiserver ingress",
				Fn:   kubeAPIServerIngress.Deploy,
			})
		)
	} else {
		kubeAPIServerIngress := kubeapiserverexposure.NewIngress(seedClient, r.GardenNamespace, kubeapiserverexposure.IngressValues{})
		var (
			_ = g.Add(flow.Task{
				Name: "Destroying kube-apiserver service",
				Fn:   kubeAPIServerService.Destroy,
			})
			_ = g.Add(flow.Task{
				Name: "Destroying kube-apiserver ingress",
				Fn:   kubeAPIServerIngress.Destroy,
			})
		)
	}

	var (
		deployCachePrometheus = g.Add(flow.Task{
			Name: "Deploying cache Prometheus",
			Fn:   cachePrometheus.Deploy,
		})
		// TODO(rfranzke): Remove this after v1.92 has been released.
		_ = g.Add(flow.Task{
			Name: "Cleaning up legacy cache Prometheus resources",
			Fn: func(ctx context.Context) error {
				return kubernetesutils.DeleteObjects(ctx, seedClient,
					&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "prometheus-rules", Namespace: r.GardenNamespace}},
					&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "prometheus-config", Namespace: r.GardenNamespace}},
					&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "prometheus-web", Namespace: r.GardenNamespace}},
					&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "prometheus", Namespace: r.GardenNamespace}},
					&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "prometheus", Namespace: r.GardenNamespace}},
					&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "prometheus-seed"}},
					&hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: "prometheus", Namespace: r.GardenNamespace}},
					&vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "prometheus-vpa", Namespace: r.GardenNamespace}},
				)
			},
			Dependencies: flow.NewTaskIDs(deployCachePrometheus),
		})
	)

	if err := g.Compile().Run(ctx, flow.Opts{
		Log:              log,
		ProgressReporter: r.reportProgress(log, seed.GetInfo()),
	}); err != nil {
		return flow.Errors(err)
	}

	return secretsManager.Cleanup(ctx)
}

func deployBackupBucketInGarden(ctx context.Context, k8sGardenClient client.Client, seed *gardencorev1beta1.Seed) error {
	// By default, we assume the seed.Spec.Backup.Provider matches the seed.Spec.Provider.Type as per the validation logic.
	// However, if the backup region is specified we take it.
	region := seed.Spec.Provider.Region
	if seed.Spec.Backup.Region != nil {
		region = *seed.Spec.Backup.Region
	}

	backupBucket := &gardencorev1beta1.BackupBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: string(seed.UID),
		},
	}

	ownerRef := metav1.NewControllerRef(seed, gardencorev1beta1.SchemeGroupVersion.WithKind("Seed"))

	_, err := controllerutils.CreateOrGetAndStrategicMergePatch(ctx, k8sGardenClient, backupBucket, func() error {
		metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		backupBucket.OwnerReferences = []metav1.OwnerReference{*ownerRef}
		backupBucket.Spec = gardencorev1beta1.BackupBucketSpec{
			Provider: gardencorev1beta1.BackupBucketProvider{
				Type:   seed.Spec.Backup.Provider,
				Region: region,
			},
			ProviderConfig: seed.Spec.Backup.ProviderConfig,
			SecretRef: corev1.SecretReference{
				Name:      seed.Spec.Backup.SecretRef.Name,
				Namespace: seed.Spec.Backup.SecretRef.Namespace,
			},
			SeedName: &seed.Name, // In future this will be moved to gardener-scheduler.
		}
		return nil
	})
	return err
}

func cleanupOrphanExposureClassHandlerResources(
	ctx context.Context,
	log logr.Logger,
	c client.Client,
	exposureClassHandlers []config.ExposureClassHandler,
	zones []string,
) error {
	// Remove ordinary, orphaned istio exposure class namespaces
	exposureClassHandlerNamespaces := &corev1.NamespaceList{}
	if err := c.List(ctx, exposureClassHandlerNamespaces, client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleExposureClassHandler}); err != nil {
		return err
	}

	for _, namespace := range exposureClassHandlerNamespaces.Items {
		if err := cleanupOrphanIstioNamespace(ctx, log, c, namespace, true, func() bool {
			for _, handler := range exposureClassHandlers {
				if *handler.SNI.Ingress.Namespace == namespace.Name {
					return true
				}
			}
			return false
		}); err != nil {
			return err
		}
	}

	// Remove zonal, orphaned istio exposure class namespaces
	zonalExposureClassHandlerNamespaces := &corev1.NamespaceList{}
	if err := c.List(ctx, zonalExposureClassHandlerNamespaces, client.MatchingLabelsSelector{
		Selector: labels.NewSelector().Add(utils.MustNewRequirement(v1beta1constants.GardenRole, selection.Exists)).Add(utils.MustNewRequirement(v1beta1constants.LabelExposureClassHandlerName, selection.Exists)),
	}); err != nil {
		return err
	}

	zoneSet := sets.New(zones...)
	for _, namespace := range zonalExposureClassHandlerNamespaces.Items {
		if ok, zone := sharedcomponent.IsZonalIstioExtension(namespace.Labels); ok {
			if err := cleanupOrphanIstioNamespace(ctx, log, c, namespace, true, func() bool {
				if !zoneSet.Has(zone) {
					return false
				}
				for _, handler := range exposureClassHandlers {
					if handler.Name == namespace.Labels[v1beta1constants.LabelExposureClassHandlerName] {
						return true
					}
				}
				return false
			}); err != nil {
				return err
			}
		}
	}

	// Remove zonal, orphaned istio default namespaces
	zonalIstioNamespaces := &corev1.NamespaceList{}
	if err := c.List(ctx, zonalIstioNamespaces, client.MatchingLabelsSelector{
		Selector: labels.NewSelector().Add(utils.MustNewRequirement(istio.DefaultZoneKey, selection.Exists)),
	}); err != nil {
		return err
	}

	for _, namespace := range zonalIstioNamespaces.Items {
		if ok, zone := sharedcomponent.IsZonalIstioExtension(namespace.Labels); ok {
			if err := cleanupOrphanIstioNamespace(ctx, log, c, namespace, false, func() bool {
				return zoneSet.Has(zone)
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func cleanupOrphanIstioNamespace(
	ctx context.Context,
	log logr.Logger,
	c client.Client,
	namespace corev1.Namespace,
	needsHandler bool,
	isAliveFunc func() bool,
) error {
	log = log.WithValues("namespace", client.ObjectKeyFromObject(&namespace))

	if isAlive := isAliveFunc(); isAlive {
		return nil
	}
	log.Info("Namespace is orphan as there is no ExposureClass handler in the gardenlet configuration anymore or the zone was removed")

	// Determine the corresponding handler name to the ExposureClass handler resources.
	handlerName, ok := namespace.Labels[v1beta1constants.LabelExposureClassHandlerName]
	if !ok && needsHandler {
		log.Info("Cannot delete ExposureClass handler resources as the corresponding handler is unknown and it is not save to remove them")
		return nil
	}

	gatewayList := &istiov1beta1.GatewayList{}
	if err := c.List(ctx, gatewayList); err != nil {
		return err
	}

	for _, gateway := range gatewayList.Items {
		if gateway.Name != v1beta1constants.DeploymentNameKubeAPIServer && gateway.Name != v1beta1constants.DeploymentNameVPNSeedServer {
			continue
		}
		if needsHandler {
			// Check if the gateway still selects the ExposureClass handler ingress gateway.
			if value, ok := gateway.Spec.Selector[v1beta1constants.LabelExposureClassHandlerName]; ok && value == handlerName {
				log.Info("Resources of ExposureClass handler cannot be deleted as they are still in use", "exposureClassHandler", handlerName)
				return nil
			}
		} else {
			_, zone := sharedcomponent.IsZonalIstioExtension(namespace.Labels)
			if value, ok := gateway.Spec.Selector[istio.DefaultZoneKey]; ok && strings.HasSuffix(value, zone) {
				log.Info("Resources of default zonal istio handler cannot be deleted as they are still in use", "zone", zone)
				return nil
			}
		}
	}

	// ExposureClass handler is orphan and not used by any Shoots anymore
	// therefore it is save to clean it up.
	log.Info("Delete orphan ExposureClass handler namespace")
	if err := c.Delete(ctx, &namespace); client.IgnoreNotFound(err) != nil {
		return err
	}

	return nil
}

func removeSeedOperationAnnotation(ctx context.Context, gardenClient client.Client, seed *seedpkg.Seed) error {
	return seed.UpdateInfo(ctx, gardenClient, false, func(seedObj *gardencorev1beta1.Seed) error {
		delete(seedObj.Annotations, v1beta1constants.GardenerOperation)
		return nil
	})
}

func renewGardenKubeconfig(ctx context.Context, seedClient client.Client, gardenClientConnection *config.GardenClientConnection) error {
	if gardenClientConnection == nil || gardenClientConnection.KubeconfigSecret == nil {
		return fmt.Errorf(
			"unable to renew garden kubeconfig. No gardenClientConnection.kubeconfigSecret specified in configuration of gardenlet. Remove \"%s=%s\" annotation from seed to reconcile successfully",
			v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationRenewKubeconfig,
		)
	}

	kubeconfigSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: gardenClientConnection.KubeconfigSecret.Name, Namespace: gardenClientConnection.KubeconfigSecret.Namespace}}
	if err := seedClient.Get(ctx, client.ObjectKeyFromObject(kubeconfigSecret), kubeconfigSecret); err != nil {
		return err
	}

	return kubernetesutils.SetAnnotationAndUpdate(ctx, seedClient, kubeconfigSecret, v1beta1constants.GardenerOperation, v1beta1constants.KubeconfigSecretOperationRenew)
}

// WaitUntilLoadBalancerIsReady is an alias for kubernetesutils.WaitUntilLoadBalancerIsReady. Exposed for tests.
var WaitUntilLoadBalancerIsReady = kubernetesutils.WaitUntilLoadBalancerIsReady

func deployNginxIngressAndWaitForIstioServiceAndGetDNSComponent(
	ctx context.Context,
	log logr.Logger,
	seed *seedpkg.Seed,
	gardenClient, seedClient client.Client,
	kubernetesVersion *semver.Version,
	gardenNamespaceName string,
	seedIsGarden bool,
	istioDefaultLabels map[string]string,
	istioDefaultNamespace string,
) (
	component.DeployMigrateWaiter,
	error,
) {
	secretData, err := getDNSProviderSecretData(ctx, gardenClient, seed.GetInfo())
	if err != nil {
		return nil, err
	}

	var ingressLoadBalancerAddress string
	if !seedIsGarden {
		providerConfig, err := getConfig(seed.GetInfo())
		if err != nil {
			return nil, err
		}

		nginxIngress, err := sharedcomponent.NewNginxIngress(
			seedClient,
			gardenNamespaceName,
			gardenNamespaceName,
			kubernetesVersion,
			providerConfig,
			seed.GetLoadBalancerServiceAnnotations(),
			nil,
			v1beta1constants.PriorityClassNameSeedSystem600,
			true,
			true,
			component.ClusterTypeSeed,
			"",
			v1beta1constants.SeedNginxIngressClass,
			[]string{seed.GetIngressFQDN("*")},
			istioDefaultLabels,
		)
		if err != nil {
			return nil, err
		}

		if err = component.OpWait(nginxIngress).Deploy(ctx); err != nil {
			return nil, err
		}
	}

	ingressLoadBalancerAddress, err = WaitUntilLoadBalancerIsReady(
		ctx,
		log,
		seedClient,
		istioDefaultNamespace,
		v1beta1constants.DefaultSNIIngressServiceName,
		time.Minute,
	)
	if err != nil {
		return nil, err
	}
	return getManagedIngressDNSRecord(log, seedClient, gardenNamespaceName, seed.GetInfo().Spec.DNS, secretData, seed.GetIngressFQDN("*"), ingressLoadBalancerAddress), nil
}

// WaitUntilRequiredExtensionsReady checks and waits until all required extensions for a seed exist and are ready.
func WaitUntilRequiredExtensionsReady(ctx context.Context, gardenClient client.Client, seed *gardencorev1beta1.Seed, interval, timeout time.Duration) error {
	return retry.UntilTimeout(ctx, interval, timeout, func(ctx context.Context) (done bool, err error) {
		if err := gardenerutils.RequiredExtensionsReady(ctx, gardenClient, seed.Name, gardenerutils.ComputeRequiredExtensionsForSeed(seed)); err != nil {
			return retry.MinorError(err)
		}

		return retry.Ok()
	})
}
