// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodelocaldns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	nodelocaldnsconstants "github.com/gardener/gardener/pkg/component/networking/nodelocaldns/constants"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenerextensions "github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	managedResourceName = "shoot-core-node-local-dns"

	labelKey = "k8s-app"
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

	configmapName = "node-local-dns-cleanup-script"
)

// Interface contains functions for a NodeLocalDNS deployer.
type Interface interface {
	component.DeployWaiter
	SetClusterDNS([]string)
	SetDNSServers([]string)
	SetIPFamilies([]gardencorev1beta1.IPFamily)
	SetWorkerPools([]WorkerPool)
	SetShootClientSet(kubernetes.Interface)
	SetLogger(logr.Logger)
}

// Values is a set of configuration values for the node-local-dns component.
type Values struct {
	// Image is the container image used for node-local-dns.
	Image string
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
	// WorkerPools is a list of worker pools for which the node-local-dns DaemonSets should be deployed.
	WorkerPools []WorkerPool
	// ShootClientSet is the client set used to interact with the shoot cluster.
	ShootClientSet kubernetes.Interface
	// Log is the logger used for logging.
	Log logr.Logger
}

// WorkerPool contains configuration for the node-local-dns daemonset for this specific worker pool.
type WorkerPool struct {
	// Name is the name of the worker pool.
	Name string
	// KubernetesVersion is the Kubernetes version of the worker pool.
	KubernetesVersion *semver.Version
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

	serviceAccount *corev1.ServiceAccount
	configMap      *corev1.ConfigMap
	service        *corev1.Service
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

	err := n.computeResourcesData()
	if err != nil {
		return err
	}

	daemonSets, vpas, err := n.computePoolResourcesData()
	if err != nil {
		return err
	}

	objects := []client.Object{n.serviceAccount, n.configMap, n.service}
	for i := range daemonSets {
		objects = append(objects, daemonSets[i])
	}
	for i := range vpas {
		objects = append(objects, vpas[i])
	}

	data, err := registry.AddAllAndSerialize(objects...)
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, n.client, n.namespace, managedResourceName, managedresources.LabelValueGardener, false, data)
}

func (n *nodeLocalDNS) Destroy(ctx context.Context) error {
	cluster, err := gardenerextensions.GetCluster(ctx, n.client, n.namespace)
	if err != nil {
		return err
	}

	err = n.createCleanupConfigMap(ctx)
	if err != nil {
		return err
	}

	// Check for node-local-dns DaemonSet existence for each worker pool
	for _, pool := range n.values.WorkerPools {
		daemonSetName := "node-local-dns-" + pool.Name
		n.values.Log.Info("Checking for DaemonSet", "daemonSetName", daemonSetName)
		daemonSet := &appsv1.DaemonSet{}
		err := n.values.ShootClientSet.Client().Get(ctx, types.NamespacedName{
			Namespace: metav1.NamespaceSystem,
			Name:      daemonSetName,
		}, daemonSet)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed to check DaemonSet %s: %w", daemonSetName, err)
			}
			// DaemonSet does not exist, skip this worker pool
			n.values.Log.Info("DaemonSet not found, skipping cleanup", "daemonSetName", daemonSetName)
			continue
		}

		// If DaemonSet exists, add cleanup annotation to the cluster resource
		if cluster.Shoot == nil {
			return fmt.Errorf("shoot object is nil in cluster")
		}
		if cluster.Shoot.Annotations == nil {
			cluster.Shoot.Annotations = make(map[string]string)
		}
		annotationKey := fmt.Sprintf("node-local-dns-cleanup-required-for-%s", pool.Name)
		cluster.Shoot.Annotations[annotationKey] = "true"
		n.values.Log.Info("Adding annotation to shoot", "annotation", annotationKey)
	}

	// Sync the updated cluster resource to the seed
	if syncErr := gardenerextensions.SyncClusterResourceToSeed(ctx, n.client, cluster.ObjectMeta.Name, cluster.Shoot, cluster.CloudProfile, cluster.Seed); syncErr != nil {
		return fmt.Errorf("cluster resource sync to seed failed: %w", syncErr)
	}

	// Delete resources
	if err := kubernetesutils.DeleteObjects(ctx, n.client,
		n.emptyScrapeConfig(""),
		n.emptyScrapeConfig("-errors"),
	); err != nil {
		return err
	}

	err = managedresources.DeleteForShoot(ctx, n.client, n.namespace, managedResourceName)
	if err != nil {
		return err
	}

	// Handle cleanup DaemonSet for each worker pool
	for _, pool := range n.values.WorkerPools {
		annotationKey := fmt.Sprintf("node-local-dns-cleanup-required-for-%s", pool.Name)
		if cluster.Shoot.Annotations[annotationKey] == "true" {

			// Create the cleanup DaemonSet
			cleanupDaemonSet, err := n.createCleanupDaemonSetForWorkerPool(pool.Name)
			if err != nil {
				return err
			}
			if err := n.values.ShootClientSet.Client().Create(ctx, cleanupDaemonSet); err != nil {
				return fmt.Errorf("failed to create cleanup DaemonSet for worker pool %s: %w", pool.Name, err)
			}
			n.values.Log.Info("Cleanup DaemonSet created", "workerPool", pool.Name)

			// Wait for the DaemonSet to complete (optional)
			if err := waitForDaemonSetCompletion(ctx, n.values.ShootClientSet.Client(), cleanupDaemonSet.Namespace, cleanupDaemonSet.Name); err != nil {
				return fmt.Errorf("cleanup DaemonSet for worker pool %s failed: %w", pool.Name, err)
			}
			n.values.Log.Info("Cleanup DaemonSet completed", "workerPool", pool.Name)
			time.Sleep(10 * time.Second) // Optional: wait a bit before deleting
			// Delete the DaemonSet after completion
			if err := n.values.ShootClientSet.Client().Delete(ctx, cleanupDaemonSet); err != nil {
				return fmt.Errorf("failed to delete cleanup DaemonSet for worker pool %s: %w", pool.Name, err)
			}
			n.values.Log.Info("Cleanup DaemonSet deleted", "workerPool", pool.Name, "delete annotation", annotationKey)
			// Remove the annotation after successful cleanup
			delete(cluster.Shoot.Annotations, annotationKey)
		}
	}

	// Delete the cleanup script ConfigMap after all cleanups are done
	cleanupScriptCM := &corev1.ConfigMap{}
	if err := n.values.ShootClientSet.Client().Get(ctx, types.NamespacedName{
		Name:      configmapName,
		Namespace: "kube-system",
	}, cleanupScriptCM); err == nil {
		if delErr := n.values.ShootClientSet.Client().Delete(ctx, cleanupScriptCM); delErr != nil {
			return fmt.Errorf("failed to delete cleanup script ConfigMap: %w", delErr)
		}
	}

	// Sync the updated cluster resource to the seed after cleanup
	if syncErr := gardenerextensions.SyncClusterResourceToSeed(ctx, n.client, cluster.ObjectMeta.Name, cluster.Shoot, cluster.CloudProfile, cluster.Seed); syncErr != nil {
		return fmt.Errorf("cluster resource sync to seed failed: %w", syncErr)
	}

	return nil
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

func (n *nodeLocalDNS) computeResourcesData() error {
	if n.getHealthAddress() == "" {
		return errors.New("empty IPVSAddress")
	}

	var (
		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "node-local-dns",
				Namespace: metav1.NamespaceSystem,
			},
			AutomountServiceAccountToken: ptr.To(false),
		}

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "node-local-dns",
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					labelKey: nodelocaldnsconstants.LabelValue,
				},
			},
			Data: map[string]string{
				configDataKey: domain + `:53 {
    errors
    cache {
            success 9984 30
            denial 9984 5
    }
    reload
    loop
    bind ` + n.bindIP() + `
    forward . ` + strings.Join(n.values.ClusterDNS, " ") + ` {
            ` + n.forceTcpToClusterDNS() + `
    }
    prometheus :` + strconv.Itoa(prometheusPort) + `
    health ` + n.getHealthAddress() + `:` + strconv.Itoa(livenessProbePort) + `
    }
in-addr.arpa:53 {
    errors
    cache 30
    reload
    loop
    bind ` + n.bindIP() + `
    forward . ` + strings.Join(n.values.ClusterDNS, " ") + ` {
            ` + n.forceTcpToClusterDNS() + `
    }
    prometheus :` + strconv.Itoa(prometheusPort) + `
    }
ip6.arpa:53 {
    errors
    cache 30
    reload
    loop
    bind ` + n.bindIP() + `
    forward . ` + strings.Join(n.values.ClusterDNS, " ") + ` {
            ` + n.forceTcpToClusterDNS() + `
    }
    prometheus :` + strconv.Itoa(prometheusPort) + `
    }
.:53 {
    errors
    cache 30
    reload
    loop
    bind ` + n.bindIP() + `
    forward . ` + n.upstreamDNSAddress() + ` {
            ` + n.forceTcpToUpstreamDNS() + `
    }
    prometheus :` + strconv.Itoa(prometheusPort) + `
    }
`,
			},
		}
	)

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))

	var (
		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					"k8s-app": "kube-dns-upstream",
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{"k8s-app": "kube-dns"},
				Ports: []corev1.ServicePort{
					{
						Name:       "dns",
						Port:       int32(portServiceServer),
						TargetPort: intstr.FromInt32(portServer),
						Protocol:   corev1.ProtocolUDP,
					},
					{
						Name:       "dns-tcp",
						Port:       int32(portServiceServer),
						TargetPort: intstr.FromInt32(portServer),
						Protocol:   corev1.ProtocolTCP,
					},
				},
			},
		}
	)
	n.serviceAccount = serviceAccount
	n.configMap = configMap
	n.service = service

	return nil
}

func (n *nodeLocalDNS) computePoolResourcesData() ([]*appsv1.DaemonSet, []*vpaautoscalingv1.VerticalPodAutoscaler, error) {
	var (
		maxUnavailable       = intstr.FromString("10%")
		hostPathFileOrCreate = corev1.HostPathFileOrCreate
		vpas                 []*vpaautoscalingv1.VerticalPodAutoscaler
		daemonSets           []*appsv1.DaemonSet
	)

	for _, pool := range n.values.WorkerPools {
		daemonSet := &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "node-local-dns-" + pool.Name,
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					labelKey:                                    nodelocaldnsconstants.LabelValue,
					v1beta1constants.GardenRole:                 v1beta1constants.GardenRoleSystemComponent,
					managedresources.LabelKeyOrigin:             managedresources.LabelValueGardener,
					v1beta1constants.LabelNodeCriticalComponent: "true",
				},
			},
			Spec: appsv1.DaemonSetSpec{
				UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
					RollingUpdate: &appsv1.RollingUpdateDaemonSet{
						MaxUnavailable: &maxUnavailable,
					},
				},
				RevisionHistoryLimit: ptr.To[int32](2),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						labelKey: nodelocaldnsconstants.LabelValue,
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							labelKey:                                    nodelocaldnsconstants.LabelValue,
							v1beta1constants.LabelNetworkPolicyToDNS:    "allowed",
							v1beta1constants.LabelNodeCriticalComponent: "true",
						},
						Annotations: map[string]string{
							"prometheus.io/port":   strconv.Itoa(prometheusPort),
							"prometheus.io/scrape": strconv.FormatBool(prometheusScrape),
						},
					},
					Spec: corev1.PodSpec{
						PriorityClassName:  "system-node-critical",
						ServiceAccountName: n.serviceAccount.Name,
						HostNetwork:        true,
						DNSPolicy:          corev1.DNSDefault,
						SecurityContext: &corev1.PodSecurityContext{
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
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
							v1beta1constants.LabelNodeLocalDNS: "true",
							v1beta1constants.LabelWorkerPool:   pool.Name,
						},
						Containers: []corev1.Container{
							{
								Name:  containerName,
								Image: n.values.Image,
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("25m"),
										corev1.ResourceMemory: resource.MustParse("25Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("200Mi"),
									},
								},
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: ptr.To(false),
									Capabilities: &corev1.Capabilities{
										Add: []corev1.Capability{"NET_ADMIN"},
									},
								},
								Args: []string{
									"-localip",
									n.containerArg(),
									"-conf",
									"/etc/Corefile",
									"-upstreamsvc",
									serviceName,
									"-health-port",
									strconv.Itoa(livenessProbePort),
								},
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: int32(53),
										Name:          "dns",
										Protocol:      corev1.ProtocolUDP,
									},
									{
										ContainerPort: int32(53),
										Name:          "dns-tcp",
										Protocol:      corev1.ProtocolTCP,
									},
									{
										ContainerPort: int32(prometheusPort),
										Name:          metricsPortName,
										Protocol:      corev1.ProtocolTCP,
									},
									{
										ContainerPort: int32(prometheusErrorPort),
										Name:          errorMetricsPortName,
										Protocol:      corev1.ProtocolTCP,
									},
								},
								LivenessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Host: n.getIPVSAddress(),
											Path: "/health",
											Port: intstr.FromInt32(livenessProbePort),
										},
									},
									InitialDelaySeconds: int32(60),
									TimeoutSeconds:      int32(5),
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										MountPath: "/run/xtables.lock",
										Name:      "xtables-lock",
										ReadOnly:  false,
									},
									{
										MountPath: "/etc/coredns",
										Name:      "config-volume",
									},
									{
										MountPath: "/etc/kube-dns",
										Name:      "kube-dns-config",
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "xtables-lock",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/run/xtables.lock",
										Type: &hostPathFileOrCreate,
									},
								},
							},
							{
								Name: "kube-dns-config",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "kube-dns",
										},
										Optional: ptr.To(true),
									},
								},
							},
							{
								Name: "config-volume",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: n.configMap.Name,
										},
										Items: []corev1.KeyToPath{
											{
												Key:  configDataKey,
												Path: "Corefile.base",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		utilruntime.Must(references.InjectAnnotations(daemonSet))
		daemonSets = append(daemonSets, daemonSet)

		if n.values.VPAEnabled {
			vpaUpdateMode := vpaautoscalingv1.UpdateModeAuto
			vpa := &vpaautoscalingv1.VerticalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "node-local-dns-" + pool.Name,
					Namespace: metav1.NamespaceSystem,
				},
				Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
					TargetRef: &autoscalingv1.CrossVersionObjectReference{
						APIVersion: appsv1.SchemeGroupVersion.String(),
						Kind:       "DaemonSet",
						Name:       daemonSet.Name,
					},
					UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
						UpdateMode: &vpaUpdateMode,
					},
					ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
						ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
							ContainerName:    vpaautoscalingv1.DefaultContainerResourcePolicy,
							ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
						}},
					},
				},
			}
			vpas = append(vpas, vpa)
		}
	}

	return daemonSets, vpas, nil
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

func (n *nodeLocalDNS) SetWorkerPools(pools []WorkerPool) {
	n.values.WorkerPools = pools
}

func (n *nodeLocalDNS) SetShootClientSet(set kubernetes.Interface) {
	n.values.ShootClientSet = set
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
func (n *nodeLocalDNS) createCleanupConfigMap(ctx context.Context) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmapName,
			Namespace: "kube-system",
		},
		Data: map[string]string{
			"cleanup-nodelocaldns.sh": `#!/bin/sh
# Check if the nodelocaldns interface exists
if ip link show nodelocaldns > /dev/null 2>&1; then
    ip link delete nodelocaldns
    # Check if the operation was successful
    if [ $? -eq 0 ]; then
        echo "Nodelocaldns interface deleted successfully."
    else
        echo "An error occurred while deleting nodelocaldns interface."
        exit 1
    fi
else
    echo "Nodelocaldns interface does not exist. Skipping deletion."
fi

# Define the comment to search for
COMMENT="NodeLocal DNS Cache:"

# Check if there are any iptables rules with the specified comment
if iptables-legacy-save | grep -- "--comment \"$COMMENT\"" > /dev/null 2>&1; then
    # Find and delete all iptables rules with the specified comment
    iptables-legacy-save | grep -- "--comment \"$COMMENT\"" | while read -r line; do
        # Extract the rule specification
        rule=$(echo "$line" | sed -e 's/^-A/-D/')
        # Delete the rule
        iptables $rule
    done

    # Check if the operation was successful
    if [ $? -eq 0 ]; then
        echo "All iptables rules with the comment \"$COMMENT\" have been deleted successfully."
    else
        echo "An error occurred while deleting iptables rules."
        exit 1
    fi
else
    echo "No iptables rules with the comment \"$COMMENT\" found for nodelocaldns. Skipping deletion."
fi
touch /tmp/healthy
sleep infinity`,
		},
	}

	// Try to create, or update if already exists
	err := n.values.ShootClientSet.Client().Create(ctx, cm)
	if apierrors.IsAlreadyExists(err) {
		// Update the existing ConfigMap
		existing := &corev1.ConfigMap{}
		if getErr := n.values.ShootClientSet.Client().Get(ctx, types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace}, existing); getErr != nil {
			return getErr
		}
		existing.Data = cm.Data
		return n.values.ShootClientSet.Client().Update(ctx, existing)
	}
	return err
}

func (n *nodeLocalDNS) createCleanupDaemonSetForWorkerPool(poolName string) (*appsv1.DaemonSet, error) {
	imageAlpine, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameAlpineIptables)
	if err != nil {
		return nil, err
	}
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("node-local-dns-cleanup-%s", poolName),
			Namespace: "kube-system",
			Labels: map[string]string{
				"app":  "node-local-dns-cleanup",
				"pool": poolName,
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":  "node-local-dns-cleanup",
					"pool": poolName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":  "node-local-dns-cleanup",
						"pool": poolName,
					},
				},
				Spec: corev1.PodSpec{
					HostNetwork:   true,
					RestartPolicy: corev1.RestartPolicyAlways,
					Tolerations: []corev1.Toleration{
						{
							Operator: corev1.TolerationOpExists,
						},
					},
					NodeSelector: map[string]string{
						"worker.gardener.cloud/pool": poolName,
					},
					Containers: []corev1.Container{
						{
							Name:  "cleanup",
							Image: imageAlpine.String(),
							Command: []string{
								"/bin/sh",
								"-c",
								"/scripts/cleanup-nodelocaldns.sh",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "cleanup-script",
									MountPath: "/scripts",
									ReadOnly:  true,
								},
							},
							ReadinessProbe: &corev1.Probe{
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
							Name: "cleanup-script",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configmapName,
									},
									DefaultMode: ptr.To[int32](0775),
								},
							},
						},
					},
				},
			},
		},
	}, nil
}

func waitForDaemonSetCompletion(ctx context.Context, client client.Client, namespace, name string) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled or timed out while waiting for DaemonSet %s/%s to become ready", namespace, name)
		default:
			daemonSet := &appsv1.DaemonSet{}
			if err := client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, daemonSet); err != nil {
				return err
			}

			if daemonSet.Status.NumberUnavailable == 0 && daemonSet.Status.DesiredNumberScheduled == daemonSet.Status.NumberReady {
				return nil // All pods are ready
			}

			time.Sleep(5 * time.Second) // Poll every 5 seconds
		}
	}
}
