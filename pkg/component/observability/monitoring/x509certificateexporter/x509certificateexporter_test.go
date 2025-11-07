package x509certificateexporter_test

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	x "github.com/gardener/gardener/pkg/component/observability/monitoring/x509certificateexporter"
)

var _ = Describe("X509Certificate Exporter", func() {
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
	var (
		unhealthyManagedResourceStatus = resourcesv1alpha1.ManagedResourceStatus{
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
		}
		ctx context.Context

		managedResourceName              = "x509-certificate-exporter-runtime"
		managedResourceNameWithoutSuffix = "x509-certificate-exporter"
		namespace                        = "some-namespace"

		fakeClient client.Client
		deployer   component.DeployWaiter
		values     x.Values

		fakeOps *retryfake.Ops

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret
	)

	BeforeEach(func() {
		ctx = context.Background()

		fakeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()

		values = x.Values{
			NameSuffix: x.SuffixRuntime,
			ConfigData: []byte(`inCluster:
  enabled: true
  secretTypes: ["Opaque"]
`),
		}

		fakeOps = &retryfake.Ops{MaxAttempts: 2}
		DeferCleanup(test.WithVars(
			&retry.Until, fakeOps.Until,
			&retry.UntilTimeout, fakeOps.UntilTimeout,
		))

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

	JustBeforeEach(func() {
		var err error
		deployer, err = x.New(fakeClient, nil, namespace, values)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("#Deploy", func() {
		DescribeTable("validate configData sets resources correctly",
			func(configData string, expectedResourcesFunc func() []string) {
				// Set the config data for this specific test
				values = x.Values{
					Image:              "some-image",
					PriorityClassName:  "some-prio",
					NameSuffix:         x.SuffixRuntime,
					PrometheusInstance: "some-instance",
					ConfigData:         []byte(configData),
				}

				// Recreate the deployer with the new values
				var err error
				deployer, err = x.New(fakeClient, nil, namespace, values)
				Expect(err).NotTo(HaveOccurred())

				// Deploy and extract manifests
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				Expect(deployer.Deploy(ctx)).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				expectedMr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResource.Name,
						Namespace:       managedResource.Namespace,
						ResourceVersion: "1",
						Labels: map[string]string{
							"gardener.cloud/role":                "seed-system-component",
							"care.gardener.cloud/condition-type": "ObservabilityComponentsHealthy",
						},
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
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
				Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
				Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

				manifests, err := test.ExtractManifestsFromManagedResourceData(managedResourceSecret.Data)
				Expect(err).NotTo(HaveOccurred())

				expectedManifests := expectedResourcesFunc()
				// Verify that we have exactly 7 total resources
				Expect(manifests).To(HaveLen(7))
				// Verify that the core resources are present
				Expect(manifests).To(ContainElements(expectedManifests))
			},
			Entry("inCluster enabled with secretTypes",
				`inCluster:
    enabled: true
    secretTypes: ["Opaque"]`,
				func() []string {
					return []string{
						`apiVersion: v1
kind: ServiceAccount
metadata:
  creationTimestamp: null
  name: x509-certificate-exporter-runtime
  namespace: some-namespace
`,
						`apiVersion: v1
kind: Service
metadata:
  annotations:
    networking.resources.gardener.cloud/from-all-garden-scrape-targets-allowed-ports: '[{"protocol":"TCP","port":9793}]'
  creationTimestamp: null
  labels:
    certificate-source: api
    role: x509-certificate-exporter
  name: x509-certificate-exporter-runtime
  namespace: some-namespace
spec:
  ports:
  - name: metrics
    port: 9793
    protocol: TCP
    targetPort: 9793
  selector:
    certificate-source: api
    role: x509-certificate-exporter
  type: ClusterIP
status:
  loadBalancer: {}
`,
					}
				},
			),
		)
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			// Create managed resource with the name that Destroy expects
			mrToDestroy := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceNameWithoutSuffix,
					Namespace: namespace,
				},
			}
			secretToDestroy := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managedresource-" + managedResourceNameWithoutSuffix,
					Namespace: namespace,
				},
			}

			Expect(fakeClient.Create(ctx, mrToDestroy)).To(Succeed())
			Expect(fakeClient.Create(ctx, secretToDestroy)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mrToDestroy), mrToDestroy)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secretToDestroy), secretToDestroy)).To(Succeed())

			Expect(deployer.Destroy(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mrToDestroy), mrToDestroy)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secretToDestroy), secretToDestroy)).To(BeNotFoundError())
		})
	})

	Context("waiting functions", func() {
		Describe("#Wait", func() {
			It("should fail because reading the runtime ManagedResource fails", func() {
				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the ManagedResource is unhealthy", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameWithoutSuffix,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should succeed because the ManagedResource is healthy and progressing", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameWithoutSuffix,
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
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out", func() {
				mrForCleanup := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      managedResourceNameWithoutSuffix,
						Namespace: namespace,
					},
				}
				Expect(fakeClient.Create(ctx, mrForCleanup)).To(Succeed())

				Expect(deployer.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it is already removed", func() {
				Expect(deployer.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})
