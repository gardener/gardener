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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/logging/vali"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
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
	port                          = 3000
	ingressTLSCertificateValidity = 730 * 24 * time.Hour
)

var (
	//go:embed dashboards/seed
	seedDashboards embed.FS
	//go:embed dashboards/shoot
	shootDashboards embed.FS
	//go:embed dashboards/common
	commonDashboards embed.FS

	seedDashboardsPath   = filepath.Join("dashboards", "seed")
	shootDashboardsPath  = filepath.Join("dashboards", "shoot")
	commonDashboardsPath = filepath.Join("dashboards", "common")
)

// Interface contains functions for a Plutono Deployer
type Interface interface {
	component.DeployWaiter
}

// Values is a set of configuration values for the plutono component.
type Values struct {
	// AuthSecretName is the secret name of plutono credentials.
	AuthSecretName string
	// ClusterType specifies the type of the cluster to which plutono is being deployed.
	ClusterType component.ClusterType
	// GardenletManagesMCM specifies whether MCM is managed by gardenlet.
	GardenletManagesMCM bool
	// Image is the container image used for plutono.
	Image string
	// IngressHost is the host name of plutono.
	IngressHost string
	// IncludeIstioDashboards specifies whether to include istio dashboard.
	IncludeIstioDashboards bool
	// IsWorkerless specifies whether the cluster managed by this API server has worker nodes.
	IsWorkerless bool
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
	// WildcardCertName is name of wildcard tls certificate which is issued for the seed's ingress domain.
	WildcardCertName *string
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
	dashboardConfigMap, data, err := p.computeResourcesData(ctx)
	if err != nil {
		return err
	}

	// dashboards configmap is not deployed as part of MR because it can breach the secret size limit.
	_, err = controllerutils.GetAndCreateOrMergePatch(ctx, p.client, dashboardConfigMap, func() error {
		metav1.SetMetaDataLabel(&dashboardConfigMap.ObjectMeta, "component", name)
		metav1.SetMetaDataLabel(&dashboardConfigMap.ObjectMeta, references.LabelKeyGarbageCollectable, references.LabelValueGarbageCollectable)
		return nil
	})
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, p.client, p.namespace, ManagedResourceName, false, data)
}

func (p *plutono) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, p.client, p.namespace, ManagedResourceName)
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

func (p *plutono) computeResourcesData(ctx context.Context) (*corev1.ConfigMap, map[string][]byte, error) {
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
				"default.yaml": `apiVersion: 1
providers:
- name: 'default'
  orgId: 1
  folder: ''
  type: file
  disableDeletion: false
  editable: false
  options:
    path: ` + plutonoMountPathDashboards + `
`,
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

		dashboardConfigMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "plutono-dashboards",
				Namespace: p.namespace,
				Labels:    getLabels(),
			},
		}
	)

	if dashboards, err := p.getDashboards(ctx); err != nil {
		return nil, nil, err
	} else {
		dashboardConfigMap.Data = dashboards
	}

	utilruntime.Must(kubernetesutils.MakeUnique(providerConfigMap))
	utilruntime.Must(kubernetesutils.MakeUnique(dataSourceConfigMap))
	utilruntime.Must(kubernetesutils.MakeUnique(dashboardConfigMap))

	var (
		deployment *appsv1.Deployment
		service    *corev1.Service
		ingress    *networkingv1.Ingress
	)

	deployment = p.getDeployment(providerConfigMap.Name, dataSourceConfigMap.Name, dashboardConfigMap.Name)
	service = p.getService()

	ingress, err = p.getIngress(ctx)
	if err != nil {
		return nil, nil, err
	}

	data, err := registry.AddAllAndSerialize(
		providerConfigMap,
		dataSourceConfigMap,
		deployment,
		service,
		ingress,
	)
	if err != nil {
		return nil, nil, err
	}

	return dashboardConfigMap, data, nil
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

func (p *plutono) getDataSource() string {
	url := "http://prometheus-web:80"
	maxLine := "1000"
	if p.values.ClusterType == component.ClusterTypeSeed {
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
- name: prometheus
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
	if p.values.ClusterType == component.ClusterTypeSeed {
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

func (p *plutono) getDashboards(ctx context.Context) (map[string]string, error) {
	var (
		requiredDashboards   map[string]embed.FS
		ignorePaths          = sets.Set[string]{}
		dashboards           = map[string]string{}
		extensionsDashboards = strings.Builder{}
	)

	if p.values.ClusterType == component.ClusterTypeSeed {
		requiredDashboards = map[string]embed.FS{seedDashboardsPath: seedDashboards, commonDashboardsPath: commonDashboards}
		if !p.values.IncludeIstioDashboards {
			ignorePaths.Insert("istio")
		}
	} else {
		requiredDashboards = map[string]embed.FS{shootDashboardsPath: shootDashboards, commonDashboardsPath: commonDashboards}
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
			if !p.values.GardenletManagesMCM {
				ignorePaths.Insert("machine-controller-manager")
			}
			if !p.values.VPNHighAvailabilityEnabled {
				ignorePaths.Insert("ha-vpn")
			}
		}

		// Fetch extensions provider-specific monitoring configuration
		existingConfigMaps := &corev1.ConfigMapList{}
		if err := p.client.List(ctx, existingConfigMaps,
			client.InNamespace(p.namespace),
			client.MatchingLabels{v1beta1constants.LabelExtensionConfiguration: v1beta1constants.LabelMonitoring}); err != nil {
			return nil, err
		}

		// Need stable order before passing the dashboards to Plutono config to avoid unnecessary changes
		kubernetesutils.ByName().Sort(existingConfigMaps)

		// Read extension monitoring configurations
		for _, cm := range existingConfigMaps.Items {
			if operatorsDashboard, ok := cm.Data[v1beta1constants.PlutonoConfigMapOperatorDashboard]; ok && operatorsDashboard != "" {
				extensionsDashboards.WriteString(fmt.Sprintln(strings.ReplaceAll(strings.ReplaceAll(operatorsDashboard, "Grafana", "Plutono"), "loki", "vali")))
			}
			if usersDashboard, ok := cm.Data[v1beta1constants.PlutonoConfigMapUserDashboard]; ok && usersDashboard != "" {
				extensionsDashboards.WriteString(fmt.Sprintln(strings.ReplaceAll(strings.ReplaceAll(usersDashboard, "Grafana", "Plutono"), "loki", "vali")))
			}
		}

		if extensionsDashboards.Len() > 0 {
			if err := yaml.Unmarshal([]byte(extensionsDashboards.String()), &dashboards); err != nil {
				return nil, err
			}
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
	return convertToCompactJSON(dashboards)
}

func (p *plutono) getService() *corev1.Service {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.namespace,
			Labels:    getLabels(),
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:     "web",
					Port:     port,
					Protocol: corev1.ProtocolTCP,
				},
			},
			Selector: getLabels(),
		},
	}

	if p.values.ClusterType == component.ClusterTypeSeed {
		service.Labels = utils.MergeStringMaps(service.Labels, map[string]string{v1beta1constants.LabelRole: v1beta1constants.LabelMonitoring})
	}

	return service
}

func (p *plutono) getDeployment(providerConfigMapName, dataSourceConfigMapName, dashboardConfigMapName string) *appsv1.Deployment {
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
					Labels: utils.MergeStringMaps(getLabels(), map[string]string{
						v1beta1constants.LabelNetworkPolicyToDNS:                          v1beta1constants.LabelNetworkPolicyAllowed,
						gardenerutils.NetworkPolicyLabel(vali.ServiceName, vali.ValiPort): v1beta1constants.LabelNetworkPolicyAllowed,
					}),
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
								{Name: "PL_ALERTING_ENABLED", Value: "false"},
								{Name: "PL_SNAPSHOTS_EXTERNAL_ENABLED", Value: "false"},
								{Name: "PL_AUTH_ANONYMOUS_ENABLED", Value: "true"},
								{Name: "PL_USERS_VIEWERS_CAN_EDIT", Value: "true"},
								{Name: "PL_DATE_FORMATS_DEFAULT_TIMEZONE", Value: "UTC"},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "plutono-dashboards",
									MountPath: plutonoMountPathDashboards,
								},
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
							Name: "plutono-dashboards",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: dashboardConfigMapName,
									},
								},
							},
						},
						{
							Name: "plutono-datasources",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: dataSourceConfigMapName,
									},
								},
							},
						},
						{
							Name: "plutono-dashboard-providers",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: providerConfigMapName,
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

	if p.values.ClusterType == component.ClusterTypeSeed {
		deployment.Labels = utils.MergeStringMaps(deployment.Labels, map[string]string{v1beta1constants.LabelRole: v1beta1constants.LabelMonitoring})
		deployment.Spec.Template.Labels = utils.MergeStringMaps(deployment.Spec.Template.Labels, map[string]string{
			v1beta1constants.LabelRole:                                         v1beta1constants.LabelMonitoring,
			"networking.gardener.cloud/to-seed-prometheus":                     v1beta1constants.LabelNetworkPolicyAllowed,
			gardenerutils.NetworkPolicyLabel("aggregate-prometheus-web", 9090): v1beta1constants.LabelNetworkPolicyAllowed,
			gardenerutils.NetworkPolicyLabel("seed-prometheus-web", 9090):      v1beta1constants.LabelNetworkPolicyAllowed,
		})
		deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, []corev1.EnvVar{
			{Name: "PL_AUTH_BASIC_ENABLED", Value: "true"},
			{Name: "PL_AUTH_DISABLE_LOGIN_FORM", Value: "false"},
		}...)
	} else {
		deployment.Labels = utils.MergeStringMaps(deployment.Labels, map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleMonitoring})
		deployment.Spec.Template.Labels = utils.MergeStringMaps(deployment.Spec.Template.Labels, map[string]string{
			v1beta1constants.GardenRole:                              v1beta1constants.GardenRoleMonitoring,
			gardenerutils.NetworkPolicyLabel("prometheus-web", 9090): v1beta1constants.LabelNetworkPolicyAllowed,
		})
		deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, []corev1.EnvVar{
			{Name: "PL_AUTH_BASIC_ENABLED", Value: "false"},
			{Name: "PL_AUTH_DISABLE_LOGIN_FORM", Value: "true"},
			{Name: "PL_AUTH_DISABLE_SIGNOUT_MENU", Value: "true"},
		}...)
	}
	utilruntime.Must(references.InjectAnnotations(deployment))

	return deployment
}

func (p *plutono) getIngress(ctx context.Context) (*networkingv1.Ingress, error) {
	var (
		pathType              = networkingv1.PathTypePrefix
		credentialsSecretName = p.values.AuthSecretName
		caName                = v1beta1constants.SecretNameCASeed
	)

	if p.values.ClusterType == component.ClusterTypeShoot {
		credentialsSecret, err := p.secretsManager.Generate(ctx, &secrets.BasicAuthSecretConfig{
			Name:           v1beta1constants.SecretNameObservabilityIngressUsers,
			Format:         secrets.BasicAuthFormatNormal,
			Username:       "admin",
			PasswordLength: 32,
		}, secretsmanager.Persist(), secretsmanager.Rotate(secretsmanager.InPlace),
		)
		if err != nil {
			return nil, err
		}

		credentialsSecretName = credentialsSecret.Name
		caName = v1beta1constants.SecretNameCACluster
	}

	var ingressTLSSecretName string
	if p.values.WildcardCertName != nil {
		ingressTLSSecretName = *p.values.WildcardCertName
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
			return nil, err
		}
		ingressTLSSecretName = ingressTLSSecret.Name
	}

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.namespace,
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/auth-realm":  "Authentication Required",
				"nginx.ingress.kubernetes.io/auth-secret": credentialsSecretName,
				"nginx.ingress.kubernetes.io/auth-type":   "basic",
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: pointer.String(v1beta1constants.SeedNginxIngressClass),
			TLS: []networkingv1.IngressTLS{{
				SecretName: ingressTLSSecretName,
				Hosts:      []string{p.values.IngressHost},
			}},
			Rules: []networkingv1.IngressRule{{
				Host: p.values.IngressHost,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: name,
										Port: networkingv1.ServiceBackendPort{
											Number: int32(port),
										},
									},
								},
								Path:     "/",
								PathType: &pathType,
							},
						},
					},
				},
			}},
		},
	}

	if p.values.ClusterType == component.ClusterTypeShoot {
		ingress.Labels = getLabels()
		ingress.Annotations = utils.MergeStringMaps(ingress.Annotations, map[string]string{
			"nginx.ingress.kubernetes.io/configuration-snippet": "proxy_set_header X-Scope-OrgID operator;",
		})
	}

	return ingress, nil
}
