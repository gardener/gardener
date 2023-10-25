// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package plutono

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/logging/vali"
	"github.com/gardener/gardener/pkg/component/plutono/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/istio"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	// ManagedResourceName is the name of the ManagedResource containing the resource specifications.
	ManagedResourceName = "plutono"

	name                          = "plutono"
	plutonoMountPathDashboards    = "/var/lib/plutono/dashboards"
	externalPort                  = 443
	port                          = constants.Port
	ingressTLSCertificateValidity = 730 * 24 * time.Hour
	labelTLSSecretOwner           = "owner"
)

var (
	//go:embed dashboards/garden/garden
	gardenDashboards embed.FS
	//go:embed dashboards/garden/global
	gardenGlobalDashboards embed.FS
	//go:embed dashboards/seed
	seedDashboards embed.FS
	//go:embed dashboards/shoot
	shootDashboards embed.FS
	//go:embed dashboards/garden-shoot
	gardenAndShootDashboards embed.FS
	//go:embed dashboards/common
	commonDashboards embed.FS

	gardenDashboardsPath         = filepath.Join("dashboards", "garden", "garden")
	gardenGlobalDashboardsPath   = filepath.Join("dashboards", "garden", "global")
	seedDashboardsPath           = filepath.Join("dashboards", "seed")
	shootDashboardsPath          = filepath.Join("dashboards", "shoot")
	gardenAndShootDashboardsPath = filepath.Join("dashboards", "garden-shoot")
	commonDashboardsPath         = filepath.Join("dashboards", "common")
)

// Interface contains functions for a Plutono Deployer
type Interface interface {
	component.DeployWaiter
	// SetWildcardCert sets the wildcard tls certificate which is issued for the seed's ingress domain.
	SetWildcardCert(*corev1.Secret)
	// SetDNSConfig sets the DNSConfig.
	SetDNSConfig(*DNSConfig)
}

// Values is a set of configuration values for the plutono component.
type Values struct {
	// AuthSecret is the secret containing plutono credentials.
	AuthSecret *corev1.Secret
	// ClusterType specifies the type of the cluster to which plutono is being deployed.
	ClusterType component.ClusterType
	// Image is the container image used for plutono.
	Image string
	// IngressHost is the host name of plutono.
	IngressHost string
	// IncludeIstioDashboards specifies whether to include istio dashboard.
	IncludeIstioDashboards bool
	// IsWorkerless specifies whether the cluster managed by this API server has worker nodes.
	IsWorkerless bool
	// IsGardenCluster specifies whether the cluster is garden cluster.
	IsGardenCluster bool
	// NodeLocalDNSEnabled specifies whether the node-local-dns is enabled for cluster.
	NodeLocalDNSEnabled bool
	// PriorityClassName is the name of the priority class.
	PriorityClassName string
	// Replicas is the number of pod replicas for the plutono.
	Replicas int32
	// VPAEnabled states whether VerticalPodAutoscaler is enabled.
	VPAEnabled bool
	// VPNHighAvailabilityEnabled specifies whether the cluster is configured with HA VPN.
	VPNHighAvailabilityEnabled bool
	// WildcardCert is the wildcard tls certificate which is issued for the seed's ingress domain.
	WildcardCert *corev1.Secret
	// IstioIngressGatewayLabels are the labels for identifying the used istio ingress gateway.
	IstioIngressGatewayLabels map[string]string
	// IstioIngressGatewayNamespace is the namespace of the used istio ingress gateway.
	IstioIngressGatewayNamespace string
	// DNSConfig contains the configuration values used to create a DNS record.
	DNSConfig *DNSConfig
}

// DNSConfig contains the configuration values used to create a DNS record.
type DNSConfig struct {
	// ProviderType is the type of the DNS provider.
	ProviderType string
	// Value is the value of the DNS record.
	Value string
	// SecretName is the name of the secret referenced by the DNS record resource.
	SecretName string
	// SecretNamespace is the namespace of the secret used by the DNS record.
	SecretNamespace string
}

// New creates a new instance of DeployWaiter for plutono.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values Values,
) Interface {
	return &plutono{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type plutono struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

func (p *plutono) Deploy(ctx context.Context) error {
	dashboardConfigMaps, tlsSecret, data, err := p.computeResourcesData(ctx)
	if err != nil {
		return err
	}

	// dashboards configmap is not deployed as part of MR because it can breach the secret size limit.
	for _, configMap := range dashboardConfigMaps {
		if configMap != nil {
			if _, err = controllerutils.GetAndCreateOrMergePatch(ctx, p.client, configMap, func() error {
				metav1.SetMetaDataLabel(&configMap.ObjectMeta, "component", name)
				metav1.SetMetaDataLabel(&configMap.ObjectMeta, references.LabelKeyGarbageCollectable, references.LabelValueGarbageCollectable)
				return nil
			}); err != nil {
				return err
			}
		}
	}

	if err := managedresources.CreateForSeed(ctx, p.client, p.namespace, ManagedResourceName, false, data); err != nil {
		return err
	}

	return p.cleanupOldIstioTLSSecrets(ctx, tlsSecret)
}

func (p *plutono) Destroy(ctx context.Context) error {
	if err := managedresources.DeleteForSeed(ctx, p.client, p.namespace, ManagedResourceName); err != nil {
		return err
	}

	return p.cleanupOldIstioTLSSecrets(ctx, nil)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (p *plutono) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, p.client, p.namespace, ManagedResourceName)
}

func (p *plutono) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, p.client, p.namespace, ManagedResourceName)
}

func (p *plutono) SetWildcardCert(secret *corev1.Secret) {
	p.values.WildcardCert = secret
}

func (p *plutono) SetDNSConfig(dnsConfig *DNSConfig) {
	p.values.DNSConfig = dnsConfig
}

func (p *plutono) computeResourcesData(ctx context.Context) ([]*corev1.ConfigMap, *corev1.Secret, map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
		err      error

		providerConfigMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "plutono-dashboard-providers",
				Namespace: p.namespace,
				Labels:    getLabels(),
			},
			Data: map[string]string{
				"default.yaml": p.getDashboardsProviders(),
			},
		}

		dataSourceConfigMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "plutono-datasources",
				Namespace: p.namespace,
				Labels:    getLabels(),
			},
			Data: map[string]string{
				"datasources.yaml": p.getDataSource(),
			},
		}

		dashboardConfigMap, dashboardConfigMapGlobal *corev1.ConfigMap
	)

	if p.values.IsGardenCluster {
		if configMap, err := p.getDashboardsConfigMap(ctx, "garden"); err != nil {
			return nil, nil, nil, err
		} else {
			dashboardConfigMap = configMap
		}

		if configMap, err := p.getDashboardsConfigMap(ctx, "global"); err != nil {
			return nil, nil, nil, err
		} else {
			dashboardConfigMapGlobal = configMap
		}

		utilruntime.Must(kubernetesutils.MakeUnique(dashboardConfigMapGlobal))
	} else {
		if configMap, err := p.getDashboardsConfigMap(ctx, ""); err != nil {
			return nil, nil, nil, err
		} else {
			dashboardConfigMap = configMap
		}
	}

	utilruntime.Must(kubernetesutils.MakeUnique(providerConfigMap))
	utilruntime.Must(kubernetesutils.MakeUnique(dashboardConfigMap))
	utilruntime.Must(kubernetesutils.MakeUnique(dataSourceConfigMap))

	var (
		deployment      *appsv1.Deployment
		service         *corev1.Service
		gateway         *istionetworkingv1beta1.Gateway
		virtualService  *istionetworkingv1beta1.VirtualService
		destinationRule *istionetworkingv1beta1.DestinationRule
		tlsSecret       *corev1.Secret
		dnsRecord       *extensionsv1alpha1.DNSRecord
	)

	deployment = p.getDeployment(providerConfigMap, dataSourceConfigMap, dashboardConfigMap, dashboardConfigMapGlobal)
	service = p.getService()

	gateway, virtualService, destinationRule, tlsSecret, err = p.getIngressResources(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	// TODO(scheererj): Remove in next release after all shoot clusters have been moved
	// Migration is performed in multiple steps
	// 0. DNS record handled via wildcard record for nginx-ingress-controller (before)
	// 1. Overwrite DNS record with more specific record to point to istio after first reconciliation (all shoots)
	// 2. Add wildcard DNS entry for istio
	// 3. Remove specific DNS records for all shoots
	dnsRecord = p.getDNSRecord()

	data, err := registry.AddAllAndSerialize(
		providerConfigMap,
		dataSourceConfigMap,
		deployment,
		service,
		gateway,
		virtualService,
		destinationRule,
		dnsRecord,
	)
	if err != nil {
		return nil, nil, nil, err
	}

	return []*corev1.ConfigMap{dashboardConfigMap, dashboardConfigMapGlobal}, tlsSecret, data, nil
}

func (p *plutono) getDashboardsProviders() string {
	dashboardsProviders := `apiVersion: 1
providers:
- name: 'default'
  orgId: 1
  folder: ''
  type: file
  disableDeletion: false
  editable: false
  options:
    path: ` + plutonoMountPathDashboards + `
`

	if p.values.IsGardenCluster {
		dashboardsProviders = `apiVersion: 1
providers:
- name: 'global'
  orgId: 1
  folder: 'Global'
  type: file
  disableDeletion: false
  editable: false
  updateIntervalSeconds: 120
  options:
    path: ` + plutonoMountPathDashboards + `/global
- name: 'garden'
  orgId: 1
  folder: 'Garden'
  type: file
  disableDeletion: false
  editable: false
  updateIntervalSeconds: 120
  options:
    path: ` + plutonoMountPathDashboards + `/garden
`
	}

	return dashboardsProviders
}

func (p *plutono) getDataSource() string {
	url := "http://prometheus-web:80"
	maxLine := "1000"
	if p.values.IsGardenCluster {
		url = "http://" + p.namespace + "-prometheus:80"
		maxLine = "5000"
	} else if p.values.ClusterType == component.ClusterTypeSeed {
		url = "http://aggregate-prometheus-web:80"
		maxLine = "5000"
	}

	datasource := `apiVersion: 1

# list of datasources that should be deleted from the database
deleteDatasources:
- name: Graphite
  orgId: 1

# list of datasources to insert/update depending
# whats available in the database
datasources:
`

	datasource += `- name: prometheus
  type: prometheus
  access: proxy
  url: ` + url + `
  basicAuth: false
  isDefault: true
  version: 1
  editable: false
  jsonData:
    timeInterval: 1m
`

	if p.values.IsGardenCluster {
		datasource += `- name: availability-prometheus
  type: prometheus
  access: proxy
  url: http://` + p.namespace + `-avail-prom:80
  basicAuth: false
  isDefault: false
  jsonData:
    timeInterval: 30s
  version: 1
  editable: false
`
	} else if p.values.ClusterType == component.ClusterTypeSeed {
		datasource += `- name: seed-prometheus
  type: prometheus
  access: proxy
  url: http://seed-prometheus-web:80
  basicAuth: false
  version: 1
  editable: false
  jsonData:
    timeInterval: 1m
`
	}

	datasource += `- name: vali
  type: vali
  access: proxy
  url: http://logging.` + p.namespace + `.svc:` + fmt.Sprint(vali.ValiPort) + `
  jsonData:
    maxLines: ` + maxLine + `
`

	return datasource
}

func (p *plutono) getDashboardsConfigMap(ctx context.Context, suffix string) (*corev1.ConfigMap, error) {
	var (
		configMap            *corev1.ConfigMap
		requiredDashboards   map[string]embed.FS
		ignorePaths          = sets.Set[string]{}
		dashboards           = map[string]string{}
		extensionsDashboards = utils.MustNewRequirement(v1beta1constants.LabelExtensionConfiguration, selection.Equals, v1beta1constants.LabelMonitoring)
		labelSelector        = client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(extensionsDashboards)}
	)

	configMap = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "plutono-dashboards",
			Namespace: p.namespace,
			Labels:    getLabels(),
		},
	}

	if suffix != "" {
		configMap.Name = configMap.Name + "-" + suffix
	}

	if p.values.IsGardenCluster {
		if suffix == "garden" {
			requiredDashboards = map[string]embed.FS{gardenDashboardsPath: gardenDashboards, gardenAndShootDashboardsPath: gardenAndShootDashboards}

			additionalDashboards, err := p.getAdditionalDashboards(ctx, labelSelector, []string{v1beta1constants.PlutonoConfigMapOperatorDashboard})
			if err != nil {
				return nil, err
			}
			if additionalDashboards != nil {
				dashboards = additionalDashboards
			}
		}

		if suffix == "global" {
			requiredDashboards = map[string]embed.FS{gardenGlobalDashboardsPath: gardenGlobalDashboards}
		}
	} else if p.values.ClusterType == component.ClusterTypeSeed {
		requiredDashboards = map[string]embed.FS{seedDashboardsPath: seedDashboards, commonDashboardsPath: commonDashboards}
		if !p.values.IncludeIstioDashboards {
			ignorePaths.Insert("istio")
		}
	} else if p.values.ClusterType == component.ClusterTypeShoot {
		requiredDashboards = map[string]embed.FS{
			shootDashboardsPath:          shootDashboards,
			gardenAndShootDashboardsPath: gardenAndShootDashboards,
			commonDashboardsPath:         commonDashboards,
		}

		if !p.values.VPAEnabled {
			ignorePaths.Insert("vpa")
		}
		if p.values.IsWorkerless {
			ignorePaths.Insert("worker")
		} else {
			ignorePaths.Insert("workerless")
			if !p.values.NodeLocalDNSEnabled {
				ignorePaths.Insert("dns")
			}
			if !p.values.IncludeIstioDashboards {
				ignorePaths.Insert("istio")
			}
			if !p.values.VPNHighAvailabilityEnabled {
				ignorePaths.Insert("ha-vpn")
			}
		}

		additionalDashboards, err := p.getAdditionalDashboards(ctx, labelSelector, []string{v1beta1constants.PlutonoConfigMapOperatorDashboard, v1beta1constants.PlutonoConfigMapUserDashboard})
		if err != nil {
			return nil, err
		}
		if additionalDashboards != nil {
			dashboards = additionalDashboards
		}
	}

	for dashboardPath, dashboardEmbed := range requiredDashboards {
		if err := fs.WalkDir(dashboardEmbed, dashboardPath, func(path string, dirEntry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			normalizedPath := strings.TrimPrefix(strings.TrimPrefix(path, dashboardPath), "/")
			if normalizedPath == "" {
				// No need to process top level.
				return nil
			}

			// Normalize to / since it will also work on Windows
			normalizedPath = filepath.ToSlash(normalizedPath)

			if dirEntry.IsDir() {
				if len(sets.New[string](strings.Split(path, "/")...).Intersection(ignorePaths)) > 0 {
					return filepath.SkipDir
				}
				return nil
			}

			data, err := dashboardEmbed.ReadFile(path)
			if err != nil {
				return fmt.Errorf("error reading %s: %s", normalizedPath, err)
			}
			dashboards[normalizedPath[strings.LastIndex(normalizedPath, "/")+1:]] = string(data)

			return nil
		}); err != nil {
			return nil, err
		}
	}

	// this is necessary to prevent hitting configmap size limit.
	if dashboards, err := convertToCompactJSON(dashboards); err != nil {
		return nil, err
	} else {
		configMap.Data = dashboards
	}

	return configMap, nil
}

func (p *plutono) getService() *corev1.Service {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   p.namespace,
			Labels:      getLabels(),
			Annotations: map[string]string{"networking.istio.io/exportTo": "*"},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "web",
					Port:       int32(port),
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt32(port),
				},
			},
			Selector: getLabels(),
		},
	}

	if p.values.ClusterType == component.ClusterTypeSeed {
		service.Labels = utils.MergeStringMaps(service.Labels, map[string]string{v1beta1constants.LabelRole: v1beta1constants.LabelMonitoring})
	}

	if strings.HasPrefix(p.namespace, v1beta1constants.TechnicalIDPrefix) {
		metav1.SetMetaDataAnnotation(&service.ObjectMeta, resourcesv1alpha1.NetworkingPodLabelSelectorNamespaceAlias, v1beta1constants.LabelNetworkPolicyShootNamespaceAlias)
	}
	utilruntime.Must(gardenerutils.InjectNetworkPolicyNamespaceSelectors(service, []metav1.LabelSelector{
		{MatchLabels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioIngress}},
	}...))

	return service
}

func (p *plutono) getDeployment(providerConfigMap, dataSourceConfigMap, dashboardConfigMap, dashboardConfigMapGlobal *corev1.ConfigMap) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.namespace,
			Labels:    getLabels(),
		},
		Spec: appsv1.DeploymentSpec{
			RevisionHistoryLimit: pointer.Int32(2),
			Replicas:             pointer.Int32(p.values.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: getLabels(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: utils.MergeStringMaps(getLabels(), p.getPodLabels()),
				},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: pointer.Bool(false),
					PriorityClassName:            p.values.PriorityClassName,
					Containers: []corev1.Container{
						{
							Name:            name,
							Image:           p.values.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: []corev1.EnvVar{
								{Name: "PL_AUTH_ANONYMOUS_ENABLED", Value: "true"},
								{Name: "PL_USERS_VIEWERS_CAN_EDIT", Value: "true"},
								{Name: "PL_DATE_FORMATS_DEFAULT_TIMEZONE", Value: "UTC"},
								{Name: "PL_AUTH_BASIC_ENABLED", Value: "false"},
								{Name: "PL_AUTH_DISABLE_LOGIN_FORM", Value: "true"},
								{Name: "PL_AUTH_DISABLE_SIGNOUT_MENU", Value: "true"},
								{Name: "PL_ALERTING_ENABLED", Value: "false"},
								{Name: "PL_SNAPSHOTS_EXTERNAL_ENABLED", Value: "false"},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "plutono-datasources",
									MountPath: "/etc/plutono/provisioning/datasources",
								},
								{
									Name:      "plutono-dashboard-providers",
									MountPath: "/etc/plutono/provisioning/dashboards",
								},
								{
									Name:      "plutono-storage",
									MountPath: "/var/lib/plutono",
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "web",
									ContainerPort: int32(port),
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("32Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("400Mi"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "plutono-datasources",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: dataSourceConfigMap.Name,
									},
								},
							},
						},
						{
							Name: "plutono-dashboard-providers",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: providerConfigMap.Name,
									},
								},
							},
						},
						{
							Name: "plutono-storage",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									SizeLimit: utils.QuantityPtr(resource.MustParse("100Mi")),
								},
							},
						},
					},
				},
			},
		},
	}

	if p.values.IsGardenCluster {
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, []corev1.Volume{
			{
				Name: "plutono-dashboards-garden",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: dashboardConfigMap.Name,
						},
					},
				},
			},
			{
				Name: "plutono-dashboards-global",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: dashboardConfigMapGlobal.Name,
						},
					},
				},
			}}...)

		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, []corev1.VolumeMount{
			{
				Name:      "plutono-dashboards-garden",
				MountPath: plutonoMountPathDashboards + "/garden",
			},
			{
				Name:      "plutono-dashboards-global",
				MountPath: plutonoMountPathDashboards + "/global",
			},
		}...)
	} else {
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "plutono-dashboards",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: dashboardConfigMap.Name,
					},
				},
			},
		})

		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "plutono-dashboards",
			MountPath: plutonoMountPathDashboards,
		})
	}

	if p.values.ClusterType == component.ClusterTypeSeed {
		deployment.Labels = utils.MergeStringMaps(deployment.Labels, map[string]string{v1beta1constants.LabelRole: v1beta1constants.LabelMonitoring})
	} else if p.values.ClusterType == component.ClusterTypeShoot {
		deployment.Labels = utils.MergeStringMaps(deployment.Labels, map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleMonitoring})
	}
	utilruntime.Must(references.InjectAnnotations(deployment))

	return deployment
}

func (p *plutono) getIngressResources(ctx context.Context) (*istionetworkingv1beta1.Gateway, *istionetworkingv1beta1.VirtualService, *istionetworkingv1beta1.DestinationRule, *corev1.Secret, error) {
	var (
		authSecret = p.values.AuthSecret
		caName     = v1beta1constants.SecretNameCASeed
	)

	if p.values.IsGardenCluster {
		credentialsSecret, err := p.secretsManager.Generate(ctx, &secrets.BasicAuthSecretConfig{
			Name:           v1beta1constants.SecretNameObservabilityIngress,
			Format:         secrets.BasicAuthFormatNormal,
			Username:       "admin",
			PasswordLength: 32,
		}, secretsmanager.Persist(), secretsmanager.Rotate(secretsmanager.InPlace),
		)
		if err != nil {
			return nil, nil, nil, nil, err
		}

		authSecret = credentialsSecret
		caName = operatorv1alpha1.SecretNameCARuntime
	}

	if p.values.ClusterType == component.ClusterTypeShoot {
		credentialsSecret, err := p.secretsManager.Generate(ctx, &secrets.BasicAuthSecretConfig{
			Name:           v1beta1constants.SecretNameObservabilityIngressUsers,
			Format:         secrets.BasicAuthFormatNormal,
			Username:       "admin",
			PasswordLength: 32,
		}, secretsmanager.Persist(),
			secretsmanager.Rotate(secretsmanager.InPlace),
		)
		if err != nil {
			return nil, nil, nil, nil, err
		}

		authSecret = credentialsSecret
		caName = v1beta1constants.SecretNameCACluster
	}

	var tlsSecret *corev1.Secret
	if p.values.WildcardCert != nil {
		tlsSecret = p.values.WildcardCert
	} else {
		ingressTLSSecret, err := p.secretsManager.Generate(ctx, &secrets.CertificateSecretConfig{
			Name:                        "plutono-tls",
			CommonName:                  "plutono",
			Organization:                []string{"gardener.cloud:monitoring:ingress"},
			DNSNames:                    []string{p.values.IngressHost},
			CertType:                    secrets.ServerCert,
			Validity:                    pointer.Duration(ingressTLSCertificateValidity),
			SkipPublishingCACertificate: true,
		}, secretsmanager.SignedByCA(caName))
		if err != nil {
			return nil, nil, nil, nil, err
		}
		tlsSecret = ingressTLSSecret
	}

	istioTLSSecret := tlsSecret.DeepCopy()
	istioTLSSecret.Type = tlsSecret.Type
	istioTLSSecret.ObjectMeta = metav1.ObjectMeta{
		Name:      fmt.Sprintf("%s-%s", p.getOwnerId(), tlsSecret.Name),
		Namespace: p.values.IstioIngressGatewayNamespace,
		Labels:    p.getIstioTLSSecretLabels(),
	}
	if err := p.ensureIstioTLSSecret(ctx, istioTLSSecret); err != nil {
		return nil, nil, nil, nil, err
	}

	gateway := &istionetworkingv1beta1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.namespace,
		},
	}
	if err := istio.GatewayWithTLSTermination(gateway, getLabels(), p.values.IstioIngressGatewayLabels, []string{p.values.IngressHost}, externalPort, istioTLSSecret.Name)(); err != nil {
		return nil, nil, nil, nil, err
	}

	virtualService := &istionetworkingv1beta1.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.namespace,
		},
	}
	destinationHost := fmt.Sprintf("%s.%s.svc.%s", name, p.namespace, gardencorev1beta1.DefaultDomain)
	if err := istio.VirtualServiceWithSNIMatchAndBasicAuth(virtualService, getLabels(), []string{p.values.IngressHost}, name, externalPort, destinationHost, port, string(authSecret.Data[corev1.BasicAuthUsernameKey]), string(authSecret.Data[corev1.BasicAuthPasswordKey]))(); err != nil {
		return nil, nil, nil, nil, err
	}
	if p.values.ClusterType == component.ClusterTypeShoot {
		if virtualService.Spec.Http[0].Headers == nil {
			virtualService.Spec.Http[0].Headers = &istioapinetworkingv1beta1.Headers{}
		}
		if virtualService.Spec.Http[0].Headers.Request == nil {
			virtualService.Spec.Http[0].Headers.Request = &istioapinetworkingv1beta1.Headers_HeaderOperations{}
		}
		virtualService.Spec.Http[0].Headers.Request.Set = map[string]string{"X-Scope-OrgID": "operator"}
	}

	destinationRule := &istionetworkingv1beta1.DestinationRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.namespace,
		},
	}
	if err := istio.DestinationRuleWithLocalityPreference(destinationRule, getLabels(), destinationHost)(); err != nil {
		return nil, nil, nil, nil, err
	}

	return gateway, virtualService, destinationRule, istioTLSSecret, nil
}

func getLabels() map[string]string {
	return map[string]string{
		"component": name,
	}
}

func convertToCompactJSON(data map[string]string) (map[string]string, error) {
	for key, value := range data {
		// Convert file contents to compacted JSON
		// this is necessary to prevent hitting configMap size limit.
		compactJSON, err := yaml.YAMLToJSON([]byte(value))
		if err != nil {
			return nil, fmt.Errorf("error marshaling %s to JSON: %s", key, err)
		}
		data[key] = string(compactJSON)
	}

	return data, nil
}

func (p *plutono) getPodLabels() map[string]string {
	labels := map[string]string{
		v1beta1constants.LabelNetworkPolicyToDNS:                          v1beta1constants.LabelNetworkPolicyAllowed,
		gardenerutils.NetworkPolicyLabel(vali.ServiceName, vali.ValiPort): v1beta1constants.LabelNetworkPolicyAllowed,
	}

	if p.values.IsGardenCluster {
		labels = utils.MergeStringMaps(labels, map[string]string{
			gardenerutils.NetworkPolicyLabel("garden-prometheus", 9090): v1beta1constants.LabelNetworkPolicyAllowed,
			gardenerutils.NetworkPolicyLabel("garden-avail-prom", 9091): v1beta1constants.LabelNetworkPolicyAllowed,
		})

		return labels
	}

	if p.values.ClusterType == component.ClusterTypeSeed {
		labels = utils.MergeStringMaps(labels, map[string]string{
			v1beta1constants.LabelRole:                                         v1beta1constants.LabelMonitoring,
			"networking.gardener.cloud/to-seed-prometheus":                     v1beta1constants.LabelNetworkPolicyAllowed,
			gardenerutils.NetworkPolicyLabel("aggregate-prometheus-web", 9090): v1beta1constants.LabelNetworkPolicyAllowed,
			gardenerutils.NetworkPolicyLabel("seed-prometheus-web", 9090):      v1beta1constants.LabelNetworkPolicyAllowed,
		})
	} else if p.values.ClusterType == component.ClusterTypeShoot {
		labels = utils.MergeStringMaps(labels, map[string]string{
			v1beta1constants.GardenRole:                              v1beta1constants.GardenRoleMonitoring,
			gardenerutils.NetworkPolicyLabel("prometheus-web", 9090): v1beta1constants.LabelNetworkPolicyAllowed,
		})
	}

	return labels
}

func (p *plutono) getAdditionalDashboards(ctx context.Context, labelSelector labels.Selector, keys []string) (map[string]string, error) {
	var (
		dashboards           = map[string]string{}
		additionalDashboards = strings.Builder{}
	)

	// Fetch additional monitoring configuration
	existingConfigMaps := &corev1.ConfigMapList{}
	if err := p.client.List(ctx, existingConfigMaps,
		client.InNamespace(p.namespace),
		client.MatchingLabelsSelector{Selector: labelSelector}); err != nil {
		return nil, err
	}

	// Need stable order before passing the dashboards to Plutono config to avoid unnecessary changes
	kubernetesutils.ByName().Sort(existingConfigMaps)

	// Read monitoring configurations
	for _, cm := range existingConfigMaps.Items {
		for _, key := range keys {
			if dashboard, ok := cm.Data[key]; ok && dashboard != "" {
				additionalDashboards.WriteString(fmt.Sprintln(strings.ReplaceAll(strings.ReplaceAll(dashboard, "Grafana", "Plutono"), "loki", "vali")))
			}
		}
	}

	if additionalDashboards.Len() > 0 {
		if err := yaml.Unmarshal([]byte(additionalDashboards.String()), &dashboards); err != nil {
			return nil, err
		}
	}

	return dashboards, nil
}

func (p *plutono) getOwnerId() string {
	if p.values.IsGardenCluster {
		return "garden"
	}
	if p.values.ClusterType == component.ClusterTypeSeed {
		return "seed"
	}
	if p.values.ClusterType == component.ClusterTypeShoot {
		return p.namespace
	}
	return ""
}

func (p *plutono) getIstioTLSSecretLabels() map[string]string {
	return utils.MergeStringMaps(getLabels(), map[string]string{labelTLSSecretOwner: p.getOwnerId()})
}

func (p *plutono) ensureIstioTLSSecret(ctx context.Context, tlsSecret *corev1.Secret) error {
	secret := &corev1.Secret{}
	if err := p.client.Get(ctx, client.ObjectKeyFromObject(tlsSecret), secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		if err := p.client.Create(ctx, tlsSecret); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return err
			}

			if err := p.client.Get(ctx, client.ObjectKeyFromObject(tlsSecret), secret); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *plutono) cleanupOldIstioTLSSecrets(ctx context.Context, tlsSecret *corev1.Secret) error {
	secretList := &corev1.SecretList{}
	if err := p.client.List(ctx, secretList, client.InNamespace(p.values.IstioIngressGatewayNamespace), client.MatchingLabels(p.getIstioTLSSecretLabels())); err != nil {
		return err
	}

	var fns []flow.TaskFn

	for _, s := range secretList.Items {
		secret := s

		if tlsSecret != nil && tlsSecret.Name == secret.Name {
			continue
		}

		fns = append(fns, func(ctx context.Context) error { return client.IgnoreNotFound(p.client.Delete(ctx, &secret)) })
	}

	return flow.Parallel(fns...)(ctx)
}

func (p *plutono) getDNSRecord() *extensionsv1alpha1.DNSRecord {
	if p.values.DNSConfig == nil {
		// DNS record for garden cluster is created externally.
		return nil
	}
	return &extensionsv1alpha1.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.namespace,
			// Allow deletion via managed resource by directly setting the confirmation annotation
			Annotations: map[string]string{gardenerutils.ConfirmationDeletion: "true"},
		},
		Spec: extensionsv1alpha1.DNSRecordSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: p.values.DNSConfig.ProviderType,
			},
			SecretRef: corev1.SecretReference{
				Name:      p.values.DNSConfig.SecretName,
				Namespace: p.values.DNSConfig.SecretNamespace,
			},
			Name:       p.values.IngressHost,
			RecordType: extensionsv1alpha1helper.GetDNSRecordType(p.values.DNSConfig.Value),
			Values:     []string{p.values.DNSConfig.Value},
		},
	}
}
