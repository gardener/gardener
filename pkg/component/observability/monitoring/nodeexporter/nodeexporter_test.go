// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeexporter_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/observability/monitoring/nodeexporter"
	componenttest "github.com/gardener/gardener/pkg/component/test"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("NodeExporter", func() {
	var (
		ctx = context.Background()

		managedResourceName = "shoot-core-node-exporter"
		namespace           = "some-namespace"
		image               = "some-image:some-tag"

		c         client.Client
		values    Values
		component component.DeployWaiter

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret

		scrapeConfig = &monitoringv1alpha1.ScrapeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "shoot-node-exporter",
				Namespace:       namespace,
				Labels:          map[string]string{"prometheus": "shoot"},
				ResourceVersion: "1",
			},
			Spec: monitoringv1alpha1.ScrapeConfigSpec{
				HonorLabels: ptr.To(false),
				Scheme:      ptr.To("HTTPS"),
				TLSConfig:   &monitoringv1.SafeTLSConfig{InsecureSkipVerify: ptr.To(true)},
				Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "shoot-access-prometheus-shoot"},
					Key:                  "token",
				}},
				KubernetesSDConfigs: []monitoringv1alpha1.KubernetesSDConfig{{
					APIServer:  ptr.To("https://kube-apiserver"),
					Role:       "Endpoints",
					Namespaces: &monitoringv1alpha1.NamespaceDiscovery{Names: []string{"kube-system"}},
					Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "shoot-access-prometheus-shoot"},
						Key:                  "token",
					}},
					TLSConfig: &monitoringv1.SafeTLSConfig{InsecureSkipVerify: ptr.To(true)},
				}},
				RelabelConfigs: []monitoringv1.RelabelConfig{
					{
						Action:      "replace",
						Replacement: ptr.To("node-exporter"),
						TargetLabel: "job",
					},
					{
						TargetLabel: "type",
						Replacement: ptr.To("shoot"),
					},
					{
						SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_service_name", "__meta_kubernetes_endpoint_port_name"},
						Action:       "keep",
						Regex:        "node-exporter;metrics",
					},
					{
						Action: "labelmap",
						Regex:  `__meta_kubernetes_service_label_(.+)`,
					},
					{
						SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_name"},
						TargetLabel:  "pod",
					},
					{
						SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_node_name"},
						TargetLabel:  "node",
					},
					{
						TargetLabel: "__address__",
						Replacement: ptr.To("kube-apiserver:443"),
					},
					{
						SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_name", "__meta_kubernetes_pod_container_port_number"},
						Regex:        `(.+);(.+)`,
						TargetLabel:  "__metrics_path__",
						Replacement:  ptr.To("/api/v1/namespaces/kube-system/pods/${1}:${2}/proxy/metrics"),
					},
				},
				MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
					SourceLabels: []monitoringv1.LabelName{"__name__"},
					Action:       "keep",
					Regex:        `^(node_boot_time_seconds|node_cpu_seconds_total|node_disk_read_bytes_total|node_disk_written_bytes_total|node_disk_io_time_weighted_seconds_total|node_disk_io_time_seconds_total|node_disk_write_time_seconds_total|node_disk_writes_completed_total|node_disk_read_time_seconds_total|node_disk_reads_completed_total|node_filesystem_avail_bytes|node_filesystem_files|node_filesystem_files_free|node_filesystem_free_bytes|node_filesystem_readonly|node_filesystem_size_bytes|node_load1|node_load15|node_load5|node_memory_.+|node_nf_conntrack_.+|node_scrape_collector_duration_seconds|node_scrape_collector_success|process_max_fds|process_open_fds)$`,
				}},
			},
		}

		prometheusRule = &monitoringv1.PrometheusRule{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "shoot-node-exporter",
				Namespace:       namespace,
				Labels:          map[string]string{"prometheus": "shoot"},
				ResourceVersion: "1",
			},
			Spec: monitoringv1.PrometheusRuleSpec{
				Groups: []monitoringv1.RuleGroup{{
					Name: "node-exporter.rules",
					Rules: []monitoringv1.Rule{
						{
							Alert: "NodeExporterDown",
							Expr:  intstr.FromString(`absent(up{job="node-exporter"} == 1)`),
							For:   ptr.To(monitoringv1.Duration("1h")),
							Labels: map[string]string{
								"service":    "node-exporter",
								"severity":   "warning",
								"type":       "shoot",
								"visibility": "owner",
							},
							Annotations: map[string]string{
								"summary":     "NodeExporter down or unreachable",
								"description": "The NodeExporter has been down or unreachable from Prometheus for more than 1 hour.",
							},
						},
						{
							Alert: "K8SNodeOutOfDisk",
							Expr:  intstr.FromString(`kube_node_status_condition{condition="OutOfDisk", status="true"} == 1`),
							For:   ptr.To(monitoringv1.Duration("1h")),
							Labels: map[string]string{
								"service":    "node-exporter",
								"severity":   "critical",
								"type":       "shoot",
								"visibility": "owner",
							},
							Annotations: map[string]string{
								"summary":     "Node ran out of disk space.",
								"description": "Node {{$labels.node}} has run out of disk space.",
							},
						},
						{
							Alert: "K8SNodeMemoryPressure",
							Expr:  intstr.FromString(`kube_node_status_condition{condition="MemoryPressure", status="true"} == 1`),
							For:   ptr.To(monitoringv1.Duration("1h")),
							Labels: map[string]string{
								"service":    "node-exporter",
								"severity":   "warning",
								"type":       "shoot",
								"visibility": "owner",
							},
							Annotations: map[string]string{
								"summary":     "Node is under memory pressure.",
								"description": "Node {{$labels.node}} is under memory pressure.",
							},
						},
						{
							Alert: "K8SNodeDiskPressure",
							Expr:  intstr.FromString(`kube_node_status_condition{condition="DiskPressure", status="true"} == 1`),
							For:   ptr.To(monitoringv1.Duration("1h")),
							Labels: map[string]string{
								"service":    "node-exporter",
								"severity":   "warning",
								"type":       "shoot",
								"visibility": "owner",
							},
							Annotations: map[string]string{
								"summary":     "Node is under disk pressure.",
								"description": "Node {{$labels.node}} is under disk pressure.",
							},
						},
						{
							Record: "instance:conntrack_entries_usage:percent",
							Expr:   intstr.FromString(`(node_nf_conntrack_entries / node_nf_conntrack_entries_limit) * 100`),
						},
						{
							Alert: "VMRootfsFull",
							Expr:  intstr.FromString(`node_filesystem_free{mountpoint="/"} < 1024`),
							For:   ptr.To(monitoringv1.Duration("1h")),
							Labels: map[string]string{
								"service":    "node-exporter",
								"severity":   "critical",
								"type":       "shoot",
								"visibility": "owner",
							},
							Annotations: map[string]string{
								"description": "Root filesystem device on instance {{$labels.instance}} is almost full.",
								"summary":     "Node's root filesystem is almost full",
							},
						},
						{
							Alert: "VMConntrackTableFull",
							Expr:  intstr.FromString(`instance:conntrack_entries_usage:percent > 90`),
							For:   ptr.To(monitoringv1.Duration("1h")),
							Labels: map[string]string{
								"service":    "node-exporter",
								"severity":   "critical",
								"type":       "shoot",
								"visibility": "owner",
							},
							Annotations: map[string]string{
								"description": "The nf_conntrack table is {{$value}}% full.",
								"summary":     "Number of tracked connections is near the limit",
							},
						},
						{
							Record: "shoot:kube_node_info:count",
							Expr:   intstr.FromString(`count(kube_node_info{type="shoot"})`),
						},
						// This recording rule creates a series for nodes with less than 5% free inodes on a not read only mount point.
						// The series exists only if there are less than 5% free inodes,
						// to keep the cardinality of these federated metrics manageable.
						// Otherwise, we would get a series for each node in each shoot in the federating Prometheus.
						{
							Record: "shoot:node_filesystem_files_free:percent",
							Expr:   intstr.FromString(`sum by (node, mountpoint) (node_filesystem_files_free / node_filesystem_files * 100 < 5 and node_filesystem_readonly == 0)`),
						},
					},
				}},
			},
		}
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		values = Values{
			Image: image,
		}

		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: namespace,
			},
		}
		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResource.Name,
				Namespace: namespace,
			},
		}
	})

	Describe("#Deploy", func() {
		var (
			manifests         []string
			expectedManifests []string

			serviceAccountYAML = `apiVersion: v1
automountServiceAccountToken: false
kind: ServiceAccount
metadata:
  creationTimestamp: null
  labels:
    component: node-exporter
  name: node-exporter
  namespace: kube-system
`
			serviceYAML = `apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    component: node-exporter
  name: node-exporter
  namespace: kube-system
spec:
  clusterIP: None
  ports:
  - name: metrics
    port: 16909
    protocol: TCP
    targetPort: 0
  selector:
    component: node-exporter
  type: ClusterIP
status:
  loadBalancer: {}
`
			daemonSetYAML = `apiVersion: apps/v1
kind: DaemonSet
metadata:
  creationTimestamp: null
  labels:
    component: node-exporter
    gardener.cloud/role: monitoring
    origin: gardener
  name: node-exporter
  namespace: kube-system
spec:
  revisionHistoryLimit: 2
  selector:
    matchLabels:
      component: node-exporter
  template:
    metadata:
      creationTimestamp: null
      labels:
        component: node-exporter
        gardener.cloud/role: monitoring
        networking.gardener.cloud/from-seed: allowed
        networking.gardener.cloud/to-public-networks: allowed
        origin: gardener
    spec:
      automountServiceAccountToken: false
      containers:
      - command:
        - /bin/node_exporter
        - --web.listen-address=:16909
        - --path.procfs=/host/proc
        - --path.sysfs=/host/sys
        - --path.rootfs=/host
        - --path.udev.data=/host/run/udev/data
        - --log.level=error
        - --collector.disable-defaults
        - --collector.conntrack
        - --collector.cpu
        - --collector.diskstats
        - --collector.filefd
        - --collector.filesystem
        - --collector.filesystem.mount-points-exclude=^/(run|var)/.+$|^/(boot|dev|sys|usr)($|/.+$)
        - --collector.loadavg
        - --collector.meminfo
        - --collector.uname
        - --collector.stat
        - --collector.pressure
        - --collector.textfile
        - --collector.textfile.directory=/textfile-collector
        image: some-image:some-tag
        imagePullPolicy: IfNotPresent
        livenessProbe:
          httpGet:
            path: /
            port: 16909
          initialDelaySeconds: 5
          timeoutSeconds: 5
        name: node-exporter
        ports:
        - containerPort: 16909
          hostPort: 16909
          name: scrape
          protocol: TCP
        readinessProbe:
          httpGet:
            path: /
            port: 16909
          initialDelaySeconds: 5
          timeoutSeconds: 5
        resources:
          requests:
            cpu: 50m
            memory: 50Mi
        securityContext:
          allowPrivilegeEscalation: false
        volumeMounts:
        - mountPath: /host
          name: host
          readOnly: true
        - mountPath: /textfile-collector
          name: textfile
          readOnly: true
      hostNetwork: true
      hostPID: true
      priorityClassName: system-cluster-critical
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
        seccompProfile:
          type: RuntimeDefault
      serviceAccountName: node-exporter
      tolerations:
      - effect: NoSchedule
        operator: Exists
      - effect: NoExecute
        operator: Exists
      volumes:
      - hostPath:
          path: /
        name: host
      - hostPath:
          path: /var/lib/node-exporter/textfile-collector
        name: textfile
  updateStrategy:
    type: RollingUpdate
status:
  currentNumberScheduled: 0
  desiredNumberScheduled: 0
  numberMisscheduled: 0
  numberReady: 0
`
			vpaYAML = `apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  creationTimestamp: null
  name: node-exporter
  namespace: kube-system
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: '*'
      controlledValues: RequestsOnly
      minAllowed:
        memory: 50Mi
  targetRef:
    apiVersion: apps/v1
    kind: DaemonSet
    name: node-exporter
  updatePolicy:
    updateMode: Auto
status: {}
`
		)

		JustBeforeEach(func() {
			component = New(c, namespace, values)
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())

			Expect(component.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResource.Name,
					Namespace:       managedResource.Namespace,
					ResourceVersion: "1",
					Labels:          map[string]string{"origin": "gardener"},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResource.Spec.SecretRefs[0].Name,
					}},
					KeepObjects: ptr.To(false),
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedMr))
			Expect(managedResource).To(DeepEqual(expectedMr))

			managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
			actualScrapeConfig := &monitoringv1alpha1.ScrapeConfig{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(scrapeConfig), actualScrapeConfig)).To(Succeed())
			Expect(actualScrapeConfig).To(DeepEqual(scrapeConfig))

			actualPrometheusRule := &monitoringv1.PrometheusRule{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(prometheusRule), actualPrometheusRule)).To(Succeed())
			Expect(actualPrometheusRule).To(DeepEqual(prometheusRule))

			componenttest.PrometheusRule(prometheusRule, "testdata/shoot-node-exporter.prometheusrule.test.yaml")

			var err error
			manifests, err = test.ExtractManifestsFromManagedResourceData(managedResourceSecret.Data)
			Expect(err).NotTo(HaveOccurred())

			expectedManifests = []string{
				serviceAccountYAML,
				serviceYAML,
				daemonSetYAML,
			}
		})

		Context("VPA disabled", func() {
			It("should successfully deploy all resources", func() {
				Expect(manifests).To(ConsistOf(expectedManifests))
			})
		})

		Context("VPA enabled", func() {
			BeforeEach(func() {
				values.VPAEnabled = true
			})

			It("should successfully deploy the VPA resource", func() {
				expectedManifests = append(expectedManifests, vpaYAML)
				Expect(manifests).To(ConsistOf(expectedManifests))
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			component = New(c, namespace, values)
			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

			scrapeConfig.ResourceVersion = ""
			prometheusRule.ResourceVersion = ""
			Expect(c.Create(ctx, scrapeConfig)).To(Succeed())
			Expect(c.Create(ctx, prometheusRule)).To(Succeed())

			Expect(component.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(scrapeConfig), scrapeConfig)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(prometheusRule), prometheusRule)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
		})
	})

	Context("waiting functions", func() {
		var (
			fakeOps   *retryfake.Ops
			resetVars func()
		)

		BeforeEach(func() {
			component = New(c, namespace, values)

			fakeOps = &retryfake.Ops{MaxAttempts: 1}
			resetVars = test.WithVars(
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			)
		})

		AfterEach(func() {
			resetVars()
		})

		Describe("#Wait", func() {
			It("should fail because reading the ManagedResource fails", func() {
				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the ManagedResource doesn't become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionFalse,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should successfully wait for the managed resource to become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})).To(Succeed())

				Expect(component.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, managedResource)).To(Succeed())

				Expect(component.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it's already removed", func() {
				Expect(component.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})
