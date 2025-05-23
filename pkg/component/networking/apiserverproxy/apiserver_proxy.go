// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserverproxy

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/kubernetes/apiserverexposure"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	managedResourceName = "shoot-core-apiserver-proxy"
	configMapName       = "apiserver-proxy-config"
	name                = "apiserver-proxy"

	adminPort           = 16910
	proxySeedServerPort = 8132
	portNameMetrics     = "metrics"

	volumeNameConfig   = "proxy-config"
	volumeNameAdminUDS = "admin-uds"

	volumeMountPathConfig = "/etc/apiserver-proxy"
	dataKeyConfig         = "envoy.yaml"
)

var (
	tplNameEnvoy = "envoy.yaml.tpl"
	//go:embed templates/envoy.yaml.tpl
	tplContentEnvoy string
	tplEnvoy        *template.Template
)

func init() {
	var err error
	tplEnvoy, err = template.
		New(tplNameEnvoy).
		Funcs(sprig.TxtFuncMap()).
		Parse(tplContentEnvoy)
	utilruntime.Must(err)
}

// Values is a set of configuration values for the apiserver-proxy component.
type Values struct {
	ProxySeedServerHost string
	Image               string
	SidecarImage        string
	DNSLookupFamily     string
	IstioTLSTermination bool

	advertiseIPAddress string
}

// New creates a new instance of DeployWaiter for apiserver-proxy
func New(client client.Client, namespace string, secretsManager secretsmanager.Interface, values Values) Interface {
	return &apiserverProxy{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

// Interface contains functions for deploying apiserver-proxy.
type Interface interface {
	component.DeployWaiter
	SetAdvertiseIPAddress(string)
}

type apiserverProxy struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

func (a *apiserverProxy) Deploy(ctx context.Context) error {
	if a.values.advertiseIPAddress == "" {
		return fmt.Errorf("run SetAdvertiseIPAddress before deploying")
	}

	scrapeConfig := a.emptyScrapeConfig()
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, a.client, scrapeConfig, func() error {
		metav1.SetMetaDataLabel(&scrapeConfig.ObjectMeta, "prometheus", shoot.Label)
		scrapeConfig.Spec = shoot.ClusterComponentScrapeConfigSpec(
			name,
			shoot.KubernetesServiceDiscoveryConfig{
				Role:             monitoringv1alpha1.KubernetesRoleEndpoint,
				ServiceName:      name,
				EndpointPortName: portNameMetrics,
			},
			"envoy_cluster_bind_errors",
			"envoy_cluster_lb_healthy_panic",
			"envoy_cluster_update_attempt",
			"envoy_cluster_update_failure",
			"envoy_cluster_upstream_cx_connect_ms_bucket",
			"envoy_cluster_upstream_cx_length_ms_bucket",
			"envoy_cluster_upstream_cx_none_healthy",
			"envoy_cluster_upstream_cx_rx_bytes_total",
			"envoy_cluster_upstream_cx_tx_bytes_total",
			"envoy_listener_downstream_cx_destroy",
			"envoy_listener_downstream_cx_length_ms_bucket",
			"envoy_listener_downstream_cx_overflow",
			"envoy_listener_downstream_cx_total",
			"envoy_tcp_downstream_cx_no_route",
			"envoy_tcp_downstream_cx_rx_bytes_total",
			"envoy_tcp_downstream_cx_total",
			"envoy_tcp_downstream_cx_tx_bytes_total",
		)

		// we don't care about admin metrics
		scrapeConfig.Spec.MetricRelabelConfigs = append(scrapeConfig.Spec.MetricRelabelConfigs,
			monitoringv1.RelabelConfig{
				SourceLabels: []monitoringv1.LabelName{"envoy_cluster_name"},
				Regex:        `^uds_admin$`,
				Action:       "drop",
			},
			monitoringv1.RelabelConfig{
				SourceLabels: []monitoringv1.LabelName{"envoy_listener_address"},
				Regex:        `^^0.0.0.0_16910$`,
				Action:       "drop",
			},
		)

		return nil
	}); err != nil {
		return err
	}

	data, err := a.computeResourcesData()
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, a.client, a.namespace, managedResourceName, managedresources.LabelValueGardener, false, data)
}

func (a *apiserverProxy) Destroy(ctx context.Context) error {
	if err := kubernetesutils.DeleteObjects(ctx, a.client,
		a.emptyScrapeConfig(),
	); err != nil {
		return err
	}

	return managedresources.DeleteForShoot(ctx, a.client, a.namespace, managedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (a *apiserverProxy) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, a.client, a.namespace, managedResourceName)
}

func (a *apiserverProxy) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, a.client, a.namespace, managedResourceName)
}

func (a *apiserverProxy) SetAdvertiseIPAddress(advertiseIPAddress string) {
	a.values.advertiseIPAddress = advertiseIPAddress
}

func (a *apiserverProxy) emptyScrapeConfig() *monitoringv1alpha1.ScrapeConfig {
	return &monitoringv1alpha1.ScrapeConfig{ObjectMeta: monitoringutils.ConfigObjectMeta("apiserver-proxy", a.namespace, shoot.Label)}
}

func (a *apiserverProxy) computeResourcesData() (map[string][]byte, error) {
	var envoyYAML bytes.Buffer
	reversedVPNHeaderValue := fmt.Sprintf("outbound|443||kube-apiserver.%s.svc.cluster.local", a.namespace)
	if a.values.IstioTLSTermination {
		reversedVPNHeaderValue = apiserverexposure.GetAPIServerProxyTargetClusterName(a.namespace)
	}

	if err := tplEnvoy.Execute(&envoyYAML, map[string]any{
		"advertiseIPAddress":     a.values.advertiseIPAddress,
		"dnsLookupFamily":        a.values.DNSLookupFamily,
		"adminPort":              adminPort,
		"proxySeedServerHost":    a.values.ProxySeedServerHost,
		"proxySeedServerPort":    proxySeedServerPort,
		"reversedVPNHeaderValue": reversedVPNHeaderValue,
	}); err != nil {
		return nil, err
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Labels:    getDefaultLabels(),
			Namespace: metav1.NamespaceSystem,
		},
		Data: map[string]string{dataKeyConfig: envoyYAML.String()},
	}
	utilruntime.Must(kubernetesutils.MakeUnique(configMap))

	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: metav1.NamespaceSystem,
				Labels:    getDefaultLabels(),
			},
			AutomountServiceAccountToken: ptr.To(false),
		}
		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: metav1.NamespaceSystem,
				Labels:    getDefaultLabels(),
			},
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: "None",
				Ports: []corev1.ServicePort{
					{
						Name:       portNameMetrics,
						Port:       adminPort,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromInt32(adminPort),
					},
				},
				Selector: getSelector(),
			},
		}
		daemonSet = &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: metav1.NamespaceSystem,
				Labels: utils.MergeStringMaps(
					getDefaultLabels(),
					map[string]string{
						v1beta1constants.LabelNodeCriticalComponent: "true",
					},
				),
			},
			Spec: appsv1.DaemonSetSpec{
				UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
					Type: appsv1.RollingUpdateDaemonSetStrategyType,
				},
				RevisionHistoryLimit: ptr.To[int32](2),
				Selector: &metav1.LabelSelector{
					MatchLabels: getSelector(),
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: utils.MergeStringMaps(
							getDefaultLabels(),
							map[string]string{
								v1beta1constants.LabelNodeCriticalComponent:         "true",
								v1beta1constants.LabelNetworkPolicyToDNS:            v1beta1constants.LabelNetworkPolicyAllowed,
								v1beta1constants.LabelNetworkPolicyShootToAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
								v1beta1constants.LabelNetworkPolicyShootFromSeed:    v1beta1constants.LabelNetworkPolicyAllowed,
							},
						),
					},
					Spec: corev1.PodSpec{
						ServiceAccountName: name,
						PriorityClassName:  "system-node-critical",
						Tolerations: []corev1.Toleration{
							{Effect: corev1.TaintEffectNoSchedule, Operator: corev1.TolerationOpExists},
							{Effect: corev1.TaintEffectNoExecute, Operator: corev1.TolerationOpExists},
						},
						HostNetwork:                  true,
						AutomountServiceAccountToken: ptr.To(false),
						SecurityContext: &corev1.PodSecurityContext{
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
						InitContainers: []corev1.Container{
							{
								Name:            "setup",
								Image:           a.values.SidecarImage,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Args: []string{
									fmt.Sprintf("--ip-address=%s", a.values.advertiseIPAddress),
									"--daemon=false",
									"--interface=lo",
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("20m"),
										corev1.ResourceMemory: resource.MustParse("20Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("200Mi"),
									},
								},
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: ptr.To(false),
									Capabilities: &corev1.Capabilities{
										Add: []corev1.Capability{
											"NET_ADMIN",
										},
									},
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name:            "sidecar",
								Image:           a.values.SidecarImage,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Args: []string{
									fmt.Sprintf("--ip-address=%s", a.values.advertiseIPAddress),
									"--interface=lo",
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("5m"),
										corev1.ResourceMemory: resource.MustParse("15Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("90Mi"),
									},
								},
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: ptr.To(false),
									Capabilities: &corev1.Capabilities{
										Add: []corev1.Capability{
											"NET_ADMIN",
										},
									},
								},
							},
							{
								Name:            "proxy",
								Image:           a.values.Image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command: []string{
									"envoy",
									"--concurrency",
									"2",
									"--use-dynamic-base-id",
									"-c",
									fmt.Sprintf("%s/%s", volumeMountPathConfig, dataKeyConfig),
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("5m"),
										corev1.ResourceMemory: resource.MustParse("30Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("1Gi"),
									},
								},
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: ptr.To(false),
									Capabilities: &corev1.Capabilities{
										Add: []corev1.Capability{
											"NET_BIND_SERVICE",
										},
									},
									RunAsUser: ptr.To[int64](0),
								},
								ReadinessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Path: "/ready",
											Port: intstr.FromInt32(adminPort),
										},
									},
									InitialDelaySeconds: 1,
									PeriodSeconds:       2,
									SuccessThreshold:    1,
									TimeoutSeconds:      1,
								},
								LivenessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Path: "/ready",
											Port: intstr.FromInt32(adminPort),
										},
									},
									InitialDelaySeconds: 1,
									PeriodSeconds:       10,
									SuccessThreshold:    1,
									TimeoutSeconds:      1,
									FailureThreshold:    3,
								},
								Ports: []corev1.ContainerPort{
									{
										Name:          "metrics",
										ContainerPort: adminPort,
										HostPort:      adminPort,
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      volumeNameConfig,
										MountPath: volumeMountPathConfig,
									},
									{
										Name:      volumeNameAdminUDS,
										MountPath: "/etc/admin-uds",
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: volumeNameConfig,
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: configMap.Name,
										},
									},
								},
							},
							{
								Name: volumeNameAdminUDS,
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
					},
				},
			},
		}
	)

	utilruntime.Must(references.InjectAnnotations(daemonSet))

	return registry.AddAllAndSerialize(
		configMap,
		serviceAccount,
		service,
		daemonSet,
	)
}

func getDefaultLabels() map[string]string {
	return utils.MergeStringMaps(
		map[string]string{
			v1beta1constants.GardenRole:     v1beta1constants.GardenRoleSystemComponent,
			managedresources.LabelKeyOrigin: managedresources.LabelValueGardener,
		}, getSelector())
}

func getSelector() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
		v1beta1constants.LabelRole: "apiserver-proxy",
	}
}
