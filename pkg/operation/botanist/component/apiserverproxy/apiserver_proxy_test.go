// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package apiserverproxy_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/apiserverproxy"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("apiserver-proxy", func() {
	var (
		ctx = context.TODO()

		c  client.Client
		sm secretsmanager.Interface

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret

		values    Values
		component Interface

		managedResourceName = "shoot-core-apiserver-proxy"
		namespace           = "some-namespace"
		image               = "some-image:some-tag"
		sidecarImage        = "sidecar-image:some-tag"
		advertiseIPAddress  = "10.2.170.21"
		adminPort           = 16910
		proxySeedServerHost = "api.internal.local."
		proxySeedServerPort = "8443"

		configMapYAML = `apiVersion: v1
data:
  envoy.yaml: |
    layered_runtime:
      layers:
        - name: static_layer_0
          static_layer:
            envoy:
              resource_limits:
                listener:
                  kube_apiserver:
                    connection_limit: 10000
            overload:
              global_downstream_max_connections: 10000
    admin:
      access_log:
      - name: envoy.access_loggers.stdout
        # Remove spammy readiness/liveness probes and metrics requests from access log
        filter:
          and_filter:
            filters:
            - header_filter:
                header:
                  name: :Path
                  string_match:
                    exact: /ready
                  invert_match: true
            - header_filter:
                header:
                  name: :Path
                  string_match:
                    exact: /stats/prometheus
                  invert_match: true
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.access_loggers.stream.v3.StdoutAccessLog
      address:
        pipe:
          # The admin interface should not be exposed as a TCP address.
          # It's only used and exposed via the metrics lister that
          # exposes only /stats/prometheus path for metrics scrape.
          path: /etc/admin-uds/admin.socket
    static_resources:
      listeners:
      - name: kube_apiserver
        address:
          socket_address:
            address: 10.2.170.21
            port_value: 443
        per_connection_buffer_limit_bytes: 32768 # 32 KiB
        filter_chains:
        - filters:
          - name: envoy.filters.network.tcp_proxy
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
              stat_prefix: kube_apiserver
              cluster: kube_apiserver
              access_log:
              - name: envoy.access_loggers.stdout
                typed_config:
                  "@type": type.googleapis.com/envoy.extensions.access_loggers.stream.v3.StdoutAccessLog
                  log_format:
                    text_format_source:
                      inline_string: "[%START_TIME%] %RESPONSE_CODE% %RESPONSE_FLAGS% %BYTES_RECEIVED% rx %BYTES_SENT% tx %DURATION%ms \"%DOWNSTREAM_REMOTE_ADDRESS%\" \"%UPSTREAM_HOST%\"\n"
      - name: metrics
        address:
          socket_address:
            address: 0.0.0.0
            port_value: 16910
        filter_chains:
        - filters:
          - name: envoy.filters.network.http_connection_manager
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
              stat_prefix: ingress_http
              use_remote_address: true
              common_http_protocol_options:
                idle_timeout: 8s
                max_connection_duration: 10s
                max_headers_count: 20
                max_stream_duration: 8s
                headers_with_underscores_action: REJECT_REQUEST
              http2_protocol_options:
                max_concurrent_streams: 5
                initial_stream_window_size: 65536
                initial_connection_window_size: 1048576
              stream_idle_timeout: 8s
              request_timeout: 9s
              codec_type: AUTO
              route_config:
                name: local_route
                virtual_hosts:
                - name: local_service
                  domains: ["*"]
                  routes:
                  - match:
                      path: /metrics
                    route:
                      cluster: uds_admin
                      prefix_rewrite: /stats/prometheus
                  - match:
                      path: /ready
                    route:
                      cluster: uds_admin
              http_filters:
              - name: envoy.filters.http.router
                typed_config:
                  "@type": type.googleapis.com/envoy.extensions.filters.http.router.v3.Router

      clusters:
      - name: kube_apiserver
        connect_timeout: 5s
        per_connection_buffer_limit_bytes: 32768 # 32 KiB
        type: LOGICAL_DNS
        dns_lookup_family: V4_ONLY
        lb_policy: ROUND_ROBIN
        load_assignment:
          cluster_name: kube_apiserver
          endpoints:
          - lb_endpoints:
            - endpoint:
                address:
                  socket_address:
                    address: api.internal.local.
                    port_value: 8443
        transport_socket:
          name: envoy.transport_sockets.upstream_proxy_protocol
          typed_config:
            "@type": type.googleapis.com/envoy.extensions.transport_sockets.proxy_protocol.v3.ProxyProtocolUpstreamTransport
            config:
              version: V2
            transport_socket:
              name: envoy.transport_sockets.raw_buffer
              typed_config:
                "@type": type.googleapis.com/envoy.extensions.transport_sockets.raw_buffer.v3.RawBuffer
        upstream_connection_options:
          tcp_keepalive:
            keepalive_time: 7200
            keepalive_interval: 55
      - name: uds_admin
        connect_timeout: 0.25s
        type: STATIC
        lb_policy: ROUND_ROBIN
        load_assignment:
          cluster_name: uds_admin
          endpoints:
          - lb_endpoints:
              - endpoint:
                  address:
                    pipe:
                      path: /etc/admin-uds/admin.socket
        transport_socket:
          name: envoy.transport_sockets.raw_buffer
          typed_config:
            "@type": type.googleapis.com/envoy.extensions.transport_sockets.raw_buffer.v3.RawBuffer
immutable: true
kind: ConfigMap
metadata:
  creationTimestamp: null
  labels:
    app: kubernetes
    gardener.cloud/role: system-component
    origin: gardener
    resources.gardener.cloud/garbage-collectable-reference: "true"
    role: apiserver-proxy
  name: apiserver-proxy-config-4baf1826
  namespace: kube-system
`

		daemonSetWithPSPYAML = `apiVersion: apps/v1
kind: DaemonSet
metadata:
  creationTimestamp: null
  labels:
    app: kubernetes
    gardener.cloud/role: system-component
    node.gardener.cloud/critical-component: "true"
    origin: gardener
    role: apiserver-proxy
  name: apiserver-proxy
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: kubernetes
      role: apiserver-proxy
  template:
    metadata:
      annotations:
        checksum/psp: 88e942e106221df42adea87d136a5c95ea8f21c368e7753c27fd8ae27e218e93
        reference.resources.gardener.cloud/configmap-12f893dc: apiserver-proxy-config-4baf1826
      creationTimestamp: null
      labels:
        app: kubernetes
        gardener.cloud/role: system-component
        networking.gardener.cloud/from-seed: allowed
        networking.gardener.cloud/to-apiserver: allowed
        networking.gardener.cloud/to-dns: allowed
        node.gardener.cloud/critical-component: "true"
        origin: gardener
        role: apiserver-proxy
    spec:
      automountServiceAccountToken: false
      containers:
      - args:
        - --ip-address=10.2.170.21
        - --setup-iptables=false
        - --interface=lo
        image: sidecar-image:some-tag
        imagePullPolicy: IfNotPresent
        name: sidecar
        resources:
          limits:
            memory: 90Mi
          requests:
            cpu: 20m
            memory: 20Mi
        securityContext:
          capabilities:
            add:
            - NET_ADMIN
      - command:
        - envoy
        - --concurrency
        - "2"
        - --use-dynamic-base-id
        - -c
        - /etc/apiserver-proxy/envoy.yaml
        image: some-image:some-tag
        imagePullPolicy: IfNotPresent
        livenessProbe:
          failureThreshold: 3
          httpGet:
            path: /ready
            port: 16910
          initialDelaySeconds: 1
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 1
        name: proxy
        ports:
        - containerPort: 16910
          hostPort: 16910
          name: metrics
        readinessProbe:
          httpGet:
            path: /ready
            port: 16910
          initialDelaySeconds: 1
          periodSeconds: 2
          successThreshold: 1
          timeoutSeconds: 1
        resources:
          limits:
            memory: 1Gi
          requests:
            cpu: 20m
            memory: 20Mi
        securityContext:
          capabilities:
            add:
            - NET_BIND_SERVICE
          runAsUser: 0
        volumeMounts:
        - mountPath: /etc/apiserver-proxy
          name: proxy-config
        - mountPath: /etc/admin-uds
          name: admin-uds
      hostNetwork: true
      initContainers:
      - args:
        - --ip-address=10.2.170.21
        - --setup-iptables=false
        - --daemon=false
        - --interface=lo
        image: sidecar-image:some-tag
        imagePullPolicy: IfNotPresent
        name: setup
        resources:
          limits:
            memory: 200Mi
          requests:
            cpu: 20m
            memory: 20Mi
        securityContext:
          capabilities:
            add:
            - NET_ADMIN
      priorityClassName: system-node-critical
      securityContext:
        seccompProfile:
          type: RuntimeDefault
      serviceAccountName: apiserver-proxy
      tolerations:
      - effect: NoSchedule
        operator: Exists
      - effect: NoExecute
        operator: Exists
      volumes:
      - configMap:
          name: apiserver-proxy-config-4baf1826
        name: proxy-config
      - emptyDir: {}
        name: admin-uds
  updateStrategy:
    type: RollingUpdate
status:
  currentNumberScheduled: 0
  desiredNumberScheduled: 0
  numberMisscheduled: 0
  numberReady: 0
`

		daemonSetYAML = `apiVersion: apps/v1
kind: DaemonSet
metadata:
  creationTimestamp: null
  labels:
    app: kubernetes
    gardener.cloud/role: system-component
    node.gardener.cloud/critical-component: "true"
    origin: gardener
    role: apiserver-proxy
  name: apiserver-proxy
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: kubernetes
      role: apiserver-proxy
  template:
    metadata:
      annotations:
        reference.resources.gardener.cloud/configmap-12f893dc: apiserver-proxy-config-4baf1826
      creationTimestamp: null
      labels:
        app: kubernetes
        gardener.cloud/role: system-component
        networking.gardener.cloud/from-seed: allowed
        networking.gardener.cloud/to-apiserver: allowed
        networking.gardener.cloud/to-dns: allowed
        node.gardener.cloud/critical-component: "true"
        origin: gardener
        role: apiserver-proxy
    spec:
      automountServiceAccountToken: false
      containers:
      - args:
        - --ip-address=10.2.170.21
        - --setup-iptables=false
        - --interface=lo
        image: sidecar-image:some-tag
        imagePullPolicy: IfNotPresent
        name: sidecar
        resources:
          limits:
            memory: 90Mi
          requests:
            cpu: 20m
            memory: 20Mi
        securityContext:
          capabilities:
            add:
            - NET_ADMIN
      - command:
        - envoy
        - --concurrency
        - "2"
        - --use-dynamic-base-id
        - -c
        - /etc/apiserver-proxy/envoy.yaml
        image: some-image:some-tag
        imagePullPolicy: IfNotPresent
        livenessProbe:
          failureThreshold: 3
          httpGet:
            path: /ready
            port: 16910
          initialDelaySeconds: 1
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 1
        name: proxy
        ports:
        - containerPort: 16910
          hostPort: 16910
          name: metrics
        readinessProbe:
          httpGet:
            path: /ready
            port: 16910
          initialDelaySeconds: 1
          periodSeconds: 2
          successThreshold: 1
          timeoutSeconds: 1
        resources:
          limits:
            memory: 1Gi
          requests:
            cpu: 20m
            memory: 20Mi
        securityContext:
          capabilities:
            add:
            - NET_BIND_SERVICE
          runAsUser: 0
        volumeMounts:
        - mountPath: /etc/apiserver-proxy
          name: proxy-config
        - mountPath: /etc/admin-uds
          name: admin-uds
      hostNetwork: true
      initContainers:
      - args:
        - --ip-address=10.2.170.21
        - --setup-iptables=false
        - --daemon=false
        - --interface=lo
        image: sidecar-image:some-tag
        imagePullPolicy: IfNotPresent
        name: setup
        resources:
          limits:
            memory: 200Mi
          requests:
            cpu: 20m
            memory: 20Mi
        securityContext:
          capabilities:
            add:
            - NET_ADMIN
      priorityClassName: system-node-critical
      securityContext:
        seccompProfile:
          type: RuntimeDefault
      serviceAccountName: apiserver-proxy
      tolerations:
      - effect: NoSchedule
        operator: Exists
      - effect: NoExecute
        operator: Exists
      volumes:
      - configMap:
          name: apiserver-proxy-config-4baf1826
        name: proxy-config
      - emptyDir: {}
        name: admin-uds
  updateStrategy:
    type: RollingUpdate
status:
  currentNumberScheduled: 0
  desiredNumberScheduled: 0
  numberMisscheduled: 0
  numberReady: 0
`

		serviceYAML = `apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    app: kubernetes
    gardener.cloud/role: system-component
    origin: gardener
    role: apiserver-proxy
  name: apiserver-proxy
  namespace: kube-system
spec:
  clusterIP: None
  ports:
  - name: metrics
    port: 16910
    protocol: TCP
    targetPort: 16910
  selector:
    app: kubernetes
    role: apiserver-proxy
  type: ClusterIP
status:
  loadBalancer: {}
`

		serviceAccountYAML = `apiVersion: v1
automountServiceAccountToken: false
kind: ServiceAccount
metadata:
  creationTimestamp: null
  labels:
    app: kubernetes
    gardener.cloud/role: system-component
    origin: gardener
    role: apiserver-proxy
  name: apiserver-proxy
  namespace: kube-system
`

		webhokkConfigYAML = `apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  annotations:
    networking.gardener.cloud/description: |-
      This webhook adds KUBERNETES_SERVICE_HOST
      environment variable to all containers and init containers matched by it.
  creationTimestamp: null
  labels:
    app: kubernetes
    gardener.cloud/role: system-component
    origin: gardener
    remediation.webhook.shoot.gardener.cloud/exclude: "true"
    resources.gardener.cloud/garbage-collectable-reference: "true"
    role: apiserver-proxy
  name: apiserver-proxy.networking.gardener.cloud
webhooks:
- admissionReviewVersions:
  - v1beta1
  clientConfig:
    caBundle: Rk9PQkFS
    url: https://127.0.0.1:9443/webhook/pod-apiserver-env
  failurePolicy: Ignore
  matchPolicy: Exact
  name: apiserver-proxy.networking.gardener.cloud
  namespaceSelector:
    matchExpressions:
    - key: apiserver-proxy.networking.gardener.cloud/inject
      operator: NotIn
      values:
      - disable
  objectSelector:
    matchExpressions:
    - key: apiserver-proxy.networking.gardener.cloud/inject
      operator: NotIn
      values:
      - disable
  reinvocationPolicy: Never
  rules:
  - apiGroups:
    - ""
    apiVersions:
    - v1
    operations:
    - CREATE
    resources:
    - pods
    scope: '*'
  sideEffects: None
  timeoutSeconds: 2
`

		clusterRoleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  labels:
    app: kubernetes
    gardener.cloud/role: system-component
    origin: gardener
    role: apiserver-proxy
  name: gardener.cloud:psp:kube-system:apiserver-proxy
rules:
- apiGroups:
  - policy
  - extensions
  resourceNames:
  - gardener.kube-system.apiserver-proxy
  resources:
  - podsecuritypolicies
  verbs:
  - use
`

		pspYAML = `apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  annotations:
    seccomp.security.alpha.kubernetes.io/allowedProfileNames: runtime/default
    seccomp.security.alpha.kubernetes.io/defaultProfileName: runtime/default
  creationTimestamp: null
  labels:
    app: kubernetes
    gardener.cloud/role: system-component
    origin: gardener
    role: apiserver-proxy
  name: gardener.kube-system.apiserver-proxy
spec:
  allowedCapabilities:
  - NET_ADMIN
  - NET_BIND_SERVICE
  fsGroup:
    rule: RunAsAny
  hostNetwork: true
  hostPorts:
  - max: 443
    min: 443
  - max: 16910
    min: 16910
  runAsUser:
    rule: RunAsAny
  seLinux:
    rule: RunAsAny
  supplementalGroups:
    rule: RunAsAny
  volumes:
  - secret
  - configMap
  - emptyDir
`

		roleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  annotations:
    resources.gardener.cloud/delete-on-invalid-update: "true"
  creationTimestamp: null
  labels:
    app: kubernetes
    gardener.cloud/role: system-component
    origin: gardener
    role: apiserver-proxy
  name: gardener.cloud:psp:apiserver-proxy
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gardener.cloud:psp:kube-system:apiserver-proxy
subjects:
- kind: ServiceAccount
  name: apiserver-proxy
  namespace: kube-system
`
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(c, namespace)

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: namespace}, Data: map[string][]byte{"bundle.crt": []byte("FOOBAR")}})).To(Succeed())

		values = Values{
			APIServerProxyImage:        image,
			APIServerProxySidecarImage: sidecarImage,
			AdminPort:                  int32(adminPort),
			ProxySeedServerHost:        proxySeedServerHost,
			ProxySeedServerPort:        proxySeedServerPort,
		}

		component = New(c, namespace, sm, values)
		component.SetAdvertiseIPAddress(advertiseIPAddress)

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
		test := func(podMutatorEnabled, pspDisabled bool) {
			By("Verify that managed resource does not exist yet")
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceSecret.Name)))

			By("Deploy the managed resource successfully")
			values.PodMutatorEnabled = podMutatorEnabled
			values.PSPDisabled = pspDisabled
			component = New(c, namespace, sm, values)
			component.SetAdvertiseIPAddress(advertiseIPAddress)
			Expect(component.Deploy(ctx)).To(Succeed())

			By("Verify that managed resource is consistent")
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(managedResource).To(DeepEqual(&resourcesv1alpha1.ManagedResource{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ManagedResource",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResource.Name,
					Namespace:       managedResource.Namespace,
					ResourceVersion: "1",
					Labels:          map[string]string{"origin": "gardener"},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResourceSecret.Name,
					}},
					KeepObjects: pointer.Bool(false),
				},
			}))

			By("Verify that referenced secret is consistent")
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			expectedLen := 4
			if pspDisabled {
				Expect(string(managedResourceSecret.Data["daemonset__kube-system__apiserver-proxy.yaml"])).To(Equal(daemonSetYAML))
			} else {
				expectedLen += 3
				Expect(string(managedResourceSecret.Data["daemonset__kube-system__apiserver-proxy.yaml"])).To(Equal(daemonSetWithPSPYAML))
				Expect(string(managedResourceSecret.Data["clusterrole____gardener.cloud_psp_kube-system_apiserver-proxy.yaml"])).To(Equal(clusterRoleYAML))
				Expect(string(managedResourceSecret.Data["podsecuritypolicy____gardener.kube-system.apiserver-proxy.yaml"])).To(Equal(pspYAML))
				Expect(string(managedResourceSecret.Data["rolebinding__kube-system__gardener.cloud_psp_apiserver-proxy.yaml"])).To(Equal(roleBindingYAML))
			}
			if podMutatorEnabled {
				expectedLen++
				Expect(string(managedResourceSecret.Data["mutatingwebhookconfiguration____apiserver-proxy.networking.gardener.cloud.yaml"])).To(Equal(webhokkConfigYAML))
			}
			Expect(managedResourceSecret.Data).To(HaveLen(expectedLen))
			Expect(string(managedResourceSecret.Data["configmap__kube-system__apiserver-proxy-config-4baf1826.yaml"])).To(Equal(configMapYAML))
			Expect(string(managedResourceSecret.Data["service__kube-system__apiserver-proxy.yaml"])).To(Equal(serviceYAML))
			Expect(string(managedResourceSecret.Data["serviceaccount__kube-system__apiserver-proxy.yaml"])).To(Equal(serviceAccountYAML))
		}

		Context("Pod mutator disabled, PSP disabled", func() {
			It("should deploy the managed resource successfully", func() {
				test(false, true)
			})
		})

		Context("Pod mutator enabled, PSP disabled", func() {
			It("should deploy the managed resource successfully", func() {
				test(true, true)
			})
		})

		Context("Pod mutator enabled, PSP enabled", func() {
			It("should deploy the managed resource successfully", func() {
				test(true, false)
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully delete all the resources", func() {
			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

			Expect(component.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceSecret.Name)))
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
