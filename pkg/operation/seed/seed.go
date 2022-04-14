// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gardener/gardener/charts"
	v1alpha1constants "github.com/gardener/gardener/pkg/apis/core/v1alpha1/constants"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenlethelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/clusterautoscaler"
	"github.com/gardener/gardener/pkg/operation/botanist/component/clusteridentity"
	"github.com/gardener/gardener/pkg/operation/botanist/component/coredns"
	"github.com/gardener/gardener/pkg/operation/botanist/component/dependencywatchdog"
	"github.com/gardener/gardener/pkg/operation/botanist/component/etcd"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/crds"
	"github.com/gardener/gardener/pkg/operation/botanist/component/gardenerkubescheduler"
	"github.com/gardener/gardener/pkg/operation/botanist/component/istio"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserverexposure"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubecontrollermanager"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeproxy"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubescheduler"
	"github.com/gardener/gardener/pkg/operation/botanist/component/metricsserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/networkpolicies"
	"github.com/gardener/gardener/pkg/operation/botanist/component/nginxingress"
	"github.com/gardener/gardener/pkg/operation/botanist/component/nodeproblemdetector"
	"github.com/gardener/gardener/pkg/operation/botanist/component/resourcemanager"
	"github.com/gardener/gardener/pkg/operation/botanist/component/seedadmissioncontroller"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpa"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnauthzserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnshoot"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/timewindow"
	versionutils "github.com/gardener/gardener/pkg/utils/version"

	"github.com/Masterminds/semver"
	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"github.com/sirupsen/logrus"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// NewBuilder returns a new Builder.
func NewBuilder() *Builder {
	return &Builder{
		seedObjectFunc: func(_ context.Context) (*gardencorev1beta1.Seed, error) {
			return nil, fmt.Errorf("seed object is required but not set")
		},
	}
}

// WithSeedObject sets the seedObjectFunc attribute at the Builder.
func (b *Builder) WithSeedObject(seedObject *gardencorev1beta1.Seed) *Builder {
	b.seedObjectFunc = func(ctx context.Context) (*gardencorev1beta1.Seed, error) { return seedObject, nil }
	return b
}

// WithSeedObjectFrom sets the seedObjectFunc attribute at the Builder after fetching it from the given lister.
func (b *Builder) WithSeedObjectFrom(gardenClient client.Reader, seedName string) *Builder {
	b.seedObjectFunc = func(ctx context.Context) (*gardencorev1beta1.Seed, error) {
		seed := &gardencorev1beta1.Seed{}
		return seed, gardenClient.Get(ctx, client.ObjectKey{Name: seedName}, seed)
	}
	return b
}

// Build initializes a new Seed object.
func (b *Builder) Build(ctx context.Context) (*Seed, error) {
	seed := &Seed{}

	seedObject, err := b.seedObjectFunc(ctx)
	if err != nil {
		return nil, err
	}
	seed.SetInfo(seedObject)

	if seedObject.Spec.Settings != nil && seedObject.Spec.Settings.LoadBalancerServices != nil {
		seed.LoadBalancerServiceAnnotations = seedObject.Spec.Settings.LoadBalancerServices.Annotations
	}

	return seed, nil
}

// GetInfo returns the seed resource of this Seed in a concurrency safe way.
// This method should be used only for reading the data of the returned seed resource. The returned seed
// resource MUST NOT BE MODIFIED (except in test code) since this might interfere with other concurrent reads and writes.
// To properly update the seed resource of this Seed use UpdateInfo or UpdateInfoStatus.
func (s *Seed) GetInfo() *gardencorev1beta1.Seed {
	return s.info.Load().(*gardencorev1beta1.Seed)
}

// SetInfo sets the seed resource of this Seed in a concurrency safe way.
// This method is not protected by a mutex and does not update the seed resource in the cluster and so
// should be used only in exceptional situations, or as a convenience in test code. The seed passed as a parameter
// MUST NOT BE MODIFIED after the call to SetInfo (except in test code) since this might interfere with other concurrent reads and writes.
// To properly update the seed resource of this Seed use UpdateInfo or UpdateInfoStatus.
func (s *Seed) SetInfo(seed *gardencorev1beta1.Seed) {
	s.info.Store(seed)
}

// UpdateInfo updates the seed resource of this Seed in a concurrency safe way,
// using the given context, client, and mutate function.
// It copies the current seed resource and then uses the copy to patch the resource in the cluster
// using either client.MergeFrom or client.StrategicMergeFrom depending on useStrategicMerge.
// This method is protected by a mutex, so only a single UpdateInfo or UpdateInfoStatus operation can be
// executed at any point in time.
func (s *Seed) UpdateInfo(ctx context.Context, c client.Client, useStrategicMerge bool, f func(*gardencorev1beta1.Seed) error) error {
	s.infoMutex.Lock()
	defer s.infoMutex.Unlock()

	seed := s.info.Load().(*gardencorev1beta1.Seed).DeepCopy()
	var patch client.Patch
	if useStrategicMerge {
		patch = client.StrategicMergeFrom(seed.DeepCopy())
	} else {
		patch = client.MergeFrom(seed.DeepCopy())
	}
	if err := f(seed); err != nil {
		return err
	}
	if err := c.Patch(ctx, seed, patch); err != nil {
		return err
	}
	s.info.Store(seed)
	return nil
}

// UpdateInfoStatus updates the status of the seed resource of this Seed in a concurrency safe way,
// using the given context, client, and mutate function.
// It copies the current seed resource and then uses the copy to patch the resource in the cluster
// using either client.MergeFrom or client.StrategicMergeFrom depending on useStrategicMerge.
// This method is protected by a mutex, so only a single UpdateInfo or UpdateInfoStatus operation can be
// executed at any point in time.
func (s *Seed) UpdateInfoStatus(ctx context.Context, c client.Client, useStrategicMerge bool, f func(*gardencorev1beta1.Seed) error) error {
	s.infoMutex.Lock()
	defer s.infoMutex.Unlock()

	seed := s.info.Load().(*gardencorev1beta1.Seed).DeepCopy()
	var patch client.Patch
	if useStrategicMerge {
		patch = client.StrategicMergeFrom(seed.DeepCopy())
	} else {
		patch = client.MergeFrom(seed.DeepCopy())
	}
	if err := f(seed); err != nil {
		return err
	}
	if err := c.Status().Patch(ctx, seed, patch); err != nil {
		return err
	}
	s.info.Store(seed)
	return nil
}

var (
	rewriteTagRegex = regexp.MustCompile(`\$tag\s+(.+?)\s+user-exposed\.\$TAG\s+true`)
	ingressClass    string
)

const (
	grafanaPrefix    = "g-seed"
	prometheusPrefix = "p-seed"

	userExposedComponentTagPrefix = "user-exposed"
)

var ingressTLSCertificateValidity = 730 * 24 * time.Hour // ~2 years, see https://support.apple.com/en-us/HT210176

// RunReconcileSeedFlow bootstraps a Seed cluster and deploys various required manifests.
func RunReconcileSeedFlow(
	ctx context.Context,
	gardenClient client.Client,
	seedClientSet kubernetes.Interface,
	seed *Seed,
	secrets map[string]*corev1.Secret,
	imageVector imagevector.ImageVector,
	componentImageVectors imagevector.ComponentImageVectors,
	conf *config.GardenletConfiguration,
	log logrus.FieldLogger,
) error {
	var (
		applier      = seedClientSet.Applier()
		seedClient   = seedClientSet.Client()
		chartApplier = seedClientSet.ChartApplier()
	)

	secretsManager, err := secretsmanager.New(
		ctx,
		logf.Log.WithName("secretsmanager"),
		clock.RealClock{},
		seedClient,
		v1beta1constants.GardenNamespace,
		v1beta1constants.SecretManagerIdentityGardenlet,
		nil,
	)
	if err != nil {
		return err
	}

	// Deploy dedicated CA certificate for seed cluster.
	validity := 30 * 24 * time.Hour
	if _, err := secretsManager.Generate(ctx, &secretutils.CertificateSecretConfig{
		Name:       v1beta1constants.SecretNameCASeed,
		CommonName: "kubernetes",
		CertType:   secretutils.CACert,
		Validity:   &validity,
	}, secretsmanager.Rotate(secretsmanager.KeepOld)); err != nil {
		return err
	}

	kubernetesVersion, err := semver.NewVersion(seedClientSet.Version())
	if err != nil {
		return err
	}

	vpaGK := schema.GroupKind{Group: "autoscaling.k8s.io", Kind: "VerticalPodAutoscaler"}

	vpaEnabled := seed.GetInfo().Spec.Settings == nil || seed.GetInfo().Spec.Settings.VerticalPodAutoscaler == nil || seed.GetInfo().Spec.Settings.VerticalPodAutoscaler.Enabled
	if !vpaEnabled {
		// VPA is a prerequisite. If it's not enabled via the seed spec it must be provided through some other mechanism.
		if _, err := seedClient.RESTMapper().RESTMapping(vpaGK); err != nil {
			return fmt.Errorf("VPA is required for seed cluster: %s", err)
		}

		vpa := vpa.New(seedClient, v1beta1constants.GardenNamespace, nil, vpa.Values{ClusterType: vpa.ClusterTypeSeed})

		if err := component.OpDestroyAndWait(vpa).Destroy(ctx); err != nil {
			return err
		}
	}

	const (
		seedBoostrapChartName     = "seed-bootstrap"
		seedBoostrapCRDsChartName = "seed-bootstrap-crds"
	)
	var (
		loggingConfig   = conf.Logging
		gardenNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: v1beta1constants.GardenNamespace,
			},
		}
	)

	// create + label garden namespace
	if _, err := controllerutils.CreateOrGetAndMergePatch(ctx, seedClient, gardenNamespace, func() error {
		metav1.SetMetaDataLabel(&gardenNamespace.ObjectMeta, "role", v1beta1constants.GardenNamespace)
		return nil
	}); err != nil {
		return err
	}

	// label kube-system namespace
	namespaceKubeSystem := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: metav1.NamespaceSystem}}
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

	globalMonitoringSecretSeed := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "seed-" + globalMonitoringSecretGarden.Name,
			Namespace: v1beta1constants.GardenNamespace,
		},
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, seedClient, globalMonitoringSecretSeed, func() error {
		globalMonitoringSecretSeed.Type = globalMonitoringSecretGarden.Type
		globalMonitoringSecretSeed.Data = globalMonitoringSecretGarden.Data
		globalMonitoringSecretSeed.Immutable = globalMonitoringSecretGarden.Immutable

		if _, ok := globalMonitoringSecretSeed.Data[secretutils.DataKeySHA1Auth]; !ok {
			globalMonitoringSecretSeed.Data[secretutils.DataKeySHA1Auth] = utils.CreateSHA1Secret(globalMonitoringSecretGarden.Data[secretutils.DataKeyUserName], globalMonitoringSecretGarden.Data[secretutils.DataKeyPassword])
		}

		return nil
	}); err != nil {
		return err
	}

	seedImages, err := imagevector.FindImages(imageVector,
		[]string{
			images.ImageNameAlertmanager,
			images.ImageNameAlpine,
			images.ImageNameConfigmapReloader,
			images.ImageNameLoki,
			images.ImageNameLokiCurator,
			images.ImageNameFluentBit,
			images.ImageNameFluentBitPluginInstaller,
			images.ImageNameGrafana,
			images.ImageNamePauseContainer,
			images.ImageNamePrometheus,
			images.ImageNameHvpaController,
			images.ImageNameKubeStateMetrics,
		},
		imagevector.RuntimeVersion(kubernetesVersion.String()),
		imagevector.TargetVersion(kubernetesVersion.String()),
	)
	if err != nil {
		return err
	}

	// HVPA feature gate
	hvpaEnabled := gardenletfeatures.FeatureGate.Enabled(features.HVPA)
	if !hvpaEnabled {
		if err := common.DeleteHvpa(ctx, seedClient, v1beta1constants.GardenNamespace); client.IgnoreNotFound(err) != nil {
			return err
		}
	} else {
		// Clean up stale vpa objects
		resources := []client.Object{
			&vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "prometheus-vpa", Namespace: v1beta1constants.GardenNamespace}},
			&vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "aggregate-prometheus-vpa", Namespace: v1beta1constants.GardenNamespace}},
		}

		if err := kutil.DeleteObjects(ctx, seedClient, resources...); err != nil {
			return err
		}
	}

	// Deploy the CRDs in the seed cluster.
	crdsChartValues := kubernetes.Values(map[string]interface{}{
		"hvpa": map[string]interface{}{
			"enabled": hvpaEnabled,
		},
	})

	if err := chartApplier.Apply(ctx, filepath.Join(charts.Path, seedBoostrapCRDsChartName), v1beta1constants.GardenNamespace, seedBoostrapCRDsChartName, crdsChartValues); err != nil {
		return err
	}

	if vpaEnabled {
		if err := vpa.NewCRD(applier, nil).Deploy(ctx); err != nil {
			return err
		}
	}

	if err := crds.NewExtensionsCRD(applier).Deploy(ctx); err != nil {
		return err
	}

	// Deploy gardener-resource-manager first since it serves central functionality (e.g., projected token mount webhook)
	// which is required for all other components to start-up.
	gardenerResourceManager, err := defaultGardenerResourceManager(seedClient, imageVector, secretsManager)
	if err != nil {
		return err
	}
	if err := component.OpWaiter(gardenerResourceManager).Deploy(ctx); err != nil {
		return err
	}

	// Fetch component-specific central monitoring configuration
	var (
		centralScrapeConfigs                            = strings.Builder{}
		centralCAdvisorScrapeConfigMetricRelabelConfigs = strings.Builder{}
	)

	for _, componentFn := range []component.CentralMonitoringConfiguration{} {
		centralMonitoringConfig, err := componentFn()
		if err != nil {
			return err
		}

		for _, config := range centralMonitoringConfig.ScrapeConfigs {
			centralScrapeConfigs.WriteString(fmt.Sprintf("- %s\n", utils.Indent(config, 2)))
		}

		for _, config := range centralMonitoringConfig.CAdvisorScrapeConfigMetricRelabelConfigs {
			centralCAdvisorScrapeConfigMetricRelabelConfigs.WriteString(fmt.Sprintf("- %s\n", utils.Indent(config, 2)))
		}
	}

	// Logging feature gate
	var (
		loggingEnabled                    bool
		filters                           = strings.Builder{}
		parsers                           = strings.Builder{}
		fluentBitConfigurationsOverwrites = map[string]interface{}{}
		lokiValues                        = map[string]interface{}{}
	)

	loggingEnabled = gardenlethelper.IsLoggingEnabled(conf)

	lokiValues["enabled"] = loggingEnabled

	if loggingEnabled {
		// check if loki is disabled in gardenlet config
		if !gardenlethelper.IsLokiEnabled(conf) {
			lokiValues["enabled"] = false
			if err := common.DeleteLoki(ctx, seedClient, gardenNamespace.Name); err != nil {
				return err
			}
		} else {
			lokiValues["authEnabled"] = false
			lokiValues["storage"] = loggingConfig.Loki.Garden.Storage
			if err := ResizeOrDeleteLokiDataVolumeIfStorageNotTheSame(ctx, seedClient, *loggingConfig.Loki.Garden.Storage, log); err != nil {
				return err
			}

			if hvpaEnabled {
				shootInfo := &corev1.ConfigMap{}
				maintenanceBegin := "220000-0000"
				maintenanceEnd := "230000-0000"
				if err := seedClient.Get(ctx, kutil.Key(metav1.NamespaceSystem, "shoot-info"), shootInfo); err != nil {
					if !apierrors.IsNotFound(err) {
						return err
					}
				} else {
					shootMaintenanceBegin, err := timewindow.ParseMaintenanceTime(shootInfo.Data["maintenanceBegin"])
					if err != nil {
						return err
					}
					maintenanceBegin = shootMaintenanceBegin.Add(1, 0, 0).Formatted()

					shootMaintenanceEnd, err := timewindow.ParseMaintenanceTime(shootInfo.Data["maintenanceEnd"])
					if err != nil {
						return err
					}
					maintenanceEnd = shootMaintenanceEnd.Add(1, 0, 0).Formatted()
				}

				lokiValues["hvpa"] = map[string]interface{}{
					"enabled": true,
					"maintenanceTimeWindow": map[string]interface{}{
						"begin": maintenanceBegin,
						"end":   maintenanceEnd,
					},
				}

				currentResources, err := kutil.GetContainerResourcesInStatefulSet(ctx, seedClient, kutil.Key(v1beta1constants.GardenNamespace, v1beta1constants.StatefulSetNameLoki))
				if err != nil {
					return err
				}
				if len(currentResources) != 0 && currentResources[v1beta1constants.StatefulSetNameLoki] != nil {
					lokiValues["resources"] = map[string]interface{}{
						v1beta1constants.StatefulSetNameLoki: currentResources[v1beta1constants.StatefulSetNameLoki],
					}
				}
			}

			if conf.Logging != nil && conf.Logging.Loki != nil && conf.Logging.Loki.Garden != nil &&
				conf.Logging.Loki.Garden.Priority != nil {
				priority := *conf.Logging.Loki.Garden.Priority
				if err := deletePriorityClassIfValueNotTheSame(ctx, seedClient, common.GardenLokiPriorityClassName, priority); err != nil {
					return err
				}
				lokiValues["priorityClass"] = map[string]interface{}{
					"value": priority,
					"name":  common.GardenLokiPriorityClassName,
				}
			} else {
				pc := &schedulingv1.PriorityClass{ObjectMeta: metav1.ObjectMeta{Name: common.GardenLokiPriorityClassName}}
				if err := seedClient.Delete(ctx, pc); client.IgnoreNotFound(err) != nil {
					return err
				}
			}
		}

		componentsFunctions := []component.CentralLoggingConfiguration{
			// seed system components
			dependencywatchdog.CentralLoggingConfiguration,
			seedadmissioncontroller.CentralLoggingConfiguration,
			resourcemanager.CentralLoggingConfiguration,
			// shoot control plane components
			etcd.CentralLoggingConfiguration,
			clusterautoscaler.CentralLoggingConfiguration,
			kubeapiserver.CentralLoggingConfiguration,
			kubescheduler.CentralLoggingConfiguration,
			kubecontrollermanager.CentralLoggingConfiguration,
			vpa.CentralLoggingConfiguration,
			// shoot system components
			coredns.CentralLoggingConfiguration,
			kubeproxy.CentralLoggingConfiguration,
			metricsserver.CentralLoggingConfiguration,
			nodeproblemdetector.CentralLoggingConfiguration,
			vpnshoot.CentralLoggingConfiguration,
		}
		userAllowedComponents := []string{
			v1beta1constants.DeploymentNameKubeAPIServer,
			v1beta1constants.DeploymentNameVPAExporter, v1beta1constants.DeploymentNameVPARecommender,
			v1beta1constants.DeploymentNameVPAAdmissionController,
		}

		// Fetch component specific logging configurations
		for _, componentFn := range componentsFunctions {
			loggingConfig, err := componentFn()
			if err != nil {
				return err
			}

			filters.WriteString(fmt.Sprintln(loggingConfig.Filters))
			parsers.WriteString(fmt.Sprintln(loggingConfig.Parsers))

			if loggingConfig.UserExposed {
				userAllowedComponents = append(userAllowedComponents, loggingConfig.PodPrefix)
			}
		}

		loggingRewriteTagFilter := `[FILTER]
    Name          modify
    Match         kubernetes.*
    Condition     Key_value_matches tag ^kubernetes\.var\.log\.containers\.(` + strings.Join(userAllowedComponents, "|") + `)-.+?_
    Add           __gardener_multitenant_id__ operator;user
`
		filters.WriteString(fmt.Sprintln(loggingRewriteTagFilter))

		// Read extension provider specific logging configuration
		existingConfigMaps := &corev1.ConfigMapList{}
		if err = seedClient.List(ctx, existingConfigMaps,
			client.InNamespace(v1beta1constants.GardenNamespace),
			client.MatchingLabels{v1beta1constants.LabelExtensionConfiguration: v1beta1constants.LabelLogging}); err != nil {
			return err
		}

		// Need stable order before passing the dashboards to Grafana config to avoid unnecessary changes
		kutil.ByName().Sort(existingConfigMaps)
		modifyFilter := `
    Name          modify
    Match         kubernetes.*
    Condition     Key_value_matches tag __PLACE_HOLDER__
    Add           __gardener_multitenant_id__ operator;user
`
		// Read all filters and parsers coming from the extension provider configurations
		for _, cm := range existingConfigMaps.Items {
			// Remove the extensions rewrite_tag filters.
			// TODO (vlvasilev): When all custom rewrite_tag filters are removed from the extensions this code snipped must be removed
			flbFilters := cm.Data[v1beta1constants.FluentBitConfigMapKubernetesFilter]
			tokens := strings.Split(flbFilters, "[FILTER]")
			var sb strings.Builder
			for _, token := range tokens {
				if strings.Contains(token, "rewrite_tag") {
					result := rewriteTagRegex.FindAllStringSubmatch(token, 1)
					if len(result) < 1 || len(result[0]) < 2 {
						continue
					}
					token = strings.Replace(modifyFilter, "__PLACE_HOLDER__", result[0][1], 1)
				}
				// In case we are processing the first token
				if strings.TrimSpace(token) != "" {
					sb.WriteString("[FILTER]")
				}
				sb.WriteString(token)
			}
			filters.WriteString(fmt.Sprintln(strings.TrimRight(sb.String(), " ")))
			parsers.WriteString(fmt.Sprintln(cm.Data[v1beta1constants.FluentBitConfigMapParser]))
		}

		if loggingConfig != nil && loggingConfig.FluentBit != nil {
			fbConfig := loggingConfig.FluentBit

			if fbConfig.ServiceSection != nil {
				fluentBitConfigurationsOverwrites["service"] = *fbConfig.ServiceSection
			}
			if fbConfig.InputSection != nil {
				fluentBitConfigurationsOverwrites["input"] = *fbConfig.InputSection
			}
			if fbConfig.OutputSection != nil {
				fluentBitConfigurationsOverwrites["output"] = *fbConfig.OutputSection
			}
		}
	} else {
		if err := common.DeleteSeedLoggingStack(ctx, seedClient); err != nil {
			return err
		}
	}

	// Monitoring resource values
	monitoringResources := map[string]interface{}{
		"prometheus":           map[string]interface{}{},
		"aggregate-prometheus": map[string]interface{}{},
	}

	if hvpaEnabled {
		for resource := range monitoringResources {
			currentResources, err := kutil.GetContainerResourcesInStatefulSet(ctx, seedClient, kutil.Key(v1beta1constants.GardenNamespace, resource))
			if err != nil {
				return err
			}
			if len(currentResources) != 0 && currentResources["prometheus"] != nil {
				monitoringResources[resource] = map[string]interface{}{
					"prometheus": currentResources["prometheus"],
				}
			}
		}
	}

	// AlertManager configuration
	alertManagerConfig := map[string]interface{}{
		"storage": seed.GetValidVolumeSize("1Gi"),
	}

	if alertingSMTPSecret, ok := secrets[v1beta1constants.GardenRoleAlerting]; ok && string(alertingSMTPSecret.Data["auth_type"]) == "smtp" {
		emailConfig := map[string]interface{}{
			"to":            string(alertingSMTPSecret.Data["to"]),
			"from":          string(alertingSMTPSecret.Data["from"]),
			"smarthost":     string(alertingSMTPSecret.Data["smarthost"]),
			"auth_username": string(alertingSMTPSecret.Data["auth_username"]),
			"auth_identity": string(alertingSMTPSecret.Data["auth_identity"]),
			"auth_password": string(alertingSMTPSecret.Data["auth_password"]),
		}
		alertManagerConfig["enabled"] = true
		alertManagerConfig["emailConfigs"] = []map[string]interface{}{emailConfig}
	} else {
		alertManagerConfig["enabled"] = false
		if err := common.DeleteAlertmanager(ctx, seedClient, v1beta1constants.GardenNamespace); err != nil {
			return err
		}
	}

	if !seed.GetInfo().Spec.Settings.ExcessCapacityReservation.Enabled {
		if err := common.DeleteReserveExcessCapacity(ctx, seedClient); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	var (
		applierOptions          = kubernetes.CopyApplierOptions(kubernetes.DefaultMergeFuncs)
		retainStatusInformation = func(new, old *unstructured.Unstructured) {
			// Apply status from old Object to retain status information
			new.Object["status"] = old.Object["status"]
		}
		hvpaGK         = schema.GroupKind{Group: "autoscaling.k8s.io", Kind: "Hvpa"}
		issuerGK       = schema.GroupKind{Group: "certmanager.k8s.io", Kind: "ClusterIssuer"}
		grafanaHost    = seed.GetIngressFQDN(grafanaPrefix)
		prometheusHost = seed.GetIngressFQDN(prometheusPrefix)
	)

	applierOptions[vpaGK] = retainStatusInformation
	applierOptions[hvpaGK] = retainStatusInformation
	applierOptions[issuerGK] = retainStatusInformation

	wildcardCert, err := GetWildcardCertificate(ctx, seedClient)
	if err != nil {
		return err
	}

	var (
		grafanaIngressTLSSecretName    string
		prometheusIngressTLSSecretName string
	)

	if wildcardCert != nil {
		grafanaIngressTLSSecretName = wildcardCert.GetName()
		prometheusIngressTLSSecretName = wildcardCert.GetName()
	} else {
		grafanaIngressTLSSecret, err := secretsManager.Generate(ctx, &secretutils.CertificateSecretConfig{
			Name:                        "grafana-tls",
			CommonName:                  "grafana",
			Organization:                []string{"gardener.cloud:monitoring:ingress"},
			DNSNames:                    []string{seed.GetIngressFQDN(grafanaPrefix)},
			CertType:                    secretutils.ServerCert,
			Validity:                    &ingressTLSCertificateValidity,
			SkipPublishingCACertificate: true,
		}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCASeed))
		if err != nil {
			return err
		}

		prometheusIngressTLSSecret, err := secretsManager.Generate(ctx, &secretutils.CertificateSecretConfig{
			Name:                        "aggregate-prometheus-tls",
			CommonName:                  "prometheus",
			Organization:                []string{"gardener.cloud:monitoring:ingress"},
			DNSNames:                    []string{seed.GetIngressFQDN(prometheusPrefix)},
			CertType:                    secretutils.ServerCert,
			Validity:                    &ingressTLSCertificateValidity,
			SkipPublishingCACertificate: true,
		}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCASeed))
		if err != nil {
			return err
		}

		grafanaIngressTLSSecretName = grafanaIngressTLSSecret.Name
		prometheusIngressTLSSecretName = prometheusIngressTLSSecret.Name
	}

	imageVectorOverwrites := make(map[string]string, len(componentImageVectors))
	for name, data := range componentImageVectors {
		imageVectorOverwrites[name] = data
	}

	anySNI, err := kubeapiserverexposure.AnyDeployedSNI(ctx, seedClient)
	if err != nil {
		return err
	}

	if gardenletfeatures.FeatureGate.Enabled(features.ManagedIstio) {
		istiodImage, err := imageVector.FindImage(images.ImageNameIstioIstiod)
		if err != nil {
			return err
		}

		igwImage, err := imageVector.FindImage(images.ImageNameIstioProxy)
		if err != nil {
			return err
		}

		istioCRDs := istio.NewIstioCRD(chartApplier, charts.Path, seedClient)
		istiod := istio.NewIstiod(
			&istio.IstiodValues{
				TrustDomain: gardencorev1beta1.DefaultDomain,
				Image:       istiodImage.String(),
			},
			common.IstioNamespace,
			chartApplier,
			charts.Path,
			seedClient,
		)
		istioDeployers := []component.DeployWaiter{istioCRDs, istiod}

		defaultIngressGatewayConfig := &istio.IngressValues{
			TrustDomain:     gardencorev1beta1.DefaultDomain,
			Image:           igwImage.String(),
			IstiodNamespace: common.IstioNamespace,
			Annotations:     seed.LoadBalancerServiceAnnotations,
			Ports:           []corev1.ServicePort{},
			LoadBalancerIP:  conf.SNI.Ingress.ServiceExternalIP,
			Labels:          conf.SNI.Ingress.Labels,
		}

		// even if SNI is being disabled, the existing ports must stay the same
		// until all APIServer SNI resources are removed.
		if gardenletfeatures.FeatureGate.Enabled(features.APIServerSNI) || anySNI {
			defaultIngressGatewayConfig.Ports = append(
				defaultIngressGatewayConfig.Ports,
				corev1.ServicePort{Name: "proxy", Port: 8443, TargetPort: intstr.FromInt(8443)},
				corev1.ServicePort{Name: "tcp", Port: 443, TargetPort: intstr.FromInt(9443)},
				corev1.ServicePort{Name: "tls-tunnel", Port: vpnseedserver.GatewayPort, TargetPort: intstr.FromInt(vpnseedserver.GatewayPort)},
			)
		}

		istioDeployers = append(istioDeployers, istio.NewIngressGateway(
			defaultIngressGatewayConfig,
			*conf.SNI.Ingress.Namespace,
			chartApplier,
			charts.Path,
			seedClient,
		))

		// Add for each ExposureClass handler in the config an own Ingress Gateway.
		for _, handler := range conf.ExposureClassHandlers {
			istioDeployers = append(istioDeployers, istio.NewIngressGateway(
				&istio.IngressValues{
					TrustDomain:     gardencorev1beta1.DefaultDomain,
					Image:           igwImage.String(),
					IstiodNamespace: common.IstioNamespace,
					Annotations:     utils.MergeStringMaps(seed.LoadBalancerServiceAnnotations, handler.LoadBalancerService.Annotations),
					Ports:           defaultIngressGatewayConfig.Ports,
					LoadBalancerIP:  handler.SNI.Ingress.ServiceExternalIP,
					Labels:          gutil.GetMandatoryExposureClassHandlerSNILabels(handler.SNI.Ingress.Labels, handler.Name),
				},
				*handler.SNI.Ingress.Namespace,
				chartApplier,
				charts.Path,
				seedClient,
			))
		}

		if err := component.OpWaiter(istioDeployers...).Deploy(ctx); err != nil {
			return err
		}
	}

	var proxyGatewayDeployers = []component.DeployWaiter{
		istio.NewProxyProtocolGateway(
			&istio.ProxyValues{
				Labels: conf.SNI.Ingress.Labels,
			},
			*conf.SNI.Ingress.Namespace,
			chartApplier,
			charts.Path,
		),
	}

	for _, handler := range conf.ExposureClassHandlers {
		proxyGatewayDeployers = append(proxyGatewayDeployers, istio.NewProxyProtocolGateway(
			&istio.ProxyValues{
				Labels: gutil.GetMandatoryExposureClassHandlerSNILabels(handler.SNI.Ingress.Labels, handler.Name),
			},
			*handler.SNI.Ingress.Namespace,
			chartApplier,
			charts.Path,
		))
	}

	if gardenletfeatures.FeatureGate.Enabled(features.APIServerSNI) {
		for _, proxyDeployer := range proxyGatewayDeployers {
			if err := proxyDeployer.Deploy(ctx); err != nil {
				return err
			}
		}
	} else {
		for _, proxyDeployer := range proxyGatewayDeployers {
			if err := proxyDeployer.Destroy(ctx); err != nil {
				return err
			}
		}
	}

	if err := cleanupOrphanExposureClassHandlerResources(ctx, seedClient, conf.ExposureClassHandlers, log); err != nil {
		return err
	}

	if seed.GetInfo().Status.ClusterIdentity == nil {
		seedClusterIdentity, err := determineClusterIdentity(ctx, seedClient)
		if err != nil {
			return err
		}

		if err := seed.UpdateInfoStatus(ctx, gardenClient, false, func(seed *gardencorev1beta1.Seed) error {
			seed.Status.ClusterIdentity = &seedClusterIdentity
			return nil
		}); err != nil {
			return err
		}
	}

	ingressClass, err = ComputeNginxIngressClass(seed, pointer.String(kubernetesVersion.String()))
	if err != nil {
		return err
	}

	values := kubernetes.Values(map[string]interface{}{
		"priorityClassName": v1beta1constants.PriorityClassNameShootControlPlane,
		"global": map[string]interface{}{
			"ingressClass": ingressClass,
			"images":       imagevector.ImageMapToValues(seedImages),
		},
		"reserveExcessCapacity": seed.GetInfo().Spec.Settings.ExcessCapacityReservation.Enabled,
		"replicas": map[string]interface{}{
			"reserve-excess-capacity": desiredExcessCapacity(),
		},
		"prometheus": map[string]interface{}{
			"resources":               monitoringResources["prometheus"],
			"storage":                 seed.GetValidVolumeSize("10Gi"),
			"additionalScrapeConfigs": centralScrapeConfigs.String(),
			"additionalCAdvisorScrapeConfigMetricRelabelConfigs": centralCAdvisorScrapeConfigMetricRelabelConfigs.String(),
		},
		"aggregatePrometheus": map[string]interface{}{
			"resources":  monitoringResources["aggregate-prometheus"],
			"storage":    seed.GetValidVolumeSize("20Gi"),
			"seed":       seed.GetInfo().Name,
			"hostName":   prometheusHost,
			"secretName": prometheusIngressTLSSecretName,
		},
		"grafana": map[string]interface{}{
			"hostName":   grafanaHost,
			"secretName": grafanaIngressTLSSecretName,
		},
		"fluent-bit": map[string]interface{}{
			"enabled":                           loggingEnabled,
			"additionalParsers":                 parsers.String(),
			"additionalFilters":                 filters.String(),
			"fluentBitConfigurationsOverwrites": fluentBitConfigurationsOverwrites,
			"exposedComponentsTagPrefix":        userExposedComponentTagPrefix,
		},
		"loki":         lokiValues,
		"alertmanager": alertManagerConfig,
		"hvpa": map[string]interface{}{
			"enabled": hvpaEnabled,
		},
		"istio": map[string]interface{}{
			"enabled": gardenletfeatures.FeatureGate.Enabled(features.ManagedIstio),
		},
		"ingress": map[string]interface{}{
			"authSecretName": globalMonitoringSecretSeed.Name,
		},
	})

	if err := chartApplier.Apply(ctx, filepath.Join(charts.Path, seedBoostrapChartName), v1beta1constants.GardenNamespace, seedBoostrapChartName, values, applierOptions); err != nil {
		return err
	}

	// TODO(rfranzke): Remove in a future release.
	if err := kutil.DeleteObjects(ctx, seedClient,
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "seed-monitoring-ingress-credentials", Namespace: v1beta1constants.GardenNamespace}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "grafana-basic-auth", Namespace: v1beta1constants.GardenNamespace}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "aggregate-prometheus-basic-auth", Namespace: v1beta1constants.GardenNamespace}},
	); err != nil {
		return err
	}

	if !managedIngress(seed) {
		nginxIngress := nginxingress.New(seedClient, v1beta1constants.GardenNamespace, nginxingress.Values{})

		if err := component.OpDestroyAndWait(nginxIngress).Destroy(ctx); err != nil {
			return err
		}
	}

	if err := migrateIngressClassForShootIngresses(ctx, gardenClient, seedClient, seed, ingressClass, kubernetesVersion); err != nil {
		return err
	}

	if err := runCreateSeedFlow(ctx, gardenClient, seedClient, kubernetesVersion, secretsManager, imageVector, imageVectorOverwrites, seed, conf, log, anySNI); err != nil {
		return err
	}

	return secretsManager.Cleanup(ctx)
}

func runCreateSeedFlow(
	ctx context.Context,
	gardenClient,
	seedClient client.Client,
	kubernetesVersion *semver.Version,
	secretsManager secretsmanager.Interface,
	imageVector imagevector.ImageVector,
	imageVectorOverwrites map[string]string,
	seed *Seed,
	conf *config.GardenletConfiguration,
	log logrus.FieldLogger,
	anySNI bool,
) error {
	secretData, err := getDNSProviderSecretData(ctx, gardenClient, seed)
	if err != nil {
		return err
	}

	// setup for flow graph
	var ingressLoadBalancerAddress string
	if managedIngress(seed) {
		providerConfig, err := getConfig(seed)
		if err != nil {
			return err
		}
		nginxIngress, err := defaultNginxIngress(seedClient, imageVector, kubernetesVersion, ingressClass, providerConfig)
		if err != nil {
			return err
		}

		if err = component.OpWaiter(nginxIngress).Deploy(ctx); err != nil {
			return err
		}

		ingressLoadBalancerAddress, err = kutil.WaitUntilLoadBalancerIsReady(ctx, seedClient, v1beta1constants.GardenNamespace, "nginx-ingress-controller", time.Minute, log)
		if err != nil {
			return err
		}
	}

	dnsEntry := getManagedIngressDNSEntry(seedClient, seed.GetIngressFQDN("*"), *seed.GetInfo().Status.ClusterIdentity, ingressLoadBalancerAddress, log)
	dnsOwner := getManagedIngressDNSOwner(seedClient, *seed.GetInfo().Status.ClusterIdentity)
	dnsRecord := getManagedIngressDNSRecord(seedClient, seed.GetInfo().Spec.DNS, secretData, seed.GetIngressFQDN("*"), ingressLoadBalancerAddress, log)

	networkPolicies, err := defaultNetworkPolicies(seedClient, seed.GetInfo(), anySNI)
	if err != nil {
		return err
	}
	etcdDruid, err := defaultEtcdDruid(seedClient, kubernetesVersion.String(), conf, imageVector, imageVectorOverwrites)
	if err != nil {
		return err
	}
	gardenerSeedAdmissionController, err := defaultGardenerSeedAdmissionController(seedClient, imageVector, secretsManager)
	if err != nil {
		return err
	}
	kubeScheduler, err := defaultKubeScheduler(seedClient, imageVector, kubernetesVersion)
	if err != nil {
		return err
	}
	dwdEndpoint, dwdProbe, err := defaultDependencyWatchdogs(seedClient, kubernetesVersion.String(), imageVector, seed.GetInfo().Spec.Settings)
	if err != nil {
		return err
	}
	vpa, err := defaultVerticalPodAutoscaler(seedClient, imageVector, secretsManager)
	if err != nil {
		return err
	}
	vpnAuthzServer, err := defaultVPNAuthzServer(ctx, seedClient, kubernetesVersion.String(), imageVector)
	if err != nil {
		return err
	}

	var (
		g = flow.NewGraph("Seed cluster creation")
		_ = g.Add(flow.Task{
			Name: "Ensuring network policies",
			Fn:   networkPolicies.Deploy,
		})
		_ = g.Add(flow.Task{
			Name: "Deploying managed ingress DNS record",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return deployDNSResources(ctx, dnsEntry, dnsOwner, dnsRecord, deployDNSProviderTask(seedClient, seed.GetInfo().Spec.DNS), destroyDNSProviderTask(seedClient))
			}).DoIf(managedIngress(seed)),
		})
		_ = g.Add(flow.Task{
			Name: "Destroying managed ingress DNS record (if existing)",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return destroyDNSResources(ctx, dnsEntry, dnsOwner, dnsRecord, destroyDNSProviderTask(seedClient))
			}).DoIf(!managedIngress(seed)),
		})
		_ = g.Add(flow.Task{
			Name: "Deploying cluster-identity",
			Fn:   clusteridentity.NewForSeed(seedClient, v1beta1constants.GardenNamespace, *seed.GetInfo().Status.ClusterIdentity).Deploy,
		})
		_ = g.Add(flow.Task{
			Name: "Deploying cluster-autoscaler",
			Fn:   clusterautoscaler.NewBootstrapper(seedClient, v1beta1constants.GardenNamespace).Deploy,
		})
		_ = g.Add(flow.Task{
			Name: "Deploying etcd-druid",
			Fn:   etcdDruid.Deploy,
		})
		_ = g.Add(flow.Task{
			Name: "Deploying gardener-seed-admission-controller",
			Fn:   gardenerSeedAdmissionController.Deploy,
		})
		_ = g.Add(flow.Task{
			Name: "Deploying kube-scheduler for shoot control plane pods",
			Fn:   kubeScheduler.Deploy,
		})
		_ = g.Add(flow.Task{
			Name: "Deploying dependency-watchdog-endpoint",
			Fn:   dwdEndpoint.Deploy,
		})
		_ = g.Add(flow.Task{
			Name: "Deploying dependency-watchdog-probe",
			Fn:   dwdProbe.Deploy,
		})
		_ = g.Add(flow.Task{
			Name: "Deploying Kubernetes vertical pod autoscaler",
			Fn:   vpa.Deploy,
		})
		_ = g.Add(flow.Task{
			Name: "Deploying VPN authorization server",
			Fn:   vpnAuthzServer.Deploy,
		})
	)

	if err := g.Compile().Run(ctx, flow.Opts{Logger: log}); err != nil {
		return flow.Errors(err)
	}

	return nil
}

// RunDeleteSeedFlow deletes certain resources from the seed cluster.
func RunDeleteSeedFlow(
	ctx context.Context,
	gardenClient client.Client,
	seedClientSet kubernetes.Interface,
	seed *Seed,
	conf *config.GardenletConfiguration,
	log logrus.FieldLogger,
) error {
	seedClient := seedClientSet.Client()
	kubernetesVersion, err := semver.NewVersion(seedClientSet.Version())
	if err != nil {
		return err
	}

	secretData, err := getDNSProviderSecretData(ctx, gardenClient, seed)
	if err != nil {
		return err
	}

	// setup for flow graph
	var (
		dnsEntry        = getManagedIngressDNSEntry(seedClient, seed.GetIngressFQDN("*"), *seed.GetInfo().Status.ClusterIdentity, "", log)
		dnsOwner        = getManagedIngressDNSOwner(seedClient, *seed.GetInfo().Status.ClusterIdentity)
		dnsRecord       = getManagedIngressDNSRecord(seedClient, seed.GetInfo().Spec.DNS, secretData, seed.GetIngressFQDN("*"), "", log)
		autoscaler      = clusterautoscaler.NewBootstrapper(seedClient, v1beta1constants.GardenNamespace)
		gsac            = seedadmissioncontroller.New(seedClient, v1beta1constants.GardenNamespace, nil, "")
		resourceManager = resourcemanager.New(seedClient, v1beta1constants.GardenNamespace, nil, "", resourcemanager.Values{})
		nginxIngress    = nginxingress.New(seedClient, v1beta1constants.GardenNamespace, nginxingress.Values{})
		etcdDruid       = etcd.NewBootstrapper(seedClient, v1beta1constants.GardenNamespace, conf, "", nil)
		networkPolicies = networkpolicies.NewBootstrapper(seedClient, v1beta1constants.GardenNamespace, networkpolicies.GlobalValues{})
		clusterIdentity = clusteridentity.NewForSeed(seedClient, v1beta1constants.GardenNamespace, "")
		dwdEndpoint     = dependencywatchdog.NewBootstrapper(seedClient, v1beta1constants.GardenNamespace, dependencywatchdog.BootstrapperValues{Role: dependencywatchdog.RoleEndpoint})
		dwdProbe        = dependencywatchdog.NewBootstrapper(seedClient, v1beta1constants.GardenNamespace, dependencywatchdog.BootstrapperValues{Role: dependencywatchdog.RoleProbe})
		vpa             = vpa.New(seedClient, v1beta1constants.GardenNamespace, nil, vpa.Values{ClusterType: vpa.ClusterTypeSeed})
		vpnAuthzServer  = vpnauthzserver.New(seedClient, v1beta1constants.GardenNamespace, "", 1)
	)

	scheduler, err := gardenerkubescheduler.Bootstrap(seedClient, v1beta1constants.GardenNamespace, nil, kubernetesVersion)
	if err != nil {
		return err
	}

	var (
		g                = flow.NewGraph("Seed cluster deletion")
		destroyDNSRecord = g.Add(flow.Task{
			Name: "Destroying managed ingress DNS record (if existing)",
			Fn: func(ctx context.Context) error {
				return destroyDNSResources(ctx, dnsEntry, dnsOwner, dnsRecord, destroyDNSProviderTask(seedClient))
			},
		})
		noControllerInstallations = g.Add(flow.Task{
			Name:         "Ensuring no ControllerInstallations are left",
			Fn:           ensureNoControllerInstallations(gardenClient, seed.GetInfo().Name),
			Dependencies: flow.NewTaskIDs(destroyDNSRecord),
		})
		destroyClusterIdentity = g.Add(flow.Task{
			Name: "Destroying cluster-identity",
			Fn:   component.OpDestroyAndWait(clusterIdentity).Destroy,
		})
		destroyClusterAutoscaler = g.Add(flow.Task{
			Name: "Destroying cluster-autoscaler",
			Fn:   component.OpDestroyAndWait(autoscaler).Destroy,
		})
		destroyEtcdDruid = g.Add(flow.Task{
			Name: "Destroying etcd druid",
			Fn:   component.OpDestroyAndWait(etcdDruid).Destroy,
		})
		destroySeedAdmissionController = g.Add(flow.Task{
			Name: "Destroying gardener-seed-admission-controller",
			Fn:   component.OpDestroyAndWait(gsac).Destroy,
		})
		destroyNginxIngress = g.Add(flow.Task{
			Name: "Destroying nginx-ingress",
			Fn:   component.OpDestroyAndWait(nginxIngress).Destroy,
		})
		destroyKubeScheduler = g.Add(flow.Task{
			Name: "Destroying kube-scheduler",
			Fn:   component.OpDestroyAndWait(scheduler).Destroy,
		})
		destroyNetworkPolicies = g.Add(flow.Task{
			Name: "Destroy network policies",
			Fn:   component.OpDestroyAndWait(networkPolicies).Destroy,
		})
		destroyDWDEndpoint = g.Add(flow.Task{
			Name: "Destroy dependency-watchdog-endpoint",
			Fn:   component.OpDestroyAndWait(dwdEndpoint).Destroy,
		})
		destroyDWDProbe = g.Add(flow.Task{
			Name: "Destroy dependency-watchdog-probe",
			Fn:   component.OpDestroyAndWait(dwdProbe).Destroy,
		})
		destroyVPA = g.Add(flow.Task{
			Name: "Destroy Kubernetes vertical pod autoscaler",
			Fn:   component.OpDestroyAndWait(vpa).Destroy,
		})
		destroyVPNAuthzServer = g.Add(flow.Task{
			Name: "Destroy VPN authorization server",
			Fn:   component.OpDestroyAndWait(vpnAuthzServer).Destroy,
		})
		_ = g.Add(flow.Task{
			Name: "Destroying gardener-resource-manager",
			Fn:   resourceManager.Destroy,
			Dependencies: flow.NewTaskIDs(
				destroySeedAdmissionController,
				destroyNginxIngress,
				destroyEtcdDruid,
				destroyClusterIdentity,
				destroyClusterAutoscaler,
				destroyKubeScheduler,
				destroyNetworkPolicies,
				destroyDWDEndpoint,
				destroyDWDProbe,
				destroyVPA,
				destroyVPNAuthzServer,
				noControllerInstallations,
			),
		})
	)

	if err := g.Compile().Run(ctx, flow.Opts{Logger: log}); err != nil {
		return flow.Errors(err)
	}

	return nil
}

func deployDNSResources(ctx context.Context, dnsEntry, dnsOwner component.DeployWaiter, dnsRecord component.DeployMigrateWaiter, deployDNSProviderTask, destroyDNSProviderTask flow.TaskFn) error {
	if err := dnsOwner.Destroy(ctx); err != nil {
		return err
	}
	if err := dnsOwner.WaitCleanup(ctx); err != nil {
		return err
	}
	if err := destroyDNSProviderTask(ctx); err != nil {
		return err
	}
	if err := dnsEntry.Destroy(ctx); err != nil {
		return err
	}
	if err := dnsEntry.WaitCleanup(ctx); err != nil {
		return err
	}
	if err := dnsRecord.Deploy(ctx); err != nil {
		return err
	}
	return dnsRecord.Wait(ctx)
}

func destroyDNSResources(ctx context.Context, dnsEntry, dnsOwner, dnsRecord component.DeployWaiter, destroyDNSProviderTask flow.TaskFn) error {
	if err := dnsEntry.Destroy(ctx); err != nil {
		return err
	}
	if err := dnsEntry.WaitCleanup(ctx); err != nil {
		return err
	}
	if err := destroyDNSProviderTask(ctx); err != nil {
		return err
	}
	if err := dnsOwner.Destroy(ctx); err != nil {
		return err
	}
	if err := dnsOwner.WaitCleanup(ctx); err != nil {
		return err
	}
	if err := dnsRecord.Destroy(ctx); err != nil {
		return err
	}
	return dnsRecord.WaitCleanup(ctx)
}

func ensureNoControllerInstallations(c client.Client, seedName string) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		associatedControllerInstallations, err := controllerutils.DetermineControllerInstallationAssociations(ctx, c, seedName)
		if err != nil {
			return err
		}
		if associatedControllerInstallations != nil {
			return fmt.Errorf("can't continue with Seed deletion, because the following objects are still referencing it: ControllerInstallations=%v", associatedControllerInstallations)
		}
		return nil
	}
}

func deployDNSProviderTask(seedClient client.Client, dnsConfig gardencorev1beta1.SeedDNS) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		var (
			dnsProvider    = emptyDNSProvider()
			providerSecret = emptyDNSProviderSecret()
		)

		_, err := controllerutils.GetAndCreateOrMergePatch(ctx, seedClient, dnsProvider, func() error {
			dnsProvider.Spec = dnsv1alpha1.DNSProviderSpec{
				Type: dnsConfig.Provider.Type,
				SecretRef: &corev1.SecretReference{
					Namespace: providerSecret.Namespace,
					Name:      providerSecret.Name,
				},
			}

			if dnsConfig.Provider.Domains != nil {
				dnsProvider.Spec.Domains = &dnsv1alpha1.DNSSelection{
					Include: dnsConfig.Provider.Domains.Include,
					Exclude: dnsConfig.Provider.Domains.Exclude,
				}
			}

			if dnsConfig.Provider.Zones != nil {
				dnsProvider.Spec.Zones = &dnsv1alpha1.DNSSelection{
					Include: dnsConfig.Provider.Zones.Include,
					Exclude: dnsConfig.Provider.Zones.Exclude,
				}
			}

			return nil
		})
		return err
	}
}

func destroyDNSProviderTask(seedClient client.Client) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return kutil.DeleteObjects(ctx, seedClient, emptyDNSProvider(), emptyDNSProviderSecret())
	}
}

func emptyDNSProvider() *dnsv1alpha1.DNSProvider {
	return &dnsv1alpha1.DNSProvider{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: v1beta1constants.GardenNamespace,
			Name:      "seed",
		},
	}
}

func emptyDNSProviderSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: v1beta1constants.GardenNamespace,
			Name:      "dnsprovider-seed",
		},
	}
}

func getDNSProviderSecretData(ctx context.Context, gardenClient client.Client, seed *Seed) (map[string][]byte, error) {
	if dnsConfig := seed.GetInfo().Spec.DNS; dnsConfig.Provider != nil {
		secret, err := kutil.GetSecretByReference(ctx, gardenClient, &dnsConfig.Provider.SecretRef)
		if err != nil {
			return nil, err
		}
		return secret.Data, nil
	}
	return nil, nil
}

// desiredExcessCapacity computes the required resources (CPU and memory) required to deploy new shoot control planes
// (on the seed) in terms of reserve-excess-capacity deployment replicas. Each deployment replica currently
// corresponds to resources of (request/limits) 2 cores of CPU and 6Gi of RAM.
// This roughly corresponds to a single, moderately large control-plane.
// The logic for computation of desired excess capacity corresponds to deploying 2 such shoot control planes.
// This excess capacity can be used for hosting new control planes or newly vertically scaled old control-planes.
func desiredExcessCapacity() int {
	var (
		replicasToSupportSingleShoot = 1
		effectiveExcessCapacity      = 2
	)

	return effectiveExcessCapacity * replicasToSupportSingleShoot
}

// GetIngressFQDN returns the fully qualified domain name of ingress sub-resource for the Seed cluster. The
// end result is '<subDomain>.<shootName>.<projectName>.<seed-ingress-domain>'.
func (s *Seed) GetIngressFQDN(subDomain string) string {
	return fmt.Sprintf("%s.%s", subDomain, s.IngressDomain())
}

// IngressDomain returns the ingress domain for the seed.
func (s *Seed) IngressDomain() string {
	seed := s.GetInfo()
	if seed.Spec.DNS.IngressDomain != nil {
		return *seed.Spec.DNS.IngressDomain
	} else if seed.Spec.Ingress != nil {
		return seed.Spec.Ingress.Domain
	}
	return ""
}

// CheckMinimumK8SVersion checks whether the Kubernetes version of the Seed cluster fulfills the minimal requirements.
func (s *Seed) CheckMinimumK8SVersion(version string) (string, error) {
	const minSeedVersion = "1.18"

	seedVersionOK, err := versionutils.CompareVersions(version, ">=", minSeedVersion)
	if err != nil {
		return "<unknown>", err
	}
	if !seedVersionOK {
		return "<unknown>", fmt.Errorf("the Kubernetes version of the Seed cluster must be at least %s", minSeedVersion)
	}
	return version, nil
}

// GetValidVolumeSize is to get a valid volume size.
// If the given size is smaller than the minimum volume size permitted by cloud provider on which seed cluster is running, it will return the minimum size.
func (s *Seed) GetValidVolumeSize(size string) string {
	seed := s.GetInfo()
	if seed.Spec.Volume == nil || seed.Spec.Volume.MinimumSize == nil {
		return size
	}

	qs, err := resource.ParseQuantity(size)
	if err == nil && qs.Cmp(*seed.Spec.Volume.MinimumSize) < 0 {
		return seed.Spec.Volume.MinimumSize.String()
	}

	return size
}

// GetWildcardCertificate gets the wildcard certificate for the seed's ingress domain.
// Nil is returned if no wildcard certificate is configured.
func GetWildcardCertificate(ctx context.Context, c client.Client) (*corev1.Secret, error) {
	wildcardCerts := &corev1.SecretList{}
	if err := c.List(
		ctx,
		wildcardCerts,
		client.InNamespace(v1beta1constants.GardenNamespace),
		client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlaneWildcardCert},
	); err != nil {
		return nil, err
	}

	if len(wildcardCerts.Items) > 1 {
		return nil, fmt.Errorf("misconfigured seed cluster: not possible to provide more than one secret with annotation %s", v1beta1constants.GardenRoleControlPlaneWildcardCert)
	}

	if len(wildcardCerts.Items) == 1 {
		return &wildcardCerts.Items[0], nil
	}
	return nil, nil
}

// determineClusterIdentity determines the identity of a cluster, in cases where the identity was
// created manually or the Seed was created as Shoot, and later registered as Seed and already has
// an identity, it should not be changed.
func determineClusterIdentity(ctx context.Context, c client.Client) (string, error) {
	clusterIdentity := &corev1.ConfigMap{}
	if err := c.Get(ctx, kutil.Key(metav1.NamespaceSystem, v1beta1constants.ClusterIdentity), clusterIdentity); err != nil {
		if !apierrors.IsNotFound(err) {
			return "", err
		}

		gardenNamespace := &corev1.Namespace{}
		if err := c.Get(ctx, kutil.Key(metav1.NamespaceSystem), gardenNamespace); err != nil {
			return "", err
		}
		return string(gardenNamespace.UID), nil
	}
	return clusterIdentity.Data[v1beta1constants.ClusterIdentity], nil
}

func deletePriorityClassIfValueNotTheSame(ctx context.Context, k8sClient client.Client, priorityClassName string, valueToCompare int32) error {
	pc := &schedulingv1.PriorityClass{}
	err := k8sClient.Get(ctx, kutil.Key(priorityClassName), pc)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}
		return nil
	}
	if valueToCompare == pc.Value {
		return nil
	}

	return client.IgnoreNotFound(k8sClient.Delete(ctx, pc))
}

func cleanupOrphanExposureClassHandlerResources(ctx context.Context, c client.Client, exposureClassHandlers []config.ExposureClassHandler, log logrus.FieldLogger) error {
	exposureClassHandlerNamespaces := &corev1.NamespaceList{}

	if err := c.List(ctx, exposureClassHandlerNamespaces, client.MatchingLabels{v1alpha1constants.GardenRole: v1alpha1constants.GardenRoleExposureClassHandler}); err != nil {
		return err
	}

	for _, namespace := range exposureClassHandlerNamespaces.Items {
		var exposureClassHandlerExists bool
		for _, handler := range exposureClassHandlers {
			if *handler.SNI.Ingress.Namespace == namespace.Name {
				exposureClassHandlerExists = true
				break
			}
		}
		if exposureClassHandlerExists {
			continue
		}
		log.Infof("Namespace %q is orphan as there is no ExposureClass handler in the gardenlet configuration anymore", namespace.Name)

		// Determine the corresponding handler name to the ExposureClass handler resources.
		handlerName, ok := namespace.Labels[v1alpha1constants.LabelExposureClassHandlerName]
		if !ok {
			log.Info("Cannot delete ExposureClass handler resources as the corresponging handler is unknown and it is not save to remove them")
			continue
		}

		gatewayList := istiov1beta1.GatewayList{}
		if err := c.List(ctx, &gatewayList); err != nil {
			return err
		}

		var exposureClassHandlerInUse bool
		for _, gateway := range gatewayList.Items {
			if gateway.Name != v1beta1constants.DeploymentNameKubeAPIServer && gateway.Name != v1beta1constants.DeploymentNameVPNSeedServer {
				continue
			}
			// Check if the gateway still selects the ExposureClass handler ingress gateway.
			if value, ok := gateway.Spec.Selector[v1alpha1constants.LabelExposureClassHandlerName]; ok && value == handlerName {
				exposureClassHandlerInUse = true
				break
			}
		}
		if exposureClassHandlerInUse {
			log.Infof("Resources of ExposureClass handler %q in namespace %q cannot be deleted as they are still in use", handlerName, namespace.Name)
			continue
		}

		// ExposureClass handler is orphan and not used by any Shoots anymore
		// therefore it is save to clean it up.
		log.Infof("Delete orphan ExposureClass handler namespace %q", namespace.Name)
		if err := c.Delete(ctx, &namespace); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	return nil
}

// ResizeOrDeleteLokiDataVolumeIfStorageNotTheSame updates the garden Loki PVC if passed storage value is not the same as the current one.
// Caution: If the passed storage capacity is less than the current one the existing PVC and its PV will be deleted.
func ResizeOrDeleteLokiDataVolumeIfStorageNotTheSame(ctx context.Context, k8sClient client.Client, newStorageQuantity resource.Quantity, log logrus.FieldLogger) error {
	// Check if we need resizing
	pvc := &corev1.PersistentVolumeClaim{}
	if err := k8sClient.Get(ctx, kutil.Key(v1beta1constants.GardenNamespace, "loki-loki-0"), pvc); err != nil {
		return client.IgnoreNotFound(err)
	}

	storageCmpResult := newStorageQuantity.Cmp(*pvc.Spec.Resources.Requests.Storage())
	if storageCmpResult == 0 {
		return nil
	}

	log.Infof("Scaling StatefulSet garden/loki to zero in order to detach PVC %q", pvc.Name)
	if err := kubernetes.ScaleStatefulSetAndWaitUntilScaled(ctx, k8sClient, kutil.Key(v1beta1constants.GardenNamespace, v1beta1constants.StatefulSetNameLoki), 0); client.IgnoreNotFound(err) != nil {
		return err
	}

	switch {
	case storageCmpResult > 0:
		patch := client.MergeFrom(pvc.DeepCopy())
		pvc.Spec.Resources.Requests = corev1.ResourceList{
			corev1.ResourceStorage: newStorageQuantity,
		}
		log.Infof("Patching garden/loki's PVC %q to %q of storage", pvc.Name, newStorageQuantity.String())
		if err := k8sClient.Patch(ctx, pvc, patch); client.IgnoreNotFound(err) != nil {
			return err
		}
	case storageCmpResult < 0:
		log.Infof("Deleting garden/loki's PVC %q because size needs to be reduced", pvc.Name)
		if err := client.IgnoreNotFound(k8sClient.Delete(ctx, pvc)); err != nil {
			return err
		}
	}

	lokiSts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.StatefulSetNameLoki, Namespace: v1beta1constants.GardenNamespace}}
	return client.IgnoreNotFound(k8sClient.Delete(ctx, lokiSts))
}
