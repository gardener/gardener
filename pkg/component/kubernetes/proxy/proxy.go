// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/Masterminds/semver/v3"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	labelKeyManagedResourceName   = "component"
	labelValueManagedResourceName = "kube-proxy"
	labelValueRole                = "pool"
	labelKeyPoolName              = "pool-name"
	labelKeyKubernetesVersion     = "kubernetes-version"
)

var (
	labelSelectorManagedResourcesAll = client.MatchingLabels{
		labelKeyManagedResourceName: labelValueManagedResourceName,
	}
	labelSelectorManagedResourcesPoolSpecific = client.MatchingLabels{
		labelKeyManagedResourceName: labelValueManagedResourceName,
		v1beta1constants.LabelRole:  labelValueRole,
	}
)

// New creates a new instance of DeployWaiter for kube-proxy.
func New(
	client client.Client,
	namespace string,
	values Values,
) Interface {
	return &kubeProxy{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

// Interface is an interface for managing kube-proxy DaemonSets.
type Interface interface {
	component.DeployWaiter
	// DeleteStaleResources deletes no longer required ManagedResource from the shoot namespace in the seed.
	DeleteStaleResources(context.Context) error
	// WaitCleanupStaleResources waits until all no longer required ManagedResource are cleaned up.
	WaitCleanupStaleResources(context.Context) error
	// SetKubeconfig sets the Kubeconfig field in the Values.
	SetKubeconfig([]byte)
	// SetWorkerPools sets the WorkerPools field in the Values.
	SetWorkerPools([]WorkerPool)
	// SetPodNetworkCIDRs sets the pod CIDRs of the shoot network.
	SetPodNetworkCIDRs([]net.IPNet)
}

type kubeProxy struct {
	client    client.Client
	namespace string
	values    Values

	serviceAccount              *corev1.ServiceAccount
	secret                      *corev1.Secret
	configMap                   *corev1.ConfigMap
	configMapCleanupScript      *corev1.ConfigMap
	configMapConntrackFixScript *corev1.ConfigMap
}

// Values is a set of configuration values for the kube-proxy component.
type Values struct {
	// IPVSEnabled states whether IPVS is enabled.
	IPVSEnabled bool
	// FeatureGates is the set of feature gates.
	FeatureGates map[string]bool
	// ImageAlpine is the alpine container image.
	ImageAlpine string
	// Kubeconfig is the kubeconfig which should be used to communicate with the kube-apiserver.
	Kubeconfig []byte
	// PodNetworkCIDRs are the CIDRs of the pod network. Only relevant when IPVSEnabled is false.
	PodNetworkCIDRs []net.IPNet
	// VPAEnabled states whether VerticalPodAutoscaler is enabled.
	VPAEnabled bool
	// WorkerPools is a list of worker pools for which the kube-proxy DaemonSets should be deployed.
	WorkerPools []WorkerPool
}

// WorkerPool contains configuration for the kube-proxy deployment for this specific worker pool.
type WorkerPool struct {
	// Name is the name of the worker pool.
	Name string
	// KubernetesVersion is the Kubernetes version of the worker pool.
	KubernetesVersion *semver.Version
	// Image is the container image used for kube-proxy for this worker pool.
	Image string
}

func (k *kubeProxy) Deploy(ctx context.Context) error {
	scrapeConfig := k.emptyScrapeConfig()
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client, scrapeConfig, func() error {
		metav1.SetMetaDataLabel(&scrapeConfig.ObjectMeta, "prometheus", shoot.Label)
		scrapeConfig.Spec = shoot.ClusterComponentScrapeConfigSpec(
			"kube-proxy",
			shoot.KubernetesServiceDiscoveryConfig{
				Role:             monitoringv1alpha1.KubernetesRoleEndpoint,
				ServiceName:      serviceName,
				EndpointPortName: portNameMetrics,
			},
			"kubeproxy_network_programming_duration_seconds_bucket",
			"kubeproxy_network_programming_duration_seconds_count",
			"kubeproxy_network_programming_duration_seconds_sum",
			"kubeproxy_sync_proxy_rules_duration_seconds_bucket",
			"kubeproxy_sync_proxy_rules_duration_seconds_count",
			"kubeproxy_sync_proxy_rules_duration_seconds_sum",
		)
		return nil
	}); err != nil {
		return err
	}

	prometheusRule := k.emptyPrometheusRule()
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client, prometheusRule, func() error {
		metav1.SetMetaDataLabel(&prometheusRule.ObjectMeta, "prometheus", shoot.Label)
		prometheusRule.Spec = monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{{
				Name: "kube-proxy.rules",
				Rules: []monitoringv1.Rule{
					{
						Record: "kubeproxy_network_latency:quantile",
						Expr:   intstr.FromString(`histogram_quantile(0.99, sum(rate(kubeproxy_network_programming_duration_seconds_bucket[10m])) by (le))`),
						Labels: map[string]string{"quantile": "0.99"},
					},
					{
						Record: "kubeproxy_network_latency:quantile",
						Expr:   intstr.FromString(`histogram_quantile(0.9, sum(rate(kubeproxy_network_programming_duration_seconds_bucket[10m])) by (le))`),
						Labels: map[string]string{"quantile": "0.9"},
					},
					{
						Record: "kubeproxy_network_latency:quantile",
						Expr:   intstr.FromString(`histogram_quantile(0.5, sum(rate(kubeproxy_network_programming_duration_seconds_bucket[10m])) by (le))`),
						Labels: map[string]string{"quantile": "0.5"},
					},
					{
						Record: "kubeproxy_sync_proxy:quantile",
						Expr:   intstr.FromString(`histogram_quantile(0.99, sum(rate(kubeproxy_sync_proxy_rules_duration_seconds_bucket[10m])) by (le))`),
						Labels: map[string]string{"quantile": "0.99"},
					},
					{
						Record: "kubeproxy_sync_proxy:quantile",
						Expr:   intstr.FromString(`histogram_quantile(0.9, sum(rate(kubeproxy_sync_proxy_rules_duration_seconds_bucket[10m])) by (le))`),
						Labels: map[string]string{"quantile": "0.9"},
					},
					{
						Record: "kubeproxy_sync_proxy:quantile",
						Expr:   intstr.FromString(`histogram_quantile(0.5, sum(rate(kubeproxy_sync_proxy_rules_duration_seconds_bucket[10m])) by (le))`),
						Labels: map[string]string{"quantile": "0.5"},
					},
				},
			}},
		}
		return nil
	}); err != nil {
		return err
	}

	data, err := k.computeCentralResourcesData()
	if err != nil {
		return err
	}

	if err := k.reconcileManagedResource(ctx, data, nil, nil); err != nil {
		return err
	}

	return k.forEachWorkerPool(ctx, false, func(ctx context.Context, pool WorkerPool) error {
		data, err := k.computePoolResourcesData(pool)
		if err != nil {
			return err
		}
		dataForMajorMinorVersionOnly, err := k.computePoolResourcesDataForMajorMinorVersionOnly(pool)
		if err != nil {
			return err
		}

		if err := k.reconcileManagedResource(ctx, data, &pool, ptr.To(false)); err != nil {
			return err
		}
		return k.reconcileManagedResource(ctx, dataForMajorMinorVersionOnly, &pool, ptr.To(true))
	})
}

func (k *kubeProxy) reconcileManagedResource(ctx context.Context, data map[string][]byte, pool *WorkerPool, useMajorMinorVersionOnly *bool) error {
	var (
		mrName             = managedResourceName(pool, useMajorMinorVersionOnly)
		secretName, secret = managedresources.NewSecret(k.client, k.namespace, mrName, data, true)
		managedResource    = managedresources.NewForShoot(k.client, k.namespace, mrName, managedresources.LabelValueGardener, false).WithSecretRef(secretName)
	)

	secret = secret.AddLabels(getManagedResourceLabels(pool, useMajorMinorVersionOnly))
	managedResource = managedResource.WithLabels(getManagedResourceLabels(pool, useMajorMinorVersionOnly))

	if err := secret.Reconcile(ctx); err != nil {
		return err
	}

	return managedResource.Reconcile(ctx)
}

func (k *kubeProxy) Destroy(ctx context.Context) error {
	if err := kubernetesutils.DeleteObjects(ctx, k.client,
		k.emptyScrapeConfig(),
		k.emptyPrometheusRule(),
	); err != nil {
		return err
	}

	return k.forEachExistingManagedResource(ctx, false, labelSelectorManagedResourcesAll, func(ctx context.Context, managedResource resourcesv1alpha1.ManagedResource) error {
		return managedresources.DeleteForShoot(ctx, k.client, k.namespace, managedResource.Name)
	})
}

func (k *kubeProxy) DeleteStaleResources(ctx context.Context) error {
	return k.forEachExistingManagedResource(ctx, false, labelSelectorManagedResourcesPoolSpecific, func(ctx context.Context, managedResource resourcesv1alpha1.ManagedResource) error {
		if k.isExistingManagedResourceStillDesired(managedResource.Labels) {
			return nil
		}
		return managedresources.DeleteForShoot(ctx, k.client, k.namespace, managedResource.Name)
	})
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (k *kubeProxy) Wait(ctx context.Context) error {
	if err := managedresources.WaitUntilHealthy(ctx, k.client, k.namespace, managedResourceName(nil, nil)); err != nil {
		return err
	}

	return k.forEachWorkerPool(ctx, true, func(ctx context.Context, pool WorkerPool) error {
		if err := managedresources.WaitUntilHealthy(ctx, k.client, k.namespace, managedResourceName(&pool, ptr.To(false))); err != nil {
			return err
		}
		return managedresources.WaitUntilHealthy(ctx, k.client, k.namespace, managedResourceName(&pool, ptr.To(true)))
	})
}

func (k *kubeProxy) WaitCleanup(ctx context.Context) error {
	return k.forEachExistingManagedResource(ctx, true, labelSelectorManagedResourcesAll, func(ctx context.Context, managedResource resourcesv1alpha1.ManagedResource) error {
		return managedresources.WaitUntilDeleted(ctx, k.client, k.namespace, managedResource.Name)
	})
}

func (k *kubeProxy) WaitCleanupStaleResources(ctx context.Context) error {
	return k.forEachExistingManagedResource(ctx, true, labelSelectorManagedResourcesPoolSpecific, func(ctx context.Context, managedResource resourcesv1alpha1.ManagedResource) error {
		if k.isExistingManagedResourceStillDesired(managedResource.Labels) {
			return nil
		}
		return managedresources.WaitUntilDeleted(ctx, k.client, k.namespace, managedResource.Name)
	})
}

func (k *kubeProxy) forEachWorkerPool(
	ctx context.Context,
	withTimeout bool,
	f func(context.Context, WorkerPool) error,
) error {
	fns := make([]flow.TaskFn, 0, len(k.values.WorkerPools))

	for _, pool := range k.values.WorkerPools {
		p := pool
		fns = append(fns, func(ctx context.Context) error {
			return f(ctx, p)
		})
	}

	return runParallelFunctions(ctx, withTimeout, fns)
}

func (k *kubeProxy) forEachExistingManagedResource(
	ctx context.Context,
	withTimeout bool,
	labelSelector client.MatchingLabels,
	f func(context.Context, resourcesv1alpha1.ManagedResource) error,
) error {
	managedResourceList := &resourcesv1alpha1.ManagedResourceList{}
	if err := k.client.List(ctx, managedResourceList, client.InNamespace(k.namespace), labelSelector); err != nil {
		return err
	}

	fns := make([]flow.TaskFn, 0, len(managedResourceList.Items))

	for _, managedResource := range managedResourceList.Items {
		m := managedResource
		fns = append(fns, func(ctx context.Context) error {
			return f(ctx, m)
		})
	}

	return runParallelFunctions(ctx, withTimeout, fns)
}

func runParallelFunctions(ctx context.Context, withTimeout bool, fns []flow.TaskFn) error {
	parallelCtx := ctx

	if withTimeout {
		timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
		defer cancel()
		parallelCtx = timeoutCtx
	}

	return flow.Parallel(fns...)(parallelCtx)
}

func (k *kubeProxy) isExistingManagedResourceStillDesired(labels map[string]string) bool {
	for _, pool := range k.values.WorkerPools {
		if pool.Name == labels[labelKeyPoolName] &&
			(pool.KubernetesVersion.String() == labels[labelKeyKubernetesVersion] || version(pool, ptr.To(true)) == labels[labelKeyKubernetesVersion]) {
			return true
		}
	}

	return false
}

func (k *kubeProxy) emptyScrapeConfig() *monitoringv1alpha1.ScrapeConfig {
	return &monitoringv1alpha1.ScrapeConfig{ObjectMeta: monitoringutils.ConfigObjectMeta("kube-proxy", k.namespace, shoot.Label)}
}

func (k *kubeProxy) emptyPrometheusRule() *monitoringv1.PrometheusRule {
	return &monitoringv1.PrometheusRule{ObjectMeta: monitoringutils.ConfigObjectMeta("kube-proxy", k.namespace, shoot.Label)}
}

func getManagedResourceLabels(pool *WorkerPool, useMajorMinorVersionOnly *bool) map[string]string {
	labels := map[string]string{
		labelKeyManagedResourceName:     labelValueManagedResourceName,
		managedresources.LabelKeyOrigin: managedresources.LabelValueGardener,
	}

	if pool != nil {
		labels[v1beta1constants.LabelRole] = labelValueRole
		labels[labelKeyPoolName] = pool.Name
		labels[labelKeyKubernetesVersion] = version(*pool, useMajorMinorVersionOnly)
	}

	return labels
}

func managedResourceName(pool *WorkerPool, useMajorMinorVersionOnly *bool) string {
	if pool == nil {
		return "shoot-core-kube-proxy"
	}
	return fmt.Sprintf("shoot-core-%s", name(*pool, useMajorMinorVersionOnly))
}

func name(pool WorkerPool, useMajorMinorVersionOnly *bool) string {
	return fmt.Sprintf("kube-proxy-%s-v%s", pool.Name, version(pool, useMajorMinorVersionOnly))
}

func version(pool WorkerPool, useMajorMinorVersionOnly *bool) string {
	if ptr.Deref(useMajorMinorVersionOnly, false) {
		return fmt.Sprintf("%d.%d", pool.KubernetesVersion.Major(), pool.KubernetesVersion.Minor())
	}
	return pool.KubernetesVersion.String()
}

func (k *kubeProxy) SetKubeconfig(kubeconfig []byte)     { k.values.Kubeconfig = kubeconfig }
func (k *kubeProxy) SetWorkerPools(pools []WorkerPool)   { k.values.WorkerPools = pools }
func (k *kubeProxy) SetPodNetworkCIDRs(pods []net.IPNet) { k.values.PodNetworkCIDRs = pods }
