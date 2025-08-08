// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodelocaldns

import (
	"context"
	_ "embed"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	nodelocaldnsconstants "github.com/gardener/gardener/pkg/component/networking/nodelocaldns/constants"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/retry"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

const (
	managedResourceName = "shoot-core-node-local-dns"

	labelKey                 = "k8s-app"
	labelKeyPool             = "pool"
	labelValueAndCleanupName = "node-local-dns-cleanup"
	labelKeyCleanupRequired  = "node-local-dns.gardener.cloud/cleanup-required"
	// portServiceServer is the service port used for the DNS server.
	portServiceServer = 53
	// portServer is the target port used for the DNS server.
	portServer = 8053
	// prometheus configuration for node-local-dns
	prometheusPort      = 9253
	prometheusScrape    = true
	prometheusErrorPort = 9353

	containerName        = "node-cache"
	metricsPortName      = "metrics"
	errorMetricsPortName = "errormetrics"

	domain            = gardencorev1beta1.DefaultDomain
	serviceName       = "kube-dns-upstream"
	livenessProbePort = 8099
	configDataKey     = "Corefile"

	cleanupConfigMapName = "node-local-dns-cleanup-script"
	dataKeyCleanupScript = "cleanup.sh"

	daemonSetPollInterval = 5 * time.Second

	volumeMountNameCleanUp     = "cleanup-script"
	volumeMountPathCleanUp     = "/scripts"
	volumeMountNameXtablesLock = "xtables-lock"
	volumeMountPathXtablesLock = "/run/xtables.lock"
)

var (
	//go:embed resources/cleanup.sh
	cleanupScript string
)

// Interface contains functions for a NodeLocalDNS deployer.
type Interface interface {
	component.DeployWaiter
	SetClusterDNS([]string)
	SetDNSServers([]string)
	SetIPFamilies([]gardencorev1beta1.IPFamily)
	SetShootClientSet(kubernetes.Interface)
	SetSeedClientSet(kubernetes.Interface)
	SetLogger(logr.Logger)
}

// Values is a set of configuration values for the node-local-dns component.
type Values struct {
	// Image is the container image used for node-local-dns.
	Image string
	// AlpineImage is the container image used for the cleanup DaemonSet.
	AlpineImage string
	// VPAEnabled marks whether VerticalPodAutoscaler is enabled for the shoot.
	VPAEnabled bool
	// Config is the node local configuration for the shoot spec
	Config *gardencorev1beta1.NodeLocalDNS
	// ClusterDNS are the ClusterIPs of kube-system/coredns Service
	ClusterDNS []string
	// DNSServer are the ClusterIPs of kube-system/coredns Service
	DNSServers []string
	// IPFamilies specifies the IP protocol versions to use for node local dns.
	IPFamilies []gardencorev1beta1.IPFamily
	// ShootClient is the client used to interact with the shoot cluster.
	ShootClient client.Client
	// SeedClient is the client used to interact with the seed cluster.
	SeedClient client.Client
	// Log is the logger used for logging.
	Log logr.Logger
	// Workers is the group of workers for which node-local-dns is deployed.
	Workers []gardencorev1beta1.Worker
	// KubeProxyConfig is the kube-proxy configuration for the shoot.
	KubeProxyConfig *gardencorev1beta1.KubeProxyConfig
}

// New creates a new instance of DeployWaiter for node-local-dns.
func New(
	client client.Client,
	namespace string,
	values Values,
) Interface {
	return &nodeLocalDNS{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type nodeLocalDNS struct {
	client    client.Client
	namespace string
	values    Values
}

func (n *nodeLocalDNS) Deploy(ctx context.Context) error {
	registry := managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
	scrapeConfig := n.emptyScrapeConfig("")
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, n.client, scrapeConfig, func() error {
		metav1.SetMetaDataLabel(&scrapeConfig.ObjectMeta, "prometheus", shoot.Label)
		scrapeConfig.Spec = shoot.ClusterComponentScrapeConfigSpec(
			"node-local-dns",
			shoot.KubernetesServiceDiscoveryConfig{
				Role:              monitoringv1alpha1.KubernetesRolePod,
				PodNamePrefix:     "node-local",
				ContainerName:     containerName,
				ContainerPortName: metricsPortName,
			},
			"coredns_build_info",
			"coredns_cache_entries",
			"coredns_cache_hits_total",
			"coredns_cache_misses_total",
			"coredns_dns_request_duration_seconds_count",
			"coredns_dns_request_duration_seconds_bucket",
			"coredns_dns_requests_total",
			"coredns_dns_responses_total",
			"coredns_forward_requests_total",
			"coredns_forward_responses_total",
			"coredns_kubernetes_dns_programming_duration_seconds_bucket",
			"coredns_kubernetes_dns_programming_duration_seconds_count",
			"coredns_kubernetes_dns_programming_duration_seconds_sum",
			"process_max_fds",
			"process_open_fds",
		)
		return nil
	}); err != nil {
		return err
	}

	scrapeConfigErrors := n.emptyScrapeConfig("-errors")
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, n.client, scrapeConfigErrors, func() error {
		metav1.SetMetaDataLabel(&scrapeConfigErrors.ObjectMeta, "prometheus", shoot.Label)
		scrapeConfigErrors.Spec = shoot.ClusterComponentScrapeConfigSpec(
			"node-local-dns-errors",
			shoot.KubernetesServiceDiscoveryConfig{
				Role:              monitoringv1alpha1.KubernetesRolePod,
				PodNamePrefix:     "node-local",
				ContainerName:     containerName,
				ContainerPortName: errorMetricsPortName,
			},
			"coredns_nodecache_setup_errors_total",
		)
		return nil
	}); err != nil {
		return err
	}

	serviceAccount, configMap, service, err := n.computeResourcesData()
	if err != nil {
		return err
	}

	data, err := registry.AddAllAndSerialize(n.computePoolResourcesData(serviceAccount, configMap, service)...)
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, n.client, n.namespace, managedResourceName, managedresources.LabelValueGardener, false, data)
}

func (n *nodeLocalDNS) Destroy(ctx context.Context) error {
	managedResourceExists := false
	// Check if the managed resource exists
	if err := n.client.Get(ctx, types.NamespacedName{Namespace: n.namespace, Name: managedResourceName}, &resourcesv1alpha1.ManagedResource{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to check managed resource existence: %w", err)
		}
	} else {
		managedResourceExists = true
	}

	// Check if at least one Kubernetes node has the cleanup label
	nodeList := &metav1.PartialObjectMetadataList{}
	nodeList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("NodeList"))
	if err := n.values.ShootClient.List(ctx, nodeList, client.MatchingLabels{
		labelKeyCleanupRequired: "true",
	}); err != nil {
		return fmt.Errorf("failed to list nodes with cleanup label: %w", err)
	}

	// If the managed resource does not exist and no nodes have the cleanup label, return early no cleanup needed
	if !managedResourceExists && len(nodeList.Items) == 0 {
		return deleteCleanupScriptConfigMap(ctx, n.values.ShootClient)
	}

	// Mark nodes for cleanup if the managed resource exists
	if managedResourceExists {
		if err := MarkNodesForCleanup(ctx, n.values.ShootClient, n.values.Workers); err != nil {
			return fmt.Errorf("failed to mark nodes for cleanup: %w", err)
		}
	}

	// Delete resources
	if err := kubernetesutils.DeleteObjects(ctx, n.client,
		n.emptyScrapeConfig(""),
		n.emptyScrapeConfig("-errors"),
	); err != nil {
		return err
	}

	if err := managedresources.DeleteForShoot(ctx, n.client, n.namespace, managedResourceName); err != nil {
		return err
	}

	if v1beta1helper.IsKubeProxyIPVSMode(n.values.KubeProxyConfig) {
		return nil
	}

	// Wait until the managed resource is successfully deleted and go through node list again to add cleanup label
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()
	if err := managedresources.WaitUntilDeleted(timeoutCtx, n.client, n.namespace, managedResourceName); err != nil {
		return err
	}

	var taskFns []flow.TaskFn
	if managedResourceExists {
		nodeList = &metav1.PartialObjectMetadataList{}
		nodeList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("NodeList"))
		if err := n.values.ShootClient.List(ctx, nodeList, client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(utils.MustNewRequirement(labelKeyCleanupRequired, selection.DoesNotExist))}); err != nil {
			return fmt.Errorf("failed to list all nodes without %s label: %w", labelKeyCleanupRequired, err)
		}

		taskFns = append(taskFns, addCleanupLabelToNodes(n.values.ShootClient, nodeList.Items)...)
	}

	if err := flow.Parallel(taskFns...)(ctx); err != nil {
		return fmt.Errorf("failed to add cleanup label to nodes: %w", err)
	}

	return RunCleanup(ctx, n.values.ShootClient, n.values.AlpineImage, n.values.Log)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (n *nodeLocalDNS) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, n.client, n.namespace, managedResourceName)
}

func (n *nodeLocalDNS) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, n.client, n.namespace, managedResourceName)
}

func selectIPAddress(addresses []string, preferIPv6 bool) string {
	if len(addresses) == 1 {
		return addresses[0]
	}
	var ipv4, ipv6 string
	for _, addr := range addresses {
		ip := net.ParseIP(addr)
		if ip.To4() != nil {
			ipv4 = addr
		} else {
			ipv6 = addr
		}
	}
	if preferIPv6 {
		return ipv6
	}
	return ipv4
}

func (n *nodeLocalDNS) bindIP() string {
	if len(n.values.DNSServers) > 0 {
		dnsAddress := selectIPAddress(n.values.DNSServers, n.values.IPFamilies[0] != gardencorev1beta1.IPFamilyIPv4)
		return n.getIPVSAddress() + " " + dnsAddress
	}
	return n.getIPVSAddress()
}

func (n *nodeLocalDNS) containerArg() string {
	if len(n.values.DNSServers) > 0 {
		dnsAddress := selectIPAddress(n.values.DNSServers, n.values.IPFamilies[0] != gardencorev1beta1.IPFamilyIPv4)
		return n.getIPVSAddress() + "," + dnsAddress
	}
	return n.getIPVSAddress()
}

func (n *nodeLocalDNS) forceTcpToClusterDNS() string {
	if n.values.Config == nil || n.values.Config.ForceTCPToClusterDNS == nil || *n.values.Config.ForceTCPToClusterDNS {
		return "force_tcp"
	}
	return "prefer_udp"
}

func (n *nodeLocalDNS) forceTcpToUpstreamDNS() string {
	// Many infrastructures struggle to handle a large number of TCP connections for DNS queries, often resulting in rate throttling leading to "connection timeout" issues during DNS resolution.
	// To address this, UDP connections will be preferred when communicating with the upstream DNS server.
	if n.values.Config != nil && n.values.Config.ForceTCPToUpstreamDNS != nil && *n.values.Config.ForceTCPToUpstreamDNS {
		return "force_tcp"
	}
	return "prefer_udp"
}

func (n *nodeLocalDNS) upstreamDNSAddress() string {
	if n.values.Config != nil && ptr.Deref(n.values.Config.DisableForwardToUpstreamDNS, false) {
		return strings.Join(n.values.ClusterDNS, " ")
	}
	return "__PILLAR__UPSTREAM__SERVERS__"
}

func (n *nodeLocalDNS) emptyScrapeConfig(suffix string) *monitoringv1alpha1.ScrapeConfig {
	return &monitoringv1alpha1.ScrapeConfig{ObjectMeta: monitoringutils.ConfigObjectMeta("node-local-dns"+suffix, n.namespace, shoot.Label)}
}

func (n *nodeLocalDNS) SetClusterDNS(dns []string) {
	n.values.ClusterDNS = dns
}

func (n *nodeLocalDNS) SetDNSServers(servers []string) {
	n.values.DNSServers = servers
}

func (n *nodeLocalDNS) SetIPFamilies(ipfamilies []gardencorev1beta1.IPFamily) {
	n.values.IPFamilies = ipfamilies
}

func (n *nodeLocalDNS) SetShootClientSet(set kubernetes.Interface) {
	n.values.ShootClient = set.Client()
}

func (n *nodeLocalDNS) SetSeedClientSet(set kubernetes.Interface) {
	n.values.SeedClient = set.Client()
}

func (n *nodeLocalDNS) SetLogger(log logr.Logger) {
	n.values.Log = log
}

func (n *nodeLocalDNS) getIPVSAddress() (ipvsAddress string) {
	return n.getAddress(false)
}

func (n *nodeLocalDNS) getHealthAddress() (healthAddress string) {
	return n.getAddress(true)
}

func (n *nodeLocalDNS) getAddress(useIPv6Brackets bool) string {
	if n.values.IPFamilies[0] == gardencorev1beta1.IPFamilyIPv4 {
		return nodelocaldnsconstants.IPVSAddress
	}
	if useIPv6Brackets {
		return fmt.Sprintf("[%s]", nodelocaldnsconstants.IPVSIPv6Address)
	}
	return nodelocaldnsconstants.IPVSIPv6Address
}

// createCleanupConfigMap creates a ConfigMap containing the cleanup shell script for node-local-dns cleanup DaemonSet.
func createCleanupConfigMap(ctx context.Context, shootClient client.Client) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cleanupConfigMapName,
			Namespace: metav1.NamespaceSystem,
		},
	}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, shootClient, cm, func() error {
		cm.Data = map[string]string{
			dataKeyCleanupScript: cleanupScript,
		}
		cm.Immutable = ptr.To(true)
		return nil
	})
	return err
}

func createCleanupDaemonSet(ctx context.Context, shootClient client.Client, alpineImage string) error {
	cleanupDaemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      labelValueAndCleanupName,
			Namespace: metav1.NamespaceSystem,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, shootClient, cleanupDaemonSet, func() error {
		metav1.SetMetaDataLabel(&cleanupDaemonSet.ObjectMeta, v1beta1constants.LabelApp, labelValueAndCleanupName)
		cleanupDaemonSet.Spec = appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					v1beta1constants.LabelApp: labelValueAndCleanupName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1beta1constants.LabelApp: labelValueAndCleanupName,
					},
				},
				Spec: corev1.PodSpec{
					HostNetwork:       true,
					RestartPolicy:     corev1.RestartPolicyAlways,
					PriorityClassName: "system-node-critical",
					Tolerations: []corev1.Toleration{
						{
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoExecute,
						},
						{
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					NodeSelector: map[string]string{
						labelKeyCleanupRequired: "true",
					},
					Containers: []corev1.Container{
						{
							Name:  "cleanup",
							Image: alpineImage,
							Command: []string{
								"/bin/sh",
								"-c",
								"/scripts/cleanup.sh",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      volumeMountNameCleanUp,
									MountPath: volumeMountPathCleanUp,
									ReadOnly:  true,
								},
								{
									Name:      volumeMountNameXtablesLock,
									MountPath: volumeMountPathXtablesLock,
									ReadOnly:  false,
								},
							},
							ReadinessProbe: &corev1.Probe{
								InitialDelaySeconds: 5,
								PeriodSeconds:       2,
								SuccessThreshold:    1,
								FailureThreshold:    3,
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"cat",
											"/tmp/healthy",
										},
									},
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{"NET_ADMIN", "NET_RAW"},
								},
								Privileged: ptr.To(false),
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: volumeMountNameCleanUp,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: cleanupConfigMapName,
									},
									DefaultMode: ptr.To[int32](0775),
								},
							},
						},
						{
							Name: volumeMountNameXtablesLock,
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: volumeMountPathXtablesLock,
									Type: ptr.To(corev1.HostPathFileOrCreate),
								},
							},
						},
					},
				},
			},
		}
		return nil
	})
	return err
}

// MarkNodesForCleanup marks nodes for cleanup by adding a label to them if the node-local-dns DaemonSet exists.
func MarkNodesForCleanup(ctx context.Context, shootClient client.Client, workers []gardencorev1beta1.Worker) error {
	nodeList := &metav1.PartialObjectMetadataList{}
	nodeList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("NodeList"))
	if err := shootClient.List(ctx, nodeList, client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(utils.MustNewRequirement(labelKeyCleanupRequired, selection.DoesNotExist))}); err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	poolToNodes := make(map[string][]metav1.PartialObjectMetadata)
	for _, node := range nodeList.Items {
		kubernetesVersionString, ok := node.Labels[v1beta1constants.LabelWorkerKubernetesVersion]
		if !ok {
			continue
		}

		kubernetesVersion, err := semver.NewVersion(kubernetesVersionString)
		if err != nil {
			return err
		}

		if versionutils.ConstraintK8sLess134.Check(kubernetesVersion) {
			continue
		}

		if poolName := node.Labels[v1beta1constants.LabelWorkerPool]; poolName != "" {
			poolToNodes[poolName] = append(poolToNodes[poolName], node)
		}
	}

	var taskFns []flow.TaskFn
	// Check for node-local-dns DaemonSet existence for each worker pool
	for _, worker := range workers {
		daemonSet := &metav1.PartialObjectMetadata{}
		daemonSet.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("DaemonSet"))
		if err := shootClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: "node-local-dns-" + worker.Name}, daemonSet); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed to check DaemonSet %s: %w", daemonSet.Name, err)
			}
			continue
		}

		// If DaemonSet exists, add cleanup label to all nodes in the worker group
		taskFns = append(taskFns, addCleanupLabelToNodes(shootClient, poolToNodes[worker.Name])...)
	}
	return flow.Parallel(taskFns...)(ctx)
}

// waitForDaemonSetCompletion waits until the cleanup job is completed and the all pods from the daemonset are marked as ready.
func waitForDaemonSetCompletion(ctx context.Context, shootClient client.Client, namespace, name string) error {
	return retry.UntilTimeout(ctx, daemonSetPollInterval, 1*time.Minute, func(ctx context.Context) (done bool, err error) {
		daemonSet := &appsv1.DaemonSet{}
		if err := shootClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, daemonSet); err != nil {
			return retry.SevereError(err)
		}

		if daemonSet.Generation != daemonSet.Status.ObservedGeneration {
			return false, nil
		}

		if daemonSet.Status.NumberUnavailable == 0 && daemonSet.Status.DesiredNumberScheduled == daemonSet.Status.NumberReady {
			return retry.Ok()
		}
		return false, nil
	})
}

// deleteCleanupScriptConfigMap deletes the ConfigMap containing the cleanup script.
func deleteCleanupScriptConfigMap(ctx context.Context, shootClient client.Client) error {
	cleanupScriptCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cleanupConfigMapName,
			Namespace: metav1.NamespaceSystem,
		},
	}
	if err := shootClient.Delete(ctx, cleanupScriptCM); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to delete cleanup script ConfigMap: %w", err)
	}
	return nil
}

// addCleanupLabelToNodes adds a cleanup label to the nodes that are part of the node-local-dns deployment.
func addCleanupLabelToNodes(shootClient client.Client, nodeList []metav1.PartialObjectMetadata) []flow.TaskFn {
	var taskFns []flow.TaskFn
	for _, node := range nodeList {
		taskFns = append(taskFns, func(ctx context.Context) error {
			patch := client.MergeFrom(node.DeepCopy())
			metav1.SetMetaDataLabel(&node.ObjectMeta, labelKeyCleanupRequired, "true")
			if err := shootClient.Patch(ctx, &node, patch); err != nil {
				return fmt.Errorf("failed to add cleanup label to node %s: %w", node.Name, err)
			}

			return nil
		})
	}
	return taskFns
}

// RunCleanup runs the cleanup DaemonSet for node-local-dns, waits for its completion, and removes the cleanup label from nodes.
func RunCleanup(ctx context.Context, shootClient client.Client, alpineImage string, logger logr.Logger) error {
	if err := createCleanupConfigMap(ctx, shootClient); err != nil {
		return fmt.Errorf("failed to create cleanup ConfigMap: %w", err)
	}

	if err := createCleanupDaemonSet(ctx, shootClient, alpineImage); err != nil {
		return fmt.Errorf("failed to create cleanup DaemonSet: %w", err)
	}

	if err := waitForDaemonSetCompletion(ctx, shootClient, metav1.NamespaceSystem, labelValueAndCleanupName); err != nil {
		return fmt.Errorf("cleanup DaemonSet %s failed: %w", labelValueAndCleanupName, err)
	}

	logger.Info("Cleanup DaemonSet for node-local-dns completed")
	cleanupDaemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      labelValueAndCleanupName,
			Namespace: metav1.NamespaceSystem,
		},
	}
	if err := shootClient.Delete(ctx, cleanupDaemonSet); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to delete cleanup DaemonSet %s: %w", labelValueAndCleanupName, err)
	}

	taskFns := []flow.TaskFn{}
	nodeList := &metav1.PartialObjectMetadataList{}
	nodeList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("NodeList"))
	if err := shootClient.List(ctx, nodeList, client.MatchingLabels{
		labelKeyCleanupRequired: "true",
	}); err != nil {
		return fmt.Errorf("failed to list nodes for cleanup: %w", err)
	}

	for _, node := range nodeList.Items {
		taskFns = append(taskFns, func(ctx context.Context) error {
			patch := client.MergeFrom(node.DeepCopy())
			delete(node.Labels, labelKeyCleanupRequired)
			return shootClient.Patch(ctx, &node, patch)
		})
	}

	if err := flow.Parallel(taskFns...)(ctx); err != nil {
		return fmt.Errorf("failed to remove cleanup label from nodes: %w", err)
	}

	return deleteCleanupScriptConfigMap(ctx, shootClient)
}
