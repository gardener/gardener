package x509certificateexporter_test

import (
	"context"
	"strings"
	// "fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	// gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"

	// "github.com/gardener/gardener/pkg/utils/retry"
	// retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"

	// "github.com/gardener/gardener/pkg/utils/test"
	// . "github.com/gardener/gardener/pkg/utils/test/matchers"

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
				ConfigData:         []byte(`{inCluster: {enabled: true, secretTypes: ["Opaque"]}}`),
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
		_ = x509Exporter
		_ = err
		_ = managedResource
		_ = managedResourceSecret
	})
	Describe("#New", func() {
		Context("with invalid target", func() {
			DescribeTable("should return unsupported cluster type error", func(suffix string) {
				obj, err := x.New(nil, nil, "", x.Values{NameSuffix: suffix})
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(x.ErrUnsuportedClusterType))
				Expect(obj).To(BeNil())
			},
				Entry("seed suffix", x.SuffixSeed),
				Entry("shoot suffix", x.SuffixShoot),
			)
		})
		Context("with invalid parameters", func() {
			DescribeTable("should return errors when config is invalid", func(configData string, expectedErrs ...error) {
				obj, err := x.New(nil, nil, "", x.Values{
					NameSuffix: x.SuffixRuntime,
					ConfigData: []byte(configData)})
				Expect(err).To(HaveOccurred())
				Expect(obj).To(BeNil())
				for _, expErr := range expectedErrs {
					Expect(err).To(MatchError(ContainSubstring(expErr.Error())))
				}
			},
				Entry("should return missing deploy errors if config is empty", "invlaid yaml", x.ErrInvalidExporterConfigFormat),
				Entry("should return empty exporter config error if neither incluster nor workergroups enabled", "", x.ErrEmptyExporterConfig),
				Entry("should return error on invalid alerting config", `inCluster:
    enabled: true
    secretTypes: ["Opaque"]
alertingConfig:
    readErrorsSeverity: 'thereisnosuchseverity'
    certificationExpirationDays: 10
    certificateRenewalDays: 15
`, x.ErrAlertingConfig, x.ErrConfigValidationFailed, x.ErrInvalidSeverity),
				Entry("should return error on invalid workergroup config", `{workerGroups: [{nameSuffix: 'some-suffix'}]}`,
					x.ErrConfigValidationFailed, x.ErrWorkerGroupsConfig, x.ErrWorkerGroupInvalid,
				),
				Entry("should return error when some group is missing volume mounts", `workerGroups: [{nameSuffix: 'some-suffix', selector: {matchLabels: {a: b}}}]`,
					x.ErrConfigValidationFailed, x.ErrWorkerGroupsConfig, x.ErrWorkerGroupInvalid, x.ErrWorkerGroupMissingMount,
				),
				Entry("should return error for multiple nodegroups when some group is missing selector or namesuffix", `workerGroups:
- {nameSuffix: 'some-suffix', selector: {matchLabels: {a: b}}, mounts: {m1: {path: "tmp"}, m2: {path: "/tmp"} }}
- {mounts: {m1: {path: "tmp"}, m2: {path: "/tmp"}}}
`,
					x.ErrConfigValidationFailed, x.ErrWorkerGroupsConfig, x.ErrWorkerGroupInvalid,
				),

				Entry("should return error for multiple nodegroups when some group is missing selector or namesuffix", `workerGroups:
- {nameSuffix: 'some-suffix', selector: {matchLabels: {a: b}}, mounts: {m1: {path: "tmp"}, m2: {path: "/tmp"} }}
- {nameSuffix: 'some-suffix', selector: {matchLabels: {a: b}}}
`,
					x.ErrConfigValidationFailed, x.ErrWorkerGroupsConfig, x.ErrWorkerGroupInvalid, x.ErrWorkerGroupMissingMount, x.ErrMountValidation, x.ErrMountPathNotAbsolute, x.ErrNoMonitorableFiles,
				),
				Entry("should return error when monitoring relative paths", `workerGroups:
- {nameSuffix: 'some-suffix', selector: {matchLabels: {a: b}}, mounts:  {m1: {path: "/tmp", watchDirs: ["abc/"]} }}
`,
					x.ErrConfigValidationFailed, x.ErrWorkerGroupsConfig, x.ErrWorkerGroupInvalid, x.ErrMountValidation, x.ErrMountValidationErrors, x.ErrWatchedFileNotAbsolutePath,
				),
				Entry("should return error when configmap key is empty", `inCluster:
  enabled: true
  secretTypes: ["Opaque"]
  configMapKeys: [""]
`,
					x.ErrConfigValidationFailed, x.ErrEmptyConfigMapKey,
				),
				Entry("should return error when configmap key is illegal", `inCluster:
  enabled: true
  secretTypes: ["Opaque"]
  configMapKeys: ["invalid@key"]
`,
					x.ErrConfigValidationFailed, x.ErrKeyIsIllegal,
				),
				Entry("should return error when configmap key exceeds maximum length", `inCluster:
  enabled: true
  secretTypes: ["Opaque"]
  configMapKeys: ["`+strings.Repeat("a", 254)+`"]
`,
					x.ErrConfigValidationFailed, x.ErrConfigMapMaxKeyLenght,
				),
				Entry("should return error when includeLabels has invalid key", `inCluster:
  enabled: true
  secretTypes: ["Opaque"]
  includeLabels:
    "invalid@key": "value"
`,
					x.ErrConfigValidationFailed, x.ErrIncludeLabelsInvalid,
				),
				Entry("should return error when namespace is invalid", `inCluster:
  enabled: true
  secretTypes: ["Opaque"]
  includeNamespaces: ["invalid..namespace"]
`,
					x.ErrConfigValidationFailed, x.ErrInvalidNamespace,
				),
				Entry("should return error when secret type is invalid", `inCluster:
  enabled: true
  secretTypes: ["InvalidSecretType"]
`,
					x.ErrConfigValidationFailed, x.ErrInvalidSecretType,
				),
			)
		})
	})
	// Describe("#Deploy", func() {
	// 	JustBeforeEach(func() {
	// 		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
	//
	// 		x509Exporter, err = x.New(
	// 			fakeClient, secretsManager, namespace, x.Values{
	// 				Image:              image,
	// 				PriorityClassName:  priorityClassName,
	// 				PrometheusInstance: prometheusInstance,
	// 			},
	// 		)
	// 		Expect(err).NotTo(HaveOccurred())
	// 		Expect(x509Exporter.Deploy(ctx)).To(Succeed())
	// 	})
	//
	// })
	//
	// Describe("#Destroy", func() {
	// 	It("should successfully delete all the resources", func() {
	// 		Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())
	// 		Expect(fakeClient.Create(ctx, managedResourceSecret)).To(Succeed())
	//
	// 		Expect(x509Exporter.Destroy(ctx)).To(Succeed())
	//
	// 		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
	// 		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
	// 	})
	// })
	// Context("waiting functions", func() {
	//
	// })
})
