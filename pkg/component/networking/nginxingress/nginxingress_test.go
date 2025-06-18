// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nginxingress_test

import (
	"context"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/networking/nginxingress"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("NginxIngress", func() {
	var (
		ctx                   = context.TODO()
		namespace             = "some-namespace"
		imageController       = "some-image:some-tag"
		imageDefaultBackend   = "some-image2:some-tag2"
		firstWildcardIngress  = "*.ingress.seed.world"
		secondWildcardIngress = "*.world.seed"
		istioLabelKey         = "my"
		istioLabelValue       = "istio"

		c            client.Client
		nginxIngress component.DeployWaiter
		values       Values

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret
		managedResourceName   string
		manifests             []string
		expectedManifests     []string

		configMapData, loadBalancerAnnotations map[string]string
		configMapName                          string
	)

	JustBeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

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

	Context("Cluster type Seed", func() {
		BeforeEach(func() {
			managedResourceName = "nginx-ingress"

			configMapData = map[string]string{
				"foo":  "bar",
				"dot":  "3",
				"dash": "false",
			}

			loadBalancerAnnotations = map[string]string{
				"some": "value",
			}
			configMapName = "nginx-ingress-controller-" + utils.ComputeConfigMapChecksum(configMapData)[:8]

			values = Values{
				ClusterType:               component.ClusterTypeSeed,
				TargetNamespace:           namespace,
				IngressClass:              v1beta1constants.SeedNginxIngressClass,
				PriorityClassName:         v1beta1constants.PriorityClassNameSeedSystem600,
				ImageController:           imageController,
				ImageDefaultBackend:       imageDefaultBackend,
				ConfigData:                configMapData,
				LoadBalancerAnnotations:   loadBalancerAnnotations,
				VPAEnabled:                true,
				WildcardIngressDomains:    []string{firstWildcardIngress, secondWildcardIngress},
				IstioIngressGatewayLabels: map[string]string{istioLabelKey: istioLabelValue},
			}
		})

		Describe("#Deploy", func() {
			var (
				configMapYAMLFor = func(configMapName string) string {
					out := `apiVersion: v1
data:
  dash: "false"
  dot: "3"
  foo: bar
immutable: true
kind: ConfigMap
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: controller
    resources.gardener.cloud/garbage-collectable-reference: "true"
  name: ` + configMapName + `
  namespace: ` + namespace + `
`
					return out
				}

				clusterRoleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
  name: gardener.cloud:seed:nginx-ingress
rules:
- apiGroups:
  - ""
  resources:
  - endpoints
  - nodes
  - pods
  - secrets
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
  - ""
  resources:
  - services
  - configmaps
  verbs:
  - get
  - list
  - update
  - watch
- apiGroups:
  - networking.k8s.io
  resources:
  - ingresses
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - events
  verbs:
  - create
  - patch
- apiGroups:
  - networking.k8s.io
  resources:
  - ingresses/status
  verbs:
  - update
- apiGroups:
  - networking.k8s.io
  resources:
  - ingressclasses
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - coordination.k8s.io
  resources:
  - leases
  verbs:
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
`
				clusterRoleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
  name: gardener.cloud:seed:nginx-ingress
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gardener.cloud:seed:nginx-ingress
subjects:
- kind: ServiceAccount
  name: nginx-ingress
  namespace: ` + namespace + `
`
				roleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
  name: gardener.cloud:seed:nginx-ingress:role
  namespace: ` + namespace + `
rules:
- apiGroups:
  - ""
  resources:
  - configmaps
  - namespaces
  - pods
  - secrets
  verbs:
  - get
- apiGroups:
  - ""
  resources:
  - endpoints
  verbs:
  - create
  - get
  - update
- apiGroups:
  - coordination.k8s.io
  resourceNames:
  - ingress-controller-seed-leader
  resources:
  - leases
  verbs:
  - get
  - update
- apiGroups:
  - coordination.k8s.io
  resources:
  - leases
  verbs:
  - create
- apiGroups:
  - discovery.k8s.io
  resources:
  - endpointslices
  verbs:
  - get
  - list
  - watch
`
				roleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
  name: gardener.cloud:seed:nginx-ingress:role-binding
  namespace: ` + namespace + `
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: gardener.cloud:seed:nginx-ingress:role
subjects:
- kind: ServiceAccount
  name: nginx-ingress
  namespace: ` + namespace + `
`
				serviceControllerYAML = `apiVersion: v1
kind: Service
metadata:
  annotations:
    networking.istio.io/exportTo: '*'
    networking.resources.gardener.cloud/namespace-selectors: '[{"matchLabels":{"gardener.cloud/role":"istio-ingress"}}]'
    some: value
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: controller
  name: nginx-ingress-controller
  namespace: ` + namespace + `
spec:
  ports:
  - name: http
    port: 80
    protocol: TCP
    targetPort: 80
  - name: https
    port: 443
    protocol: TCP
    targetPort: 443
  selector:
    app: nginx-ingress
    component: controller
  type: ClusterIP
status:
  loadBalancer: {}
`
				serviceBackendYAML = `apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: nginx-ingress-k8s-backend
  name: nginx-ingress-k8s-backend
  namespace: ` + namespace + `
spec:
  ports:
  - port: 80
    targetPort: 8080
  selector:
    app: nginx-ingress
    component: nginx-ingress-k8s-backend
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
    app: nginx-ingress
  name: nginx-ingress
  namespace: ` + namespace + `
`
				ingressClassYAML = `apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: controller
  name: ` + v1beta1constants.SeedNginxIngressClass + `
spec:
  controller: k8s.io/` + v1beta1constants.SeedNginxIngressClass + `
`
				podDisruptionBudgetYAML = `apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: controller
  name: nginx-ingress-controller
  namespace: ` + namespace + `
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: nginx-ingress
      component: controller
  unhealthyPodEvictionPolicy: AlwaysAllow
status:
  currentHealthy: 0
  desiredHealthy: 0
  disruptionsAllowed: 0
  expectedPods: 0
`
				vpaYAML = `apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  creationTimestamp: null
  name: nginx-ingress-controller
  namespace: ` + namespace + `
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: '*'
      minAllowed:
        memory: 100Mi
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: nginx-ingress-controller
  updatePolicy:
    updateMode: Auto
status: {}
`
				deploymentBackendYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: nginx-ingress-k8s-backend
  name: nginx-ingress-k8s-backend
  namespace: ` + namespace + `
spec:
  replicas: 1
  revisionHistoryLimit: 2
  selector:
    matchLabels:
      app: nginx-ingress
      component: nginx-ingress-k8s-backend
      release: addons
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: nginx-ingress
        component: nginx-ingress-k8s-backend
        release: addons
    spec:
      containers:
      - image: ` + imageDefaultBackend + `
        imagePullPolicy: IfNotPresent
        livenessProbe:
          httpGet:
            path: /healthy
            port: 8080
            scheme: HTTP
          initialDelaySeconds: 30
          timeoutSeconds: 5
        name: nginx-ingress-k8s-backend
        ports:
        - containerPort: 8080
          protocol: TCP
        resources:
          limits:
            memory: 100Mi
          requests:
            cpu: 20m
            memory: 20Mi
        securityContext:
          allowPrivilegeEscalation: false
      priorityClassName: gardener-system-600
      securityContext:
        fsGroup: 65534
        runAsNonRoot: true
        runAsUser: 65534
      terminationGracePeriodSeconds: 60
status: {}
`
				deploymentControllerYAMLFor = func(configMapName string) string {
					out := `apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    ` + references.AnnotationKey(references.KindConfigMap, configMapName) + `: ` + configMapName + `
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: controller
  name: nginx-ingress-controller
  namespace: ` + namespace + `
spec:
  replicas: 2
  revisionHistoryLimit: 2
  selector:
    matchLabels:
      app: nginx-ingress
      component: controller
      release: addons
  strategy: {}
  template:
    metadata:
      annotations:
        ` + references.AnnotationKey(references.KindConfigMap, configMapName) + `: ` + configMapName + `
      creationTimestamp: null
      labels:
        app: nginx-ingress
        component: controller
        networking.gardener.cloud/to-dns: allowed
        networking.gardener.cloud/to-runtime-apiserver: allowed
        networking.resources.gardener.cloud/to-nginx-ingress-k8s-backend-tcp-8080: allowed
        release: addons
        seccompprofile.resources.gardener.cloud/skip: "true"
    spec:
      containers:
      - args:
        - /nginx-ingress-controller
        - --default-backend-service=` + namespace + `/nginx-ingress-k8s-backend
        - --enable-ssl-passthrough=true
        - --publish-service=` + namespace + `/nginx-ingress-controller
        - --election-id=ingress-controller-seed-leader
        - --update-status=true
        - --annotations-prefix=nginx.ingress.kubernetes.io
        - --configmap=` + namespace + `/` + configMapName + `
        - --ingress-class=` + v1beta1constants.SeedNginxIngressClass + `
        - --controller-class=k8s.io/` + v1beta1constants.SeedNginxIngressClass + `
        - --enable-annotation-validation=true
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        image: ` + imageController + `
        imagePullPolicy: IfNotPresent
        livenessProbe:
          failureThreshold: 3
          httpGet:
            path: /healthz
            port: 10254
            scheme: HTTP
          initialDelaySeconds: 40
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 1
        name: nginx-ingress-controller
        ports:
        - containerPort: 80
          name: http
          protocol: TCP
        - containerPort: 443
          name: https
          protocol: TCP
        readinessProbe:
          failureThreshold: 3
          httpGet:
            path: /healthz
            port: 10254
            scheme: HTTP
          initialDelaySeconds: 40
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 1
        resources:
          limits:
            memory: 1500Mi
          requests:
            cpu: 100m
            memory: 375Mi
        securityContext:
          allowPrivilegeEscalation: true
          capabilities:
            add:
            - NET_BIND_SERVICE
            - SYS_CHROOT` + `
            drop:
            - ALL
          runAsNonRoot: true
          runAsUser: 101
          seccompProfile:
            type: Unconfined
      priorityClassName: gardener-system-600
      serviceAccountName: nginx-ingress
      terminationGracePeriodSeconds: 60
status: {}
`
					return out
				}

				destinationRuleYAML = `apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: controller
  name: nginx-ingress-controller
  namespace: ` + namespace + `
spec:
  exportTo:
  - '*'
  host: nginx-ingress-controller.` + namespace + `.svc.cluster.local
  trafficPolicy:
    connectionPool:
      tcp:
        maxConnections: 5000
        tcpKeepalive:
          interval: 75s
          time: 7200s
    loadBalancer:
      localityLbSetting:
        enabled: true
        failoverPriority:
        - topology.kubernetes.io/zone
    outlierDetection: {}
    tls: {}
status: {}
`

				gatewayYAML = `apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: controller
  name: nginx-ingress-controller
  namespace: ` + namespace + `
spec:
  selector:
    ` + istioLabelKey + `: ` + istioLabelValue + `
  servers:
  - hosts:
    - '` + firstWildcardIngress + `'
    - '` + secondWildcardIngress + `'
    port:
      name: tls
      number: 443
      protocol: TLS
    tls: {}
status: {}
`

				virtualServiceYAML = func(index int) string {
					return `apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: controller
  name: nginx-ingress-controller-` + strconv.Itoa(index) + `
  namespace: ` + namespace + `
spec:
  exportTo:
  - '*'
  gateways:
  - nginx-ingress-controller
  hosts:
  - '` + values.WildcardIngressDomains[index] + `'
  tls:
  - match:
    - port: 443
      sniHosts:
      - '` + values.WildcardIngressDomains[index] + `'
    route:
    - destination:
        host: nginx-ingress-controller.` + namespace + `.svc.cluster.local
        port:
          number: 443
status: {}
`
				}

				leaseYAML = `apiVersion: coordination.k8s.io/v1
kind: Lease
metadata:
  annotations:
    resources.gardener.cloud/ignore: "true"
  creationTimestamp: null
  name: ingress-controller-seed-leader
  namespace: ` + namespace + `
spec: {}
`
			)

			It("should successfully deploy all resources", func() {
				nginxIngress = New(c, namespace, values)

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())

				Expect(nginxIngress.Deploy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				expectedMr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResource.Name,
						Namespace:       managedResource.Namespace,
						Labels:          map[string]string{"gardener.cloud/role": "seed-system-component"},
						ResourceVersion: "1",
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						Class: ptr.To("seed"),
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

				expectedManifests = []string{
					clusterRoleYAML,
					clusterRoleBindingYAML,
					roleYAML,
					roleBindingYAML,
					serviceControllerYAML,
					serviceBackendYAML,
					serviceAccountYAML,
					vpaYAML,
					configMapYAMLFor(configMapName),
					deploymentBackendYAML,
					deploymentControllerYAMLFor(configMapName),
					ingressClassYAML,
					destinationRuleYAML,
					gatewayYAML,
					podDisruptionBudgetYAML,
					virtualServiceYAML(0),
					virtualServiceYAML(1),
					leaseYAML,
				}

				Expect(manifests).To(ConsistOf(expectedManifests))
			})
		})

		Describe("#Destroy", func() {
			It("should successfully destroy all resources", func() {
				nginxIngress = New(c, namespace, values)

				Expect(c.Create(ctx, managedResource)).To(Succeed())
				Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

				Expect(nginxIngress.Destroy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
			})
		})
	})

	Context("Cluster type Shoot", func() {
		var (
			configMapYAML = `apiVersion: v1
data:
  dash: "false"
  dot: "3"
  foo: bar
kind: ConfigMap
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: controller
    release: addons
  name: addons-nginx-ingress-controller
  namespace: kube-system
`

			ingressClassYAML = `apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: controller
    gardener.cloud/role: optional-addon
    origin: gardener
    release: addons
  name: nginx
spec:
  controller: k8s.io/nginx
`

			serviceAccountYAML = `apiVersion: v1
automountServiceAccountToken: false
kind: ServiceAccount
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    release: addons
  name: addons-nginx-ingress
  namespace: kube-system
`

			networkPolicyYAML = `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  annotations:
    gardener.cloud/description: Allows all egress and ingress traffic for the nginx-ingress
      controller.
  creationTimestamp: null
  labels:
    origin: gardener
  name: gardener.cloud--allow-to-from-nginx
  namespace: kube-system
spec:
  egress:
  - {}
  ingress:
  - {}
  podSelector:
    matchLabels:
      app: nginx-ingress
      component: controller
      release: addons
  policyTypes:
  - Ingress
  - Egress
`

			serviceBackendYAML = `apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: nginx-ingress-k8s-backend
    release: addons
  name: addons-nginx-ingress-nginx-ingress-k8s-backend
  namespace: kube-system
spec:
  ports:
  - port: 80
    targetPort: 8080
  selector:
    app: nginx-ingress
    component: nginx-ingress-k8s-backend
    release: addons
  type: ClusterIP
status:
  loadBalancer: {}
`

			clusterRoleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    release: addons
  name: addons-nginx-ingress
rules:
- apiGroups:
  - ""
  resources:
  - endpoints
  - nodes
  - pods
  - secrets
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
  - ""
  resources:
  - services
  - configmaps
  verbs:
  - get
  - list
  - update
  - watch
- apiGroups:
  - networking.k8s.io
  resources:
  - ingresses
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - events
  verbs:
  - create
  - patch
- apiGroups:
  - networking.k8s.io
  resources:
  - ingresses/status
  verbs:
  - update
- apiGroups:
  - networking.k8s.io
  resources:
  - ingressclasses
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - coordination.k8s.io
  resources:
  - leases
  verbs:
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
`

			clusterRoleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  annotations:
    resources.gardener.cloud/delete-on-invalid-update: "true"
  creationTimestamp: null
  labels:
    app: nginx-ingress
    release: addons
  name: addons-nginx-ingress
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: addons-nginx-ingress
subjects:
- kind: ServiceAccount
  name: addons-nginx-ingress
  namespace: kube-system
`

			serviceControllerYAML = `apiVersion: v1
kind: Service
metadata:
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-proxy-protocol: '*'
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: controller
    release: addons
  name: addons-nginx-ingress-controller
  namespace: kube-system
spec:
  loadBalancerSourceRanges:
  - 10.0.0.0/8
  ports:
  - name: http
    port: 80
    protocol: TCP
    targetPort: 80
  - name: https
    port: 443
    protocol: TCP
    targetPort: 443
  selector:
    app: nginx-ingress
    component: controller
    release: addons
  type: LoadBalancer
status:
  loadBalancer: {}
`

			deploymentBackendYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: nginx-ingress-k8s-backend
    gardener.cloud/role: optional-addon
    origin: gardener
    release: addons
  name: addons-nginx-ingress-nginx-ingress-k8s-backend
  namespace: kube-system
spec:
  replicas: 1
  revisionHistoryLimit: 2
  selector:
    matchLabels:
      app: nginx-ingress
      component: nginx-ingress-k8s-backend
      release: addons
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: nginx-ingress
        component: nginx-ingress-k8s-backend
        gardener.cloud/role: optional-addon
        origin: gardener
        release: addons
    spec:
      containers:
      - image: ` + imageDefaultBackend + `
        imagePullPolicy: IfNotPresent
        livenessProbe:
          httpGet:
            path: /healthy
            port: 8080
            scheme: HTTP
          initialDelaySeconds: 30
          timeoutSeconds: 5
        name: nginx-ingress-nginx-ingress-k8s-backend
        ports:
        - containerPort: 8080
          protocol: TCP
        resources:
          limits:
            memory: 100Mi
          requests:
            cpu: 20m
            memory: 20Mi
        securityContext:
          allowPrivilegeEscalation: false
      nodeSelector:
        worker.gardener.cloud/system-components: "true"
      priorityClassName: gardener-shoot-system-600
      securityContext:
        fsGroup: 65534
        runAsNonRoot: true
        runAsUser: 65534
        seccompProfile:
          type: RuntimeDefault
        supplementalGroups:
        - 1
      terminationGracePeriodSeconds: 60
status: {}
`

			deploymentControllerYAMLFor = func(configMapData map[string]string) string {
				out := `apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    component: controller
    gardener.cloud/role: optional-addon
    origin: gardener
    release: addons
  name: addons-nginx-ingress-controller
  namespace: kube-system
spec:
  replicas: 1
  revisionHistoryLimit: 2
  selector:
    matchLabels:
      app: nginx-ingress
      component: controller
      release: addons
  strategy: {}
  template:
    metadata:
      annotations:
        checksum/config: ` + utils.ComputeChecksum(configMapData) + `
      creationTimestamp: null
      labels:
        app: nginx-ingress
        component: controller
        gardener.cloud/role: optional-addon
        origin: gardener
        release: addons
    spec:
      containers:
      - args:
        - /nginx-ingress-controller
        - --default-backend-service=kube-system/addons-nginx-ingress-nginx-ingress-k8s-backend
        - --enable-ssl-passthrough=true
        - --publish-service=kube-system/addons-nginx-ingress-controller
        - --election-id=ingress-controller-leader
        - --update-status=true
        - --annotations-prefix=nginx.ingress.kubernetes.io
        - --configmap=kube-system/addons-nginx-ingress-controller
        - --ingress-class=nginx
        - --controller-class=k8s.io/nginx
        - --enable-annotation-validation=true
        - --watch-ingress-without-class=true
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        image: ` + imageController + `
        imagePullPolicy: IfNotPresent
        livenessProbe:
          failureThreshold: 3
          httpGet:
            path: /healthz
            port: 10254
            scheme: HTTP
          initialDelaySeconds: 10
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 1
        name: nginx-ingress-controller
        ports:
        - containerPort: 80
          name: http
          protocol: TCP
        - containerPort: 443
          name: https
          protocol: TCP
        readinessProbe:
          failureThreshold: 3
          httpGet:
            path: /healthz
            port: 10254
            scheme: HTTP
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 1
        resources:
          limits:
            memory: 4Gi
          requests:
            cpu: 100m
            memory: 100Mi
        securityContext:
          allowPrivilegeEscalation: true
          capabilities:
            add:
            - NET_BIND_SERVICE
            - SYS_CHROOT
            drop:
            - ALL
          runAsNonRoot: true
          runAsUser: 101
          seccompProfile:
            type: Unconfined
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
      dnsPolicy: ClusterFirst
      nodeSelector:
        worker.gardener.cloud/system-components: "true"
      priorityClassName: gardener-shoot-system-600
      restartPolicy: Always
      schedulerName: default-scheduler
      serviceAccountName: addons-nginx-ingress
      terminationGracePeriodSeconds: 60
status: {}
`
				return out
			}

			roleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  creationTimestamp: null
  labels:
    app: nginx-ingress
    release: addons
  name: addons-nginx-ingress
  namespace: kube-system
rules:
- apiGroups:
  - ""
  resources:
  - configmaps
  - namespaces
  - pods
  - secrets
  verbs:
  - get
- apiGroups:
  - ""
  resources:
  - endpoints
  verbs:
  - create
  - get
  - update
- apiGroups:
  - coordination.k8s.io
  resourceNames:
  - ingress-controller-leader
  resources:
  - leases
  verbs:
  - get
  - update
- apiGroups:
  - coordination.k8s.io
  resources:
  - leases
  verbs:
  - create
- apiGroups:
  - discovery.k8s.io
  resources:
  - endpointslices
  verbs:
  - get
  - list
  - watch
`

			roleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  annotations:
    resources.gardener.cloud/delete-on-invalid-update: "true"
  creationTimestamp: null
  labels:
    app: nginx-ingress
    release: addons
  name: addons-nginx-ingress
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: addons-nginx-ingress
subjects:
- kind: ServiceAccount
  name: addons-nginx-ingress
  namespace: kube-system
`

			vpaYAML = `apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  creationTimestamp: null
  name: addons-nginx-ingress-controller
  namespace: kube-system
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: '*'
      minAllowed:
        memory: 100Mi
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: addons-nginx-ingress-controller
  updatePolicy:
    updateMode: Auto
status: {}
`

			leaseYAML = `apiVersion: coordination.k8s.io/v1
kind: Lease
metadata:
  annotations:
    resources.gardener.cloud/ignore: "true"
  creationTimestamp: null
  name: ingress-controller-leader
  namespace: kube-system
spec: {}
`
		)

		BeforeEach(func() {
			managedResourceName = "shoot-addon-nginx-ingress"

			configMapData = map[string]string{
				"foo":  "bar",
				"dot":  "3",
				"dash": "false",
			}

			values = Values{
				ClusterType:              component.ClusterTypeShoot,
				TargetNamespace:          metav1.NamespaceSystem,
				IngressClass:             v1beta1constants.ShootNginxIngressClass,
				PriorityClassName:        v1beta1constants.PriorityClassNameShootSystem600,
				ImageController:          imageController,
				ImageDefaultBackend:      imageDefaultBackend,
				LoadBalancerSourceRanges: []string{"10.0.0.0/8"},
				ConfigData:               configMapData,
			}
		})

		Describe("#Deploy", func() {
			JustBeforeEach(func() {
				nginxIngress = New(c, namespace, values)

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())

				Expect(nginxIngress.Deploy(ctx)).To(Succeed())

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

				expectedManifests = []string{
					serviceAccountYAML,
					clusterRoleYAML,
					clusterRoleBindingYAML,
					serviceControllerYAML,
					serviceBackendYAML,
					configMapYAML,
					roleYAML,
					roleBindingYAML,
					deploymentBackendYAML,
					networkPolicyYAML,
					ingressClassYAML,
					deploymentControllerYAMLFor(configMapData),
					leaseYAML,
				}
			})

			Context("w/ VPA", func() {
				BeforeEach(func() {
					values.VPAEnabled = true
				})

				It("should successfully deploy all resources", func() {
					expectedManifests = append(expectedManifests, vpaYAML)
					Expect(manifests).To(ConsistOf(expectedManifests))
				})
			})

			Context("w/o VPA", func() {
				BeforeEach(func() {
					values.VPAEnabled = false
				})

				It("should successfully deploy all resources", func() {
					Expect(manifests).To(ConsistOf(expectedManifests))
				})
			})
		})

		Describe("#Destroy", func() {
			It("should successfully destroy all resources", func() {
				nginxIngress = New(c, namespace, Values{ClusterType: component.ClusterTypeShoot})

				Expect(c.Create(ctx, managedResource)).To(Succeed())
				Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

				Expect(nginxIngress.Destroy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
			})
		})
	})

	Context("waiting functions", func() {
		var (
			fakeOps   *retryfake.Ops
			resetVars func()
		)

		BeforeEach(func() {
			managedResourceName = "nginx-ingress"
			namespace = "some-namespace"

			fakeOps = &retryfake.Ops{MaxAttempts: 1}
			resetVars = test.WithVars(
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			)
		})

		JustBeforeEach(func() {
			nginxIngress = New(c, namespace, Values{ClusterType: component.ClusterTypeSeed})
		})

		AfterEach(func() {
			resetVars()
		})

		Describe("#Wait", func() {
			It("should fail because reading the ManagedResource fails", func() {
				Expect(nginxIngress.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
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

				Expect(nginxIngress.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
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

				Expect(nginxIngress.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, managedResource)).To(Succeed())

				Expect(nginxIngress.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it's already removed", func() {
				Expect(nginxIngress.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})
