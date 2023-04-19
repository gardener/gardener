// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	istionetworkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/istio"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("istiod", func() {
	const (
		deployNS        = "test"
		deployNSIngress = "test-ingress"
	)

	var (
		ctx            context.Context
		c              client.Client
		istiod         component.DeployWaiter
		igw            []IngressGatewayValues
		igwAnnotations map[string]string
		labels         map[string]string

		managedResourceIstioName   string
		managedResourceIstio       *resourcesv1alpha1.ManagedResource
		managedResourceIstioSecret *corev1.Secret

		managedResourceIstioSystemName   string
		managedResourceIstioSystem       *resourcesv1alpha1.ManagedResource
		managedResourceIstioSystemSecret *corev1.Secret

		renderer chartrenderer.Interface

		minReplicas = 2
		maxReplicas = 5

		externalTrafficPolicy corev1.ServiceExternalTrafficPolicyType

		ignoreAnnotation = `
    resources.gardener.cloud/mode: Ignore`

		istiodService = func(ignore bool) string {
			var additionalAnnotation string
			if ignore {
				additionalAnnotation += ignoreAnnotation
			}

			return `apiVersion: v1
kind: Service
metadata:
  name: istiod
  namespace: test
  annotations:
    networking.resources.gardener.cloud/from-policy-allowed-ports: '[{"port":15014,"protocol":"TCP"}]'
    networking.resources.gardener.cloud/from-policy-pod-label-selector: all-seed-scrape-targets
    networking.resources.gardener.cloud/from-world-to-ports: '[{"port":10250,"protocol":"TCP"}]'
    networking.resources.gardener.cloud/namespace-selectors: '[{"matchLabels":{"gardener.cloud/role":"istio-ingress"}},{"matchExpressions":[{"key":"handler.exposureclass.gardener.cloud/name","operator":"Exists"}]},{"matchLabels":{"kubernetes.io/metadata.name":"garden"}}]'` + additionalAnnotation + `
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
		}
		istioClusterRole = func(ignore bool) string {
			var annotations string
			if ignore {
				annotations = `
  annotations:` + ignoreAnnotation
			}

			return `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: istiod` + annotations + `
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
# removing CRD permissions can break older versions of Istio running alongside this control plane (https://github.com/istio/istio/issues/29382)
# please proceed with caution
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
  - networking.k8s.io
  resources:
  - ingresses
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - extensions
  - networking.k8s.io
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

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: istiod-gateway-controller
  labels:
    app: istiod
    istio: pilot
    
rules:
  - apiGroups: ["apps"]
    verbs: [ "get", "watch", "list", "update", "patch", "create", "delete" ]
    resources: [ "deployments" ]
  - apiGroups: [""]
    verbs: [ "get", "watch", "list", "update", "patch", "create", "delete" ]
    resources: [ "services" ]
  - apiGroups: [""]
    verbs: [ "get", "watch", "list", "update", "patch", "create", "delete" ]
    resources: [ "serviceaccounts" ]
`
		}

		istiodClusterRoleBinding = func(ignore bool) string {
			var annotations string
			if ignore {
				annotations = `
  annotations:` + ignoreAnnotation
			}

			return `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: istiod` + annotations + `
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

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: istiod-gateway-controller
  labels:
    app: istiod
    istio: pilot
    
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: istiod-gateway-controller
subjects:
- kind: ServiceAccount
  name: istiod
  namespace: ` + deployNS + `
`
		}

		istiodDestinationRule = func(ignore bool) string {
			var annotations string
			if ignore {
				annotations = `
  annotations:` + ignoreAnnotation
			}

			return `apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: default
  namespace: ` + deployNS + annotations + `
spec:
  host: "*"
  exportTo:
  - "*"
  trafficPolicy:
    tls:
      mode: ISTIO_MUTUAL
`
		}

		istiodPeerAuthentication = func(ignore bool) string {
			var annotations string
			if ignore {
				annotations = `
  annotations:` + ignoreAnnotation
			}

			return `apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata:
  name: default
  namespace: ` + deployNS + annotations + `
spec:
  mtls:
    mode: STRICT
`
		}

		istiodPodDisruptionBudgetFor = func(k8sGreaterEqual121 bool, ignore bool) string {
			var annotations string
			if ignore {
				annotations = `
  annotations:` + ignoreAnnotation
			}

			apiVersion := "policy/v1beta1"
			if k8sGreaterEqual121 {
				apiVersion = "policy/v1"
			}
			out := `
apiVersion: ` + apiVersion + `
kind: PodDisruptionBudget
metadata:
  name: istiod
  namespace: ` + deployNS + annotations + `
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
			return out
		}

		istiodRole = func(ignore bool) string {
			var annotations string
			if ignore {
				annotations = `
  annotations:` + ignoreAnnotation
			}
			return `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: istiod
  namespace: ` + deployNS + annotations + `
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
  # TODO lock this down to istio-ca-cert if not using the DNS cert mesh config
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
- apiGroups:
  - ''
  resources:
  - configmaps
  verbs:
  - delete
`
		}

		istiodRoleBinding = func(ignore bool) string {
			var annotations string
			if ignore {
				annotations = `
  annotations:` + ignoreAnnotation
			}
			return `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: istiod
  namespace: ` + deployNS + annotations + `
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
		}

		istiodServiceAccount = func(ignore bool) string {
			var annotations string
			if ignore {
				annotations = `
  annotations:` + ignoreAnnotation
			}

			return `apiVersion: v1
kind: ServiceAccount
metadata:
  name: istiod
  namespace: ` + deployNS + annotations + `
  labels:
    app: istiod
    istio: pilot
    
automountServiceAccountToken: false
`
		}

		istiodSidecar = func(ignore bool) string {
			var annotations string
			if ignore {
				annotations = `
  annotations:` + ignoreAnnotation
			}
			return `apiVersion: networking.istio.io/v1alpha3
kind: Sidecar
metadata:
  name: default
  namespace: ` + deployNS + annotations + `
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
		}

		istiodAutoscale = func(ignore bool) string {
			var annotations string
			if ignore {
				annotations = `
  annotations:` + ignoreAnnotation
			}
			return `apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: istiod
  namespace: ` + deployNS + annotations + `
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
`
		}

		istiodValidationWebhook = func(ignore bool) string {
			var annotations string
			if ignore {
				annotations = `
  annotations:` + ignoreAnnotation
			}

			return `apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: istiod` + annotations + `
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
		}

		istiodConfigMap = func(ignore bool) string {
			var annotations string
			if ignore {
				annotations = `
  annotations:` + ignoreAnnotation
			}

			return `apiVersion: v1
kind: ConfigMap
metadata:
  name: istio
  namespace: ` + deployNS +
				annotations + `
  labels:
    app: istiod
    istio: pilot
    
data:

  # Configuration file for the mesh networks to be used by the Split Horizon EDS.
  meshNetworks: |-
    networks: {}

  mesh: |-
    # TCP connection timeout between Envoy & the application, and between Envoys.
    connectTimeout: 10s

    # Set enableTracing to false to disable request tracing.
    enableTracing: false

    # Set accessLogFile to empty string to disable access log.
    accessLogFile: ""

    accessLogFormat: ""

    accessLogEncoding: 'TEXT'

    enableEnvoyAccessLogService: false

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
    # deprecated
    certificates: []

    # Disable the advertisment of services and endpoints which are no explictly marked in
    # ` + "`" + `exportTo` + "`" + `. Improves security and isolation.
    # Refer to https://istio.io/docs/reference/config/istio.mesh.v1alpha1/#MeshConfig
    defaultServiceExportTo: ["-"]
    defaultVirtualServiceExportTo: ["-"]
    defaultDestinationRuleExportTo: ["-"]

    defaultConfig:
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

    rootNamespace: ` + deployNS + `
    enablePrometheusMerge: true
`
		}

		istiodDeployment = func(ignore bool, checksum string) string {
			var annotations string
			if ignore {
				annotations = `
  annotations:` + ignoreAnnotation
			}

			return `apiVersion: apps/v1
kind: Deployment
metadata:
  name: istiod
  namespace: ` + deployNS +
				annotations + `
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
        networking.gardener.cloud/to-dns: allowed
        networking.gardener.cloud/to-runtime-apiserver: allowed
        app: istiod
        istio: pilot
        
      annotations:
        sidecar.istio.io/inject: "false"
        checksum/istio-config: ` + checksum + `
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
        - --domain
        - foo.local
        - --keepaliveMaxServerConnectionAge
        - "30m"
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
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          runAsUser: 1337
          runAsGroup: 1337
          runAsNonRoot: true
          capabilities:
            drop:
            - ALL
        volumeMounts:
        - name: config-volume
          mountPath: /etc/istio/config
        - name: istio-token
          mountPath: /var/run/secrets/tokens
          readOnly: true
        - name: local-certs
          mountPath: /var/run/secrets/istio-dns
        - name: cacerts
          mountPath: /etc/cacerts
          readOnly: true
        - name: istio-kubeconfig
          mountPath: /var/run/secrets/remote
          readOnly: true
        - name: istio-csr-dns-cert
          mountPath: /var/run/secrets/istiod/tls
          readOnly: true
        - name: istio-csr-ca-configmap
          mountPath: /var/run/secrets/istiod/ca
          readOnly: true
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
      # Optional: user-generated root
      - name: cacerts
        secret:
          secretName: cacerts
          optional: true
      - name: istio-kubeconfig
        secret:
          secretName: istio-kubeconfig
          optional: true
      - name: config-volume
        configMap:
          name: istio
      - name: istio-csr-dns-cert
        secret:
          secretName: istiod-tls
          optional: true
      - name: istio-csr-ca-configmap
        configMap:
          name: istio-ca-root-cert
          defaultMode: 420
          optional: true
      priorityClassName: gardener-system-critical
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
		}

		istioIngressAutoscaler = func(min *int, max *int) string {
			return `
apiVersion: autoscaling/v2beta1
kind: HorizontalPodAutoscaler
metadata:
  name: istio-ingressgateway
  namespace: ` + deployNSIngress + `
  labels:
    app.kubernetes.io/version: 1.17.1
    app: istio-ingressgateway
    foo: bar
    
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: istio-ingressgateway
  minReplicas: ` + fmt.Sprintf("%d", pointer.IntDeref(min, 2)) + `
  maxReplicas: ` + fmt.Sprintf("%d", pointer.IntDeref(max, 5)) + `
  metrics:
  - type: Resource
    resource:
      name: cpu
      targetAverageUtilization: 80
`
		}

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

		istioIngressEnvoyVPNFilter = `apiVersion: networking.istio.io/v1alpha3
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
  name: http-connect-listener
  namespace: ` + deployNSIngress + `
spec:
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
  workloadSelector:
    labels:
      app: istio-ingressgateway
      foo: bar
      
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
  name: stats-filter-1.13
  namespace: ` + deployNSIngress + `
spec:
  priority: -1
  configPatches:
    - applyTo: HTTP_FILTER
      match:
        context: GATEWAY
        proxy:
          proxyVersion: '^1\.13.*'
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
  name: tcp-stats-filter-1.13
  namespace: ` + deployNSIngress + `
spec:
  priority: -1
  configPatches:
    - applyTo: NETWORK_FILTER
      match:
        context: GATEWAY
        proxy:
          proxyVersion: '^1\.13.*'
        listener:
          filterChain:
            filter:
              name: "envoy.filters.network.tcp_proxy"
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
                  "@type": "type.googleapis.com/google.protobuf.StringValue"
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
  name: stats-filter-1.14
  namespace: ` + deployNSIngress + `
spec:
  priority: -1
  configPatches:
    - applyTo: HTTP_FILTER
      match:
        context: GATEWAY
        proxy:
          proxyVersion: '^1\.14.*'
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
  name: tcp-stats-filter-1.14
  namespace: ` + deployNSIngress + `
spec:
  priority: -1
  configPatches:
    - applyTo: NETWORK_FILTER
      match:
        context: GATEWAY
        proxy:
          proxyVersion: '^1\.14.*'
        listener:
          filterChain:
            filter:
              name: "envoy.filters.network.tcp_proxy"
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
                  "@type": "type.googleapis.com/google.protobuf.StringValue"
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
  name: stats-filter-1.15
  namespace: ` + deployNSIngress + `
spec:
  priority: -1
  configPatches:
    - applyTo: HTTP_FILTER
      match:
        context: GATEWAY
        proxy:
          proxyVersion: '^1\.15.*'
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
  name: tcp-stats-filter-1.15
  namespace: ` + deployNSIngress + `
spec:
  priority: -1
  configPatches:
    - applyTo: NETWORK_FILTER
      match:
        context: GATEWAY
        proxy:
          proxyVersion: '^1\.15.*'
        listener:
          filterChain:
            filter:
              name: "envoy.filters.network.tcp_proxy"
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
                  "@type": "type.googleapis.com/google.protobuf.StringValue"
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
  name: stats-filter-1.16
  namespace: ` + deployNSIngress + `
  labels:
    istio.io/rev: default
spec:
  priority: -1
  configPatches:
  - applyTo: HTTP_FILTER
    match:
      context: GATEWAY
      proxy:
        proxyVersion: '^1\.16.*'
      listener:
        filterChain:
          filter:
            name: "envoy.filters.network.http_connection_manager"
            subFilter:
              name: envoy.filters.http.router
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
                    inline_string: "envoy.wasm.stats"

---
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: tcp-stats-filter-1.16
  namespace: ` + deployNSIngress + `
spec:
  priority: -1
  configPatches:
  - applyTo: NETWORK_FILTER
    match:
      context: GATEWAY
      proxy:
        proxyVersion: '^1\.16.*'
      listener:
        filterChain:
          filter:
            name: "envoy.filters.network.tcp_proxy"
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
                "@type": "type.googleapis.com/google.protobuf.StringValue"
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
  name: stats-filter-1.17
  namespace: ` + deployNSIngress + `
spec:
  priority: -1
  configPatches:
  - applyTo: HTTP_FILTER
    match:
      context: GATEWAY
      proxy:
        proxyVersion: '^1\.17.*'
      listener:
        filterChain:
          filter:
            name: envoy.filters.network.http_connection_manager
            subFilter:
              name: "envoy.filters.http.router"
    patch:
      operation: INSERT_BEFORE
      value:
        name: istio.stats
        typed_config:
          "@type": type.googleapis.com/udpa.type.v1.TypedStruct
          type_url: type.googleapis.com/stats.PluginConfig
          value:
            disable_host_header_fallback: true

---
# Source: istiod/templates/telemetryv2_1.17.yaml
# Note: tcp stats filter is wasm enabled only in sidecars.
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: tcp-stats-filter-1.17
  namespace: ` + deployNSIngress + `
spec:
  priority: -1
  configPatches:
  - applyTo: NETWORK_FILTER
    match:
      context: GATEWAY
      proxy:
        proxyVersion: '^1\.17.*'
      listener:
        filterChain:
          filter:
            name: "envoy.filters.network.tcp_proxy"
    patch:
      operation: INSERT_BEFORE
      value:
        name: istio.stats
        typed_config:
          "@type": type.googleapis.com/udpa.type.v1.TypedStruct
          type_url: type.googleapis.com/stats.PluginConfig
          value: {}

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

		istioIngressVPNGateway = `apiVersion: networking.istio.io/v1beta1
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

		istioIngressPodDisruptionBudgetFor = func(k8sGreaterEqual121 bool) string {
			apiVersion := "policy/v1beta1"
			if k8sGreaterEqual121 {
				apiVersion = "policy/v1"
			}
			out := `
apiVersion: ` + apiVersion + `
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
			return out
		}

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

		istioIngressService = func(externalTrafficPolicy *corev1.ServiceExternalTrafficPolicyType) string {
			out := `apiVersion: v1
kind: Service
metadata:
  name: istio-ingressgateway
  namespace: ` + deployNSIngress + `
  annotations:
    service.alpha.kubernetes.io/aws-load-balancer-type: "nlb"
    service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
    networking.resources.gardener.cloud/from-world-to-ports: '[{"port":8132,"protocol":"TCP"},{"port":8443,"protocol":"TCP"},{"port":9443,"protocol":"TCP"}]'
    networking.resources.gardener.cloud/namespace-selectors: '[{"matchLabels":{"gardener.cloud/role":"shoot"}},{"matchLabels":{"kubernetes.io/metadata.name":"garden"}}]'
    networking.resources.gardener.cloud/pod-label-selector-namespace-alias: all-istio-ingresses
    networking.resources.gardener.cloud/from-policy-allowed-ports: '[{"port":15020,"protocol":"TCP"}]'
    networking.resources.gardener.cloud/from-policy-pod-label-selector: all-seed-scrape-targets
    foo: bar
    
  labels:
    app.kubernetes.io/version: 1.17.1
    app: istio-ingressgateway
    foo: bar
    
spec:
  type: LoadBalancer
  selector:
    app: istio-ingressgateway
    foo: bar
    
  ports:
  - name: foo
    port: 999
    targetPort: 999
  
`
			if externalTrafficPolicy != nil {
				out += `  externalTrafficPolicy: ` + string(*externalTrafficPolicy) + `
`
			}
			return out
		}

		istioIngressServiceAccount = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: istio-ingressgateway
  namespace: ` + deployNSIngress + `
  labels:
    app.kubernetes.io/version: 1.17.1
    app: istio-ingressgateway
    foo: bar
    
automountServiceAccountToken: false
`

		istioIngressDeployment = func(vpnEnabled bool, replicas *int) string {
			var additionalLabels string
			if vpnEnabled {
				additionalLabels = `
        networking.resources.gardener.cloud/to-all-shoots-vpn-seed-server-tcp-1194: allowed
        networking.resources.gardener.cloud/to-garden-reversed-vpn-auth-server-tcp-9001: allowed
        networking.resources.gardener.cloud/to-all-shoots-vpn-seed-server-0-tcp-1194: allowed
        networking.resources.gardener.cloud/to-all-shoots-vpn-seed-server-1-tcp-1194: allowed`
			}

			return `apiVersion: apps/v1
kind: Deployment
metadata:
  name: istio-ingressgateway
  namespace: ` + deployNSIngress + `
  labels:
    app.kubernetes.io/version: 1.17.1
    app: istio-ingressgateway
    foo: bar
    
spec:
  replicas: ` + fmt.Sprintf("%d", pointer.IntDeref(replicas, 2)) + `
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
        networking.gardener.cloud/to-dns: allowed
        networking.resources.gardener.cloud/to-all-shoots-kube-apiserver-tcp-443: allowed
        networking.resources.gardener.cloud/to-istio-system-istiod-tcp-15012: allowed` + additionalLabels + `
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
          # Safe since 1.22: https://github.com/kubernetes/kubernetes/pull/103326
          capabilities:
            drop:
            - ALL
          allowPrivilegeEscalation: false
          privileged: false
          readOnlyRootFilesystem: true
          runAsUser: 1337
          runAsGroup: 1337
          runAsNonRoot: true
        ports:
        - containerPort: 999
          protocol: TCP
        args:
        - proxy
        - router
        - --domain=` + deployNSIngress + `.svc.foo.bar
        - --proxyLogLevel=warning
        - --proxyComponentLogLevel=misc:error
        - --log_output_level=all:warn,ads:error
        - --serviceCluster=istio-ingressgateway
        - --concurrency=4
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
        - name: ISTIO_META_NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: ISTIO_BOOTSTRAP_OVERRIDE
          value: /etc/istio/custom-bootstrap/custom_bootstrap.yaml
        volumeMounts:
        - name: workload-socket
          mountPath: /var/run/secrets/workload-spiffe-uds
        - name: credential-socket
          mountPath: /var/run/secrets/credential-uds
        - name: workload-certs
          mountPath: /var/run/secrets/workload-spiffe-credentials
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
        - name: istio-data
          mountPath: /var/lib/istio/data
        - name: podinfo
          mountPath: /etc/istio/pod
      volumes:
      - emptyDir: {}
        name: workload-socket
      - emptyDir: {}
        name: credential-socket
      - emptyDir: {}
        name: workload-certs
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
      - name: istio-data
        emptyDir: {}
      - name: istio-token
        projected:
          sources:
          - serviceAccountToken:
              path: istio-token
              expirationSeconds: 43200
              audience: istio-ca
      priorityClassName: gardener-system-critical
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
		}

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

		c = fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		renderer = chartrenderer.NewWithServerVersion(&version.Info{GitVersion: "v1.21.4"})

		gardenletfeatures.RegisterFeatureGates()

		igw = makeIngressGateway(deployNSIngress, igwAnnotations, labels)

		istiod = NewIstio(
			c,
			renderer,
			Values{
				Istiod: IstiodValues{
					Enabled:     true,
					Image:       "foo/bar",
					Namespace:   deployNS,
					TrustDomain: "foo.local",
					Zones:       []string{"a", "b", "c"},
				},
				IngressGateway: igw,
			},
		)

		managedResourceIstioName = "istio"
		managedResourceIstio = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceIstioName,
				Namespace: deployNS,
			},
		}
		managedResourceIstioSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResourceIstio.Name,
				Namespace: deployNS,
			},
		}

		managedResourceIstioSystemName = "istio-system"
		managedResourceIstioSystem = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceIstioSystemName,
				Namespace: deployNS,
			},
		}
		managedResourceIstioSystemSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResourceIstioSystem.Name,
				Namespace: deployNS,
			},
		}
	})

	Describe("#Deploy", func() {
		JustBeforeEach(func() {
			Expect(istiod.Deploy(ctx)).ToNot(HaveOccurred(), "istiod deploy succeeds")
		})

		It("deploys istiod namespace", func() {
			actualNS := &corev1.Namespace{}

			Expect(c.Get(ctx, client.ObjectKey{Name: deployNS}, actualNS)).ToNot(HaveOccurred())

			Expect(actualNS.Labels).To(And(
				HaveKeyWithValue("istio-operator-managed", "Reconcile"),
				HaveKeyWithValue("istio-injection", "disabled"),
				HaveKeyWithValue("high-availability-config.resources.gardener.cloud/consider", "true"),
				HaveKeyWithValue("gardener.cloud/role", "istio-system"),
			))
			Expect(actualNS.Annotations).To(And(
				HaveKeyWithValue("high-availability-config.resources.gardener.cloud/zones", "a,b,c"),
			))
		})

		It("deploys istio-ingress namespace", func() {
			actualNS := &corev1.Namespace{}

			Expect(c.Get(ctx, client.ObjectKey{Name: deployNSIngress}, actualNS)).ToNot(HaveOccurred())

			Expect(actualNS.Labels).To(And(
				HaveKeyWithValue("istio-operator-managed", "Reconcile"),
				HaveKeyWithValue("istio-injection", "disabled"),
				HaveKeyWithValue("high-availability-config.resources.gardener.cloud/consider", "true"),
				HaveKeyWithValue("gardener.cloud/role", "istio-ingress"),
			))
			Expect(actualNS.Annotations).To(And(
				HaveKeyWithValue("high-availability-config.resources.gardener.cloud/zones", "a,b,c"),
			))
		})

		It("should successfully deploy all resources", func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstio), managedResourceIstio)).To(Succeed())
			Expect(managedResourceIstio).To(DeepEqual(&resourcesv1alpha1.ManagedResource{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ManagedResource",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResourceIstioName,
					Namespace:       deployNS,
					Labels:          map[string]string{"gardener.cloud/role": "seed-system-component"},
					ResourceVersion: "1",
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class: pointer.String("seed"),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResourceIstioSecret.Name,
					}},
					KeepObjects: pointer.Bool(false),
				},
			}))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSecret), managedResourceIstioSecret)).To(Succeed())
			Expect(managedResourceIstioSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceIstioSecret.Data).To(HaveLen(30))

			By("Verify istio-system resources in `Ignore` mode")
			Expect(string(managedResourceIstioSecret.Data["istio-istiod_templates_configmap.yaml"])).To(Equal(istiodConfigMap(true)))
			Expect(string(managedResourceIstioSecret.Data["istio-istiod_templates_deployment.yaml"])).To(Equal(istiodDeployment(true, "b1493f472a93df6b9764ee8530150faad2fb61b22b68acded9fdf4cf2a815c15")))
			Expect(string(managedResourceIstioSecret.Data["istio-istiod_templates_service.yaml"])).To(Equal(istiodService(true)))
			Expect(string(managedResourceIstioSecret.Data["istio-istiod_templates_clusterrole.yaml"])).To(Equal(istioClusterRole(true)))
			Expect(string(managedResourceIstioSecret.Data["istio-istiod_templates_clusterrolebinding.yaml"])).To(Equal(istiodClusterRoleBinding(true)))
			Expect(string(managedResourceIstioSecret.Data["istio-istiod_templates_destinationrule.yaml"])).To(Equal(istiodDestinationRule(true)))
			Expect(string(managedResourceIstioSecret.Data["istio-istiod_templates_namespace.yaml"])).To(BeEmpty())
			Expect(string(managedResourceIstioSecret.Data["istio-istiod_templates_peerauthentication.yaml"])).To(Equal(istiodPeerAuthentication(true)))
			Expect(string(managedResourceIstioSecret.Data["istio-istiod_templates_poddisruptionbudget.yaml"])).To(Equal(istiodPodDisruptionBudgetFor(true, true)))
			Expect(string(managedResourceIstioSecret.Data["istio-istiod_templates_role.yaml"])).To(Equal(istiodRole(true)))
			Expect(string(managedResourceIstioSecret.Data["istio-istiod_templates_rolebinding.yaml"])).To(Equal(istiodRoleBinding(true)))
			Expect(string(managedResourceIstioSecret.Data["istio-istiod_templates_serviceaccount.yaml"])).To(Equal(istiodServiceAccount(true)))
			Expect(string(managedResourceIstioSecret.Data["istio-istiod_templates_sidecar.yaml"])).To(Equal(istiodSidecar(true)))
			Expect(string(managedResourceIstioSecret.Data["istio-istiod_templates_autoscale.yaml"])).To(Equal(istiodAutoscale(true)))
			Expect(string(managedResourceIstioSecret.Data["istio-istiod_templates_validatingwebhookconfiguration.yaml"])).To(Equal(istiodValidationWebhook(true)))

			By("Verify istio-ingress resources")
			Expect(string(managedResourceIstioSecret.Data["istio-ingress_templates_autoscale_test-ingress.yaml"])).To(Equal(istioIngressAutoscaler(nil, nil)))
			Expect(string(managedResourceIstioSecret.Data["istio-ingress_templates_bootstrap-config-override_test-ingress.yaml"])).To(Equal(istioIngressBootstrapConfig))
			Expect(string(managedResourceIstioSecret.Data["istio-ingress_templates_envoy-filter_test-ingress.yaml"])).To(Equal(istioIngressEnvoyFilter))
			Expect(string(managedResourceIstioSecret.Data["istio-ingress_templates_poddisruptionbudget_test-ingress.yaml"])).To(Equal(istioIngressPodDisruptionBudgetFor(true)))
			Expect(string(managedResourceIstioSecret.Data["istio-ingress_templates_role_test-ingress.yaml"])).To(Equal(istioIngressRole))
			Expect(string(managedResourceIstioSecret.Data["istio-ingress_templates_rolebindings_test-ingress.yaml"])).To(Equal(istioIngressRoleBinding))
			Expect(string(managedResourceIstioSecret.Data["istio-ingress_templates_service_test-ingress.yaml"])).To(Equal(istioIngressService(nil)))
			Expect(string(managedResourceIstioSecret.Data["istio-ingress_templates_serviceaccount_test-ingress.yaml"])).To(Equal(istioIngressServiceAccount))
			Expect(string(managedResourceIstioSecret.Data["istio-ingress_templates_deployment_test-ingress.yaml"])).To(Equal(istioIngressDeployment(true, nil)))

			By("Verify istio-proxy-protocol resources")
			Expect(string(managedResourceIstioSecret.Data["istio-ingress_templates_proxy-protocol-envoyfilter_test-ingress.yaml"])).To(Equal(istioProxyProtocolEnvoyFilter))
			Expect(string(managedResourceIstioSecret.Data["istio-ingress_templates_proxy-protocol-gateway_test-ingress.yaml"])).To(Equal(istioProxyProtocolGateway))
			Expect(string(managedResourceIstioSecret.Data["istio-ingress_templates_proxy-protocol-virtualservice_test-ingress.yaml"])).To(Equal(istioProxyProtocolVirtualService))

			By("Verify istio-reversed-vpn resources")
			Expect(string(managedResourceIstioSecret.Data["istio-ingress_templates_vpn-gateway_test-ingress.yaml"])).To(Equal(istioIngressVPNGateway))
			Expect(string(managedResourceIstioSecret.Data["istio-ingress_templates_vpn-envoy-filter_test-ingress.yaml"])).To(Equal(istioIngressEnvoyVPNFilter))

			By("Verify istio-system resources")
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSystemSecret), managedResourceIstioSystemSecret)).To(Succeed())
			Expect(managedResourceIstioSystemSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceIstioSystemSecret.Data).To(HaveLen(15))

			Expect(string(managedResourceIstioSystemSecret.Data["istio-istiod_templates_configmap.yaml"])).To(Equal(istiodConfigMap(false)))
			Expect(string(managedResourceIstioSystemSecret.Data["istio-istiod_templates_deployment.yaml"])).To(Equal(istiodDeployment(false, "d34796e6fc25a26d4a8a4cb3276e34961b18f867d70f5a1984255d57bfefb4c6")))
			Expect(string(managedResourceIstioSystemSecret.Data["istio-istiod_templates_service.yaml"])).To(Equal(istiodService(false)))
			Expect(string(managedResourceIstioSystemSecret.Data["istio-istiod_templates_clusterrole.yaml"])).To(Equal(istioClusterRole(false)))
			Expect(string(managedResourceIstioSystemSecret.Data["istio-istiod_templates_clusterrolebinding.yaml"])).To(Equal(istiodClusterRoleBinding(false)))
			Expect(string(managedResourceIstioSystemSecret.Data["istio-istiod_templates_destinationrule.yaml"])).To(Equal(istiodDestinationRule(false)))
			Expect(string(managedResourceIstioSystemSecret.Data["istio-istiod_templates_namespace.yaml"])).To(BeEmpty())
			Expect(string(managedResourceIstioSystemSecret.Data["istio-istiod_templates_peerauthentication.yaml"])).To(Equal(istiodPeerAuthentication(false)))
			Expect(string(managedResourceIstioSystemSecret.Data["istio-istiod_templates_poddisruptionbudget.yaml"])).To(Equal(istiodPodDisruptionBudgetFor(true, false)))
			Expect(string(managedResourceIstioSystemSecret.Data["istio-istiod_templates_role.yaml"])).To(Equal(istiodRole(false)))
			Expect(string(managedResourceIstioSystemSecret.Data["istio-istiod_templates_rolebinding.yaml"])).To(Equal(istiodRoleBinding(false)))
			Expect(string(managedResourceIstioSystemSecret.Data["istio-istiod_templates_serviceaccount.yaml"])).To(Equal(istiodServiceAccount(false)))
			Expect(string(managedResourceIstioSystemSecret.Data["istio-istiod_templates_sidecar.yaml"])).To(Equal(istiodSidecar(false)))
			Expect(string(managedResourceIstioSystemSecret.Data["istio-istiod_templates_autoscale.yaml"])).To(Equal(istiodAutoscale(false)))
			Expect(string(managedResourceIstioSystemSecret.Data["istio-istiod_templates_validatingwebhookconfiguration.yaml"])).To(Equal(istiodValidationWebhook(false)))
		})

		Context("with outdated stats filters", func() {
			var statsFilterNames []string

			BeforeEach(func() {
				statsFilterNames = []string{"tcp-stats-filter-1.11", "stats-filter-1.11", "tcp-stats-filter-1.12", "stats-filter-1.12"}

				for _, ingressGateway := range igw {
					for _, statsFilterName := range statsFilterNames {
						statsFilter := istionetworkingv1alpha3.EnvoyFilter{
							ObjectMeta: metav1.ObjectMeta{
								Name:      statsFilterName,
								Namespace: ingressGateway.Namespace,
							},
						}
						Expect(c.Create(ctx, &statsFilter)).To(Succeed())
					}
				}
			})

			It("should have removed all outdated stats filters", func() {
				for _, ingressGateway := range igw {
					for _, statsFilterName := range statsFilterNames {
						statsFilter := &istionetworkingv1alpha3.EnvoyFilter{
							ObjectMeta: metav1.ObjectMeta{
								Name:      statsFilterName,
								Namespace: ingressGateway.Namespace,
							},
						}
						Expect(c.Get(ctx, client.ObjectKeyFromObject(statsFilter), statsFilter)).To(BeNotFoundError())
					}
				}
			})
		})

		Context("kubernetes version <v1.21", func() {
			BeforeEach(func() {
				renderer = chartrenderer.NewWithServerVersion(&version.Info{GitVersion: "v1.20.11"})

				istiod = NewIstio(
					c,
					renderer,
					Values{
						Istiod: IstiodValues{
							Enabled:     true,
							Image:       "foo/bar",
							Namespace:   deployNS,
							TrustDomain: "foo.local",
						},
						IngressGateway: igw,
					},
				)
			})

			It("should succesfully deploy pdb with correct apiVersion ", func() {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSecret), managedResourceIstioSecret)).To(Succeed())
				Expect(managedResourceIstioSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceIstioSecret.Data).To(HaveLen(30))

				Expect(string(managedResourceIstioSecret.Data["istio-istiod_templates_poddisruptionbudget.yaml"])).To(Equal(istiodPodDisruptionBudgetFor(false, true)))
				Expect(string(managedResourceIstioSecret.Data["istio-ingress_templates_poddisruptionbudget_test-ingress.yaml"])).To(Equal(istioIngressPodDisruptionBudgetFor(false)))
			})
		})

		Context("horizontal ingress gateway scaling", func() {
			BeforeEach(func() {
				minReplicas = 3
				maxReplicas = 8
				igw[0].MinReplicas = &minReplicas
				igw[0].MaxReplicas = &maxReplicas
				istiod = NewIstio(
					c,
					renderer,
					Values{
						Istiod: IstiodValues{
							Enabled:     true,
							Image:       "foo/bar",
							Namespace:   deployNS,
							TrustDomain: "foo.local",
							Zones:       []string{"a", "b", "c"},
						},
						IngressGateway: igw,
					},
				)
			})

			It("should successfully deploy correct autoscaling", func() {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSecret), managedResourceIstioSecret)).To(Succeed())
				Expect(string(managedResourceIstioSecret.Data["istio-ingress_templates_autoscale_test-ingress.yaml"])).To(Equal(istioIngressAutoscaler(&minReplicas, &maxReplicas)))
				Expect(string(managedResourceIstioSecret.Data["istio-ingress_templates_deployment_test-ingress.yaml"])).To(Equal(istioIngressDeployment(true, &minReplicas)))
			})
		})

		Context("external traffic policy cluster", func() {
			BeforeEach(func() {
				externalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeCluster
				igw[0].ExternalTrafficPolicy = &externalTrafficPolicy
				istiod = NewIstio(
					c,
					renderer,
					Values{
						Istiod: IstiodValues{
							Enabled:     true,
							Image:       "foo/bar",
							Namespace:   deployNS,
							TrustDomain: "foo.local",
							Zones:       []string{"a", "b", "c"},
						},
						IngressGateway: igw,
					},
				)
			})

			It("should successfully deploy correct external traffic policy", func() {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSecret), managedResourceIstioSecret)).To(Succeed())
				Expect(string(managedResourceIstioSecret.Data["istio-ingress_templates_service_test-ingress.yaml"])).To(Equal(istioIngressService(&externalTrafficPolicy)))
			})
		})

		Context("external traffic policy local", func() {
			BeforeEach(func() {
				externalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeLocal
				igw[0].ExternalTrafficPolicy = &externalTrafficPolicy
				istiod = NewIstio(
					c,
					renderer,
					Values{
						Istiod: IstiodValues{
							Enabled:     true,
							Image:       "foo/bar",
							Namespace:   deployNS,
							TrustDomain: "foo.local",
							Zones:       []string{"a", "b", "c"},
						},
						IngressGateway: igw,
					},
				)
			})

			It("should successfully deploy correct external traffic policy", func() {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSecret), managedResourceIstioSecret)).To(Succeed())
				Expect(string(managedResourceIstioSecret.Data["istio-ingress_templates_service_test-ingress.yaml"])).To(Equal(istioIngressService(&externalTrafficPolicy)))
			})
		})

		Context("VPN disabled", func() {
			BeforeEach(func() {
				for i := range igw {
					igw[i].VPNEnabled = false
				}

				istiod = NewIstio(
					c,
					renderer,
					Values{
						Istiod: IstiodValues{
							Enabled:     true,
							Image:       "foo/bar",
							Namespace:   deployNS,
							TrustDomain: "foo.local",
							Zones:       []string{"a", "b", "c"},
						},
						IngressGateway: igw,
					},
				)
			})

			It("should successfully deploy all resources", func() {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSecret), managedResourceIstioSecret)).To(Succeed())
				Expect(managedResourceIstioSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceIstioSecret.Data).To(HaveLen(30))

				Expect(string(managedResourceIstioSecret.Data["istio-ingress_templates_vpn-gateway_test-ingress.yaml"])).To(BeEmpty())
				Expect(string(managedResourceIstioSecret.Data["istio-ingress_templates_vpn-envoy-filter_test-ingress.yaml"])).To(BeEmpty())
				Expect(string(managedResourceIstioSecret.Data["istio-ingress_templates_deployment_test-ingress.yaml"])).To(Equal(istioIngressDeployment(false, nil)))
			})
		})

		Context("Proxy Protocol disabled", func() {
			BeforeEach(func() {
				for i := range igw {
					igw[i].ProxyProtocolEnabled = false
				}

				istiod = NewIstio(
					c,
					renderer,
					Values{
						Istiod: IstiodValues{
							Enabled:     true,
							Image:       "foo/bar",
							Namespace:   deployNS,
							TrustDomain: "foo.local",
							Zones:       []string{"a", "b", "c"},
						},
						IngressGateway: igw,
					},
				)
			})

			It("should successfully deploy all resources", func() {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSecret), managedResourceIstioSecret)).To(Succeed())
				Expect(managedResourceIstioSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceIstioSecret.Data).To(HaveLen(30))

				Expect(string(managedResourceIstioSecret.Data["istio-ingress_templates_proxy-protocol-envoyfilter_test-ingress.yaml"])).To(BeEmpty())
				Expect(string(managedResourceIstioSecret.Data["istio-ingress_templates_proxy-protocol-gateway_test-ingress.yaml"])).To(BeEmpty())
				Expect(string(managedResourceIstioSecret.Data["istio-ingress_templates_proxy-protocol-virtualservice_test-ingress.yaml"])).To(BeEmpty())
			})
		})

		Context("istiod disabled", func() {
			BeforeEach(func() {
				for i := range igw {
					igw[i].ProxyProtocolEnabled = false
				}

				istiod = NewIstio(
					c,
					renderer,
					Values{
						Istiod: IstiodValues{
							Enabled:     false,
							Image:       "foo/bar",
							Namespace:   deployNS,
							TrustDomain: "foo.local",
							Zones:       []string{"a", "b", "c"},
						},
						IngressGateway: igw,
					},
				)
			})

			It("should successfully deploy all resources", func() {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSecret), managedResourceIstioSecret)).To(Succeed())
				Expect(managedResourceIstioSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceIstioSecret.Data).To(HaveLen(15))

				Expect(managedResourceIstioSecret.Data).ToNot(HaveKey("istio-istiod_templates_configmap.yaml"))
				Expect(managedResourceIstioSecret.Data).ToNot(HaveKey("istio-istiod_templates_deployment.yaml"))
				Expect(managedResourceIstioSecret.Data).ToNot(HaveKey("istio-istiod_templates_service.yaml"))
				Expect(managedResourceIstioSecret.Data).ToNot(HaveKey("istio-istiod_templates_clusterrole.yaml"))
				Expect(managedResourceIstioSecret.Data).ToNot(HaveKey("istio-istiod_templates_clusterrolebinding.yaml"))
				Expect(managedResourceIstioSecret.Data).ToNot(HaveKey("istio-istiod_templates_destinationrule.yaml"))
				Expect(managedResourceIstioSecret.Data).ToNot(HaveKey("istio-istiod_templates_namespace.yaml"))
				Expect(managedResourceIstioSecret.Data).ToNot(HaveKey("istio-istiod_templates_peerauthentication.yaml"))
				Expect(managedResourceIstioSecret.Data).ToNot(HaveKey("istio-istiod_templates_poddisruptionbudget.yaml"))
				Expect(managedResourceIstioSecret.Data).ToNot(HaveKey("istio-istiod_templates_role.yaml"))
				Expect(managedResourceIstioSecret.Data).ToNot(HaveKey("istio-istiod_templates_rolebinding.yaml"))
				Expect(managedResourceIstioSecret.Data).ToNot(HaveKey("istio-istiod_templates_serviceaccount.yaml"))
				Expect(managedResourceIstioSecret.Data).ToNot(HaveKey("istio-istiod_templates_sidecar.yaml"))
				Expect(managedResourceIstioSecret.Data).ToNot(HaveKey("istio-istiod_templates_autoscale.yaml"))
				Expect(managedResourceIstioSecret.Data).ToNot(HaveKey("istio-istiod_templates_validatingwebhookconfiguration.yaml"))
			})
		})
	})

	Describe("#Destroy", func() {
		BeforeEach(func() {
			Expect(istiod.Deploy(ctx)).To(Succeed())
		})

		It("should successfully destroy all resources", func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstio), managedResourceIstio)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSecret), managedResourceIstioSecret)).To(Succeed())

			Expect(istiod.Destroy(ctx)).To(Succeed())

			namespace := &corev1.Namespace{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstio), managedResourceIstio)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResourceIstio.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSecret), managedResourceIstioSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceIstioSecret.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSystem), managedResourceIstioSystem)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResourceIstioSystem.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSystemSecret), managedResourceIstioSystemSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceIstioSystemSecret.Name)))
			Expect(c.Get(ctx, client.ObjectKey{Name: deployNS}, namespace)).To(MatchError(apierrors.NewNotFound(corev1.Resource("namespaces"), deployNS)))
			Expect(c.Get(ctx, client.ObjectKey{Name: deployNSIngress}, namespace)).To(MatchError(apierrors.NewNotFound(corev1.Resource("namespaces"), deployNSIngress)))
		})

		Context("istiod disabled", func() {
			It("should not destroy istiod resources", func() {
				istiod = NewIstio(
					c,
					renderer,
					Values{
						Istiod: IstiodValues{
							Enabled:     false,
							Image:       "foo/bar",
							Namespace:   deployNS,
							TrustDomain: "foo.local",
						},
						IngressGateway: igw,
					},
				)

				Expect(istiod.Destroy(ctx)).To(Succeed())

				namespace := &corev1.Namespace{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSystem), managedResourceIstio)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSystemSecret), managedResourceIstioSecret)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKey{Name: deployNS}, namespace)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKey{Name: deployNSIngress}, namespace)).To(MatchError(apierrors.NewNotFound(corev1.Resource("namespaces"), deployNSIngress)))
			})
		})
	})

	Context("waiting functions", func() {
		var (
			fakeOps *retryfake.Ops
		)

		BeforeEach(func() {
			fakeOps = &retryfake.Ops{MaxAttempts: 1}

			DeferCleanup(test.WithVars(
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			))
		})

		Describe("#Wait", func() {
			It("should fail because reading the ManagedResource fails", func() {
				Expect(istiod.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the ManagedResource doesn't become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceIstioName,
						Namespace:  deployNS,
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

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceIstioSystemName,
						Namespace:  deployNS,
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

				Expect(istiod.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should successfully wait for the managed resource to become healthy", func() {
				fakeOps.MaxAttempts = 2

				for _, mr := range []string{managedResourceIstioName, managedResourceIstioSystemName} {
					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       mr,
							Namespace:  deployNS,
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
				}

				Expect(istiod.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, managedResourceIstio)).To(Succeed())

				Expect(istiod.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it's already removed", func() {
				Expect(istiod.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})

func makeIngressGateway(namespace string, annotations, labels map[string]string) []IngressGatewayValues {
	return []IngressGatewayValues{
		{
			Image:           "foo/bar",
			TrustDomain:     "foo.bar",
			IstiodNamespace: "istio-test-system",
			Annotations:     annotations,
			Labels:          labels,
			Ports: []corev1.ServicePort{
				{Name: "foo", Port: 999, TargetPort: intstr.FromInt(999)},
			},
			Namespace:            namespace,
			ProxyProtocolEnabled: true,
			VPNEnabled:           true,
		},
	}
}
