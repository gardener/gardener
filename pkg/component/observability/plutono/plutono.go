// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package plutono

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	valiconstants "github.com/gardener/gardener/pkg/component/observability/logging/vali/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	managedResourceName = "plutono"
	name                = "plutono"

	port                          = 3000
	ingressTLSCertificateValidity = 730 * 24 * time.Hour
	dashboardLabelValue           = "true"

	volumeNameDataSources        = "datasources"
	volumeNameDashboardProviders = "dashboard-providers"
	volumeNameDashboards         = "dashboards"
	volumeNameConfig             = "config"
	volumeNameStorage            = "storage"
	volumeNameAdminUser          = "admin-user"

	dataKeyConfig             = "plutono.ini"
	volumeMountPathStorage    = "/var/lib/plutono"
	volumeMountPathDashboards = volumeMountPathStorage + "/dashboards"
	volumeMountPathConfig     = "/usr/local/etc/plutono"
	volumeMountPathAdminUser  = "/etc/dashboard-refresher/plutono-admin"
)

var (
	//go:embed dashboards/garden
	gardenDashboards embed.FS
	//go:embed dashboards/seed
	seedDashboards embed.FS
	//go:embed dashboards/shoot
	shootDashboards embed.FS
	//go:embed dashboards/garden-shoot
	gardenAndShootDashboards embed.FS
	//go:embed dashboards/common
	commonDashboards embed.FS

	gardenDashboardsPath         = filepath.Join("dashboards", "garden")
	seedDashboardsPath           = filepath.Join("dashboards", "seed")
	shootDashboardsPath          = filepath.Join("dashboards", "shoot")
	gardenAndShootDashboardsPath = filepath.Join("dashboards", "garden-shoot")
	commonDashboardsPath         = filepath.Join("dashboards", "common")
)

// Interface contains functions for a Plutono Deployer
type Interface interface {
	component.DeployWaiter
	// SetWildcardCertName sets the WildcardCertSecretName components.
	SetWildcardCertName(*string)
}

// Values is a set of configuration values for the plutono component.
type Values struct {
	// AuthSecretName is the secret name of plutono credentials.
	AuthSecretName string
	// ClusterType specifies the type of the cluster to which plutono is being deployed.
	ClusterType component.ClusterType
	// Image is the container image used for plutono.
	Image string
	// ImageDashboardRefresher is the container image used for the sidecar responsible for refreshing the dashboards.
	ImageDashboardRefresher string
	// IngressHost is the host name of plutono.
	IngressHost string
	// IncludeIstioDashboards specifies whether to include istio dashboard.
	IncludeIstioDashboards bool
	// IsWorkerless specifies whether the cluster managed by this API server has worker nodes.
	IsWorkerless bool
	// IsGardenCluster specifies whether the cluster is garden cluster.
	IsGardenCluster bool
	// PriorityClassName is the name of the priority class.
	PriorityClassName string
	// Replicas is the number of pod replicas for the plutono.
	Replicas int32
	// VPAEnabled states whether VerticalPodAutoscaler is enabled.
	VPAEnabled bool
	// VPNHighAvailabilityEnabled specifies whether the cluster is configured with HA VPN.
	VPNHighAvailabilityEnabled bool
	// WildcardCertName is name of wildcard TLS certificate which is issued for the seed's ingress domain.
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
	plutonoAdminUserSecret, err := p.secretsManager.Generate(ctx, &secretsutils.BasicAuthSecretConfig{
		Name:           "plutono-admin",
		Format:         secretsutils.BasicAuthFormatNormal,
		Username:       "admin",
		PasswordLength: 32,
	}, secretsmanager.Rotate(secretsmanager.InPlace), secretsmanager.Validity(24*time.Hour*30))
	if err != nil {
		return err
	}

	dashboardConfigMap, data, err := p.computeResourcesData(ctx, plutonoAdminUserSecret)
	if err != nil {
		return err
	}

	// dashboards configmap is not deployed as part of MR because it can breach the secret size limit.
	if dashboardConfigMap != nil {
		configMap := p.emptyDashboardConfigMap()
		if _, err = controllerutils.GetAndCreateOrMergePatch(ctx, p.client, configMap, func() error {
			for k, v := range dashboardConfigMap.Annotations {
				metav1.SetMetaDataAnnotation(&configMap.ObjectMeta, k, v)
			}

			for k, v := range dashboardConfigMap.Labels {
				metav1.SetMetaDataLabel(&configMap.ObjectMeta, k, v)
			}

			configMap.Data = dashboardConfigMap.Data
			configMap.BinaryData = dashboardConfigMap.BinaryData
			return nil
		}); err != nil {
			return err
		}
	}

	return managedresources.CreateForSeedWithLabels(ctx, p.client, p.namespace, managedResourceName, false, map[string]string{v1beta1constants.LabelCareConditionType: v1beta1constants.ObservabilityComponentsHealthy}, data)
}

func (p *plutono) Destroy(ctx context.Context) error {
	if err := kubernetesutils.DeleteObject(ctx, p.client, p.emptyDashboardConfigMap()); err != nil {
		return fmt.Errorf("failed deleting dashboard ConfigMap: %w", err)
	}
	return managedresources.DeleteForSeed(ctx, p.client, p.namespace, managedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (p *plutono) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, p.client, p.namespace, managedResourceName)
}

func (p *plutono) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, p.client, p.namespace, managedResourceName)
}

func (p *plutono) SetWildcardCertName(secretName *string) {
	p.values.WildcardCertName = secretName
}

func (p *plutono) computeResourcesData(ctx context.Context, plutonoAdminUserSecret *corev1.Secret) (*corev1.ConfigMap, map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

		providerConfigMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "plutono-dashboard-providers",
				Namespace: p.namespace,
				Labels:    getLabels(),
			},
			Data: map[string]string{"default.yaml": p.getDashboardsProviders()},
		}

		plutonoConfigSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "plutono-config",
				Namespace: p.namespace,
				Labels:    getLabels(),
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{dataKeyConfig: []byte(p.getConfig(plutonoAdminUserSecret.Data))},
		}

		dataSourceConfigMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "plutono-datasources",
				Namespace: p.namespace,
				Labels:    getLabels(),
			},
			Data: map[string]string{"datasources.yaml": p.getDataSource()},
		}
	)

	dashboardConfigMap, err := p.getDashboardConfigMap()
	if err != nil {
		return nil, nil, err
	}

	utilruntime.Must(kubernetesutils.MakeUnique(plutonoConfigSecret))
	utilruntime.Must(kubernetesutils.MakeUnique(providerConfigMap))
	utilruntime.Must(kubernetesutils.MakeUnique(dataSourceConfigMap))

	ingress, err := p.getIngress(ctx)
	if err != nil {
		return nil, nil, err
	}

	data, err := registry.AddAllAndSerialize(
		plutonoConfigSecret,
		providerConfigMap,
		dataSourceConfigMap,
		p.getDeployment(providerConfigMap, dataSourceConfigMap, plutonoConfigSecret, plutonoAdminUserSecret),
		p.getService(),
		ingress,
		p.getServiceAccount(),
		p.getRole(),
		p.getRoleBinding(),
	)
	if err != nil {
		return nil, nil, err
	}

	return dashboardConfigMap, data, nil
}

func (p *plutono) getConfig(adminUserData map[string][]byte) string {
	return `[auth.basic]
enabled = true
[security]
admin_user = ` + string(adminUserData[secretsutils.DataKeyUserName]) + `
admin_password = ` + string(adminUserData[secretsutils.DataKeyPassword])
}

func (p *plutono) getDashboardsProviders() string {
	return `apiVersion: 1
providers:
- name: 'default'
  orgId: 1
  folder: ''
  type: file
  disableDeletion: false
  editable: false
  options:
    path: ` + volumeMountPathDashboards
}

func (p *plutono) getDataSource() string {
	prometheusSuffix, maxLine := "shoot", "1000"
	if p.values.IsGardenCluster {
		prometheusSuffix, maxLine = "garden", "5000"
	} else if p.values.ClusterType == component.ClusterTypeSeed {
		prometheusSuffix, maxLine = "aggregate", "5000"
	}

	url := "http://prometheus-" + prometheusSuffix + ":80"

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
		datasource += `- name: prometheus-longterm
  type: prometheus
  access: proxy
  url: http://prometheus-longterm:80
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
  url: http://prometheus-seed:80
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
  url: http://logging.` + p.namespace + `.svc:` + strconv.Itoa(valiconstants.ValiPort) + `
  jsonData:
    maxLines: ` + maxLine + `
`

	return datasource
}

func (p *plutono) emptyDashboardConfigMap() *corev1.ConfigMap {
	name := "plutono-dashboards"
	if p.values.IsGardenCluster {
		name += "-garden"
	}
	return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: p.namespace}}
}

func (p *plutono) getDashboardConfigMap() (*corev1.ConfigMap, error) {
	var (
		requiredDashboards map[string]embed.FS
		ignorePaths        = sets.Set[string]{}
		dashboards         = map[string]string{}
	)

	configMap := p.emptyDashboardConfigMap()
	configMap.Labels = utils.MergeStringMaps(getLabels(), map[string]string{p.dashboardLabel(): dashboardLabelValue})

	if p.values.IsGardenCluster {
		requiredDashboards = map[string]embed.FS{gardenDashboardsPath: gardenDashboards, gardenAndShootDashboardsPath: gardenAndShootDashboards}
	} else if p.values.ClusterType == component.ClusterTypeSeed {
		requiredDashboards = map[string]embed.FS{seedDashboardsPath: seedDashboards, commonDashboardsPath: commonDashboards}
		if !p.values.IncludeIstioDashboards {
			ignorePaths.Insert("istio")
		}
		if !p.values.VPAEnabled {
			ignorePaths.Insert("vpa")
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
			if !p.values.IncludeIstioDashboards {
				ignorePaths.Insert("istio")
			}
			if p.values.VPNHighAvailabilityEnabled {
				ignorePaths.Insert("envoy-proxy")
			} else {
				ignorePaths.Insert("ha-vpn")
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
	var err error
	configMap.Data, err = convertToCompactJSON(dashboards)
	if err != nil {
		return nil, err
	}

	return configMap, nil
}

func (p *plutono) getServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.namespace,
			Labels:    getLabels(),
		},
		AutomountServiceAccountToken: ptr.To(false),
	}
}

const rbacNameDashboardRefresher = name + "-dashboard-refresher"

func (p *plutono) getRole() *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbacNameDashboardRefresher,
			Namespace: p.namespace,
			Labels:    getLabels(),
		},
		Rules: []rbacv1.PolicyRule{{
			APIGroups: []string{""},
			Resources: []string{"configmaps"},
			Verbs:     []string{"get", "list", "watch"},
		}},
	}
}

func (p *plutono) getRoleBinding() *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbacNameDashboardRefresher,
			Namespace: p.namespace,
			Labels:    getLabels(),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     rbacNameDashboardRefresher,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      name,
			Namespace: p.namespace,
		}},
	}
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

	return service
}

func (p *plutono) getDeployment(providerConfigMap, dataSourceConfigMap *corev1.ConfigMap, plutonoConfigSecret, plutonoAdminUserSecret *corev1.Secret) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.namespace,
			Labels:    getLabels(),
		},
		Spec: appsv1.DeploymentSpec{
			RevisionHistoryLimit: ptr.To[int32](2),
			Replicas:             ptr.To(p.values.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: getLabels(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: utils.MergeStringMaps(getLabels(), p.getPodLabels()),
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: name,
					PriorityClassName:  p.values.PriorityClassName,
					Containers: []corev1.Container{
						{
							Name:            name,
							Image:           p.values.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: []corev1.EnvVar{
								{Name: "PL_AUTH_ANONYMOUS_ENABLED", Value: "true"},
								{Name: "PL_USERS_VIEWERS_CAN_EDIT", Value: "true"},
								{Name: "PL_DATE_FORMATS_DEFAULT_TIMEZONE", Value: "UTC"},
								{Name: "PL_AUTH_DISABLE_LOGIN_FORM", Value: "true"},
								{Name: "PL_AUTH_DISABLE_SIGNOUT_MENU", Value: "true"},
								{Name: "PL_ALERTING_ENABLED", Value: "false"},
								{Name: "PL_SNAPSHOTS_EXTERNAL_ENABLED", Value: "false"},
								{Name: "PL_PATHS_CONFIG", Value: volumeMountPathConfig + "/" + dataKeyConfig},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      volumeNameDataSources,
									MountPath: "/etc/plutono/provisioning/datasources",
								},
								{
									Name:      volumeNameDashboardProviders,
									MountPath: "/etc/plutono/provisioning/dashboards",
								},
								{
									Name:      volumeNameStorage,
									MountPath: volumeMountPathStorage,
								},
								{
									Name:      volumeNameConfig,
									MountPath: volumeMountPathConfig,
								},
							},
							Ports: []corev1.ContainerPort{{
								Name:          "web",
								ContainerPort: int32(port),
							}},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("5m"),
									corev1.ResourceMemory: resource.MustParse("45Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
							},
						},
						{
							Name:            "dashboard-refresher",
							Image:           p.values.ImageDashboardRefresher,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"python",
								"-u",
								"sidecar.py",
								"--req-username-file=" + volumeMountPathAdminUser + "/" + secretsutils.DataKeyUserName,
								"--req-password-file=" + volumeMountPathAdminUser + "/" + secretsutils.DataKeyPassword,
							},
							Env: []corev1.EnvVar{
								{Name: "LOG_LEVEL", Value: "INFO"},
								{Name: "RESOURCE", Value: "configmap"},
								{Name: "NAMESPACE", Value: p.namespace},
								{Name: "FOLDER", Value: volumeMountPathDashboards},
								{Name: "LABEL", Value: p.dashboardLabel()},
								{Name: "LABEL_VALUE", Value: dashboardLabelValue},
								{Name: "METHOD", Value: "WATCH"},
								{Name: "REQ_URL", Value: "http://localhost:" + strconv.Itoa(port) + "/api/admin/provisioning/dashboards/reload"},
								{Name: "REQ_METHOD", Value: "POST"},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      volumeNameStorage,
									MountPath: volumeMountPathStorage,
								},
								{
									Name:      volumeNameAdminUser,
									MountPath: volumeMountPathAdminUser,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("5m"),
									corev1.ResourceMemory: resource.MustParse("85M"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: volumeNameDataSources,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: dataSourceConfigMap.Name,
									},
								},
							},
						},
						{
							Name: volumeNameDashboardProviders,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: providerConfigMap.Name,
									},
								},
							},
						},
						{
							Name: volumeNameConfig,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: plutonoConfigSecret.Name,
								},
							},
						},
						{
							Name: volumeNameAdminUser,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: plutonoAdminUserSecret.Name,
								},
							},
						},
						{
							Name: volumeNameStorage,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									SizeLimit: ptr.To(resource.MustParse("100Mi")),
								},
							},
						},
						{
							Name: volumeNameDashboards,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	if p.values.ClusterType == component.ClusterTypeSeed {
		deployment.Labels = utils.MergeStringMaps(deployment.Labels, map[string]string{v1beta1constants.LabelRole: v1beta1constants.LabelMonitoring})
	} else if p.values.ClusterType == component.ClusterTypeShoot {
		deployment.Labels = utils.MergeStringMaps(deployment.Labels, map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleMonitoring})
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

	if p.values.IsGardenCluster {
		pathType = networkingv1.PathTypeImplementationSpecific

		credentialsSecret, found := p.secretsManager.Get(v1beta1constants.SecretNameObservabilityIngress)
		if !found {
			return nil, fmt.Errorf("secret %q not found", v1beta1constants.SecretNameObservabilityIngress)
		}

		credentialsSecretName = credentialsSecret.Name
		caName = operatorv1alpha1.SecretNameCARuntime
	}

	if p.values.ClusterType == component.ClusterTypeShoot {
		credentialsSecret, found := p.secretsManager.Get(v1beta1constants.SecretNameObservabilityIngressUsers)
		if !found {
			return nil, fmt.Errorf("secret %q not found", v1beta1constants.SecretNameObservabilityIngressUsers)
		}

		credentialsSecretName = credentialsSecret.Name
		caName = v1beta1constants.SecretNameCACluster
	}

	var ingressTLSSecretName string
	if p.values.WildcardCertName != nil {
		ingressTLSSecretName = *p.values.WildcardCertName
	} else {
		ingressTLSSecret, err := p.secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
			Name:                        "plutono-tls",
			CommonName:                  "plutono",
			Organization:                []string{"gardener.cloud:monitoring:ingress"},
			DNSNames:                    []string{p.values.IngressHost},
			CertType:                    secretsutils.ServerCert,
			Validity:                    ptr.To(ingressTLSCertificateValidity),
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
				"nginx.ingress.kubernetes.io/server-snippet": `location /api/admin/ {
  return 403;
}`,
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ptr.To(v1beta1constants.SeedNginxIngressClass),
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
	}

	return ingress, nil
}

func (p *plutono) dashboardLabel() string {
	label := v1beta1constants.LabelPrefixMonitoringDashboard

	if p.values.IsGardenCluster {
		label += "garden"
	} else if p.values.ClusterType == component.ClusterTypeSeed {
		label += "seed"
	} else if p.values.ClusterType == component.ClusterTypeShoot {
		label += "shoot"
	}

	return label
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
		v1beta1constants.LabelObservabilityApplication:                                      name,
		v1beta1constants.LabelNetworkPolicyToDNS:                                            v1beta1constants.LabelNetworkPolicyAllowed,
		v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer:                               v1beta1constants.LabelNetworkPolicyAllowed,
		gardenerutils.NetworkPolicyLabel(valiconstants.ServiceName, valiconstants.ValiPort): v1beta1constants.LabelNetworkPolicyAllowed,
	}

	if p.values.IsGardenCluster {
		labels = utils.MergeStringMaps(labels, map[string]string{
			gardenerutils.NetworkPolicyLabel("prometheus-garden", 9090):   v1beta1constants.LabelNetworkPolicyAllowed,
			gardenerutils.NetworkPolicyLabel("prometheus-longterm", 9091): v1beta1constants.LabelNetworkPolicyAllowed,
		})

		return labels
	}

	if p.values.ClusterType == component.ClusterTypeSeed {
		labels = utils.MergeStringMaps(labels, map[string]string{
			v1beta1constants.LabelRole:                                     v1beta1constants.LabelMonitoring,
			gardenerutils.NetworkPolicyLabel("prometheus-aggregate", 9090): v1beta1constants.LabelNetworkPolicyAllowed,
			gardenerutils.NetworkPolicyLabel("prometheus-seed", 9090):      v1beta1constants.LabelNetworkPolicyAllowed,
		})
	} else if p.values.ClusterType == component.ClusterTypeShoot {
		labels = utils.MergeStringMaps(labels, map[string]string{
			v1beta1constants.GardenRole:                                v1beta1constants.GardenRoleMonitoring,
			gardenerutils.NetworkPolicyLabel("prometheus-shoot", 9090): v1beta1constants.LabelNetworkPolicyAllowed,
		})
	}

	return labels
}
