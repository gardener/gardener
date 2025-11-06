package x509certificateexporter_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	x "github.com/gardener/gardener/pkg/component/observability/monitoring/x509certificateexporter"
)

var _ = Describe("X509Certificate Exporter", func() {
	var (
		x509Exporter              component.DeployWaiter
		err                       error
		managedResource           *resourcesv1alpha1.ManagedResource
		managedResourceSecret     *corev1.Secret
		managedResourceName       string
		managedResourceSecretName string
		ctx                       = context.TODO()

		fakeClient     client.Client
		secretsManager secretsmanager.Interface
		namespace      = "some-namespace"

		image              = "some-image:latest"
		priorityClassName  = "some-priority-class"
		prometheusInstance = "some-prometheus"
		defaultSuffix      = x.SuffixRuntime
		configData         = []byte(`
inCluster:
    enabled: false
    secretTypes: 
        - kubernetes.io/tls
        - Opaque
    configMapKeys:
        - tls.crt
`)
		promRuleYAML = func(_ string) string {
			return ``
		}
		serviceYAML = func(name string) string {
			return `apiVersion: v1
kind: Service
metadata:
  name: ` + name + `
  namespace: ` + namespace + `
spec:
  selector:
    app.kubernetes.io/name: MyApp
  ports:
    - protocol: TCP
      port: ` + fmt.Sprintf("%d", x.Port) + `
      targetPort: ` + fmt.Sprintf("%d", x.Port) + `
`
		}
		serviceMonitorYAML = func(name string) string {
			return `apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: ` + name + `
  namespace: default
  labels:
    app: gitlab-runner-gitlab-runner
    release: prometheus
spec:
  selector:
    matchLabels:
      app: gitlab-runner-gitlab-runner
  namespaceSelector:
    # matchNames:
    # - default
    any: true
  endpoints:
  - port: http-metrics
    interval: 15s
`
		}
		serviceAccountYAML = func(name string) string {
			return `apiVersion: v1
automountServiceAccountToken: false
kind: ServiceAccount
metadata:
	name: ` + name + `
	namespace: namespace
`
		}
		inClusterRoleYAML = func(name string) string {
			return `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: "2025-11-04T11:50:21Z"
  labels:
    k8s-app: metrics-server
  name: system:metrics-server
  resourceVersion: "736"
  uid: 38369851-829f-401d-9893-bad2d86fa7d2
rules:
- apiGroups:
  - ""
  resources:
  - nodes/metrics
  verbs:
  - get
- apiGroups:
  - ""
  resources:
  - pods
  - nodes
  verbs:
  - get
  - list
  - watch`
		}
		inClusterRoleBindingYAML = func(name string) string {
			return `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: "2025-11-04T11:50:21Z"
  labels:
    k8s-app: metrics-server
  name: system:metrics-server
  resourceVersion: "742"
  uid: d7e5d0d2-d002-42d9-9388-b3ea828cb89e
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:metrics-server
subjects:
- kind: ServiceAccount
  name: metrics-server
  namespace: kube-system
		`
		}
	)
	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().Build()
		secretsManager = fakesecretsmanager.New(fakeClient, namespace)

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-metrics-server", Namespace: namespace}})).To(Succeed())

		x509Exporter, err = x.New(
			fakeClient, secretsManager, namespace, x.Values{
				Image:              image,
				PriorityClassName:  priorityClassName,
				PrometheusInstance: prometheusInstance,
			},
		)
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
	Describe("#New", func() {
		Context("with invalid target", func() {
			DescribeTable("should return unsupported cluster type error", func(suffix string) {
				obj, err := x.New(nil, nil, suffix, x.Values{})
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(x.ErrUnsuportedClusterType))
				Expect(obj).To(BeNil())
			},
				Entry("seed suffix", x.SuffixSeed),
				Entry("shoot suffix", x.SuffixShoot),
			)
		})
		Context("with invalid parameters", func() {
			DescribeTable("should return errors when config is invalid", func(configData string, expectedErr error) {
				obj, err := x.New(nil, nil, x.SuffixRuntime, x.Values{
					ConfigData: []byte(configData)})
				Expect(err).To(HaveOccurred())
				Expect(obj).To(BeNil())
				Expect(err).To(MatchError(expectedErr))

			},
				Entry("should return missing deploy errors if config is empty", "invlaid yaml", x.ErrInvalidExporterConfigFormat),
				Entry("should return empty exporter config error if neither incluster nor workergroups enabled", "", x.ErrEmptyExporterConfig),
				Entry("should return error on invalid alerting config", `{inCluster: {enabled: true}, alertingConfigs: {readErrorsSeverity: 'thereisnosuchseverity'}}`, x.ErrAlertingConfig),
				Entry("should return error on invalid workergroup config", `{workerGroups: {secretTypes: ['no-such-type']}}`, x.ErrConfigValidationFailed),
			)
		})
	})
	Describe("#Deploy", func() {
		JustBeforeEach(func() {
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())

			x509Exporter, err = x.New(
				fakeClient, secretsManager, namespace, x.Values{
					Image:              image,
					PriorityClassName:  priorityClassName,
					PrometheusInstance: prometheusInstance,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(x509Exporter.Deploy(ctx)).To(Succeed())
		})

	})

	Describe("#Destroy", func() {
		It("should successfully delete all the resources", func() {
			Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceSecret)).To(Succeed())

			Expect(x509Exporter.Destroy(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
		})
	})
	Context("waiting functions", func() {

	})
})
