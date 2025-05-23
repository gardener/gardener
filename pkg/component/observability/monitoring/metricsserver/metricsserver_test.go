// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package metricsserver_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/observability/monitoring/metricsserver"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("MetricsServer", func() {
	var (
		fakeClient    client.Client
		sm            secretsmanager.Interface
		metricsServer component.DeployWaiter

		ctx               = context.Background()
		namespace         = "shoot--foo--bar"
		image             = "registry.k8s.io/metrics-server:v4.5.6"
		kubeAPIServerHost = "foo.bar"

		values Values

		serviceYAML = `apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    kubernetes.io/name: metrics-server
  name: metrics-server
  namespace: kube-system
spec:
  ports:
  - port: 443
    protocol: TCP
    targetPort: 8443
  selector:
    k8s-app: metrics-server
status:
  loadBalancer: {}
`
		apiServiceYAML = `apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  creationTimestamp: null
  name: v1beta1.metrics.k8s.io
spec:
  group: metrics.k8s.io
  groupPriorityMinimum: 100
  service:
    name: metrics-server
    namespace: kube-system
  version: v1beta1
  versionPriority: 100
status: {}
`
		vpaYAML = `apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  creationTimestamp: null
  name: metrics-server
  namespace: kube-system
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: '*'
      controlledValues: RequestsOnly
      minAllowed:
        memory: 60Mi
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: metrics-server
  updatePolicy:
    updateMode: Auto
status: {}
`
		clusterRoleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  name: system:metrics-server
rules:
- apiGroups:
  - ""
  resources:
  - pods
  - nodes
  - nodes/metrics
  - namespaces
  - configmaps
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
  name: system:metrics-server
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:metrics-server
subjects:
- kind: ServiceAccount
  name: metrics-server
  namespace: kube-system
`
		clusterRoleBindingAuthDelegatorYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  annotations:
    resources.gardener.cloud/delete-on-invalid-update: "true"
  creationTimestamp: null
  name: metrics-server:system:auth-delegator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:auth-delegator
subjects:
- kind: ServiceAccount
  name: metrics-server
  namespace: kube-system
`
		roleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  annotations:
    resources.gardener.cloud/delete-on-invalid-update: "true"
  creationTimestamp: null
  name: metrics-server-auth-reader
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: extension-apiserver-authentication-reader
subjects:
- kind: ServiceAccount
  name: metrics-server
  namespace: kube-system
`
		serviceAccountYAML = `apiVersion: v1
automountServiceAccountToken: false
kind: ServiceAccount
metadata:
  creationTimestamp: null
  name: metrics-server
  namespace: kube-system
`

		deploymentYAMLFor = func(secretName string, withHostEnv, vpaEnabled bool) string {
			memoryRequests := "150Mi"
			if vpaEnabled {
				memoryRequests = "60Mi"
			}

			out := `apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    ` + references.AnnotationKey(references.KindSecret, secretName) + `: ` + secretName + `
  creationTimestamp: null
  labels:
    gardener.cloud/role: system-component
    high-availability-config.resources.gardener.cloud/type: server
    k8s-app: metrics-server
    origin: gardener
  name: metrics-server
  namespace: kube-system
spec:
  replicas: 1
  revisionHistoryLimit: 2
  selector:
    matchLabels:
      k8s-app: metrics-server
  strategy:
    rollingUpdate:
      maxUnavailable: 0
  template:
    metadata:
      annotations:
        ` + references.AnnotationKey(references.KindSecret, secretName) + `: ` + secretName + `
      creationTimestamp: null
      labels:
        gardener.cloud/role: system-component
        k8s-app: metrics-server
        networking.gardener.cloud/from-seed: allowed
        networking.gardener.cloud/to-apiserver: allowed
        networking.gardener.cloud/to-dns: allowed
        networking.gardener.cloud/to-kubelet: allowed
        origin: gardener
    spec:
      containers:
      - command:
        - /metrics-server
        - --authorization-always-allow-paths=/livez,/readyz
        - --profiling=false
        - --cert-dir=/home/certdir
        - --secure-port=8443
        - --kubelet-insecure-tls
        - --kubelet-preferred-address-types=InternalIP,InternalDNS,ExternalDNS,ExternalIP,Hostname
        - --tls-cert-file=/srv/metrics-server/tls/tls.crt
        - --tls-private-key-file=/srv/metrics-server/tls/tls.key`

			if withHostEnv {
				out += `
        env:
        - name: KUBERNETES_SERVICE_HOST
          value: ` + kubeAPIServerHost
			}

			out += `
        image: ` + image + `
        imagePullPolicy: IfNotPresent
        livenessProbe:
          failureThreshold: 1
          httpGet:
            path: /livez
            port: 8443
            scheme: HTTPS
          initialDelaySeconds: 30
          periodSeconds: 30
        name: metrics-server
        readinessProbe:
          failureThreshold: 1
          httpGet:
            path: /readyz
            port: 8443
            scheme: HTTPS
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          limits:
            memory: 1Gi
          requests:
            cpu: 50m
            memory: ` + memoryRequests + `
        securityContext:
          allowPrivilegeEscalation: false
        volumeMounts:
        - mountPath: /srv/metrics-server/tls
          name: metrics-server
      dnsPolicy: Default
      priorityClassName: system-cluster-critical
      securityContext:
        fsGroup: 65534
        runAsNonRoot: true
        runAsUser: 65534
        seccompProfile:
          type: RuntimeDefault
        supplementalGroups:
        - 1
      serviceAccountName: metrics-server
      volumes:
      - name: metrics-server
        secret:
          secretName: ` + secretName + `
status: {}
`

			return out
		}

		pdbYAML = `apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  creationTimestamp: null
  labels:
    k8s-app: metrics-server
  name: metrics-server
  namespace: kube-system
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      k8s-app: metrics-server
  unhealthyPodEvictionPolicy: AlwaysAllow
status:
  currentHealthy: 0
  desiredHealthy: 0
  disruptionsAllowed: 0
  expectedPods: 0
`

		serverSecretFor = func(data map[string][]byte) (string, string) {
			serverSecret := &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "metrics-server",
					Namespace: "kube-system",
				},
				Data: make(map[string][]byte),
				Type: corev1.SecretTypeTLS,
			}

			serverSecret.Data["ca.crt"] = data["ca.crt"]
			serverSecret.Data["ca.key"] = data["ca.key"]

			ExpectWithOffset(1, kubernetesutils.MakeUnique(serverSecret)).To(Succeed())
			serverSecretYAML, err := kubernetesutils.Serialize(serverSecret, kubernetes.ShootScheme)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			return serverSecret.Name, serverSecretYAML
		}

		managedResourceName       = "shoot-core-metrics-server"
		managedResourceSecretName = "managedresource-shoot-core-metrics-server"

		managedResourceSecret *corev1.Secret
		managedResource       *resourcesv1alpha1.ManagedResource
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(fakeClient, namespace)

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-metrics-server", Namespace: namespace}})).To(Succeed())

		values = Values{
			Image:             image,
			VPAEnabled:        false,
			KubeAPIServerHost: nil,
		}

		metricsServer = New(fakeClient, namespace, sm, values)

		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceSecretName,
				Namespace: namespace,
			},
		}
		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: namespace,
			},
		}
	})

	Describe("#Deploy", func() {
		var (
			secretName        string
			manifests         []string
			expectedManifests []string
		)

		JustBeforeEach(func() {
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())

			metricsServer = New(fakeClient, namespace, sm, values)

			Expect(metricsServer.Deploy(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
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
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
			var err error
			manifests, err = test.ExtractManifestsFromManagedResourceData(managedResourceSecret.Data)
			Expect(err).NotTo(HaveOccurred())

			serverSecret, found := sm.Get("metrics-server")
			Expect(found).To(BeTrue())

			serverSecretName, serverSecretYAML := serverSecretFor(serverSecret.Data)
			secretName = serverSecretName

			expectedManifests = []string{
				apiServiceYAML,
				clusterRoleYAML,
				clusterRoleBindingYAML,
				clusterRoleBindingAuthDelegatorYAML,
				roleBindingYAML,
				serviceYAML,
				serviceAccountYAML,
				pdbYAML,
				serverSecretYAML,
			}
		})

		Context("w/o VPA, w/o host env", func() {
			It("should successfully deploy all resources", func() {
				expectedManifests = append(expectedManifests, deploymentYAMLFor(secretName, false, false))
				Expect(manifests).To(ConsistOf(expectedManifests))
			})
		})

		Context("w/ VPA, w/ host env", func() {
			BeforeEach(func() {
				values.VPAEnabled = true
				values.KubeAPIServerHost = &kubeAPIServerHost
			})

			It("should successfully deploy all resources (w/ VPA, w/ host env)", func() {
				expectedManifests = append(expectedManifests, vpaYAML, deploymentYAMLFor(secretName, true, true))
				Expect(manifests).To(ConsistOf(expectedManifests))
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully delete all the resources", func() {
			Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceSecret)).To(Succeed())

			Expect(metricsServer.Destroy(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
		})
	})

	Describe("#Wait", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(metricsServer.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(metricsServer.WaitCleanup(ctx)).To(Succeed())
		})
	})
})
