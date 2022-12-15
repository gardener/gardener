package prommetric

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
)

//#region Fakes

type testIsolation struct {
	DeployedResourceConfigs component.ResourceConfigs
}

func (ti *testIsolation) DeployResourceConfigs(
	ctx context.Context,
	client client.Client,
	namespace string,
	clusterType component.ClusterType,
	managedResourceName string,
	registry *managedresources.Registry,
	allResources component.ResourceConfigs,
) error {
	ti.DeployedResourceConfigs = allResources

	return nil
}

func newTestIsolation() *testIsolation {
	return &testIsolation{}
}

//#endregion Fakes

func convertResourceConfigToJson(config *component.ResourceConfig) (string, error) {
	json, err := json.MarshalIndent(config.Obj.(*unstructured.Unstructured), "", "\t")
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("\n%s", strings.TrimSuffix(string(json), "\n")), nil
}

var _ = Describe("PrometheusMetricsAdapter", func() {
	var (
		caSecretName  = "ca-seed"
		imageName     = "test-image"
		namespaceName = "test-namespace"

		ctx                = context.TODO()
		seedClient         client.Client
		fakeSecretsManager secretsmanager.Interface

		//#region Helpers
		newPma = func(isEnabled bool) (*PrometheusMetricsAdapter, *testIsolation) {
			pma := NewPrometheusMetricsAdapter(namespaceName, imageName, isEnabled, seedClient, fakeSecretsManager)
			ti := newTestIsolation()
			// We isolate the deployment workflow at the DeployResourceConfigs() level, because that point offers a
			// convenient, declarative representation
			pma.testIsolation.DeployResourceConfigs = ti.DeployResourceConfigs

			return pma, ti
		}

		assertNoServerCertificateOnServer = func() {
			actualServerCertificateSecret := corev1.Secret{}
			err := seedClient.Get(
				ctx,
				client.ObjectKey{Namespace: namespaceName, Name: serverCertificateSecretName},
				&actualServerCertificateSecret)

			ExpectWithOffset(1, err).To(HaveOccurred())
			ExpectWithOffset(1, err.Error()).To(MatchRegexp(".*not.*found.*"))
		}

		assertNoManagedResourceOnServer = func() {
			mr := resourcesv1alpha1.ManagedResource{}
			err := seedClient.Get(ctx, client.ObjectKey{Namespace: namespaceName, Name: managedResourceName}, &mr)
			ExpectWithOffset(1, err).To(HaveOccurred())
			ExpectWithOffset(1, err.Error()).To(MatchRegexp(".*not.*found.*"))
		}

		createObjectOnSeed = func(obj client.Object, name string) {
			obj.SetNamespace(namespaceName)
			obj.SetName(name)
			ExpectWithOffset(1, seedClient.Create(ctx, obj)).To(Succeed())
		}
		//#endregion Helpers
	)

	BeforeEach(func() {
		seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeSecretsManager = fakesecretsmanager.New(seedClient, namespaceName)
	})

	Describe(".Deploy()", func() {
		Context("in enabled state", func() {
			It("should deploy the correct resources to the seed", func() {
				//#region Expected resource config values as bulk JSON
				expectedResourceConfigsAsJson := []string{
					`
{
	"apiVersion": "v1",
	"data": {
		"config.yaml": "rules:\n- seriesQuery: '{__name__=~\"shoot:apiserver_request_total:sum\",cluster!=\"\",pod!=\"\"}'\n  seriesFilters: []\n  resources:\n    overrides:\n      cluster:\n        resource: namespace\n      pod:\n        resource: pod\n  name:\n    matches: \"\"\n    as: \"\"\n  metricsQuery: sum(\u003c\u003c.Series\u003e\u003e{\u003c\u003c.LabelMatchers\u003e\u003e}) by (\u003c\u003c.GroupBy\u003e\u003e)    \n- seriesQuery: '{__name__=~\"^container_.*\",container!=\"POD\",namespace!=\"\",pod!=\"\"}'\n  seriesFilters: []\n  resources:\n    overrides:\n      namespace:\n        resource: namespace\n      pod:\n        resource: pod\n  name:\n    matches: ^container_(.*)_seconds_total$\n    as: \"\"\n  metricsQuery: sum(rate(\u003c\u003c.Series\u003e\u003e{\u003c\u003c.LabelMatchers\u003e\u003e,container!=\"POD\"}[1m])) by (\u003c\u003c.GroupBy\u003e\u003e)\n- seriesQuery: '{__name__=~\"^container_.*\",container!=\"POD\",namespace!=\"\",pod!=\"\"}'\n  seriesFilters:\n  - isNot: ^container_.*_seconds_total$\n  resources:\n    overrides:\n      namespace:\n        resource: namespace\n      pod:\n        resource: pod\n  name:\n    matches: ^container_(.*)_total$\n    as: \"\"\n  metricsQuery: sum(rate(\u003c\u003c.Series\u003e\u003e{\u003c\u003c.LabelMatchers\u003e\u003e,container!=\"POD\"}[1m])) by (\u003c\u003c.GroupBy\u003e\u003e)\n- seriesQuery: '{__name__=~\"^container_.*\",container!=\"POD\",namespace!=\"\",pod!=\"\"}'\n  seriesFilters:\n  - isNot: ^container_.*_total$\n  resources:\n    overrides:\n      namespace:\n        resource: namespace\n      pod:\n        resource: pod\n  name:\n    matches: ^container_(.*)$\n    as: \"\"\n  metricsQuery: sum(\u003c\u003c.Series\u003e\u003e{\u003c\u003c.LabelMatchers\u003e\u003e,container!=\"POD\"}) by (\u003c\u003c.GroupBy\u003e\u003e)\n- seriesQuery: '{namespace!=\"\",__name__!~\"^container_.*\"}'\n  seriesFilters:\n  - isNot: .*_total$\n  resources:\n    template: \u003c\u003c.Resource\u003e\u003e\n  name:\n    matches: \"\"\n    as: \"\"\n  metricsQuery: sum(\u003c\u003c.Series\u003e\u003e{\u003c\u003c.LabelMatchers\u003e\u003e}) by (\u003c\u003c.GroupBy\u003e\u003e)\n- seriesQuery: '{namespace!=\"\",__name__!~\"^container_.*\"}'\n  seriesFilters:\n  - isNot: .*_seconds_total\n  resources:\n    template: \u003c\u003c.Resource\u003e\u003e\n  name:\n    matches: ^(.*)_total$\n    as: \"\"\n  metricsQuery: sum(rate(\u003c\u003c.Series\u003e\u003e{\u003c\u003c.LabelMatchers\u003e\u003e}[1m])) by (\u003c\u003c.GroupBy\u003e\u003e)\n- seriesQuery: '{namespace!=\"\",__name__!~\"^container_.*\"}'\n  seriesFilters: []\n  resources:\n    template: \u003c\u003c.Resource\u003e\u003e\n  name:\n    matches: ^(.*)_seconds_total$\n    as: \"\"\n  metricsQuery: sum(rate(\u003c\u003c.Series\u003e\u003e{\u003c\u003c.LabelMatchers\u003e\u003e}[1m])) by (\u003c\u003c.GroupBy\u003e\u003e)\nresourceRules:\n  cpu:\n    containerQuery: sum(rate(container_cpu_usage_seconds_total{\u003c\u003c.LabelMatchers\u003e\u003e}[1m])) by (\u003c\u003c.GroupBy\u003e\u003e)\n    nodeQuery: sum(rate(container_cpu_usage_seconds_total{\u003c\u003c.LabelMatchers\u003e\u003e, id='/'}[1m])) by (\u003c\u003c.GroupBy\u003e\u003e)\n    resources:\n      overrides:\n        instance:\n          resource: node\n        namespace:\n          resource: namespace\n        pod:\n          resource: pod\n    containerLabel: container\n  memory:\n    containerQuery: sum(container_memory_working_set_bytes{\u003c\u003c.LabelMatchers\u003e\u003e}) by (\u003c\u003c.GroupBy\u003e\u003e)\n    nodeQuery: sum(container_memory_working_set_bytes{\u003c\u003c.LabelMatchers\u003e\u003e,id='/'}) by (\u003c\u003c.GroupBy\u003e\u003e)\n    resources:\n      overrides:\n        instance:\n          resource: node\n        namespace:\n          resource: namespace\n        pod:\n          resource: pod\n    containerLabel: container\n  window: 1m\nexternalRules:\n- seriesQuery: '{__name__=~\"^.*_queue_(length|size)$\",namespace!=\"\"}'\n  resources:\n    overrides:\n      namespace:\n        resource: namespace\n  name:\n    matches: ^.*_queue_(length|size)$\n    as: \"$0\"\n  metricsQuery: max(\u003c\u003c.Series\u003e\u003e{\u003c\u003c.LabelMatchers\u003e\u003e})\n- seriesQuery: '{__name__=~\"^.*_queue$\",namespace!=\"\"}'\n  resources:\n    overrides:\n      namespace:\n        resource: namespace\n  name:\n    matches: ^.*_queue$\n    as: \"$0\"\n  metricsQuery: max(\u003c\u003c.Series\u003e\u003e{\u003c\u003c.LabelMatchers\u003e\u003e})\n"
	},
	"kind": "ConfigMap",
	"metadata": {
		"name": "prometheus-metrics-adapter",
		"namespace": "test-namespace"
	}
}`,
					`
{
	"apiVersion": "v1",
	"kind": "ServiceAccount",
	"metadata": {
		"name": "prometheus-metrics-adapter",
		"namespace": "test-namespace"
	}
}`,
					`
{
	"apiVersion": "rbac.authorization.k8s.io/v1",
	"kind": "ClusterRole",
	"metadata": {
		"name": "prometheus-metrics-adapter-resources"
	},
	"rules": [
		{
			"apiGroups": [
				"custom.metrics.k8s.io",
				"external.metrics.k8s.io"
			],
			"resources": [
				"*"
			],
			"verbs": [
				"*"
			]
		}
	]
}`,
					`
{
	"apiVersion": "rbac.authorization.k8s.io/v1",
	"kind": "ClusterRole",
	"metadata": {
		"name": "prometheus-metrics-adapter-resource-reader"
	},
	"rules": [
		{
			"apiGroups": [
				""
			],
			"resources": [
				"pods",
				"nodes",
				"nodes/stats"
			],
			"verbs": [
				"get",
				"list",
				"watch"
			]
		}
	]
}`,
					`
{
	"apiVersion": "rbac.authorization.k8s.io/v1",
	"kind": "ClusterRoleBinding",
	"metadata": {
		"name": "prometheus-metrics-adapter:system:auth-delegator"
	},
	"roleRef": {
		"apiGroup": "rbac.authorization.k8s.io",
		"kind": "ClusterRole",
		"name": "system:auth-delegator"
	},
	"subjects": [
		{
			"kind": "ServiceAccount",
			"name": "prometheus-metrics-adapter",
			"namespace": "test-namespace"
		}
	]
}`,
					`
{
	"apiVersion": "rbac.authorization.k8s.io/v1",
	"kind": "ClusterRoleBinding",
	"metadata": {
		"name": "prometheus-metrics-adapter-resource-reader"
	},
	"roleRef": {
		"apiGroup": "rbac.authorization.k8s.io",
		"kind": "ClusterRole",
		"name": "prometheus-metrics-adapter-resource-reader"
	},
	"subjects": [
		{
			"kind": "ServiceAccount",
			"name": "prometheus-metrics-adapter",
			"namespace": "test-namespace"
		}
	]
}`,
					`
{
	"apiVersion": "rbac.authorization.k8s.io/v1",
	"kind": "ClusterRoleBinding",
	"metadata": {
		"name": "hpa-prometheus-metrics-adapter-resources"
	},
	"roleRef": {
		"apiGroup": "rbac.authorization.k8s.io",
		"kind": "ClusterRole",
		"name": "prometheus-metrics-adapter-resources"
	},
	"subjects": [
		{
			"kind": "ServiceAccount",
			"name": "horizontal-pod-autoscaler",
			"namespace": "kube-system"
		}
	]
}`,
					`
{
	"apiVersion": "rbac.authorization.k8s.io/v1",
	"kind": "RoleBinding",
	"metadata": {
		"name": "prometheus-metrics-adapter-auth-reader",
		"namespace": "kube-system"
	},
	"roleRef": {
		"apiGroup": "rbac.authorization.k8s.io",
		"kind": "Role",
		"name": "extension-apiserver-authentication-reader"
	},
	"subjects": [
		{
			"kind": "ServiceAccount",
			"name": "prometheus-metrics-adapter",
			"namespace": "test-namespace"
		}
	]
}`,
					`
{
	"apiVersion": "apps/v1",
	"kind": "Deployment",
	"metadata": {
		"labels": {
			"app": "custom-metrics-apiserver"
		},
		"name": "prometheus-metrics-adapter",
		"namespace": "test-namespace"
	},
	"spec": {
		"replicas": 1,
		"selector": {
			"matchLabels": {
				"app": "custom-metrics-apiserver"
			}
		},
		"template": {
			"metadata": {
				"labels": {
					"app": "custom-metrics-apiserver"
				},
				"name": "custom-metrics-apiserver"
			},
			"spec": {
				"containers": [
					{
						"args": [
							"--secure-port=6443",
							"--tls-cert-file=/var/run/serving-cert/tls.crt",
							"--tls-private-key-file=/var/run/serving-cert/tls.key",
							"--logtostderr=true",
							"--prometheus-url=http://aggregate-prometheus-web.garden.svc:80/",
							"--metrics-relist-interval=2m",
							"--v=2",
							"--config=/etc/adapter/config.yaml"
						],
						"image": "test-image",
						"name": "custom-metrics-apiserver",
						"ports": [
							{
								"containerPort": 6443
							}
						],
						"volumeMounts": [
							{
								"mountPath": "/var/run/serving-cert",
								"name": "volume-serving-cert",
								"readOnly": true
							},
							{
								"mountPath": "/etc/adapter/",
								"name": "config",
								"readOnly": true
							},
							{
								"mountPath": "/tmp",
								"name": "tmp-vol"
							}
						]
					}
				],
				"serviceAccountName": "prometheus-metrics-adapter",
				"volumes": [
					{
						"name": "volume-serving-cert",
						"secret": {
							"secretName": "prometheus-metrics-adapter-server"
						}
					},
					{
						"configMap": {
							"name": "prometheus-metrics-adapter"
						},
						"name": "config"
					},
					{
						"emptyDir": {},
						"name": "tmp-vol"
					}
				]
			}
		}
	}
}`,
					`
{
	"apiVersion": "v1",
	"kind": "Service",
	"metadata": {
		"name": "prometheus-metrics-adapter",
		"namespace": "test-namespace"
	},
	"spec": {
		"ports": [
			{
				"port": 443,
				"targetPort": 6443
			}
		],
		"selector": {
			"app": "custom-metrics-apiserver"
		}
	}
}`,
					`
{
	"apiVersion": "apiregistration.k8s.io/v1",
	"kind": "APIService",
	"metadata": {
		"name": "v1beta1.custom.metrics.k8s.io"
	},
	"spec": {
		"group": "custom.metrics.k8s.io",
		"groupPriorityMinimum": 100,
		"insecureSkipTLSVerify": true,
		"service": {
			"name": "prometheus-metrics-adapter",
			"namespace": "test-namespace"
		},
		"version": "v1beta1",
		"versionPriority": 100
	}
}`,
					`
{
	"apiVersion": "apiregistration.k8s.io/v1",
	"kind": "APIService",
	"metadata": {
		"name": "v1beta2.custom.metrics.k8s.io"
	},
	"spec": {
		"group": "custom.metrics.k8s.io",
		"groupPriorityMinimum": 100,
		"insecureSkipTLSVerify": true,
		"service": {
			"name": "prometheus-metrics-adapter",
			"namespace": "test-namespace"
		},
		"version": "v1beta2",
		"versionPriority": 200
	}
}`,
					`
{
	"apiVersion": "apiregistration.k8s.io/v1",
	"kind": "APIService",
	"metadata": {
		"name": "v1beta1.external.metrics.k8s.io"
	},
	"spec": {
		"group": "external.metrics.k8s.io",
		"groupPriorityMinimum": 100,
		"insecureSkipTLSVerify": true,
		"service": {
			"name": "prometheus-metrics-adapter",
			"namespace": "test-namespace"
		},
		"version": "v1beta1",
		"versionPriority": 100
	}
}`,
				}
				//#endregion Expected resource config values as bulk JSON

				// Arrange
				createObjectOnSeed(&corev1.Secret{}, caSecretName)
				pma, ti := newPma(true)

				// Act
				Expect(pma.Deploy(ctx)).To(Succeed())

				// Assert
				actualServerCertificateSecret := corev1.Secret{}
				Expect(seedClient.Get(
					ctx,
					client.ObjectKey{Namespace: namespaceName, Name: serverCertificateSecretName},
					&actualServerCertificateSecret),
				).To(Succeed())

				Expect(ti.DeployedResourceConfigs).To(HaveLen(len(expectedResourceConfigsAsJson)))

				for i := range expectedResourceConfigsAsJson {
					actualJson, err := convertResourceConfigToJson(&ti.DeployedResourceConfigs[i])
					Expect(err).To(Succeed())
					message := fmt.Sprintf(
						"The actual resource config JSON at position %d had unexpected value. Actual:\n%s\n"+
							"Expected:\n%s\n",
						i,
						actualJson,
						expectedResourceConfigsAsJson[i])
					Expect(actualJson).To(Equal(expectedResourceConfigsAsJson[i]), message)
				}
			})
			It("should fail if CA certificate is missing on the seed", func() {
				// Arrange
				pma, ti := newPma(true)

				// Act
				err := pma.Deploy(ctx)

				// Assert
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(MatchRegexp(".*CA.*certificate.*secret.*"))
				Expect(ti.DeployedResourceConfigs).To(BeNil())
			})
		})
		Context("in disabled state", func() {
			It("should not fail if CA certificate is missing on the seed", func() {
				// Arrange
				actualServerCertificateSecret := corev1.Secret{}
				err := seedClient.Get(
					ctx,
					client.ObjectKey{Namespace: namespaceName, Name: caSecretName},
					&actualServerCertificateSecret)
				Expect(err.Error()).To(MatchRegexp(".*not.*found.*"))

				pma, _ := newPma(false)

				// Act
				Expect(pma.Deploy(ctx)).To(Succeed())

				// Assert
			})
			It("should not deploy any resources to the seed", func() {
				// Arrange
				pma, ti := newPma(false)

				// Act
				Expect(pma.Deploy(ctx)).To(Succeed())

				// Assert
				assertNoServerCertificateOnServer()
				Expect(ti.DeployedResourceConfigs).To(BeNil())
			})
		})
	})
	Describe(".Destroy()", func() {
		Context("in enabled state", func() {
			It("should destroy the resources on the seed", func() {
				// Arrange
				createObjectOnSeed(&corev1.Secret{}, serverCertificateSecretName)
				createObjectOnSeed(&resourcesv1alpha1.ManagedResource{}, managedResourceName)
				pma, _ := newPma(true)

				// Act
				Expect(pma.Destroy(ctx)).To(Succeed())

				// Assert
				assertNoManagedResourceOnServer()
			})
			It("should not fail if resources are missing on the seed", func() {
				// Arrange
				pma, _ := newPma(true)

				// Act
				Expect(pma.Destroy(ctx)).To(Succeed())

				// Assert
			})
		})
		Context("in disabled state", func() {
			It("should destroy the resources on the seed", func() {
				// Arrange
				createObjectOnSeed(&corev1.Secret{}, serverCertificateSecretName)
				createObjectOnSeed(&resourcesv1alpha1.ManagedResource{}, managedResourceName)
				pma, _ := newPma(false)

				// Act
				Expect(pma.Destroy(ctx)).To(Succeed())

				// Assert
				assertNoManagedResourceOnServer()
			})
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

		Describe(".Wait()", func() {
			It("should fail because reading the ManagedResource fails", func() {
				// Arrange
				pma, _ := newPma(true)

				// Act
				Expect(pma.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the ManagedResource doesn't become healthy", func() {
				// Arrange
				pma, _ := newPma(true)
				fakeOps.MaxAttempts = 2

				Expect(seedClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespaceName,
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

				// Act
				Expect(pma.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should successfully wait for the managed resource to become healthy", func() {
				// Arrange
				pma, _ := newPma(true)
				fakeOps.MaxAttempts = 2

				Expect(seedClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespaceName,
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

				// Act
				Expect(pma.Wait(ctx)).To(Succeed())
			})
		})

		Describe(".WaitCleanup()", func() {
			It("should fail when the wait for the managed resource deletion times out", func() {
				// Arrange
				createObjectOnSeed(&corev1.Secret{}, serverCertificateSecretName)
				createObjectOnSeed(&resourcesv1alpha1.ManagedResource{}, managedResourceName)
				pma, _ := newPma(true)
				fakeOps.MaxAttempts = 2

				// Act
				Expect(pma.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it's already removed", func() {
				// Arrange
				pma, _ := newPma(true)
				Expect(pma.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})
