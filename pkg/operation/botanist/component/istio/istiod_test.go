// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package istio_test

import (
	"context"
	"path/filepath"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	cr "github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/istio"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("istiod", func() {
	const (
		deployNS        = "test"
		deployNSIngress = "test-ingress"
	)

	var (
		ctx                   context.Context
		c                     client.Client
		istiod                component.DeployWaiter
		igw                   []*IngressGateway
		ipp                   []*IstioProxyProtocol
		igwAnnotations        map[string]string
		labels                map[string]string
		managedResourceName   string
		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret

		istiodService = `apiVersion: v1
kind: Service
metadata:
  name: istiod
  namespace: test
  labels:
    app: istiod
    istio: pilot
    
spec:
  type: ClusterIP
  ports:
  - name: https-sds # mTLS with k8s-signed cert
    port: 15012
    protocol: TCP
  - name: https-webhook # validation and injection
    port: 443
    protocol: TCP
    targetPort: 10250
  - name: metrics # prometheus metrics
    port: 15014
    protocol: TCP
    targetPort: 15014
  selector:
    app: istiod
    istio: pilot
    
`
		istioClusterRole = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: istiod
  labels:
    app: istiod
    istio: pilot
    
rules:
# sidecar injection controller Do we need it?
- apiGroups:
  - admissionregistration.k8s.io
  resources:
  - mutatingwebhookconfigurations
  verbs:
  - get
  - list
  - watch
  - update
  - patch
# configuration validation webhook controller
- apiGroups:
  - admissionregistration.k8s.io
  resources:
  - validatingwebhookconfigurations
  verbs:
  - get
  - list
  - watch
  - update
# istio configuration
- apiGroups:
  - config.istio.io
  - security.istio.io
  - networking.istio.io
  - authentication.istio.io
  - rbac.istio.io
  - telemetry.istio.io
  - extensions.istio.io
  verbs:
  - get
  - watch
  - list
  resources:
  - "*"
- apiGroups:
  - networking.istio.io
  verbs:
  - get
  - watch
  - list
  - update
  - patch
  - create
  - delete
  resources:
  - workloadentries
- apiGroups:
  - networking.istio.io
  verbs:
  - get
  - watch
  - list
  - update
  - patch
  - create
  - delete
  resources:
  - workloadentries/status

# auto-detect installed CRD definitions
- apiGroups:
  - apiextensions.k8s.io
  resources:
  - customresourcedefinitions
  verbs:
  - get
  - list
  - watch
# discovery and routing
- apiGroups:
  - ''
  resources:
  - pods
  - nodes
  - services
  - namespaces
  - endpoints
  verbs:
  - get
  - list
  - watch
# discovery and routing
- apiGroups:
  - discovery.k8s.io
  resources:
  - endpointslices
  verbs:
  - get
  - list
  - watch
# ingress controller TODO: is this needed???
- apiGroups:
  - extensions
  resources:
  - ingresses
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - extensions
  resources:
  - ingresses/status
  verbs:
  - "*"
- apiGroups:
  - networking.k8s.io
  resources:
  - ingresses
  - ingressclasses
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - networking.k8s.io
  resources:
  - ingresses/status
  verbs:
  - "*"
# required for CA's namespace controller
- apiGroups:
  - ''
  resources:
  - configmaps
  verbs:
  - create
  - get
  - list
  - watch
  - update
# Istiod and bootstrap.
- apiGroups:
  - certificates.k8s.io
  resources:
  - certificatesigningrequests
  - certificatesigningrequests/approval
  - certificatesigningrequests/status
  verbs:
  - update
  - create
  - get
  - delete
  - watch
- apiGroups:
  - certificates.k8s.io
  resources:
  - signers
  resourceNames:
  - kubernetes.io/legacy-unknown
  verbs:
  - approve
# Used by Istiod to verify the JWT tokens
- apiGroups:
  - authentication.k8s.io
  resources:
  - tokenreviews
  verbs:
  - create
# Used by Istiod to verify gateway SDS
- apiGroups:
  - authorization.k8s.io
  resources:
  - subjectaccessreviews
  verbs:
  - create
# Use for Kubernetes Service APIs
- apiGroups:
  - networking.x-k8s.io
  - gateway.networking.k8s.io
  resources:
  - "*"
  verbs:
  - get
  - watch
  - list
- apiGroups:
  - networking.x-k8s.io
  - gateway.networking.k8s.io
  resources:
  - "*" # TODO: should be on just */status but wildcard is not supported
  verbs:
  - update
  - patch
# Needed for multicluster secret reading, possibly ingress certs in the future
- apiGroups:
  - ''
  resources:
  - secrets
  verbs:
  - get
  - watch
  - list
- apiGroups:
  - extensions.istio.io
  resources:
  - wasmplugins
  verbs:
  - get
  - watch
  - list

# Used for MCS serviceexport management
- apiGroups:
  - multicluster.x-k8s.io
  resources:
  - serviceexports
  verbs:
  - get
  - watch
  - list
  - create
  - delete

# Used for MCS serviceimport management
- apiGroups:
  - multicluster.x-k8s.io
  resources:
  - serviceimports
  verbs:
  - get
  - watch
  - list
`

		istiodClusterRoleBinding = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: istiod
  labels:
    app: istiod
    istio: pilot
    
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: istiod
subjects:
- kind: ServiceAccount
  name: istiod
  namespace: ` + deployNS + `
`

		istiodDestinationRule = `apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: default
  namespace: ` + deployNS + `
spec:
  host: "*"
  exportTo:
  - "*"
  trafficPolicy:
    tls:
      mode: ISTIO_MUTUAL
`

		istiodPeerAuthentication = `apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata:
  name: default
  namespace: ` + deployNS + `
spec:
  mtls:
    mode: STRICT
`

		istiodPodDisruptionBudget = `apiVersion: policy/v1beta1
kind: PodDisruptionBudget
metadata:
  name: istiod
  namespace: ` + deployNS + `
  labels:
    app: istiod
    istio: pilot
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: istiod
      istio: pilot
`

		istiodPriorityClass = `---
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: istio
value: 1000000000
globalDefault: false
description: "This class is used to ensure that istiod has a high priority and is not preempted in favor of other pods."`

		istiodRole = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: istiod
  namespace: ` + deployNS + `
  labels:
    app: istiod
    istio: pilot
    
rules:
# permissions to verify the webhook is ready and rejecting
# invalid config. We use --server-dry-run so no config is persisted.
- apiGroups:
  - networking.istio.io
  verbs:
  - create
  resources:
  - gateways

# For storing CA secret
- apiGroups:
  - ''
  resources:
  - secrets
  verbs:
  - create
  - get
  - watch
  - list
  - update
  - delete
- apiGroups:
  - ''
  resources:
  - serviceaccounts
  verbs:
  - get
  - watch
  - list
`

		istiodRoleBinding = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: istiod
  namespace: ` + deployNS + `
  labels:
    app: istiod
    istio: pilot
    
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: istiod
subjects:
- kind: ServiceAccount
  name: istiod
`

		istiodServiceAccount = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: istiod
  namespace: ` + deployNS + `
  labels:
    app: istiod
    istio: pilot
    
automountServiceAccountToken: false
`

		istiodSidecar = `apiVersion: networking.istio.io/v1alpha3
kind: Sidecar
metadata:
  name: default
  namespace: ` + deployNS + `
  labels:
    app: istiod
    istio: pilot
    
spec:
  egress:
  - hosts:
    - "*/*"
  outboundTrafficPolicy:
    mode: REGISTRY_ONLY
`
		istiodAutoscale = `apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: istiod
  namespace: ` + deployNS + `
  labels:
    app: istiod
    istio: pilot
    
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: istiod
  updatePolicy:
    updateMode: Auto
  resourcePolicy:
    containerPolicies:
      - containerName: discovery
        minAllowed:
          memory: 128Mi
          cpu: 100m
`

		istiodValidationWebhook = `apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: istiod
  labels:
    # The istio revision is required so that the web hook is found at runtime for the caBundle update
    # Currently, we do not set the istio revision. Hence, it is just empty.
    istio.io/rev: ""
    app: istiod
    istio: pilot
    
webhooks:
  - name: validation.istio.io
    admissionReviewVersions:  ["v1", "v1beta1"]
    timeoutSeconds: 10
    clientConfig:
      service:
        name: istiod
        namespace: ` + deployNS + `
        path: "/validate"
      caBundle: "" # patched at runtime when the webhook is ready.
    rules:
      - operations:
        - CREATE
        - UPDATE
        apiGroups:
        - config.istio.io
        - rbac.istio.io
        - security.istio.io
        - authentication.istio.io
        - networking.istio.io
        apiVersions:
        - "*"
        resources:
        - "*"
    # Fail open until the validation webhook is ready. The webhook controller
    # will update this to ` + "`" + `Fail` + "`" + ` and patch in the ` + "`" + `caBundle` + "`" + ` when the webhook
    # endpoint is ready.
    failurePolicy: Ignore
    matchPolicy: Exact
    sideEffects: None
`

		istiodConfigMap = `apiVersion: v1
kind: ConfigMap
metadata:
  name: istio
  namespace: ` + deployNS + `
  labels:
    app: istiod
    istio: pilot
    
data:

  # Configuration file for the mesh networks to be used by the Split Horizon EDS.
  meshNetworks: |-
    networks: {}

  mesh: |-
    # Set enableTracing to false to disable request tracing.
    enableTracing: false

    # Set accessLogFile to empty string to disable access log.
    accessLogFile: ""

    accessLogFormat: ""

    accessLogEncoding: 'TEXT'

    enableEnvoyAccessLogService: false
    # reportBatchMaxEntries is the number of requests that are batched before telemetry data is sent to the mixer server
    reportBatchMaxEntries: 100
    # reportBatchMaxTime is the max waiting time before the telemetry data of a request is sent to the mixer server
    reportBatchMaxTime: 1s
    disableMixerHttpReports: false

    # Set the following variable to true to disable policy checks by the Mixer.
    # Note that metrics will still be reported to the Mixer.
    disablePolicyChecks: true

    # Automatic protocol detection uses a set of heuristics to
    # determine whether the connection is using TLS or not (on the
    # server side), as well as the application protocol being used
    # (e.g., http vs tcp). These heuristics rely on the client sending
    # the first bits of data. For server first protocols like MySQL,
    # MongoDB, etc., Envoy will timeout on the protocol detection after
    # the specified period, defaulting to non mTLS plain TCP
    # traffic. Set this field to tweak the period that Envoy will wait
    # for the client to send the first bits of data. (MUST BE >=1ms)
    protocolDetectionTimeout: 100ms

    # This is the k8s ingress service name, not used.
    ingressService: "istio-ingressgateway"
    ingressControllerMode: "OFF"
    ingressClass: "istio"

    # The trust domain corresponds to the trust root of a system.
    # Refer to https://github.com/spiffe/spiffe/blob/master/standards/SPIFFE-ID.md#21-trust-domain
    trustDomain: "foo.local"

    # Used by pilot-agent
    sdsUdsPath: "unix:/etc/istio/proxy/SDS"

    # If true, automatically configure client side mTLS settings to match the corresponding service's
    # server side mTLS authentication policy, when destination rule for that service does not specify
    # TLS settings.
    enableAutoMtls: true

    outboundTrafficPolicy:
      # allow traffic only to services in the mesh
      mode: REGISTRY_ONLY
    localityLbSetting:
      enabled: true

    # Configures DNS certificates provisioned through Chiron linked into Pilot.
    # The DNS certificate provisioning is enabled by default now so it get tested.
    certificates: []

    # Disable the advertisment of services and endpoints which are no explictly marked in
    # ` + "`" + `exportTo` + "`" + `. Improves security and isolation.
    # Refer to https://istio.io/docs/reference/config/istio.mesh.v1alpha1/#MeshConfig
    defaultServiceExportTo: ["-"]
    defaultVirtualServiceExportTo: ["-"]
    defaultDestinationRuleExportTo: ["-"]

    defaultConfig:
      #
      # TCP connection timeout between Envoy & the application, and between Envoys.
      connectTimeout: 10s
      #
      ### ADVANCED SETTINGS #############
      # Where should envoy's configuration be stored in the istio-proxy container
      configPath: "/etc/istio/proxy"
      # The pseudo service name used for Envoy.
      serviceCluster: istio-proxy
      # These settings that determine how long an old Envoy
      # process should be kept alive after an occasional reload.
      drainDuration: 45s
      parentShutdownDuration: 1m0s
      #
      # Port where Envoy listens (on local host) for admin commands
      # You can exec into the istio-proxy container in a pod and
      # curl the admin port (curl http://localhost:15000/) to obtain
      # diagnostic information from Envoy. See
      # https://lyft.github.io/envoy/docs/operations/admin.html
      # for more details
      proxyAdminPort: 15000
      #
      # Set concurrency to a specific number to control the number of Proxy worker threads.
      # If set to 0 (default), then start worker thread for each CPU thread/core.
      concurrency: 2
      #
      # If port is 15012, will use SDS.
      # controlPlaneAuthPolicy is for mounted secrets, will wait for the files.
      controlPlaneAuthPolicy: NONE
      discoveryAddress: istiod.` + deployNS + `.svc:15012

    rootNamespace: istio-system
    enablePrometheusMerge: true
`

		istiodDeployment = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: istiod
  namespace: ` + deployNS + `
  labels:
    app: istiod
    istio: pilot
    
spec:
  replicas: 2
  revisionHistoryLimit: 1
  strategy:
    rollingUpdate:
      maxSurge: 100%
      maxUnavailable: 25%
  selector:
    matchLabels:
      app: istiod
      istio: pilot
      
  template:
    metadata:
      labels:
        app: istiod
        istio: pilot
        
      annotations:
        sidecar.istio.io/inject: "false"
        checksum/istio-config: 8af0ee1ba7d53be8bcb9cda04e3f601a771448f2a460e6455dc5710c1e753f43
    spec:
      serviceAccountName: istiod
      securityContext:
        fsGroup: 1337
      containers:
      - name: discovery
        image: "foo/bar"
        imagePullPolicy: IfNotPresent
        args:
        - "discovery"
        - --monitoringAddr=:15014
        - --grpcAddr=
        - --httpsAddr=:10250
        - --log_output_level=all:warn,ads:error
        - --domain=foo.local
        - --plugins=authn,authz,health # remove mixer plugin
        - --keepaliveMaxServerConnectionAge=30m
        ports:
        - containerPort: 15012
          protocol: TCP
        - containerPort: 10250
          protocol: TCP
        - containerPort: 8080
          protocol: TCP
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 1
          periodSeconds: 3
          timeoutSeconds: 5
        env:
        - name: JWT_POLICY
          value: third-party-jwt
        - name: PILOT_CERT_PROVIDER
          value: istiod
        - name: POD_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
        - name: SERVICE_ACCOUNT
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.serviceAccountName
        - name: PILOT_TRACE_SAMPLING
          value: "0.1"
        - name: PILOT_ENABLE_PROTOCOL_SNIFFING_FOR_OUTBOUND
          value: "false"
        - name: PILOT_ENABLE_PROTOCOL_SNIFFING_FOR_INBOUND
          value: "false"
        - name: INJECTION_WEBHOOK_CONFIG_NAME
          value: istio-sidecar-injector
        - name: ISTIOD_ADDR
          value: istiod.` + deployNS + `.svc:15012
        - name: VALIDATION_WEBHOOK_CONFIG_NAME
          value: istiod
        - name: PILOT_EXTERNAL_GALLEY
          value: "false"
        - name: CLUSTER_ID
          value: "Kubernetes"
        - name: EXTERNAL_ISTIOD
          value: "false"
        - name: PILOT_ENDPOINT_TELEMETRY_LABEL
          value: "true"
        resources:
          requests:
            cpu: 250m
            memory: 256Mi
        securityContext:
          capabilities:
            drop:
            - ALL
          runAsGroup: 1337
          runAsNonRoot: true
          runAsUser: 1337
        volumeMounts:
        - name: config-volume
          mountPath: /etc/istio/config
        - name: istio-token
          mountPath: /var/run/secrets/tokens
          readOnly: true
        - name: local-certs
          mountPath: /var/run/secrets/istio-dns
      volumes:
      # Technically not needed on this pod - but it helps debugging/testing SDS
      # Should be removed after everything works.
      - emptyDir:
          medium: Memory
        name: local-certs
      - name: istio-token
        projected:
          sources:
            - serviceAccountToken:
                audience: istio-ca
                expirationSeconds: 43200
                path: istio-token
      - name: config-volume
        configMap:
          name: istio
      priorityClassName: istio
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: app
                  operator: In
                  values:
                  - istiod
                - key: istio
                  operator: In
                  values:
                  - pilot
              topologyKey: "kubernetes.io/hostname"
`

		istioIngressAutoscale = `apiVersion: autoscaling/v2beta1
kind: HorizontalPodAutoscaler
metadata:
  name: istio-ingressgateway
  namespace: ` + deployNSIngress + `
  labels:
    app: istio-ingressgateway
    foo: bar
    
spec:
  maxReplicas: 5
  minReplicas: 2
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: istio-ingressgateway
  metrics:
  - type: Resource
    resource:
      name: cpu
      targetAverageUtilization: 80
`

		istioIngressBootstrapConfig = `apiVersion: v1
kind: ConfigMap
metadata:
  name: istio-custom-bootstrap-config
  namespace: ` + deployNSIngress + `
  labels:
    app: istio-ingressgateway
    foo: bar
    
data:
  custom_bootstrap.yaml: |
    layered_runtime:
      layers:
      - name: static_layer_0
        static_layer:
          overload:
            # Fix for https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2020-8663
            # https://istio.io/latest/news/security/istio-security-2020-007/
            global_downstream_max_connections: 750000
`

		istioIngressEnvoyFilter = `
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: istio-ingressgateway
  namespace: ` + deployNSIngress + `
  labels:
    app: istio-ingressgateway
    foo: bar
    
spec:
  configPatches:
  - applyTo: LISTENER
    match:
      context: GATEWAY
      listener:
        portNumber: 999
    patch:
      operation: MERGE
      value:
        per_connection_buffer_limit_bytes: 32768 # 32 KiB
  - applyTo: CLUSTER
    match:
      context: GATEWAY
      cluster:
        portNumber: 999
    patch:
      operation: MERGE
      value:
        per_connection_buffer_limit_bytes: 32768 # 32 KiB
  # Some LoadBalancers do not set KEEPALIVE when they open a TCP connection
  # to the Istio Ingress Gateway. For long living connections it can cause
  # silent timeouts.
  # Therefore envoy must be configured to send KEEPALIVE to downstream (LB).
  # See https://github.com/envoyproxy/envoy/issues/3634
  - applyTo: LISTENER
    match:
      context: GATEWAY
      listener:
        name: 0.0.0.0_999
        portNumber: 999
    patch:
      operation: MERGE
      value:
        socket_options:
        # SOL_SOCKET = 1
        # SO_KEEPALIVE = 9
        - level: 1
          name: 9
          int_value: 1
          state: STATE_LISTENING
        # IPPROTO_TCP = 6
        # TCP_KEEPIDLE = 4
        - level: 6
          name: 4
          int_value: 55
          state: STATE_LISTENING
        # IPPROTO_TCP = 6
        # TCP_KEEPINTVL = 5
        - level: 6
          name: 5
          int_value: 55
          state: STATE_LISTENING

---

apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: stats-filter-1.10
  namespace: ` + deployNSIngress + `
spec:
  configPatches:
    - applyTo: HTTP_FILTER
      match:
        context: GATEWAY
        proxy:
          proxyVersion: '^1\.10.*'
        listener:
          filterChain:
            filter:
              name: "envoy.filters.network.http_connection_manager"
              subFilter:
                name: "envoy.filters.http.router"
      patch:
        operation: INSERT_BEFORE
        value:
          name: istio.stats
          typed_config:
            "@type": type.googleapis.com/udpa.type.v1.TypedStruct
            type_url: type.googleapis.com/envoy.extensions.filters.http.wasm.v3.Wasm
            value:
              config:
                root_id: stats_outbound
                configuration:
                  "@type": "type.googleapis.com/google.protobuf.StringValue"
                  value: |
                    {
                    }
                vm_config:
                  vm_id: stats_outbound
                  runtime: envoy.wasm.runtime.null
                  code:
                    local:
                      inline_string: envoy.wasm.stats

---

apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: tcp-stats-filter-1.10
  namespace: ` + deployNSIngress + `
spec:
  configPatches:
  - applyTo: NETWORK_FILTER
    match:
      context: GATEWAY
      listener:
        filterChain:
          filter:
            name: envoy.filters.network.tcp_proxy
      proxy:
        proxyVersion: ^1\.10.*
    patch:
      operation: INSERT_BEFORE
      value:
        name: istio.stats
        typed_config:
          "@type": type.googleapis.com/udpa.type.v1.TypedStruct
          type_url: type.googleapis.com/envoy.extensions.filters.network.wasm.v3.Wasm
          value:
            config:
              configuration:
                "@type": type.googleapis.com/google.protobuf.StringValue
                value: |
                  {
                  }
              root_id: stats_outbound
              vm_config:
                code:
                  local:
                    inline_string: envoy.wasm.stats
                runtime: envoy.wasm.runtime.null
                vm_id: tcp_stats_outbound

---

apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: stats-filter-1.12
  namespace: ` + deployNSIngress + `
spec:
  configPatches:
    - applyTo: HTTP_FILTER
      match:
        context: GATEWAY
        proxy:
          proxyVersion: '^1\.12.*'
        listener:
          filterChain:
            filter:
              name: "envoy.filters.network.http_connection_manager"
              subFilter:
                name: "envoy.filters.http.router"
      patch:
        operation: INSERT_BEFORE
        value:
          name: istio.stats
          typed_config:
            "@type": type.googleapis.com/udpa.type.v1.TypedStruct
            type_url: type.googleapis.com/envoy.extensions.filters.http.wasm.v3.Wasm
            value:
              config:
                root_id: stats_outbound
                configuration:
                  "@type": "type.googleapis.com/google.protobuf.StringValue"
                  value: |
                    {
                      "debug": "false",
                      "stat_prefix": "istio",
                      "disable_host_header_fallback": true
                    }
                vm_config:
                  vm_id: stats_outbound
                  runtime: envoy.wasm.runtime.null
                  code:
                    local:
                      inline_string: envoy.wasm.stats

---

apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: tcp-stats-filter-1.12
  namespace: ` + deployNSIngress + `
spec:
  configPatches:
  - applyTo: NETWORK_FILTER
    match:
      context: GATEWAY
      proxy:
        proxyVersion: '^1\.12.*'
      listener:
        filterChain:
          filter:
            name: envoy.filters.network.tcp_proxy
    patch:
      operation: INSERT_BEFORE
      value:
        name: istio.stats
        typed_config:
          "@type": type.googleapis.com/udpa.type.v1.TypedStruct
          type_url: type.googleapis.com/envoy.extensions.filters.network.wasm.v3.Wasm
          value:
            config:
              root_id: stats_outbound
              configuration:
                "@type": type.googleapis.com/google.protobuf.StringValue
                value: |
                  {
                    "debug": "false",
                    "stat_prefix": "istio"
                  }
              vm_config:
                vm_id: tcp_stats_outbound
                runtime: envoy.wasm.runtime.null
                code:
                  local:
                    inline_string: "envoy.wasm.stats"

---

apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: http-connect-listener
  namespace: ` + deployNSIngress + `
spec:
  workloadSelector:
    labels:
      app: istio-ingressgateway
      foo: bar
      
  configPatches:
  - applyTo: NETWORK_FILTER
    match:
      context: GATEWAY
      listener:
        name: 0.0.0.0_8132
        portNumber: 8132
        filterChain:
          filter:
            name: envoy.filters.network.http_connection_manager
    patch:
      operation: MERGE
      value:
        name: envoy.filters.network.http_connection_manager
        typed_config:
          "@type": "type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager"
          http_protocol_options:
            accept_http_10: true
          upgrade_configs:
          - upgrade_type: CONNECT

---

apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: reversed-vpn
  namespace: ` + deployNSIngress + `
spec:
  configPatches:
  - applyTo: NETWORK_FILTER
    match:
      context: GATEWAY
      listener:
        filterChain:
          filter:
            name: envoy.filters.network.http_connection_manager
        name: 0.0.0.0_8132
        portNumber: 8132
    patch:
      operation: MERGE
      value:
        name: envoy.filters.network.http_connection_manager
        typed_config:
          '@type': type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          route_config:
            virtual_hosts:
            - domains:
              - api.*
              name: reversed-vpn
              routes:
              - match:
                  connect_matcher: {}
                route:
                  cluster_header: Reversed-VPN
                  upgrade_configs:
                  - connect_config: {}
                    upgrade_type: CONNECT
  - applyTo: HTTP_FILTER
    match:
      context: GATEWAY
      listener:
        name: 0.0.0.0_8132
        portNumber: 8132
        filterChain:
          filter:
            name: "envoy.filters.network.http_connection_manager"
            subFilter:
              name: "envoy.filters.http.router"
    patch:
      operation: INSERT_BEFORE
      filterClass: AUTHZ # This filter will run *after* the Istio authz filter.
      value:
        name: envoy.filters.http.ext_authz
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.http.ext_authz.v3.ExtAuthz
          transport_api_version: V3
          grpc_service:
            envoy_grpc:
              cluster_name: outbound|9001||reversed-vpn-auth-server.garden.svc.cluster.local
            timeout: 0.250s
  workloadSelector:
    labels:
      app: istio-ingressgateway
      foo: bar
      

---

apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: access-log
  namespace: ` + deployNSIngress + `
  labels:
    app: istio-ingressgateway
    foo: bar
    
spec:
  workloadSelector:
    labels:
      app: istio-ingressgateway
      foo: bar
      
  configPatches:
  - applyTo: NETWORK_FILTER
    match:
      context: ANY
      listener:
        filterChain:
          filter:
            name: envoy.filters.network.tcp_proxy
    patch:
      operation: MERGE
      value:
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
          access_log:
          - name: envoy.access_loggers.stdout
  - applyTo: NETWORK_FILTER
    match:
      context: ANY
      listener:
        filterChain:
          filter:
            name: envoy.filters.network.http_connection_manager
    patch:
      operation: MERGE
      value:
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          access_log:
          - name: envoy.access_loggers.stdout
`

		istioIngressGateway = `apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: reversed-vpn-auth-server
  namespace: ` + deployNSIngress + `
spec:
  selector:
    app: istio-ingressgateway
    foo: bar
    
  servers:
  - hosts:
    - reversed-vpn-auth-server.garden.svc.cluster.local
    port:
      name: tls-tunnel
      number: 8132
      protocol: HTTP
`

		istioIngressPodDisruptionBudget = `apiVersion: policy/v1beta1
kind: PodDisruptionBudget
metadata:
  name: istio-ingressgateway
  namespace: ` + deployNSIngress + `
  labels:
    app: istio-ingressgateway
    foo: bar
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: istio-ingressgateway
      foo: bar
`

		istioIngressPriorityClass = `---
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: istio-ingressgateway
value: 1000000000
globalDefault: false
description: "This class is used to ensure that the istio-ingressgateway has a high priority and is not preempted in favor of other pods."`

		istioIngressRole = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: istio-ingressgateway-sds
  namespace: ` + deployNSIngress + `
  labels:
    app: istio-ingressgateway
    foo: bar
    
rules:
- apiGroups:
  - ""
  resources:
  - secrets
  verbs:
  - get
  - watch
  - list
`

		istioIngressRoleBinding = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: istio-ingressgateway-sds
  namespace: ` + deployNSIngress + `
  labels:
    app: istio-ingressgateway
    foo: bar
    
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: istio-ingressgateway-sds
subjects:
- kind: ServiceAccount
  name: istio-ingressgateway
`

		istioIngressService = `apiVersion: v1
kind: Service
metadata:
  name: istio-ingressgateway
  namespace: ` + deployNSIngress + `
  annotations:
    service.alpha.kubernetes.io/aws-load-balancer-type: "nlb"
    service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
    foo: bar
    
  labels:
    app: istio-ingressgateway
    foo: bar
    
spec:
  type: LoadBalancer
  selector:
    app: istio-ingressgateway
    foo: bar
    
  ports:
  - name: status-port
    port: 15021
    targetPort: 15021
    protocol: TCP
  - name: foo
    port: 999
    targetPort: 999
  
`

		istioIngressServiceAccount = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: istio-ingressgateway
  namespace: ` + deployNSIngress + `
  labels:
    app: istio-ingressgateway
    foo: bar
    
automountServiceAccountToken: false
`

		istioIngressDeployment = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: istio-ingressgateway
  namespace: ` + deployNSIngress + `
  labels:
    app: istio-ingressgateway
    foo: bar
    
spec:
  revisionHistoryLimit: 1
  selector:
    matchLabels:
      app: istio-ingressgateway
      foo: bar
      
  strategy:
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  template:
    metadata:
      labels:
        app: istio-ingressgateway
        foo: bar
        
        service.istio.io/canonical-name: "istio-ingressgateway"
        service.istio.io/canonical-revision: "1.7"
      annotations:
        sidecar.istio.io/inject: "false"
        checksum/configmap-bootstrap-config-override: a357fe81829c12ad57e92721b93fd6efa1670d19e4cab94dfb7c792f9665c51a
    spec:
      serviceAccountName: istio-ingressgateway
      securityContext:
        fsGroup: 1337
        runAsGroup: 1337
        runAsNonRoot: true
        runAsUser: 1337
      containers:
      - name: istio-proxy
        image: foo/bar
        imagePullPolicy: IfNotPresent
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
          privileged: false
          readOnlyRootFilesystem: true
        ports:
        - containerPort: 15021
        - containerPort: 999
          protocol: TCP
        args:
        - proxy
        - router
        - --domain=` + deployNSIngress + `.svc.foo.bar
        - --proxyLogLevel=warning
        - --proxyComponentLogLevel=misc:error
        - --log_output_level=all:warn,ads:error
        - --drainDuration=45s
        - --parentShutdownDuration=1m0s
        - --connectTimeout=10s
        - --serviceCluster=istio-ingressgateway
        - --proxyAdminPort=15000
        - --concurrency=4
        - --statusPort=15021
        - --controlPlaneAuthPolicy=NONE
        - --discoveryAddress=istiod.istio-test-system.svc:15012
        readinessProbe:
          failureThreshold: 30
          httpGet:
            path: /healthz/ready
            port: 15021
            scheme: HTTP
          initialDelaySeconds: 1
          periodSeconds: 2
          successThreshold: 1
          timeoutSeconds: 1
        resources:
          requests:
            cpu: 1000m
            memory: 2Gi
          limits:
            memory: 8Gi
        env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
        - name: POD_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
        - name: INSTANCE_IP
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: status.podIP
        - name: HOST_IP
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: status.hostIP
        - name: SERVICE_ACCOUNT
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.serviceAccountName
        - name: ISTIO_META_POD_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.name
        - name: ISTIO_META_CONFIG_NAMESPACE
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
        - name: JWT_POLICY
          value: third-party-jwt
        - name: PILOT_CERT_PROVIDER
          value: istiod
        - name: ISTIO_META_USER_SDS
          value: "true"
        - name: CA_ADDR
          value: istiod.istio-test-system.svc:15012
        - name: ISTIO_META_WORKLOAD_NAME
          value: istio-ingressgateway
        - name: ISTIO_META_OWNER
          value: kubernetes://apis/apps/v1/namespaces/` + deployNSIngress + `/deployments/istio-ingressgateway
        - name: ISTIO_AUTO_MTLS_ENABLED
          value: "true"
        - name: ISTIO_META_ROUTER_MODE
          value: standard
        - name: ISTIO_META_CLUSTER_ID
          value: "Kubernetes"
        - name: ISTIO_BOOTSTRAP_OVERRIDE
          value: /etc/istio/custom-bootstrap/custom_bootstrap.yaml
        volumeMounts:
        - mountPath: /etc/istio/proxy
          name: istio-envoy
        - mountPath: /var/run/secrets/istio
          name: istiod-ca-cert
        - mountPath: /etc/istio/custom-bootstrap
          name: custom-bootstrap-volume
          readOnly: true
        - name: istio-token
          mountPath: /var/run/secrets/tokens
          readOnly: true
        - name: ingressgatewaysdsudspath
          mountPath: /var/run/ingress_gateway
        - name: istio-data
          mountPath: /var/lib/istio/data
        - name: podinfo
          mountPath: /etc/istio/pod
      volumes:
      - name: istiod-ca-cert
        configMap:
          name: istio-ca-root-cert
      - name: custom-bootstrap-volume
        configMap:
          name: istio-custom-bootstrap-config
      - name: podinfo
        downwardAPI:
          items:
          - path: "labels"
            fieldRef:
              fieldPath: metadata.labels
          - path: "annotations"
            fieldRef:
              fieldPath: metadata.annotations
      - emptyDir: {}
        name: istio-envoy
      - name: ingressgatewaysdsudspath
        emptyDir: {}
      - name: istio-data
        emptyDir: {}
      - name: istio-token
        projected:
          sources:
          - serviceAccountToken:
              path: istio-token
              expirationSeconds: 43200
              audience: istio-ca
      priorityClassName: istio-ingressgateway
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: app
                  operator: In
                  values:
                  - istio-ingressgateway
                - key: foo
                  operator: In
                  values:
                  - bar
              topologyKey: "kubernetes.io/hostname"
`

		istioProxyProtocolEnvoyFilter = `# this adds "envoy.listener.proxy_protocol" filter to the listener.
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: proxy-protocol
  namespace: ` + deployNSIngress + `
  labels:
    app: istio-ingressgateway
    foo: bar
    
spec:
  workloadSelector:
    labels:
      app: istio-ingressgateway
      foo: bar
      
  configPatches:
  - applyTo: LISTENER
    match:
      context: GATEWAY
      listener:
        portNumber: 8443
        name: 0.0.0.0_8443
    patch:
      operation: MERGE
      value:
        per_connection_buffer_limit_bytes: 32768 # 32 KiB
        listener_filters:
        - name: envoy.filters.listener.proxy_protocol
`

		istioProxyProtocolGateway = `apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: proxy-protocol
  namespace: ` + deployNSIngress + `
  labels:
    app: istio-ingressgateway
    foo: bar
    
spec:
  selector:
    app: istio-ingressgateway
    foo: bar
    
  servers:
  - port:
      number: 8443
      name: tcp
      protocol: TCP
    hosts:
    - "*"
`

		istioProxyProtocolVirtualService = `# this dummy virtual service is added so the Envoy listener is added
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: proxy-protocol-blackhole
  namespace: ` + deployNSIngress + `
  labels:
    app: istio-ingressgateway
    foo: bar
    
spec:
  hosts:
  - blackhole.local
  gateways:
  - proxy-protocol
  exportTo:
  - "."
  tcp:
  - match:
    - port: 8443
    route:
    - destination:
        host: localhost
        port:
          number: 9999
`
	)

	BeforeEach(func() {
		ctx = context.TODO()
		igwAnnotations = map[string]string{"foo": "bar"}
		labels = map[string]string{"foo": "bar"}
		managedResourceName = "istio"

		c = fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		renderer := cr.NewWithServerVersion(&version.Info{GitVersion: "v1.21.4"})

		gardenletfeatures.RegisterFeatureGates()

		igw = makeIngressGateway(deployNSIngress, igwAnnotations, labels)
		ipp = makeProxyProtocol(deployNSIngress, labels)

		istiod = NewIstio(
			c,
			renderer,
			&IstiodValues{Image: "foo/bar", TrustDomain: "foo.local"},
			deployNS,
			chartsRootPath,
			igw,
			ipp,
		)

		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: deployNS,
			},
		}
		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResource.Name,
				Namespace: deployNS,
			},
		}
	})

	Describe("#Deploy", func() {
		BeforeEach(func() {
			Expect(istiod.Deploy(ctx)).ToNot(HaveOccurred(), "istiod deploy succeeds")
		})

		It("deploys istiod namespace", func() {
			actualNS := &corev1.Namespace{}

			Expect(c.Get(ctx, client.ObjectKey{Name: deployNS}, actualNS)).ToNot(HaveOccurred())

			Expect(actualNS.Labels).To(HaveKeyWithValue("istio-operator-managed", "Reconcile"))
			Expect(actualNS.Labels).To(HaveKeyWithValue("istio-injection", "disabled"))
		})

		It("deploys istio-ingress namespace", func() {
			actualNS := &corev1.Namespace{}

			Expect(c.Get(ctx, client.ObjectKey{Name: deployNSIngress}, actualNS)).ToNot(HaveOccurred())

			Expect(actualNS.Labels).To(HaveKeyWithValue("istio-operator-managed", "Reconcile"))
			Expect(actualNS.Labels).To(HaveKeyWithValue("istio-injection", "disabled"))
		})

		It("should successfully deploy all resources", func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(managedResource).To(DeepEqual(&resourcesv1alpha1.ManagedResource{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ManagedResource",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResourceName,
					Namespace:       deployNS,
					ResourceVersion: "1",
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class: pointer.String("seed"),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResourceSecret.Name,
					}},
					KeepObjects: pointer.Bool(false),
				},
			}))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Data).To(HaveLen(31))

			By("checking istio-istiod resources")
			Expect(string(managedResourceSecret.Data["istio-istiod_templates_configmap.yaml"])).To(Equal(istiodConfigMap))
			Expect(string(managedResourceSecret.Data["istio-istiod_templates_deployment.yaml"])).To(Equal(istiodDeployment))
			Expect(string(managedResourceSecret.Data["istio-istiod_templates_service.yaml"])).To(Equal(istiodService))
			Expect(string(managedResourceSecret.Data["istio-istiod_templates_clusterrole.yaml"])).To(Equal(istioClusterRole))
			Expect(string(managedResourceSecret.Data["istio-istiod_templates_clusterrolebinding.yaml"])).To(Equal(istiodClusterRoleBinding))
			Expect(string(managedResourceSecret.Data["istio-istiod_templates_destinationrule.yaml"])).To(Equal(istiodDestinationRule))
			Expect(string(managedResourceSecret.Data["istio-istiod_templates_peerauthentication.yaml"])).To(Equal(istiodPeerAuthentication))
			Expect(string(managedResourceSecret.Data["istio-istiod_templates_poddisruptionbudget.yaml"])).To(Equal(istiodPodDisruptionBudget))
			Expect(string(managedResourceSecret.Data["istio-istiod_templates_priorityclass-istiod.yaml"])).To(Equal(istiodPriorityClass))
			Expect(string(managedResourceSecret.Data["istio-istiod_templates_role.yaml"])).To(Equal(istiodRole))
			Expect(string(managedResourceSecret.Data["istio-istiod_templates_rolebinding.yaml"])).To(Equal(istiodRoleBinding))
			Expect(string(managedResourceSecret.Data["istio-istiod_templates_serviceaccount.yaml"])).To(Equal(istiodServiceAccount))
			Expect(string(managedResourceSecret.Data["istio-istiod_templates_sidecar.yaml"])).To(Equal(istiodSidecar))
			Expect(string(managedResourceSecret.Data["istio-istiod_templates_autoscale.yaml"])).To(Equal(istiodAutoscale))
			Expect(string(managedResourceSecret.Data["istio-istiod_templates_validatingwebhookconfiguration.yaml"])).To(Equal(istiodValidationWebhook))

			By("checking istio-ingress resources")
			Expect(string(managedResourceSecret.Data["istio-ingress_templates_autoscale.yaml"])).To(Equal(istioIngressAutoscale))
			Expect(string(managedResourceSecret.Data["istio-ingress_templates_bootstrap-config-override.yaml"])).To(Equal(istioIngressBootstrapConfig))
			Expect(string(managedResourceSecret.Data["istio-ingress_templates_envoy-filter.yaml"])).To(Equal(istioIngressEnvoyFilter))
			Expect(string(managedResourceSecret.Data["istio-ingress_templates_gateway.yaml"])).To(Equal(istioIngressGateway))
			Expect(string(managedResourceSecret.Data["istio-ingress_templates_poddisruptionbudget.yaml"])).To(Equal(istioIngressPodDisruptionBudget))
			Expect(string(managedResourceSecret.Data["istio-ingress_templates_priorityclass-istioingressgateway.yaml"])).To(Equal(istioIngressPriorityClass))
			Expect(string(managedResourceSecret.Data["istio-ingress_templates_role.yaml"])).To(Equal(istioIngressRole))
			Expect(string(managedResourceSecret.Data["istio-ingress_templates_rolebindings.yaml"])).To(Equal(istioIngressRoleBinding))
			Expect(string(managedResourceSecret.Data["istio-ingress_templates_service.yaml"])).To(Equal(istioIngressService))
			Expect(string(managedResourceSecret.Data["istio-ingress_templates_serviceaccount.yaml"])).To(Equal(istioIngressServiceAccount))
			Expect(string(managedResourceSecret.Data["istio-ingress_templates_deployment.yaml"])).To(Equal(istioIngressDeployment))

			By("checking istio-proxy-protocol resources")
			Expect(string(managedResourceSecret.Data["istio-proxy-protocol_templates_envoyfilter.yaml"])).To(Equal(istioProxyProtocolEnvoyFilter))
			Expect(string(managedResourceSecret.Data["istio-proxy-protocol_templates_gateway.yaml"])).To(Equal(istioProxyProtocolGateway))
			Expect(string(managedResourceSecret.Data["istio-proxy-protocol_templates_virtualservice.yaml"])).To(Equal(istioProxyProtocolVirtualService))
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			namespace := &corev1.Namespace{}
			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

			Expect(istiod.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceSecret.Name)))
			Expect(c.Get(ctx, client.ObjectKey{Name: deployNS}, namespace)).To(MatchError(apierrors.NewNotFound(corev1.Resource("namespaces"), deployNS)))
			Expect(c.Get(ctx, client.ObjectKey{Name: deployNSIngress}, namespace)).To(MatchError(apierrors.NewNotFound(corev1.Resource("namespaces"), deployNSIngress)))
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

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, managedResource)).To(Succeed())

				Expect(istiod.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it's already removed", func() {
				Expect(istiod.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})

func makeIngressGateway(namespace string, annotations, labels map[string]string) []*IngressGateway {
	values := IngressValues{
		Image:           "foo/bar",
		TrustDomain:     "foo.bar",
		IstiodNamespace: "istio-test-system",
		Annotations:     annotations,
		Labels:          labels,
		Ports: []corev1.ServicePort{
			{Name: "foo", Port: 999, TargetPort: intstr.FromInt(999)},
		},
	}

	chartPath := filepath.Join(chartsRootPath, "istio", "istio-ingress")

	return []*IngressGateway{&IngressGateway{Values: &values, Namespace: namespace, ChartPath: chartPath}}
}

func makeProxyProtocol(namespace string, labels map[string]string) []*IstioProxyProtocol {
	values := ProxyValues{
		Labels: labels,
	}

	chartPath := filepath.Join(chartsRootPath, "istio", "istio-proxy-protocol")

	return []*IstioProxyProtocol{&IstioProxyProtocol{Values: &values, Namespace: namespace, ChartPath: chartPath}}
}
