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
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/Masterminds/semver"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/operator/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/etcd"
	"github.com/gardener/gardener/pkg/operation/botanist/component/gardeneraccess"
	"github.com/gardener/gardener/pkg/operation/botanist/component/gardensystem"
	"github.com/gardener/gardener/pkg/operation/botanist/component/istio"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserverexposure"
	"github.com/gardener/gardener/pkg/operation/botanist/component/resourcemanager"
	sharedcomponent "github.com/gardener/gardener/pkg/operation/botanist/component/shared"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/timewindow"
)

const namePrefix = "virtual-garden-"

func (r *Reconciler) newGardenerResourceManager(garden *operatorv1alpha1.Garden, secretsManager secretsmanager.Interface) (component.DeployWaiter, error) {
	var defaultNotReadyTolerationSeconds, defaultUnreachableTolerationSeconds *int64
	if nodeToleration := r.Config.NodeToleration; nodeToleration != nil {
		defaultNotReadyTolerationSeconds = nodeToleration.DefaultNotReadyTolerationSeconds
		defaultUnreachableTolerationSeconds = nodeToleration.DefaultUnreachableTolerationSeconds
	}

	return sharedcomponent.NewRuntimeGardenerResourceManager(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		r.RuntimeVersion,
		r.ImageVector,
		secretsManager,
		r.Config.LogLevel, r.Config.LogFormat,
		operatorv1alpha1.SecretNameCARuntime,
		v1beta1constants.PriorityClassNameGardenSystemCritical,
		defaultNotReadyTolerationSeconds,
		defaultUnreachableTolerationSeconds,
		features.DefaultFeatureGate.Enabled(features.DefaultSeccompProfile),
		helper.TopologyAwareRoutingEnabled(garden.Spec.RuntimeCluster.Settings),
		features.DefaultFeatureGate.Enabled(features.FullNetworkPoliciesInRuntimeCluster),
		garden.Spec.RuntimeCluster.Provider.Zones,
	)
}

func (r *Reconciler) newVirtualGardenGardenerResourceManager(garden *operatorv1alpha1.Garden, secretsManager secretsmanager.Interface) (resourcemanager.Interface, error) {
	return sharedcomponent.NewTargetGardenerResourceManager(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		r.ImageVector,
		secretsManager,
		nil,
		nil,
		nil,
		r.RuntimeVersion,
		r.Config.LogLevel, r.Config.LogFormat,
		namePrefix,
		false,
		v1beta1constants.PriorityClassNameGardenSystem500,
		nil,
		operatorv1alpha1.SecretNameCARuntime,
		nil,
		false,
	)
}

func (r *Reconciler) newVerticalPodAutoscaler(garden *operatorv1alpha1.Garden, secretsManager secretsmanager.Interface) (component.DeployWaiter, error) {
	return sharedcomponent.NewVerticalPodAutoscaler(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		r.RuntimeVersion,
		r.ImageVector,
		secretsManager,
		vpaEnabled(garden.Spec.RuntimeCluster.Settings),
		operatorv1alpha1.SecretNameCARuntime,
		v1beta1constants.PriorityClassNameGardenSystem300,
		v1beta1constants.PriorityClassNameGardenSystem200,
		v1beta1constants.PriorityClassNameGardenSystem200,
	)
}

func (r *Reconciler) newHVPA() (component.DeployWaiter, error) {
	return sharedcomponent.NewHVPA(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		r.RuntimeVersion,
		r.ImageVector,
		hvpaEnabled(),
		v1beta1constants.PriorityClassNameGardenSystem200,
	)
}

func (r *Reconciler) newEtcdDruid() (component.DeployWaiter, error) {
	return sharedcomponent.NewEtcdDruid(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		r.RuntimeVersion,
		r.ImageVector,
		r.ComponentImageVectors,
		r.Config.Controllers.Garden.ETCDConfig,
		v1beta1constants.PriorityClassNameGardenSystem300,
	)
}

func (r *Reconciler) newSystem() component.DeployWaiter {
	return gardensystem.New(r.RuntimeClientSet.Client(), r.GardenNamespace)
}

func (r *Reconciler) newEtcd(
	log logr.Logger,
	garden *operatorv1alpha1.Garden,
	secretsManager secretsmanager.Interface,
	role string,
	class etcd.Class,
) (
	etcd.Interface,
	error,
) {
	var (
		hvpaScaleDownUpdateMode       *string
		defragmentationScheduleFormat string
		storageClassName              *string
		storageCapacity               string
	)

	switch role {
	case v1beta1constants.ETCDRoleMain:
		hvpaScaleDownUpdateMode = pointer.String(hvpav1alpha1.UpdateModeOff)
		defragmentationScheduleFormat = "%d %d * * *" // defrag main etcd daily in the maintenance window
		storageCapacity = "25Gi"
		if etcd := garden.Spec.VirtualCluster.ETCD; etcd != nil && etcd.Main != nil && etcd.Main.Storage != nil {
			storageClassName = etcd.Main.Storage.ClassName
			if etcd.Main.Storage.Capacity != nil {
				storageCapacity = etcd.Main.Storage.Capacity.String()
			}
		}

	case v1beta1constants.ETCDRoleEvents:
		hvpaScaleDownUpdateMode = pointer.String(hvpav1alpha1.UpdateModeMaintenanceWindow)
		defragmentationScheduleFormat = "%d %d */3 * *"
		storageCapacity = "10Gi"
		if etcd := garden.Spec.VirtualCluster.ETCD; etcd != nil && etcd.Events != nil && etcd.Events.Storage != nil {
			storageClassName = etcd.Events.Storage.ClassName
			if etcd.Events.Storage.Capacity != nil {
				storageCapacity = etcd.Events.Storage.Capacity.String()
			}
		}
	}

	defragmentationSchedule, err := timewindow.DetermineSchedule(
		defragmentationScheduleFormat,
		garden.Spec.VirtualCluster.Maintenance.TimeWindow.Begin,
		garden.Spec.VirtualCluster.Maintenance.TimeWindow.End,
		garden.UID,
		garden.CreationTimestamp,
		timewindow.RandomizeWithinTimeWindow,
	)
	if err != nil {
		return nil, err
	}

	highAvailabilityEnabled := helper.HighAvailabilityEnabled(garden)

	replicas := pointer.Int32(1)
	if highAvailabilityEnabled {
		replicas = pointer.Int32(3)
	}

	return etcd.New(
		log,
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		secretsManager,
		etcd.Values{
			NamePrefix:               namePrefix,
			Role:                     role,
			Class:                    class,
			Replicas:                 replicas,
			StorageCapacity:          storageCapacity,
			StorageClassName:         storageClassName,
			DefragmentationSchedule:  &defragmentationSchedule,
			CARotationPhase:          helper.GetCARotationPhase(garden.Status.Credentials),
			RuntimeKubernetesVersion: r.RuntimeVersion,
			HvpaConfig: &etcd.HVPAConfig{
				Enabled:               hvpaEnabled(),
				MaintenanceTimeWindow: garden.Spec.VirtualCluster.Maintenance.TimeWindow,
				ScaleDownUpdateMode:   hvpaScaleDownUpdateMode,
			},
			PriorityClassName:           v1beta1constants.PriorityClassNameGardenSystem500,
			HighAvailabilityEnabled:     highAvailabilityEnabled,
			TopologyAwareRoutingEnabled: helper.TopologyAwareRoutingEnabled(garden.Spec.RuntimeCluster.Settings),
		},
	), nil
}

func (r *Reconciler) newKubeAPIServerService(log logr.Logger, garden *operatorv1alpha1.Garden) component.DeployWaiter {
	var (
		annotations map[string]string
		clusterIP   string
	)

	if settings := garden.Spec.RuntimeCluster.Settings; settings != nil && settings.LoadBalancerServices != nil {
		annotations = settings.LoadBalancerServices.Annotations
	}

	if os.Getenv("GARDENER_OPERATOR_LOCAL") == "true" {
		clusterIP = "10.2.10.2"
	}

	return kubeapiserverexposure.NewService(
		log,
		r.RuntimeClientSet.Client(),
		&kubeapiserverexposure.ServiceValues{
			AnnotationsFunc:             func() map[string]string { return annotations },
			SNIPhase:                    component.PhaseDisabled,
			TopologyAwareRoutingEnabled: helper.TopologyAwareRoutingEnabled(garden.Spec.RuntimeCluster.Settings),
			RuntimeKubernetesVersion:    r.RuntimeVersion,
		},
		func() client.ObjectKey {
			return client.ObjectKey{Name: namePrefix + v1beta1constants.DeploymentNameKubeAPIServer, Namespace: r.GardenNamespace}
		},
		nil,
		nil,
		nil,
		nil,
		false,
		clusterIP,
	)
}

func (r *Reconciler) newKubeAPIServer(
	ctx context.Context,
	garden *operatorv1alpha1.Garden,
	secretsManager secretsmanager.Interface,
	targetVersion *semver.Version,
) (
	kubeapiserver.Interface,
	error,
) {
	var (
		err                          error
		apiServerConfig              *gardencorev1beta1.KubeAPIServerConfig
		auditWebhookConfig           *kubeapiserver.AuditWebhook
		authenticationWebhookConfig  *kubeapiserver.AuthenticationWebhook
		authorizationWebhookConfig   *kubeapiserver.AuthorizationWebhook
		resourcesToStoreInETCDEvents []schema.GroupResource
		minReplicas                  int32 = 2
	)

	if garden.Spec.VirtualCluster.ControlPlane != nil && garden.Spec.VirtualCluster.ControlPlane.HighAvailability != nil {
		minReplicas = 3
	}

	if apiServer := garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer; apiServer != nil {
		apiServerConfig = apiServer.KubeAPIServerConfig

		auditWebhookConfig, err = r.computeKubeAPIServerAuditWebhookConfig(ctx, apiServer.AuditWebhook)
		if err != nil {
			return nil, err
		}

		authenticationWebhookConfig, err = r.computeKubeAPIServerAuthenticationWebhookConfig(ctx, apiServer.Authentication)
		if err != nil {
			return nil, err
		}

		authorizationWebhookConfig, err = r.computeKubeAPIServerAuthorizationWebhookConfig(ctx, apiServer.Authorization)
		if err != nil {
			return nil, err
		}

		for _, gr := range apiServer.ResourcesToStoreInETCDEvents {
			resourcesToStoreInETCDEvents = append(resourcesToStoreInETCDEvents, schema.GroupResource{Group: gr.Group, Resource: gr.Resource})
		}
	}

	return sharedcomponent.NewKubeAPIServer(
		ctx,
		r.RuntimeClientSet,
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		metav1.ObjectMeta{Namespace: r.GardenNamespace, Name: garden.Name},
		r.RuntimeVersion,
		targetVersion,
		r.ImageVector,
		secretsManager,
		namePrefix,
		apiServerConfig,
		kubeapiserver.AutoscalingConfig{
			APIServerResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("600m"),
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
			},
			HVPAEnabled:               hvpaEnabled(),
			MinReplicas:               minReplicas,
			MaxReplicas:               6,
			UseMemoryMetricForHvpaHPA: true,
			ScaleDownDisabledForHvpa:  false,
		},
		garden.Spec.VirtualCluster.Networking.Services,
		kubeapiserver.VPNConfig{Enabled: false},
		v1beta1constants.PriorityClassNameGardenSystem500,
		true,
		pointer.Bool(false),
		auditWebhookConfig,
		authenticationWebhookConfig,
		authorizationWebhookConfig,
		resourcesToStoreInETCDEvents,
	)
}

func (r *Reconciler) computeKubeAPIServerAuditWebhookConfig(ctx context.Context, config *operatorv1alpha1.AuditWebhook) (*kubeapiserver.AuditWebhook, error) {
	if config == nil {
		return nil, nil
	}

	key := client.ObjectKey{Namespace: r.GardenNamespace, Name: config.KubeconfigSecretName}
	kubeconfig, err := fetchKubeconfigFromSecret(ctx, r.RuntimeClientSet.Client(), key)
	if err != nil {
		return nil, fmt.Errorf("failed reading kubeconfig for audit webhook from referenced secret %s: %w", key, err)
	}

	return &kubeapiserver.AuditWebhook{
		Kubeconfig:   kubeconfig,
		BatchMaxSize: config.BatchMaxSize,
		Version:      config.Version,
	}, nil
}

func (r *Reconciler) computeKubeAPIServerAuthenticationWebhookConfig(ctx context.Context, config *operatorv1alpha1.Authentication) (*kubeapiserver.AuthenticationWebhook, error) {
	if config == nil || config.Webhook == nil {
		return nil, nil
	}

	key := client.ObjectKey{Namespace: r.GardenNamespace, Name: config.Webhook.KubeconfigSecretName}
	kubeconfig, err := fetchKubeconfigFromSecret(ctx, r.RuntimeClientSet.Client(), key)
	if err != nil {
		return nil, fmt.Errorf("failed reading kubeconfig for audit webhook from referenced secret %s: %w", key, err)
	}

	var cacheTTL *time.Duration
	if config.Webhook.CacheTTL != nil {
		cacheTTL = &config.Webhook.CacheTTL.Duration
	}

	return &kubeapiserver.AuthenticationWebhook{
		Kubeconfig: kubeconfig,
		CacheTTL:   cacheTTL,
		Version:    config.Webhook.Version,
	}, nil
}

func (r *Reconciler) computeKubeAPIServerAuthorizationWebhookConfig(ctx context.Context, config *operatorv1alpha1.Authorization) (*kubeapiserver.AuthorizationWebhook, error) {
	if config == nil || config.Webhook == nil {
		return nil, nil
	}

	key := client.ObjectKey{Namespace: r.GardenNamespace, Name: config.Webhook.KubeconfigSecretName}
	kubeconfig, err := fetchKubeconfigFromSecret(ctx, r.RuntimeClientSet.Client(), key)
	if err != nil {
		return nil, fmt.Errorf("failed reading kubeconfig for audit webhook from referenced secret %s: %w", key, err)
	}

	var cacheAuthorizedTTL, cacheUnauthorizedTTL *time.Duration
	if config.Webhook.CacheAuthorizedTTL != nil {
		cacheAuthorizedTTL = &config.Webhook.CacheAuthorizedTTL.Duration
	}
	if config.Webhook.CacheUnauthorizedTTL != nil {
		cacheUnauthorizedTTL = &config.Webhook.CacheUnauthorizedTTL.Duration
	}

	return &kubeapiserver.AuthorizationWebhook{
		Kubeconfig:           kubeconfig,
		CacheAuthorizedTTL:   cacheAuthorizedTTL,
		CacheUnauthorizedTTL: cacheUnauthorizedTTL,
		Version:              config.Webhook.Version,
	}, nil
}

func fetchKubeconfigFromSecret(ctx context.Context, c client.Client, key client.ObjectKey) ([]byte, error) {
	secret := &corev1.Secret{}
	if err := c.Get(ctx, key, secret); err != nil {
		return nil, err
	}

	kubeconfig, ok := secret.Data["kubeconfig"]
	if !ok || len(kubeconfig) == 0 {
		return nil, errors.New("the secret's field 'kubeconfig' is empty")
	}

	return kubeconfig, nil
}

func (r *Reconciler) newKubeStateMetrics() (component.DeployWaiter, error) {
	return sharedcomponent.NewKubeStateMetrics(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		r.RuntimeVersion,
		r.ImageVector,
		v1beta1constants.PriorityClassNameGardenSystem100,
	)
}

func (r *Reconciler) newIstio(garden *operatorv1alpha1.Garden) (istio.Interface, error) {
	var annotations map[string]string
	if settings := garden.Spec.RuntimeCluster.Settings; settings != nil && settings.LoadBalancerServices != nil {
		annotations = settings.LoadBalancerServices.Annotations
	}

	return sharedcomponent.NewIstio(
		r.RuntimeClientSet.Client(),
		r.ImageVector,
		r.RuntimeClientSet.ChartRenderer(),
		namePrefix,
		v1beta1constants.DefaultSNIIngressNamespace,
		v1beta1constants.PriorityClassNameGardenSystemCritical,
		true,
		sharedcomponent.GetIstioZoneLabels(nil, nil),
		annotations,
		nil,
		nil,
		[]corev1.ServicePort{
			{Name: "tcp", Port: 443, TargetPort: intstr.FromInt(9443)},
		},
		false,
		false,
		garden.Spec.RuntimeCluster.Provider.Zones,
	)
}

func (r *Reconciler) newGardenerAccess(secretsManager secretsmanager.Interface, domain string) component.Deployer {
	return gardeneraccess.New(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		secretsManager,
		gardeneraccess.Values{
			ServerInCluster:    fmt.Sprintf("%s%s.%s.svc.cluster.local", namePrefix, v1beta1constants.DeploymentNameKubeAPIServer, r.GardenNamespace),
			ServerOutOfCluster: gardenerutils.GetAPIServerDomain(domain),
		},
	)
}
