// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package istio

import (
	"context"
	"embed"
	"path/filepath"
	"strings"
	"time"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	podsecurityadmissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/aggregate"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// DefaultZoneKey is the label key for the istio default ingress gateway.
	DefaultZoneKey = "istio"
	// IstiodServiceName is the name of the istiod service.
	IstiodServiceName = "istiod"
	// IstiodPort is the port of the istiod service.
	IstiodPort = 15012
	// PortWebhookServer is the port of the validating webhook server.
	PortWebhookServer = 10250

	// managedResourceControlName is the name of the ManagedResource containing the resource specifications.
	managedResourceControlName = "istio"
	// managedResourceIstioSystemName is the name of the ManagedResource containing Istio-System resource specifications.
	managedResourceIstioSystemName = "istio-system"

	istiodServicePortNameMetrics = "metrics"
	releaseName                  = "istio"
)

var (
	//go:embed charts/istio/istio-istiod
	chartIstiod     embed.FS
	chartPathIstiod = filepath.Join("charts", "istio", "istio-istiod")
)

type istiod struct {
	client        client.Client
	chartRenderer chartrenderer.Interface
	values        Values

	managedResourceIstioIngressName string
}

// IstiodValues contains configuration values for the Istiod component.
type IstiodValues struct {
	// Enabled controls if `istiod` is deployed.
	Enabled bool
	// Namespace (a.k.a. Istio-System namespace) is the namespace `istiod` is deployed to.
	Namespace string
	// Image is the image used for the `istiod` deployment.
	Image string
	// PriorityClassName is the name of the priority class used for the Istiod deployment.
	PriorityClassName string
	// TrustDomain is the domain used for service discovery, e.g. `cluster.local`.
	TrustDomain string
	// Zones are the availability zones used for this `istiod` deployment.
	Zones []string
	// DualStack
	DualStack bool
}

// Values contains configuration values for the Istio component.
type Values struct {
	// Istiod are configuration values for the istiod chart.
	Istiod IstiodValues
	// IngressGateway are configuration values for ingress gateway deployments and objects.
	IngressGateway []IngressGatewayValues
	// NamePrefix can be used to prepend arbitrary identifiers to resources which are deployed to common namespaces.
	NamePrefix string
}

// Interface contains functions for an Istio deployer.
type Interface interface {
	component.DeployWaiter
	// AddIngressGateway adds another ingress gateway to the existing Istio deployer.
	AddIngressGateway(values IngressGatewayValues)
	// GetValues returns the configured values of the Istio deployer.
	GetValues() Values
}

var _ Interface = (*istiod)(nil)

// NewIstio can be used to deploy istio's istiod in a namespace.
// Destroy does nothing.
func NewIstio(
	client client.Client,
	chartRenderer chartrenderer.Interface,
	values Values,
) Interface {
	return &istiod{
		client:        client,
		chartRenderer: chartRenderer,
		values:        values,

		managedResourceIstioIngressName: values.NamePrefix + managedResourceControlName,
	}
}

func (i *istiod) deployIstiod(ctx context.Context) error {
	if !i.values.Istiod.Enabled {
		return nil
	}

	istiodNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: i.values.Istiod.Namespace}}
	if _, err := controllerutils.CreateOrGetAndMergePatch(ctx, i.client, istiodNamespace, func() error {
		metav1.SetMetaDataLabel(&istiodNamespace.ObjectMeta, "istio-operator-managed", "Reconcile")
		metav1.SetMetaDataLabel(&istiodNamespace.ObjectMeta, "istio-injection", "disabled")
		metav1.SetMetaDataLabel(&istiodNamespace.ObjectMeta, podsecurityadmissionapi.EnforceLevelLabel, string(podsecurityadmissionapi.LevelBaseline))
		metav1.SetMetaDataLabel(&istiodNamespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigConsider, "true")
		metav1.SetMetaDataLabel(&istiodNamespace.ObjectMeta, v1beta1constants.GardenRole, v1beta1constants.GardenRoleIstioSystem)
		metav1.SetMetaDataAnnotation(&istiodNamespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigZones, strings.Join(i.values.Istiod.Zones, ","))
		return nil
	}); err != nil {
		return err
	}

	renderedChart, err := i.generateIstiodChart()
	if err != nil {
		return err
	}

	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
	if err := registry.Add(&monitoringv1.ServiceMonitor{
		ObjectMeta: monitoringutils.ConfigObjectMeta("istiod", v1beta1constants.IstioSystemNamespace, aggregate.Label),
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{MatchLabels: getIstiodLabels()},
			Endpoints: []monitoringv1.Endpoint{{
				Port: istiodServicePortNameMetrics,
				MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
					"galley_validation_failed",
					"galley_validation_passed",
					"pilot_conflict_inbound_listener",
					"pilot_conflict_outbound_listener_http_over_current_tcp",
					"pilot_conflict_outbound_listener_tcp_over_current_http",
					"pilot_conflict_outbound_listener_tcp_over_current_tcp",
					"pilot_k8s_cfg_events",
					"pilot_proxy_convergence_time_bucket",
					"pilot_services",
					"pilot_total_xds_internal_errors",
					"pilot_total_xds_rejects",
					"pilot_virt_services",
					"pilot_xds",
					"pilot_xds_cds_reject",
					"pilot_xds_eds_reject",
					"pilot_xds_lds_reject",
					"pilot_xds_push_context_errors",
					"pilot_xds_pushes",
					"pilot_xds_rds_reject",
					"pilot_xds_write_timeout",
					"go_goroutines",
					"go_memstats_alloc_bytes",
					"go_memstats_heap_alloc_bytes",
					"go_memstats_heap_inuse_bytes",
					"go_memstats_heap_sys_bytes",
					"go_memstats_stack_inuse_bytes",
					"istio_build",
					"process_cpu_seconds_total",
					"process_open_fds",
					"process_resident_memory_bytes",
					"process_virtual_memory_bytes",
				),
			}},
		},
	}); err != nil {
		return err
	}

	serializedObjects, err := serializeRenderedChartAndRegistry(renderedChart, registry)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, i.client, i.values.Istiod.Namespace, managedResourceIstioSystemName, false, serializedObjects)
}

func (i *istiod) Deploy(ctx context.Context) error {
	if err := i.deployIstiod(ctx); err != nil {
		return err
	}

	for _, ingressGateway := range i.values.IngressGateway {
		for _, filterName := range []string{"tcp-stats-filter-1.11", "stats-filter-1.11", "tcp-stats-filter-1.12", "stats-filter-1.12"} {
			if err := client.IgnoreNotFound(i.client.Delete(ctx, &networkingv1alpha3.EnvoyFilter{
				ObjectMeta: metav1.ObjectMeta{Name: filterName, Namespace: ingressGateway.Namespace},
			})); err != nil {
				return err
			}
		}
	}

	for _, istioIngressGateway := range i.values.IngressGateway {
		gatewayNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: istioIngressGateway.Namespace}}
		if _, err := controllerutils.CreateOrGetAndMergePatch(ctx, i.client, gatewayNamespace, func() error {
			metav1.SetMetaDataLabel(&gatewayNamespace.ObjectMeta, "istio-operator-managed", "Reconcile")
			metav1.SetMetaDataLabel(&gatewayNamespace.ObjectMeta, "istio-injection", "disabled")
			metav1.SetMetaDataLabel(&gatewayNamespace.ObjectMeta, v1beta1constants.GardenRole, v1beta1constants.GardenRoleIstioIngress)

			if value, ok := istioIngressGateway.Labels[v1beta1constants.GardenRole]; ok && strings.HasPrefix(value, v1beta1constants.GardenRoleExposureClassHandler) {
				metav1.SetMetaDataLabel(&gatewayNamespace.ObjectMeta, v1beta1constants.GardenRole, value)
			}
			if value, ok := istioIngressGateway.Labels[v1beta1constants.LabelExposureClassHandlerName]; ok {
				metav1.SetMetaDataLabel(&gatewayNamespace.ObjectMeta, v1beta1constants.LabelExposureClassHandlerName, value)
			}

			if value, ok := istioIngressGateway.Labels[DefaultZoneKey]; ok {
				metav1.SetMetaDataLabel(&gatewayNamespace.ObjectMeta, DefaultZoneKey, value)
			}

			metav1.SetMetaDataLabel(&gatewayNamespace.ObjectMeta, podsecurityadmissionapi.EnforceLevelLabel, string(podsecurityadmissionapi.LevelBaseline))
			metav1.SetMetaDataLabel(&gatewayNamespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigConsider, "true")
			zones := i.values.Istiod.Zones
			if len(istioIngressGateway.Zones) > 0 {
				zones = istioIngressGateway.Zones
			}
			metav1.SetMetaDataAnnotation(&gatewayNamespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigZones, strings.Join(zones, ","))
			if len(zones) == 1 {
				metav1.SetMetaDataAnnotation(&gatewayNamespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigZonePinning, "true")
			}
			return nil
		}); err != nil {
			return err
		}
	}

	renderedChart, err := i.generateIstioIngressGatewayChart(ctx)
	if err != nil {
		return err
	}

	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
	// Only add a ServiceMonitor for the istio-ingressgateway if istio is deployed on a seed. Otherwise, the resource
	// ends up in two managed resources, one managed by gardener operator and one managed by gardenlet. This can result
	// in a race between the corresponding gardener resource managers during seed deletion.
	// The alternative to use the name prefix to prevent the name clash is not useful as the service monitor covers all
	// namespaces.
	if i.values.NamePrefix == "" {
		if err := registry.Add(&monitoringv1.ServiceMonitor{
			ObjectMeta: monitoringutils.ConfigObjectMeta("istio-ingressgateway", v1beta1constants.IstioSystemNamespace, aggregate.Label),
			Spec: monitoringv1.ServiceMonitorSpec{
				Selector:          metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.LabelApp: "istio-ingressgateway"}},
				NamespaceSelector: monitoringv1.NamespaceSelector{Any: true},
				Endpoints: []monitoringv1.Endpoint{{
					Path: "/stats/prometheus",
					Port: "tls-tunnel",
					RelabelConfigs: []monitoringv1.RelabelConfig{{
						SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_ip"},
						Action:       "replace",
						TargetLabel:  "__address__",
						Regex:        `(.+)`,
						Replacement:  ptr.To("${1}:15020"),
					}},
					MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
						"envoy_cluster_upstream_cx_active",
						"envoy_cluster_upstream_cx_connect_fail",
						"envoy_cluster_upstream_cx_total",
						"envoy_cluster_upstream_cx_tx_bytes_total",
						"envoy_server_hot_restart_epoch",
						"istio_request_bytes_bucket",
						"istio_request_bytes_sum",
						"istio_request_duration_milliseconds_bucket",
						"istio_request_duration_seconds_bucket",
						"istio_requests_total",
						"istio_response_bytes_bucket",
						"istio_response_bytes_sum",
						"istio_tcp_connections_closed_total",
						"istio_tcp_connections_opened_total",
						"istio_tcp_received_bytes_total",
						"istio_tcp_sent_bytes_total",
						"go_goroutines",
						"go_memstats_alloc_bytes",
						"go_memstats_heap_alloc_bytes",
						"go_memstats_heap_inuse_bytes",
						"go_memstats_heap_sys_bytes",
						"go_memstats_stack_inuse_bytes",
						"istio_build",
						"process_cpu_seconds_total",
						"process_open_fds",
						"process_resident_memory_bytes",
						"process_virtual_memory_bytes",
					),
				}},
			},
		}); err != nil {
			return err
		}
	}

	serializedObjects, err := serializeRenderedChartAndRegistry(renderedChart, registry)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, i.client, i.values.Istiod.Namespace, i.managedResourceIstioIngressName, false, serializedObjects)
}

func (i *istiod) Destroy(ctx context.Context) error {
	for _, mr := range ManagedResourceNames(i.values.Istiod.Enabled, i.values.NamePrefix) {
		if err := managedresources.DeleteForSeed(ctx, i.client, i.values.Istiod.Namespace, mr); err != nil {
			return err
		}
	}

	if i.values.Istiod.Enabled {
		if err := i.client.Delete(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: i.values.Istiod.Namespace,
			},
		}); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	for _, istioIngressGateway := range i.values.IngressGateway {
		if err := i.client.Delete(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: istioIngressGateway.Namespace,
			},
		}); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	return nil
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (i *istiod) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	managedResources := ManagedResourceNames(i.values.Istiod.Enabled, i.values.NamePrefix)
	taskFns := make([]flow.TaskFn, 0, len(managedResources))
	for _, mr := range managedResources {
		name := mr
		taskFns = append(taskFns, func(ctx context.Context) error {
			return managedresources.WaitUntilHealthy(ctx, i.client, i.values.Istiod.Namespace, name)
		})
	}

	return flow.Parallel(taskFns...)(timeoutCtx)
}

func (i *istiod) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	managedResources := ManagedResourceNames(i.values.Istiod.Enabled, i.values.NamePrefix)
	taskFns := make([]flow.TaskFn, 0, len(managedResources))
	for _, mr := range managedResources {
		name := mr
		taskFns = append(taskFns, func(_ context.Context) error {
			return managedresources.WaitUntilDeleted(timeoutCtx, i.client, i.values.Istiod.Namespace, name)
		})
	}

	return flow.Parallel(taskFns...)(timeoutCtx)
}

func (i *istiod) AddIngressGateway(values IngressGatewayValues) {
	i.values.IngressGateway = append(i.values.IngressGateway, values)
}

func (i *istiod) GetValues() Values {
	return i.values
}

func (i *istiod) generateIstiodChart() (*chartrenderer.RenderedChart, error) {
	return i.chartRenderer.RenderEmbeddedFS(chartIstiod, chartPathIstiod, releaseName, i.values.Istiod.Namespace, map[string]any{
		"serviceName":       IstiodServiceName,
		"trustDomain":       i.values.Istiod.TrustDomain,
		"labels":            getIstiodLabels(),
		"deployNamespace":   false,
		"priorityClassName": i.values.Istiod.PriorityClassName,
		"ports": map[string]any{
			"https": PortWebhookServer,
		},
		"portsNames": map[string]any{
			"metrics": istiodServicePortNameMetrics,
		},
		"image":     i.values.Istiod.Image,
		"dualStack": i.values.Istiod.DualStack,
	})
}

func getIstiodLabels() map[string]string {
	return map[string]string{
		"app":   "istiod",
		"istio": "pilot",
	}
}

// ManagedResourceNames returns the names of the `ManagedResource`s being used by Istio.
func ManagedResourceNames(istiodEnabled bool, namePrefix string) []string {
	names := []string{namePrefix + managedResourceControlName}
	if istiodEnabled {
		names = append(names, managedResourceIstioSystemName)
	}
	return names
}

func serializeRenderedChartAndRegistry(chart *chartrenderer.RenderedChart, registry *managedresources.Registry) (map[string][]byte, error) {
	for name, data := range chart.AsSecretData() {
		registry.AddSerialized(name, data)
	}

	return registry.SerializedObjects()
}
