// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package x509certificateexporter_test

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
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
				Entry("should return missing deploy errors if config is empty", "invalid yaml", x.ErrInvalidExporterConfigFormat),
				Entry("should return empty exporter config error if neither incluster nor workergroups enabled", "", x.ErrEmptyExporterConfig),
				Entry("should return error on invalid alerting config", `inCluster:
    enabled: true
    configMapKeys: ["dummy"]
    secrets:
      - type: Opaque
alertingConfig:
    readErrorsSeverity: 'thereisnosuchseverity'
    certificateExpirationDays: 10
    certificateRenewalDays: 15
  `, x.ErrAlertingConfig, x.ErrConfigValidationFailed, x.ErrInvalidSeverity),
				Entry("should return error when expiration days are higher than renewal days", `inCluster:
    enabled: true
    configMapKeys: ["dummy"]
    secrets:
      - type: Opaque
alertingConfig:
    certificateExpirationDays: 20
    certificateRenewalDays: 10
  `, x.ErrAlertingConfig, x.ErrConfigValidationFailed, x.ErrInvalidExpirationRenewalConf),
				Entry("should return error on invalid workergroup config", `{workerGroups: [{nameSuffix: 'some-suffix'}]}`,
					x.ErrConfigValidationFailed, x.ErrWorkerGroupsConfig, x.ErrWorkerGroupInvalid, x.ErrWorkerGroupMissingMount,
				),
				Entry("should return error when some group is missing volume mounts", `workerGroups: [{nameSuffix: 'some-suffix', selector: {matchLabels: {a: b}}}]`,
					x.ErrConfigValidationFailed, x.ErrWorkerGroupsConfig, x.ErrWorkerGroupInvalid, x.ErrWorkerGroupMissingMount,
				),
				Entry("should return error for multiple nodegroups when some group is missing selector or namesuffix", `workerGroups:
- {nameSuffix: 'some-suffix', selector: {matchLabels: {a: b}}, mounts: {m1: {hostPath: "tmp"}, m2: {hostPath: "/tmp"} }}
- {mounts: {m1: {hostPath: "tmp"}, m2: {hostPath: "/tmp"}}}
`,
					x.ErrConfigValidationFailed, x.ErrWorkerGroupsConfig, x.ErrWorkerGroupInvalid, x.ErrMultipleGroupsNoSelectorOrSuffix,
				),

				Entry("should return error for multiple nodegroups when some group is missing selector or namesuffix", `workerGroups:
- {nameSuffix: 'some-suffix', selector: {matchLabels: {a: b}}, mounts: {m1: {hostPath: "tmp"}, m2: {hostPath: "/tmp"} }}
- {nameSuffix: 'some-suffix', selector: {matchLabels: {a: b}}}
`,
					x.ErrConfigValidationFailed, x.ErrWorkerGroupsConfig, x.ErrWorkerGroupInvalid, x.ErrWorkerGroupMissingMount, x.ErrMountValidation, x.ErrHostPathNotAbsolute, x.ErrMountPathEmpty,
				),
				Entry("should return error when monitoring relative paths", `workerGroups:
- {nameSuffix: 'some-suffix', selector: {matchLabels: {a: b}}, mounts:  {m1: {hostPath: "tmp", watchDirs: ["abc/"]} }}
`,
					x.ErrConfigValidationFailed, x.ErrWorkerGroupsConfig, x.ErrWorkerGroupInvalid, x.ErrMountValidation, x.ErrHostPathNotAbsolute,
				),
				Entry("should return error when watchDir is not absolute path", `workerGroups:
- {nameSuffix: 'some-suffix', selector: {matchLabels: {a: b}}, mounts:  {m1: {hostPath: "/tmp", mountPath: "/tmp", watchDirs: ["relative/path"]} }}
`,
					x.ErrConfigValidationFailed, x.ErrWorkerGroupsConfig, x.ErrWorkerGroupInvalid, x.ErrMountValidation, x.ErrWatchedFileNotAbsolutePath,
				),
				Entry("should return error when configmap key is empty", `inCluster:
      enabled: true
      secrets:
        - type: Opaque
      configMapKeys: [""]
  `,
					x.ErrConfigValidationFailed, x.ErrEmptyConfigMapKey,
				),
				Entry("should return error when configmap key is illegal", `inCluster:
      enabled: true
      secrets:
        - type: Opaque
      configMapKeys: ["invalid@key"]
  `,
					x.ErrConfigValidationFailed, x.ErrKeyIsIllegal,
				),
				Entry("should return error when configmap key exceeds maximum length", `inCluster:
      enabled: true
      secrets:
        - type: Opaque
      configMapKeys: ["`+strings.Repeat("a", 254)+`"]
  `,
					x.ErrConfigValidationFailed, x.ErrConfigMapMaxKeyLenght,
				),
				Entry("should return error when includeLabels has invalid key", `inCluster:
      enabled: true
      configMapKeys: ["dummy"]
      includeLabels:
        "invalid@key": "value"
  `,
					x.ErrConfigValidationFailed, x.ErrIncludeLabelsInvalid,
				),
				Entry("should return error when namespace is invalid", `inCluster:
      enabled: true
      configMapKeys: ["dummy"]
      includeNamespaces: ["invalid..namespace"]
  `,
					x.ErrConfigValidationFailed, x.ErrInvalidNamespace,
				),
				Entry("should return error when secret type is invalid", `inCluster:
    enabled: true
    configMapKeys: ["dummy"]
    secretTypes:
      - type: InvalidSecretType
  `, x.ErrConfigValidationFailed, x.ErrInClusterConfig, x.ErrInvalidSecretType),
			)
		})
		Context("fullblown config", func() {
			It("should succeed", func() {
				obj, err := x.New(fakeclient.NewFakeClient(), nil, "garden", x.Values{
					Image:              "some-image:some-tag",
					PriorityClassName:  "some-priority-class",
					NameSuffix:         x.SuffixRuntime,
					PrometheusInstance: "some-prometheus-instance",
					ConfigData: []byte(`inCluster:
  enabled: true
  configMapKeys:
    - ca-crl.pem
    - root-cert.pem
    - ca.crt
  secrets:
    - type: Opaque
    - type: kubernetes.io/tls
alertingConfig:
  certificateExpirationDays: 1
workerGroups:
  - nameSuffix: gol-cplane
    selector:
      kubernetes.io/hostname: gardener-operator-local-control-plane
    mounts:
      etc:
        hostPath: /etc/ssl
        mountPath: /x509-mon
        watchDirs:
          - /x509-mon/etc/ssl/certs/`),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(obj).NotTo(BeNil())
			})
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
      configMapKeys: ["dummy"]
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
		Context("complete resource generation validation", func() {
			DescribeTable("should generate all expected resources for different configurations",
				func(configData string, expectedResourceTypes []string) {
					values = x.Values{
						Image:              "some-image",
						PriorityClassName:  "some-prio",
						NameSuffix:         x.SuffixRuntime,
						PrometheusInstance: "some-instance",
						ConfigData:         []byte(configData),
					}

					var err error
					deployer, err = x.New(fakeClient, nil, namespace, values)
					Expect(err).NotTo(HaveOccurred())

					Expect(deployer.Deploy(ctx)).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

					manifests, err := test.ExtractManifestsFromManagedResourceData(managedResourceSecret.Data)
					Expect(err).NotTo(HaveOccurred())

					foundResourceTypes := make(map[string]bool)
					for _, manifest := range manifests {
						if strings.Contains(manifest, "kind: ServiceAccount") {
							foundResourceTypes["ServiceAccount"] = true
						}
						if strings.Contains(manifest, "kind: Service") {
							foundResourceTypes["Service"] = true
						}
						if strings.Contains(manifest, "kind: Deployment") {
							foundResourceTypes["Deployment"] = true
						}
						if strings.Contains(manifest, "kind: DaemonSet") {
							foundResourceTypes["DaemonSet"] = true
						}
						if strings.Contains(manifest, "kind: ClusterRole") {
							foundResourceTypes["ClusterRole"] = true
						}
						if strings.Contains(manifest, "kind: ClusterRoleBinding") {
							foundResourceTypes["ClusterRoleBinding"] = true
						}
						if strings.Contains(manifest, "kind: PrometheusRule") {
							foundResourceTypes["PrometheusRule"] = true
						}
					}

					for _, expectedType := range expectedResourceTypes {
						Expect(foundResourceTypes[expectedType]).To(BeTrue(), "Expected resource type %s to be present", expectedType)
					}
				},
				Entry("inCluster only configuration",
					`inCluster:
    enabled: true
    configMapKeys: ["dummy"]
    secrets:
      - type: Opaque`,
					[]string{"ServiceAccount", "Service", "Deployment", "ClusterRole", "ClusterRoleBinding", "PrometheusRule"},
				),
				Entry("workerGroups only configuration",
					`workerGroups:
  - nameSuffix: xmon-cplane
    nodeSelector:
      kubernetes.io/hostname: gardener-operator-local-control-plane
    mounts:
      etc:
        mountPath: /x509-mon
        hostPath: /etc
        watchDirs:
          - /etc/ssl/certs/`,
					[]string{"ServiceAccount", "Service", "DaemonSet", "PrometheusRule"},
				),
				Entry("mixed inCluster and workerGroups configuration",
					`inCluster:
    enabled: true
    configMapKeys: ["dummy"]
    secrets:
      - type: Opaque
workerGroups:
  - nameSuffix: xmon-cplane
    nodeSelector:
      kubernetes.io/hostname: gardener-operator-local-control-plane
    mounts:
      etc:
        mountPath: /x509-mon
        hostPath: /etc
        watchDirs:
          - /etc/ssl/certs/`,
					[]string{"ServiceAccount", "Service", "Deployment", "DaemonSet", "ClusterRole", "ClusterRoleBinding", "PrometheusRule"},
				),
			)

			It("should generate correct resource names and labels", func() {
				values = x.Values{
					Image:              "some-image",
					PriorityClassName:  "some-prio",
					NameSuffix:         x.SuffixRuntime,
					PrometheusInstance: "some-instance",
					ConfigData: []byte(`inCluster:
    enabled: true
    configMapKeys: ["dummy"]
    secrets:
      - type: Opaque`),
				}

				var err error
				deployer, err = x.New(fakeClient, nil, namespace, values)
				Expect(err).NotTo(HaveOccurred())

				Expect(deployer.Deploy(ctx)).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

				manifests, err := test.ExtractManifestsFromManagedResourceData(managedResourceSecret.Data)
				Expect(err).NotTo(HaveOccurred())

				for _, manifest := range manifests {
					if strings.Contains(manifest, "kind: ServiceAccount") {
						Expect(manifest).To(ContainSubstring("name: x509-certificate-exporter-runtime"))
					}
					if strings.Contains(manifest, "kind: Service\n") {
						Expect(manifest).To(ContainSubstring("name: x509-certificate-exporter-runtime"))
						Expect(manifest).To(ContainSubstring("role: x509-certificate-exporter"))
						Expect(manifest).To(ContainSubstring("certificate-source: api"))
					}
					if strings.Contains(manifest, "kind: Deployment") {
						Expect(manifest).To(ContainSubstring("name: x509-certificate-exporter-runtime"))
						Expect(manifest).To(ContainSubstring("role: x509-certificate-exporter"))
						Expect(manifest).To(ContainSubstring("certificate-source: api"))
					}
					if strings.Contains(manifest, "\nkind: ClusterRole\n") {
						Expect(manifest).To(ContainSubstring("name: gardener.cloud:monitoring:x509-certificate-exporter"))
					}
					if strings.Contains(manifest, "\nkind: ClusterRoleBinding\n") {
						Expect(manifest).To(ContainSubstring("name: gardener.cloud:monitoring:x509-certificate-exporter"))
					}
				}
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
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
