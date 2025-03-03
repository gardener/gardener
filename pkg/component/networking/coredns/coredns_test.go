// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package coredns_test

import (
	"context"
	"net"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/networking/coredns"
	componenttest "github.com/gardener/gardener/pkg/component/test"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("CoreDNS", func() {
	var (
		ctx = context.Background()

		managedResourceName = "shoot-core-coredns"
		namespace           = "some-namespace"
		clusterDomain       = "foo.bar"
		clusterIPs          = []net.IP{net.ParseIP("1.2.3.4"), net.ParseIP("2001:db8:3::10")}
		image               = "some-image:some-tag"
		cpaImage            = "cpa-image:cpa-tag"
		podNetworkCIDRs     = []net.IPNet{
			{IP: net.ParseIP("5.6.7.8"), Mask: []byte{255, 255, 255, 0}},
			{IP: net.ParseIP("2001:db8:2::"), Mask: []byte{255, 255, 255, 255, 255, 255, 255, 255, 0, 0, 0, 0, 0, 0, 0, 0}},
		}
		nodeNetworkCIDRs = []net.IPNet{
			{IP: net.ParseIP("10.11.12.13"), Mask: []byte{255, 255, 0, 0}},
			{IP: net.ParseIP("2001:db8:1::"), Mask: []byte{255, 255, 255, 255, 255, 255, 255, 255, 0, 0, 0, 0, 0, 0, 0, 0}},
		}

		c         client.Client
		values    Values
		component component.DeployWaiter

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret
		manifests             []string

		serviceAccountYAML = `apiVersion: v1
automountServiceAccountToken: false
kind: ServiceAccount
metadata:
  creationTimestamp: null
  name: coredns
  namespace: kube-system
`
		clusterRoleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  name: system:coredns
rules:
- apiGroups:
  - ""
  resources:
  - endpoints
  - services
  - pods
  - namespaces
  verbs:
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - nodes
  verbs:
  - get
- apiGroups:
  - discovery.k8s.io
  resources:
  - endpointslices
  verbs:
  - list
  - watch
`
		clusterRoleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  annotations:
    resources.gardener.cloud/delete-on-invalid-update: "true"
  creationTimestamp: null
  name: system:coredns
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:coredns
subjects:
- kind: ServiceAccount
  name: coredns
  namespace: kube-system
`
		configMapYAML = func(commonSuffixes []string) string {
			out := `apiVersion: v1
data:
  Corefile: |
    .:8053 {
      errors
      log . {
          class error
      }
      health {
          lameduck 15s
      }
      ready
      rewrite stop {
        name regex (^(?:[^\.]+\.)+)svc\.foo\.bar\.svc\.foo\.bar {1}svc.foo.bar
        answer name (^(?:[^\.]+\.)+)svc\.foo\.bar {1}svc.foo.bar.svc.foo.bar
        answer value (^(?:[^\.]+\.)+)svc\.foo\.bar {1}svc.foo.bar.svc.foo.bar
      }`
			for _, suffix := range commonSuffixes {
				out += `
      rewrite stop {
        name regex (.*)\.` + regexp.QuoteMeta(suffix) + `\.svc\.foo\.bar {1}.` + suffix + `
        answer name (.*)\.` + regexp.QuoteMeta(suffix) + ` {1}.` + suffix + `.svc.foo.bar
        answer value (.*)\.` + regexp.QuoteMeta(suffix) + ` {1}.` + suffix + `.svc.foo.bar
      }`
			}
			out += `
      kubernetes ` + clusterDomain + ` in-addr.arpa ip6.arpa {
          pods insecure
          fallthrough in-addr.arpa ip6.arpa
          ttl 30
      }
      prometheus :9153
      loop
      import custom/*.override
      errors
      log . {
          class error
      }
      forward . /etc/resolv.conf
      cache 30
      reload
      loadbalance round_robin
    }
    import custom/*.server
kind: ConfigMap
metadata:
  creationTimestamp: null
  name: coredns
  namespace: kube-system
`
			return out
		}
		configMapCustomYAML = `apiVersion: v1
data:
  changeme.override: '# checkout the docs on how to use: https://github.com/gardener/gardener/blob/master/docs/usage/networking/custom-dns-config.md'
  changeme.server: '# checkout the docs on how to use: https://github.com/gardener/gardener/blob/master/docs/usage/networking/custom-dns-config.md'
kind: ConfigMap
metadata:
  annotations:
    resources.gardener.cloud/ignore: "true"
  creationTimestamp: null
  name: coredns-custom
  namespace: kube-system
`
		serviceYAML = func(ipfp string) string {
			out := `apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    k8s-app: kube-dns
    kubernetes.io/cluster-service: "true"
    kubernetes.io/name: CoreDNS
  name: kube-dns
  namespace: kube-system
spec:
  clusterIP: ` + clusterIPs[0].String() + `
  clusterIPs:
  - ` + clusterIPs[0].String() + `
  - ` + clusterIPs[1].String() + `
  ipFamilyPolicy: ` + ipfp + `
  ports:
  - name: dns
    port: 53
    protocol: UDP
    targetPort: 8053
  - name: dns-tcp
    port: 53
    protocol: TCP
    targetPort: 8053
  - name: metrics
    port: 9153
    targetPort: 9153
  selector:
    k8s-app: kube-dns
status:
  loadBalancer: {}
`
			return out
		}
		networkPolicyYAML = `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  annotations:
    gardener.cloud/description: Allows CoreDNS to lookup DNS records, talk to the
      API Server. Also allows CoreDNS to be reachable via its service and its metrics
      endpoint.
  creationTimestamp: null
  name: gardener.cloud--allow-dns
  namespace: kube-system
spec:
  egress:
  - ports:
    - port: 443
      protocol: TCP
    - port: 53
      protocol: TCP
    - port: 53
      protocol: UDP
  ingress:
  - from:
    - namespaceSelector: {}
      podSelector: {}
    - ipBlock:
        cidr: ` + nodeNetworkCIDRs[0].String() + `
    - ipBlock:
        cidr: ` + nodeNetworkCIDRs[1].String() + `
    - ipBlock:
        cidr: ` + podNetworkCIDRs[0].String() + `
    - ipBlock:
        cidr: ` + podNetworkCIDRs[1].String() + `
    ports:
    - port: 9153
      protocol: TCP
    - port: 8053
      protocol: TCP
    - port: 8053
      protocol: UDP
  podSelector:
    matchExpressions:
    - key: k8s-app
      operator: In
      values:
      - kube-dns
  policyTypes:
  - Ingress
  - Egress
`
		deploymentYAMLFor = func(apiserverHost string, podAnnotations map[string]string, keepReplicas bool, useHALabel bool) string {
			out := `apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    gardener.cloud/role: system-component
`
			if useHALabel {
				out += `    high-availability-config.resources.gardener.cloud/type: server
`
			}

			out += `    k8s-app: kube-dns
    origin: gardener
  name: coredns
  namespace: kube-system
spec:
`
			if keepReplicas {
				out += `  replicas: 2
`
			}

			out += `  revisionHistoryLimit: 2
  selector:
    matchLabels:
      k8s-app: kube-dns
  strategy:
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
    type: RollingUpdate
  template:
    metadata:
`
			if len(podAnnotations) > 0 {
				out += `      annotations:
`
			}

			for k, v := range podAnnotations {
				out += `        ` + k + `: ` + v + `
`
			}

			out += `      creationTimestamp: null
      labels:
        gardener.cloud/role: system-component
        k8s-app: kube-dns
        origin: gardener
    spec:
      containers:
      - args:
        - -conf
        - /etc/coredns/Corefile
`

			if apiserverHost != "" {
				out += `        env:
        - name: KUBERNETES_SERVICE_HOST
          value: ` + apiserverHost + `
`
			}
			// TODO(marc1404): When updating coredns to v1.13.x check if the NET_BIND_SERVICE capability can be removed.
			out += `        image: ` + image + `
        imagePullPolicy: IfNotPresent
        livenessProbe:
          failureThreshold: 5
          httpGet:
            path: /health
            port: 8080
            scheme: HTTP
          initialDelaySeconds: 60
          successThreshold: 1
          timeoutSeconds: 5
        name: coredns
        ports:
        - containerPort: 8053
          name: dns-udp
          protocol: UDP
        - containerPort: 8053
          name: dns-tcp
          protocol: TCP
        - containerPort: 9153
          name: metrics
          protocol: TCP
        readinessProbe:
          failureThreshold: 1
          httpGet:
            path: /ready
            port: 8181
            scheme: HTTP
          initialDelaySeconds: 30
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 2
        resources:
          limits:
            memory: 1500Mi
          requests:
            cpu: 50m
            memory: 15Mi
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            add:
            - NET_BIND_SERVICE
            drop:
            - all
          readOnlyRootFilesystem: true
        volumeMounts:
        - mountPath: /etc/coredns
          name: config-volume
          readOnly: true
        - mountPath: /etc/coredns/custom
          name: custom-config-volume
          readOnly: true
      dnsPolicy: Default
      priorityClassName: system-cluster-critical
      securityContext:
        fsGroup: 1
        runAsNonRoot: true
        runAsUser: 65534
        seccompProfile:
          type: RuntimeDefault
        supplementalGroups:
        - 1
      serviceAccountName: coredns
      volumes:
      - configMap:
          items:
          - key: Corefile
            path: Corefile
          name: coredns
        name: config-volume
      - configMap:
          defaultMode: 420
          name: coredns-custom
          optional: true
        name: custom-config-volume
status: {}
`
			return out
		}

		pdbYAML = `apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  creationTimestamp: null
  labels:
    k8s-app: kube-dns
  name: coredns
  namespace: kube-system
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      k8s-app: kube-dns
  unhealthyPodEvictionPolicy: AlwaysAllow
status:
  currentHealthy: 0
  desiredHealthy: 0
  disruptionsAllowed: 0
  expectedPods: 0
`

		hpaYAML = `apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  creationTimestamp: null
  labels:
    high-availability-config.resources.gardener.cloud/type: server
  name: coredns
  namespace: kube-system
spec:
  maxReplicas: 5
  metrics:
  - resource:
      name: cpu
      target:
        averageUtilization: 70
        type: Utilization
    type: Resource
  minReplicas: 2
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: coredns
status:
  currentMetrics: null
  desiredReplicas: 0
`

		cpasaYAML = `apiVersion: v1
automountServiceAccountToken: false
kind: ServiceAccount
metadata:
  creationTimestamp: null
  name: coredns-autoscaler
  namespace: kube-system
`
		cpacrYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  name: system:coredns-autoscaler
rules:
- apiGroups:
  - ""
  resources:
  - nodes
  verbs:
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - replicationcontrollers/scale
  verbs:
  - get
  - update
- apiGroups:
  - apps
  resources:
  - deployments/scale
  - replicasets/scale
  verbs:
  - get
  - update
- apiGroups:
  - ""
  resources:
  - configmaps
  verbs:
  - get
  - create
`
		cpacrbYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  name: system:coredns-autoscaler
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:coredns-autoscaler
subjects:
- kind: ServiceAccount
  name: coredns-autoscaler
  namespace: kube-system
`
		cpaDeploymentYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    gardener.cloud/role: system-component
    k8s-app: coredns-autoscaler
    kubernetes.io/cluster-service: "true"
    origin: gardener
  name: coredns-autoscaler
  namespace: kube-system
spec:
  selector:
    matchLabels:
      k8s-app: coredns-autoscaler
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        gardener.cloud/role: system-component
        k8s-app: coredns-autoscaler
        kubernetes.io/cluster-service: "true"
        origin: gardener
    spec:
      containers:
      - command:
        - /cluster-proportional-autoscaler
        - --namespace=kube-system
        - --configmap=coredns-autoscaler
        - --target=deployment/coredns
        - --default-params={"linear":{"coresPerReplica":256,"nodesPerReplica":16,"min":2,"preventSinglePointFailure":true,"includeUnschedulableNodes":true}}
        - --logtostderr=true
        - --v=2
        image: ` + cpaImage + `
        imagePullPolicy: IfNotPresent
        name: autoscaler
        resources:
          limits:
            memory: 70Mi
          requests:
            cpu: 20m
            memory: 10Mi
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - all
          readOnlyRootFilesystem: true
      priorityClassName: system-cluster-critical
      securityContext:
        fsGroup: 65534
        runAsNonRoot: true
        runAsUser: 65534
        seccompProfile:
          type: RuntimeDefault
        supplementalGroups:
        - 65534
      serviceAccountName: coredns-autoscaler
status: {}
`

		cpaDeploymentVpaYAML = `apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  creationTimestamp: null
  name: coredns-autoscaler
  namespace: kube-system
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: '*'
      controlledValues: RequestsOnly
      minAllowed:
        memory: 10Mi
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: coredns-autoscaler
  updatePolicy:
    updateMode: Auto
status: {}
`

		scrapeConfig = &monitoringv1alpha1.ScrapeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "shoot-coredns",
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
						Replacement: ptr.To("coredns"),
						TargetLabel: "job",
					},
					{
						TargetLabel: "type",
						Replacement: ptr.To("shoot"),
					},
					{
						SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_service_name", "__meta_kubernetes_endpoint_port_name"},
						Action:       "keep",
						Regex:        "kube-dns;metrics",
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
					Regex:        `^(coredns_build_info|coredns_cache_entries|coredns_cache_hits_total|coredns_cache_misses_total|coredns_dns_request_duration_seconds_count|coredns_dns_request_duration_seconds_bucket|coredns_dns_requests_total|coredns_dns_responses_total|coredns_proxy_request_duration_seconds_count|coredns_kubernetes_dns_programming_duration_seconds_bucket|coredns_kubernetes_dns_programming_duration_seconds_count|coredns_kubernetes_dns_programming_duration_seconds_sum|process_max_fds|process_open_fds)$`,
				}},
			},
		}

		prometheusRule = &monitoringv1.PrometheusRule{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "shoot-coredns",
				Namespace:       namespace,
				Labels:          map[string]string{"prometheus": "shoot"},
				ResourceVersion: "1",
			},
			Spec: monitoringv1.PrometheusRuleSpec{
				Groups: []monitoringv1.RuleGroup{{
					Name: "coredns.rules",
					Rules: []monitoringv1.Rule{
						{
							Alert: "CoreDNSDown",
							Expr:  intstr.FromString(`absent(up{job="coredns"} == 1)`),
							For:   ptr.To(monitoringv1.Duration("20m")),
							Labels: map[string]string{
								"service":    "kube-dns",
								"severity":   "critical",
								"type":       "shoot",
								"visibility": "all",
							},
							Annotations: map[string]string{
								"description": "CoreDNS could not be found. Cluster DNS resolution will not work.",
								"summary":     "CoreDNS is down",
							},
						},
					},
				}},
			},
		}
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		values = Values{
			ClusterDomain:    clusterDomain,
			ClusterIPs:       clusterIPs,
			Image:            image,
			PodNetworkCIDRs:  podNetworkCIDRs,
			NodeNetworkCIDRs: nodeNetworkCIDRs,
		}

		component = New(c, namespace, values)

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
			cpaEnabled     bool
			vpaEnabled     bool
			commonSuffixes []string
		)
		BeforeEach(func() {
			cpaEnabled = false
			vpaEnabled = false
			commonSuffixes = []string{}
			values.IPFamilies = []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4}
		})

		JustBeforeEach(func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			component = New(c, namespace, values)
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

			var err error
			manifests, err = test.ExtractManifestsFromManagedResourceData(managedResourceSecret.Data)
			Expect(err).NotTo(HaveOccurred())

			if cpaEnabled {
				if vpaEnabled {
					Expect(manifests).To(HaveLen(14))
				} else {
					Expect(manifests).To(HaveLen(13))
				}
			} else {
				Expect(manifests).To(HaveLen(10))
			}

			Expect(manifests).To(ContainElements(
				serviceAccountYAML,
				clusterRoleYAML,
				clusterRoleBindingYAML,
				configMapYAML(commonSuffixes),
				configMapCustomYAML,
				pdbYAML,
				serviceYAML(ipFamilyPolicy(values.IPFamilies)),
				networkPolicyYAML,
			))

			if cpaEnabled {
				Expect(manifests).To(ContainElements(
					cpasaYAML,
					cpacrYAML,
					cpacrbYAML,
					cpaDeploymentYAML,
				))
				if vpaEnabled {
					Expect(manifests).To(ContainElement(cpaDeploymentVpaYAML))
				} else {
					Expect(manifests).NotTo(ContainElement(cpaDeploymentVpaYAML))
				}
			} else {
				Expect(manifests).NotTo(ContainElements(
					cpasaYAML,
					cpacrYAML,
					cpacrbYAML,
					cpaDeploymentYAML,
				))
			}
			if !cpaEnabled {
				Expect(manifests).To(ContainElement(hpaYAML))
			}

			actualScrapeConfig := &monitoringv1alpha1.ScrapeConfig{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(scrapeConfig), actualScrapeConfig)).To(Succeed())
			Expect(actualScrapeConfig).To(DeepEqual(scrapeConfig))

			actualPrometheusRule := &monitoringv1.PrometheusRule{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(prometheusRule), actualPrometheusRule)).To(Succeed())
			Expect(actualPrometheusRule).To(DeepEqual(prometheusRule))

			componenttest.PrometheusRule(prometheusRule, "testdata/shoot-coredns.prometheusrule.test.yaml")
		})

		Context("w/o apiserver host, w/o pod annotations", func() {
			It("should successfully deploy all resources", func() {
				Expect(manifests).To(ContainElement(deploymentYAMLFor("", nil, true, true)))
			})
		})

		Context("w/ apiserver host, w/ pod annotations", func() {
			var (
				apiserverHost  = "apiserver.host"
				podAnnotations = map[string]string{"foo": "bar"}
			)

			BeforeEach(func() {
				values.APIServerHost = &apiserverHost
				values.PodAnnotations = podAnnotations
			})

			It("should successfully deploy all resources", func() {
				Expect(manifests).To(ContainElement(deploymentYAMLFor(apiserverHost, podAnnotations, true, true)))
			})
		})

		Context("w/ rewriting common suffixes enabled", func() {
			BeforeEach(func() {
				commonSuffixes = []string{"gardener.cloud", "github.com"}
				values.SearchPathRewriteCommonSuffixes = commonSuffixes
			})

			It("should successfully deploy all resources", func() {
				Expect(manifests).To(ContainElement(deploymentYAMLFor("", nil, true, true)))
			})

			AfterEach(func() {
				commonSuffixes = []string{}
				values.SearchPathRewriteCommonSuffixes = commonSuffixes
			})
		})

		Context("w/ cluster proportional autoscaler enabled", func() {
			BeforeEach(func() {
				cpaEnabled = true
				values.AutoscalingMode = gardencorev1beta1.CoreDNSAutoscalingModeClusterProportional
				values.ClusterProportionalAutoscalerImage = cpaImage
			})

			Context("w/o apiserver host, w/o pod annotations", func() {
				It("should successfully deploy all resources", func() {
					Expect(manifests).To(ContainElement(deploymentYAMLFor("", nil, false, false)))
				})
			})

			Context("w/ apiserver host, w/ pod annotations", func() {
				var (
					apiserverHost  = "apiserver.host"
					podAnnotations = map[string]string{"foo": "bar"}
				)

				BeforeEach(func() {
					values.APIServerHost = &apiserverHost
					values.PodAnnotations = podAnnotations
				})

				It("should successfully deploy all resources", func() {
					Expect(manifests).To(ContainElement(deploymentYAMLFor(apiserverHost, podAnnotations, false, false)))
				})
			})

			Context("w/ vpa enabled", func() {
				BeforeEach(func() {
					vpaEnabled = true
					values.WantsVerticalPodAutoscaler = true
				})

				Context("w/o apiserver host, w/o pod annotations", func() {
					It("should successfully deploy all resources", func() {
						Expect(manifests).To(ContainElement(deploymentYAMLFor("", nil, false, false)))
					})
				})
			})
		})

		Context("with dual-stack", func() {
			BeforeEach(func() {
				values.IPFamilies = []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4, gardencorev1beta1.IPFamilyIPv6}
			})

			Context("w/o apiserver host, w/o pod annotations", func() {
				It("should successfully deploy all resources", func() {
					Expect(manifests).To(ContainElement(serviceYAML(ipFamilyPolicy(values.IPFamilies))))
				})
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			scrapeConfig.ResourceVersion = ""
			prometheusRule.ResourceVersion = ""

			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())
			Expect(c.Create(ctx, scrapeConfig)).To(Succeed())
			Expect(c.Create(ctx, prometheusRule)).To(Succeed())

			Expect(component.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(scrapeConfig), scrapeConfig)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(prometheusRule), prometheusRule)).To(BeNotFoundError())
		})
	})

	Context("waiting functions", func() {
		var (
			fakeOps   *retryfake.Ops
			resetVars func()
		)

		BeforeEach(func() {
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

func ipFamilyPolicy(ipfamilies []gardencorev1beta1.IPFamily) string {
	ipFamiliesSet := sets.New[gardencorev1beta1.IPFamily](ipfamilies...)
	if ipFamiliesSet.Has(gardencorev1beta1.IPFamilyIPv4) && ipFamiliesSet.Has(gardencorev1beta1.IPFamilyIPv6) {
		return string(corev1.IPFamilyPolicyPreferDualStack)
	}
	return string(corev1.IPFamilyPolicySingleStack)
}
