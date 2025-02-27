// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package admissioncontroller_test

import (
	"context"
	"encoding/json"

	"github.com/Masterminds/semver/v3"
	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	admissioncontrollerconfigv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/gardener/admissioncontroller"
	"github.com/gardener/gardener/pkg/logger"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

const (
	managedResourceNameRuntime = "gardener-admission-controller-runtime"
	managedResourceNameVirtual = "gardener-admission-controller-virtual"
)

var _ = Describe("GardenerAdmissionController", func() {
	var (
		ctx context.Context

		fakeOps           *retryfake.Ops
		fakeClient        client.Client
		fakeSecretManager secretsmanager.Interface
		deployer          component.DeployWaiter
		testValues        Values
		consistOf         func(...client.Object) types.GomegaMatcher

		namespace = "some-namespace"
	)

	BeforeEach(func() {
		ctx = context.Background()

		fakeOps = &retryfake.Ops{MaxAttempts: 2}
		DeferCleanup(test.WithVars(
			&retry.Until, fakeOps.Until,
		))

		fakeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()
		fakeSecretManager = fakesecretsmanager.New(fakeClient, namespace)
		consistOf = NewManagedResourceConsistOfObjectsMatcher(fakeClient)

		testValues = Values{}
	})

	JustBeforeEach(func() {
		deployer = New(fakeClient, namespace, fakeSecretManager, testValues)
	})

	Describe("#Deploy", func() {
		BeforeEach(func() {
			blockMode := admissioncontrollerconfigv1alpha1.ResourceAdmissionWebhookMode("block")

			// These are typical configuration values set for the admission controller and serves as the base for the following tests.
			testValues = Values{
				ResourceAdmissionConfiguration: &admissioncontrollerconfigv1alpha1.ResourceAdmissionConfiguration{
					Limits: []admissioncontrollerconfigv1alpha1.ResourceLimit{
						{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"secrets", "configmaps"},
							Size:        resource.MustParse("1Mi"),
						},
						{
							APIGroups:   []string{"core.gardener.cloud"},
							APIVersions: []string{"v1beta1"},
							Resources:   []string{"shoots"},
							Size:        resource.MustParse("100Ki"),
						},
					},
					UnrestrictedSubjects: []rbacv1.Subject{{
						Kind:      "ServiceAccount",
						Name:      "foo",
						Namespace: "default",
					}},
					OperationMode: &blockMode,
				},
				SeedRestrictionEnabled:      true,
				TopologyAwareRoutingEnabled: false,
			}

			Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-gardener", Namespace: namespace}})).To(Succeed())
			Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: namespace}})).To(Succeed())
		})

		Context("with common values", func() {
			It("should successfully deploy", func() {
				Expect(deployer.Deploy(ctx)).To(Succeed())
				verifyExpectations(ctx, fakeClient, consistOf, fakeSecretManager, namespace, "4ef77c17", testValues)
			})
		})

		Context("when TopologyAwareRouting is disabled", func() {
			BeforeEach(func() {
				testValues.TopologyAwareRoutingEnabled = false
			})

			It("should successfully deploy", func() {
				Expect(deployer.Deploy(ctx)).To(Succeed())
				verifyExpectations(ctx, fakeClient, consistOf, fakeSecretManager, namespace, "4ef77c17", testValues)
			})
		})

		Context("when TopologyAwareRouting is enabled", func() {
			BeforeEach(func() {
				testValues.TopologyAwareRoutingEnabled = true
			})

			When("runtime Kubernetes version is >= 1.32", func() {
				BeforeEach(func() {
					testValues.RuntimeVersion = semver.MustParse("1.32.1")
				})

				It("should successfully deploy", func() {
					Expect(deployer.Deploy(ctx)).To(Succeed())
					verifyExpectations(ctx, fakeClient, consistOf, fakeSecretManager, namespace, "4ef77c17", testValues)
				})
			})

			When("runtime Kubernetes version is 1.31", func() {
				BeforeEach(func() {
					testValues.RuntimeVersion = semver.MustParse("1.31.2")
				})

				It("should successfully deploy", func() {
					Expect(deployer.Deploy(ctx)).To(Succeed())
					verifyExpectations(ctx, fakeClient, consistOf, fakeSecretManager, namespace, "4ef77c17", testValues)
				})
			})

			When("runtime Kubernetes version is < 1.31", func() {
				BeforeEach(func() {
					testValues.RuntimeVersion = semver.MustParse("1.30.3")
				})

				It("should successfully deploy", func() {
					Expect(deployer.Deploy(ctx)).To(Succeed())
					verifyExpectations(ctx, fakeClient, consistOf, fakeSecretManager, namespace, "4ef77c17", testValues)
				})
			})
		})

		Context("without ResourceAdmissionConfiguration", func() {
			BeforeEach(func() {
				testValues.ResourceAdmissionConfiguration = nil
			})

			It("should successfully deploy", func() {
				Expect(deployer.Deploy(ctx)).To(Succeed())
				verifyExpectations(ctx, fakeClient, consistOf, fakeSecretManager, namespace, "6d282905", testValues)
			})
		})

		Context("without seed restriction webhook", func() {
			BeforeEach(func() {
				testValues.SeedRestrictionEnabled = false
			})

			It("should successfully deploy", func() {
				Expect(deployer.Deploy(ctx)).To(Succeed())
				verifyExpectations(ctx, fakeClient, consistOf, fakeSecretManager, namespace, "4ef77c17", testValues)
			})
		})
	})

	Describe("#Wait", func() {
		Context("when ManagedResources are ready", func() {
			It("should successfully wait", func() {
				runtimeManagedResource := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      managedResourceNameRuntime,
						Namespace: namespace,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						Conditions: []gardencorev1beta1.Condition{
							{Type: resourcesv1alpha1.ResourcesApplied, Status: gardencorev1beta1.ConditionTrue},
							{Type: resourcesv1alpha1.ResourcesHealthy, Status: gardencorev1beta1.ConditionTrue},
						},
					},
				}
				Expect(fakeClient.Create(ctx, runtimeManagedResource)).To(Succeed())

				virtualManagedResource := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      managedResourceNameVirtual,
						Namespace: namespace,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						Conditions: []gardencorev1beta1.Condition{
							{Type: resourcesv1alpha1.ResourcesApplied, Status: gardencorev1beta1.ConditionTrue},
							{Type: resourcesv1alpha1.ResourcesHealthy, Status: gardencorev1beta1.ConditionTrue},
						},
					},
				}
				Expect(fakeClient.Create(ctx, virtualManagedResource)).To(Succeed())

				Expect(deployer.Wait(ctx)).To(Succeed())
			})
		})

		Context("when Runtime ManagedResource doesn't get ready", func() {
			It("should time out waiting", func() {
				runtimeManagedResource := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      managedResourceNameRuntime,
						Namespace: namespace,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						Conditions: []gardencorev1beta1.Condition{
							{Type: resourcesv1alpha1.ResourcesApplied, Status: gardencorev1beta1.ConditionFalse},
							{Type: resourcesv1alpha1.ResourcesHealthy, Status: gardencorev1beta1.ConditionTrue},
						},
					},
				}
				Expect(fakeClient.Create(ctx, runtimeManagedResource)).To(Succeed())

				virtualManagedResource := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      managedResourceNameVirtual,
						Namespace: namespace,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						Conditions: []gardencorev1beta1.Condition{
							{Type: resourcesv1alpha1.ResourcesApplied, Status: gardencorev1beta1.ConditionTrue},
							{Type: resourcesv1alpha1.ResourcesHealthy, Status: gardencorev1beta1.ConditionTrue},
						},
					},
				}
				Expect(fakeClient.Create(ctx, virtualManagedResource)).To(Succeed())

				err := deployer.Wait(ctx)

				multiErr, ok := err.(*multierror.Error)
				Expect(ok).To(BeTrue())
				Expect(multiErr.Errors).To(HaveLen(1))
				Expect(multiErr.Errors[0]).To(MatchError("retry failed with max attempts reached, last error: managed resource some-namespace/gardener-admission-controller-runtime is not healthy"))
			})
		})

		Context("when Virtual ManagedResource doesn't get ready", func() {
			It("should time out waiting", func() {
				runtimeManagedResource := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      managedResourceNameRuntime,
						Namespace: namespace,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						Conditions: []gardencorev1beta1.Condition{
							{Type: resourcesv1alpha1.ResourcesApplied, Status: gardencorev1beta1.ConditionTrue},
							{Type: resourcesv1alpha1.ResourcesHealthy, Status: gardencorev1beta1.ConditionTrue},
						},
					},
				}
				Expect(fakeClient.Create(ctx, runtimeManagedResource)).To(Succeed())

				virtualManagedResource := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      managedResourceNameVirtual,
						Namespace: namespace,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						Conditions: []gardencorev1beta1.Condition{
							{Type: resourcesv1alpha1.ResourcesApplied, Status: gardencorev1beta1.ConditionTrue},
							{Type: resourcesv1alpha1.ResourcesHealthy, Status: gardencorev1beta1.ConditionProgressing},
						},
					},
				}
				Expect(fakeClient.Create(ctx, virtualManagedResource)).To(Succeed())

				err := deployer.Wait(ctx)

				multiErr, ok := err.(*multierror.Error)
				Expect(ok).To(BeTrue())
				Expect(multiErr.Errors).To(HaveLen(1))
				Expect(multiErr.Errors[0]).To(MatchError("retry failed with max attempts reached, last error: managed resource some-namespace/gardener-admission-controller-virtual is not healthy"))
			})
		})

		Context("when ManagedResources are not available", func() {
			It("should time out waiting", func() {
				Expect(deployer.Wait(ctx).Error()).To(And(
					ContainSubstring("managedresources.resources.gardener.cloud \"gardener-admission-controller-virtual\" not found"),
					ContainSubstring("managedresources.resources.gardener.cloud \"gardener-admission-controller-runtime\" not found"),
				))
			})
		})
	})

	Describe("#WaitCleanup", func() {
		Context("when ManagedResources are not available", func() {
			It("should successfully wait", func() {
				Expect(deployer.WaitCleanup(ctx)).To(Succeed())
			})
		})

		Context("when Runtime ManagedResource is still available", func() {
			It("should time out waiting", func() {
				runtimeManagedResource := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      managedResourceNameRuntime,
						Namespace: namespace,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						Conditions: []gardencorev1beta1.Condition{
							{Type: resourcesv1alpha1.ResourcesApplied, Status: gardencorev1beta1.ConditionFalse},
							{Type: resourcesv1alpha1.ResourcesHealthy, Status: gardencorev1beta1.ConditionTrue},
						},
					},
				}
				Expect(fakeClient.Create(ctx, runtimeManagedResource)).To(Succeed())

				err := deployer.WaitCleanup(ctx)

				multiErr, ok := err.(*multierror.Error)
				Expect(ok).To(BeTrue())
				Expect(multiErr.Errors).To(HaveLen(1))
				Expect(multiErr.Errors[0]).To(MatchError("retry failed with max attempts reached, last error: resource some-namespace/gardener-admission-controller-runtime still exists"))
			})
		})

		Context("when Virtual ManagedResource is still available", func() {
			It("should time out waiting", func() {
				runtimeManagedResource := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      managedResourceNameVirtual,
						Namespace: namespace,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						Conditions: []gardencorev1beta1.Condition{
							{Type: resourcesv1alpha1.ResourcesApplied, Status: gardencorev1beta1.ConditionTrue},
							{Type: resourcesv1alpha1.ResourcesHealthy, Status: gardencorev1beta1.ConditionTrue},
						},
					},
				}
				Expect(fakeClient.Create(ctx, runtimeManagedResource)).To(Succeed())

				err := deployer.WaitCleanup(ctx)

				multiErr, ok := err.(*multierror.Error)
				Expect(ok).To(BeTrue())
				Expect(multiErr.Errors).To(HaveLen(1))
				Expect(multiErr.Errors[0]).To(MatchError("retry failed with max attempts reached, last error: resource some-namespace/gardener-admission-controller-virtual still exists"))
			})
		})
	})

	Describe("#Destroy", func() {
		Context("when resources don't exist", func() {
			It("should successful destroy", func() {
				Expect(deployer.Destroy(ctx)).To(Succeed())

				verifyResourcesGone(ctx, fakeClient, namespace)
			})
		})

		It("should successful destroy", func() {
			runtimeManagedResource := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceNameRuntime,
					Namespace: namespace,
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					SecretRefs: []corev1.LocalObjectReference{{Name: "managedresource-" + managedResourceNameRuntime}},
				},
			}
			runtimeManagedResourceSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      runtimeManagedResource.Spec.SecretRefs[0].Name,
					Namespace: namespace,
				},
			}

			Expect(fakeClient.Create(ctx, runtimeManagedResourceSecret)).To(Succeed())
			Expect(fakeClient.Create(ctx, runtimeManagedResource)).To(Succeed())

			Expect(deployer.Destroy(ctx)).To(Succeed())

			verifyResourcesGone(ctx, fakeClient, namespace)
		})
	})
})

func verifyResourcesGone(ctx context.Context, fakeClient client.Client, namespace string) {
	ExpectWithOffset(1, fakeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "managedresource-" + managedResourceNameRuntime}, &corev1.Secret{})).To(BeNotFoundError())
	ExpectWithOffset(1, fakeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: managedResourceNameRuntime}, &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError())
	ExpectWithOffset(1, fakeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "managedresource-" + managedResourceNameVirtual}, &corev1.Secret{})).To(BeNotFoundError())
	ExpectWithOffset(1, fakeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: managedResourceNameVirtual}, &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError())
	ExpectWithOffset(1, fakeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "shoot-access-gardener-admission-controller"}, &corev1.Secret{})).To(BeNotFoundError())
}

func verifyExpectations(ctx context.Context, fakeClient client.Client, consistOf func(...client.Object) types.GomegaMatcher, fakeSecretManager secretsmanager.Interface, namespace, configMapChecksum string, testValues Values) {
	By("Check Gardener Access Secret")
	accessSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shoot-access-gardener-admission-controller",
			Namespace: namespace,
			Labels: map[string]string{
				"resources.gardener.cloud/purpose": "token-requestor",
				"resources.gardener.cloud/class":   "shoot",
			},
			Annotations: map[string]string{
				"serviceaccount.resources.gardener.cloud/name":      "gardener-admission-controller",
				"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
			},
		},
		Type: corev1.SecretTypeOpaque,
	}

	actualShootAccessSecret := &corev1.Secret{}
	ExpectWithOffset(1, fakeClient.Get(ctx, client.ObjectKeyFromObject(accessSecret), actualShootAccessSecret)).To(Succeed())
	accessSecret.ResourceVersion = "1"
	ExpectWithOffset(1, actualShootAccessSecret).To(Equal(accessSecret))

	By("Check Runtime Cluster Resources")
	serverCert, ok := fakeSecretManager.Get("gardener-admission-controller-cert")
	ExpectWithOffset(1, ok).To(BeTrue())

	runtimeMr := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{
		Name:      managedResourceNameRuntime,
		Namespace: namespace,
	}}
	ExpectWithOffset(1, fakeClient.Get(ctx, client.ObjectKeyFromObject(runtimeMr), runtimeMr)).To(Succeed())
	ExpectWithOffset(1, runtimeMr.Labels).To(Equal(map[string]string{
		"gardener.cloud/role":                "seed-system-component",
		"care.gardener.cloud/condition-type": "VirtualComponentsHealthy",
	}))
	ExpectWithOffset(1, runtimeMr).To(consistOf(
		configMap(namespace, testValues),
		deployment(namespace, "gardener-admission-controller-"+configMapChecksum, serverCert.Name, testValues),
		service(namespace, testValues),
		vpa(namespace),
		podDisruptionBudget(namespace),
		serviceMonitor(namespace),
	))

	runtimeManagedResourceSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      runtimeMr.Spec.SecretRefs[0].Name,
			Namespace: namespace,
		},
	}
	ExpectWithOffset(1, fakeClient.Get(ctx, client.ObjectKeyFromObject(runtimeManagedResourceSecret), runtimeManagedResourceSecret)).To(Succeed())
	ExpectWithOffset(1, runtimeManagedResourceSecret.Immutable).To(Equal(ptr.To(true)))
	ExpectWithOffset(1, runtimeManagedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

	By("Check Virtual Cluster Resources")
	virtualMr := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{
		Name:      managedResourceNameVirtual,
		Namespace: namespace,
	}}
	ExpectWithOffset(1, fakeClient.Get(ctx, client.ObjectKeyFromObject(virtualMr), virtualMr)).To(Succeed())
	ExpectWithOffset(1, virtualMr.Labels).To(Equal(map[string]string{
		"origin":                             "gardener",
		"care.gardener.cloud/condition-type": "VirtualComponentsHealthy",
	}))
	caGardener, ok := fakeSecretManager.Get("ca-gardener")
	ExpectWithOffset(1, virtualMr).To(consistOf(
		clusterRole(),
		clusterRoleBinding(),
		validatingWebhookConfiguration(namespace, caGardener.Data["bundle.crt"], testValues),
	))

	virtualManagedResourceSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      virtualMr.Spec.SecretRefs[0].Name,
			Namespace: namespace,
		},
	}
	ExpectWithOffset(1, fakeClient.Get(ctx, client.ObjectKeyFromObject(virtualManagedResourceSecret), virtualManagedResourceSecret)).To(Succeed())
	ExpectWithOffset(1, ok).To(BeTrue())
	ExpectWithOffset(1, virtualManagedResourceSecret.Immutable).To(Equal(ptr.To(true)))
	ExpectWithOffset(1, virtualManagedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
}

func configMap(namespace string, testValues Values) *corev1.ConfigMap {
	admissionConfig := &admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admissioncontroller.config.gardener.cloud/v1alpha1",
			Kind:       "AdmissionControllerConfiguration",
		},
		GardenClientConnection: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
			QPS:        100,
			Burst:      130,
			Kubeconfig: "/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
		},
		LogLevel:  testValues.LogLevel,
		LogFormat: logger.FormatJSON,
		Server: admissioncontrollerconfigv1alpha1.ServerConfiguration{
			Webhooks: admissioncontrollerconfigv1alpha1.HTTPSServer{
				Server: admissioncontrollerconfigv1alpha1.Server{Port: 2719},
				TLS:    admissioncontrollerconfigv1alpha1.TLSServer{ServerCertDir: "/etc/gardener-admission-controller/srv"},
			},
			HealthProbes:                   &admissioncontrollerconfigv1alpha1.Server{Port: 2722},
			Metrics:                        &admissioncontrollerconfigv1alpha1.Server{Port: 2723},
			ResourceAdmissionConfiguration: testValues.ResourceAdmissionConfiguration,
		},
	}

	data, err := json.Marshal(admissionConfig)
	utilruntime.Must(err)
	data, err = yaml.JSONToYAML(data)
	utilruntime.Must(err)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app":  "gardener",
				"role": "admission-controller",
			},
			Name:      "gardener-admission-controller",
			Namespace: namespace,
		},
		Data: map[string]string{
			"config.yaml": string(data),
		},
	}
	utilruntime.Must(kubernetesutils.MakeUnique(configMap))

	return configMap
}

func deployment(namespace, configSecretName, serverCertSecretName string, testValues Values) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gardener-admission-controller",
			Namespace: namespace,
			Labels: map[string]string{
				"app":  "gardener",
				"role": "admission-controller",
				"high-availability-config.resources.gardener.cloud/type": "server",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:             ptr.To[int32](1),
			RevisionHistoryLimit: ptr.To[int32](2),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":  "gardener",
					"role": "admission-controller",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: gardenerutils.MergeStringMaps(GetLabels(), map[string]string{
						"app":                              "gardener",
						"role":                             "admission-controller",
						"networking.gardener.cloud/to-dns": "allowed",
						"networking.resources.gardener.cloud/to-virtual-garden-kube-apiserver-tcp-443": "allowed",
					}),
				},
				Spec: corev1.PodSpec{
					PriorityClassName:            "gardener-garden-system-400",
					AutomountServiceAccountToken: ptr.To(false),
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: ptr.To(true),
						RunAsUser:    ptr.To[int64](65532),
						RunAsGroup:   ptr.To[int64](65532),
						FSGroup:      ptr.To[int64](65532),
					},
					Containers: []corev1.Container{
						{
							Name:            "gardener-admission-controller",
							Image:           testValues.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args: []string{
								"--config=/etc/gardener-admission-controller/config/config.yaml",
							},
							Resources: corev1.ResourceRequirements{
								Requests: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("200Mi"),
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/healthz",
										Port:   intstr.FromInt32(2722),
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 15,
								TimeoutSeconds:      5,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/readyz",
										Port:   intstr.FromInt32(2722),
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 10,
								TimeoutSeconds:      5,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "gardener-admission-controller-cert",
									MountPath: "/etc/gardener-admission-controller/srv",
									ReadOnly:  true,
								},
								{
									Name:      "gardener-admission-controller-config",
									MountPath: "/etc/gardener-admission-controller/config",
								},
								{
									Name:      "kubeconfig",
									MountPath: "/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "gardener-admission-controller-cert",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  serverCertSecretName,
									DefaultMode: ptr.To[int32](0640),
								},
							},
						},
						{
							Name: "gardener-admission-controller-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: configSecretName},
								},
							},
						},
						{
							Name: "kubeconfig",
							VolumeSource: corev1.VolumeSource{
								Projected: &corev1.ProjectedVolumeSource{
									DefaultMode: ptr.To[int32](420),
									Sources: []corev1.VolumeProjection{
										{
											Secret: &corev1.SecretProjection{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: "generic-token-kubeconfig",
												},
												Items: []corev1.KeyToPath{{
													Key:  "kubeconfig",
													Path: "kubeconfig",
												}},
												Optional: ptr.To(false),
											},
										},
										{
											Secret: &corev1.SecretProjection{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: "shoot-access-gardener-admission-controller",
												},
												Items: []corev1.KeyToPath{{
													Key:  "token",
													Path: "token",
												}},
												Optional: ptr.To(false),
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	utilruntime.Must(references.InjectAnnotations(deployment))

	return deployment
}

func service(namespace string, testValues Values) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gardener-admission-controller",
			Namespace: namespace,
			Labels: map[string]string{
				"app":  "gardener",
				"role": "admission-controller",
			},
			Annotations: map[string]string{
				"networking.resources.gardener.cloud/from-all-webhook-targets-allowed-ports":       `[{"protocol":"TCP","port":2719}]`,
				"networking.resources.gardener.cloud/from-all-garden-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":2723}]`,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app":  "gardener",
				"role": "admission-controller",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "https",
					Protocol:   corev1.ProtocolTCP,
					Port:       443,
					TargetPort: intstr.FromInt32(2719),
				},
				{
					Name:       "metrics",
					Protocol:   corev1.ProtocolTCP,
					Port:       2723,
					TargetPort: intstr.FromInt32(2723),
				},
			},
		},
	}

	if testValues.TopologyAwareRoutingEnabled {
		if versionutils.ConstraintK8sGreaterEqual132.Check(testValues.RuntimeVersion) {
			svc.Spec.TrafficDistribution = ptr.To(corev1.ServiceTrafficDistributionPreferClose)
		} else if versionutils.ConstraintK8sEqual131.Check(testValues.RuntimeVersion) {
			svc.Spec.TrafficDistribution = ptr.To(corev1.ServiceTrafficDistributionPreferClose)
			metav1.SetMetaDataLabel(&svc.ObjectMeta, "endpoint-slice-hints.resources.gardener.cloud/consider", "true")
		} else {
			metav1.SetMetaDataAnnotation(&svc.ObjectMeta, "service.kubernetes.io/topology-mode", "auto")
			metav1.SetMetaDataLabel(&svc.ObjectMeta, "endpoint-slice-hints.resources.gardener.cloud/consider", "true")
		}
	}

	return svc
}

func podDisruptionBudget(namespace string) *policyv1.PodDisruptionBudget {
	return &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName,
			Namespace: namespace,
			Labels:    GetLabels(),
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable:             ptr.To(intstr.FromInt32(1)),
			Selector:                   &metav1.LabelSelector{MatchLabels: GetLabels()},
			UnhealthyPodEvictionPolicy: ptr.To(policyv1.AlwaysAllow),
		},
	}
}

func serviceMonitor(namespace string) *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "garden-gardener-admission-controller",
			Namespace: namespace,
			Labels:    map[string]string{"prometheus": "garden"},
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "gardener", "role": "admission-controller"}},
			Endpoints: []monitoringv1.Endpoint{{
				Port: "metrics",
				MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
					SourceLabels: []monitoringv1.LabelName{"__name__"},
					Action:       "keep",
					Regex:        `^(gardener_admission_controller_.+|rest_client_.+|controller_runtime_.+|go_.+)$`,
				}},
			}},
		},
	}
}

func vpa(namespace string) *vpaautoscalingv1.VerticalPodAutoscaler {
	autoUpdateMode := vpaautoscalingv1.UpdateModeAuto

	return &vpaautoscalingv1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gardener-admission-controller",
			Namespace: namespace,
			Labels: map[string]string{
				"app":  "gardener",
				"role": "admission-controller",
			},
		},
		Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingv1.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "gardener-admission-controller",
			},
			UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: &autoUpdateMode,
			},
			ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
				ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
					{
						ContainerName: "*",
						MinAllowed: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("25Mi"),
						},
					},
				},
			},
		},
	}
}

func clusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gardener.cloud:system:admission-controller",
			Labels: map[string]string{
				"app":  "gardener",
				"role": "admission-controller",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"core.gardener.cloud"},
				Resources: []string{
					"backupbuckets",
					"backupentries",
					"controllerinstallations",
					"secretbindings",
					"seeds",
					"shoots",
					"projects",
				},
				Verbs: []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"seedmanagement.gardener.cloud"},
				Resources: []string{
					"gardenlets",
					"managedseeds",
				},
				Verbs: []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"operations.gardener.cloud"},
				Resources: []string{
					"bastions",
				},
				Verbs: []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{
					"configmaps",
				},
				Verbs: []string{"get"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{
					"namespaces",
					"secrets",
					"serviceaccounts",
				},
				Verbs: []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{
					"leases",
				},
				Verbs: []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"certificates.k8s.io"},
				Resources: []string{
					"certificatesigningrequests",
				},
				Verbs: []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"security.gardener.cloud"},
				Resources: []string{
					"credentialsbindings",
				},
				Verbs: []string{"get", "list", "watch"},
			},
		},
	}
}

func clusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gardener.cloud:admission-controller",
			Labels: map[string]string{
				"app":  "gardener",
				"role": "admission-controller",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "gardener.cloud:system:admission-controller",
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      "gardener-admission-controller",
			Namespace: "kube-system",
		}},
	}
}

func validatingWebhookConfiguration(namespace string, caBundle []byte, testValues Values) *admissionregistrationv1.ValidatingWebhookConfiguration {
	var (
		failurePolicyFail     = admissionregistrationv1.Fail
		sideEffectsNone       = admissionregistrationv1.SideEffectClassNone
		matchPolicyEquivalent = admissionregistrationv1.Equivalent
	)

	webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gardener-admission-controller",
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				Name:                    "validate-namespace-deletion.gardener.cloud",
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				TimeoutSeconds:          ptr.To[int32](10),
				Rules: []admissionregistrationv1.RuleWithOperations{{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Delete},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"namespaces"},
					},
				}},
				FailurePolicy: &failurePolicyFail,
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"gardener.cloud/role": "project",
					},
				},
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL:      ptr.To("https://gardener-admission-controller." + namespace + "/webhooks/validate-namespace-deletion"),
					CABundle: caBundle,
				},
				SideEffects: &sideEffectsNone,
			},
			{
				Name:                    "validate-kubeconfig-secrets.gardener.cloud",
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				TimeoutSeconds:          ptr.To[int32](10),
				Rules: []admissionregistrationv1.RuleWithOperations{{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"secrets"},
					},
				}},
				FailurePolicy: &failurePolicyFail,
				NamespaceSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{Key: "gardener.cloud/role", Operator: metav1.LabelSelectorOpIn, Values: []string{"project"}},
						{Key: "app", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"gardener"}},
					},
				},
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL:      ptr.To("https://gardener-admission-controller." + namespace + "/webhooks/validate-kubeconfig-secrets"),
					CABundle: caBundle,
				},
				SideEffects: &sideEffectsNone,
			},
			{
				Name:                    "internal-domain-secret.gardener.cloud",
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				TimeoutSeconds:          ptr.To[int32](10),
				Rules: []admissionregistrationv1.RuleWithOperations{{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update, admissionregistrationv1.Delete},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"secrets"},
					},
				}},
				FailurePolicy: &failurePolicyFail,
				ObjectSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"role": "internal-domain",
					},
				},
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL:      ptr.To("https://gardener-admission-controller." + namespace + "/webhooks/admission/validate-internal-domain"),
					CABundle: caBundle,
				},
				SideEffects: &sideEffectsNone,
			},
			{
				Name:                    "audit-policies.gardener.cloud",
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				TimeoutSeconds:          ptr.To[int32](10),
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{gardencorev1beta1.GroupName},
							APIVersions: []string{"v1beta1"},
							Resources:   []string{"shoots"},
						},
					},
					{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"configmaps"},
						},
					},
				},
				FailurePolicy: &failurePolicyFail,
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"gardener.cloud/role": "project",
					},
				},
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL:      ptr.To("https://gardener-admission-controller." + namespace + "/webhooks/audit-policies"),
					CABundle: caBundle,
				},
				SideEffects: &sideEffectsNone,
			},
			{
				Name:                    "authentication-configuration.gardener.cloud",
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				TimeoutSeconds:          ptr.To[int32](10),
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{gardencorev1beta1.GroupName},
							APIVersions: []string{"v1beta1"},
							Resources:   []string{"shoots"},
						},
					},
					{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"configmaps"},
						},
					},
				},
				FailurePolicy: &failurePolicyFail,
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"gardener.cloud/role": "project",
					},
				},
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL:      ptr.To("https://gardener-admission-controller." + namespace + "/webhooks/authentication-configuration"),
					CABundle: caBundle,
				},
				SideEffects: &sideEffectsNone,
			},
			{
				Name:                    "authorization-configuration.gardener.cloud",
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				TimeoutSeconds:          ptr.To[int32](10),
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{gardencorev1beta1.GroupName},
							APIVersions: []string{"v1beta1"},
							Resources:   []string{"shoots"},
						},
					},
					{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"configmaps"},
						},
					},
				},
				FailurePolicy: &failurePolicyFail,
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"gardener.cloud/role": "project",
					},
				},
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL:      ptr.To("https://gardener-admission-controller." + namespace + "/webhooks/authorization-configuration"),
					CABundle: caBundle,
				},
				SideEffects: &sideEffectsNone,
			},
			{
				Name:                    "shoot-kubeconfig-secret-ref.gardener.cloud",
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				TimeoutSeconds:          ptr.To[int32](10),
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"secrets"},
						},
					},
				},
				FailurePolicy: &failurePolicyFail,
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"gardener.cloud/role": "project",
					},
				},
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL:      ptr.To("https://gardener-admission-controller." + namespace + "/webhooks/validate-shoot-kubeconfig-secret-ref"),
					CABundle: caBundle,
				},
				SideEffects: &sideEffectsNone,
			},
			{
				Name:                    "update-restriction.gardener.cloud",
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				TimeoutSeconds:          ptr.To[int32](10),
				Rules: []admissionregistrationv1.RuleWithOperations{
					{
						Operations: []admissionregistrationv1.OperationType{
							admissionregistrationv1.Create,
							admissionregistrationv1.Update,
							admissionregistrationv1.Delete,
						},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{corev1.GroupName},
							APIVersions: []string{"v1"},
							Resources:   []string{"secrets", "configmaps"},
						},
					},
				},
				FailurePolicy: &failurePolicyFail,
				ObjectSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"gardener.cloud/update-restriction": "true",
					},
				},
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL:      ptr.To("https://gardener-admission-controller." + namespace + "/webhooks/update-restriction"),
					CABundle: caBundle,
				},
				SideEffects: &sideEffectsNone,
			},
		},
	}

	if testValues.ResourceAdmissionConfiguration != nil {
		webhookConfig.Webhooks = append(webhookConfig.Webhooks, admissionregistrationv1.ValidatingWebhook{
			Name:                    "validate-resource-size.gardener.cloud",
			AdmissionReviewVersions: []string{"v1", "v1beta1"},
			TimeoutSeconds:          ptr.To[int32](10),
			Rules: []admissionregistrationv1.RuleWithOperations{
				{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"secrets", "configmaps"},
					},
				},
				{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"core.gardener.cloud"},
						APIVersions: []string{"v1beta1"},
						Resources:   []string{"shoots"},
					},
				},
			},
			FailurePolicy: &failurePolicyFail,
			NamespaceSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "gardener.cloud/role", Operator: metav1.LabelSelectorOpIn, Values: []string{"project"}},
					{Key: "app", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"gardener"}},
				},
			},
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				URL:      ptr.To("https://gardener-admission-controller." + namespace + "/webhooks/validate-resource-size"),
				CABundle: caBundle,
			},
			SideEffects: &sideEffectsNone,
		})
	}

	if testValues.SeedRestrictionEnabled {
		webhookConfig.Webhooks = append(webhookConfig.Webhooks, admissionregistrationv1.ValidatingWebhook{
			Name:                    "seed-restriction.gardener.cloud",
			AdmissionReviewVersions: []string{"v1", "v1beta1"},
			TimeoutSeconds:          ptr.To[int32](10),
			Rules: []admissionregistrationv1.RuleWithOperations{
				{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"secrets", "serviceaccounts"},
					},
				},
				{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{rbacv1.GroupName},
						APIVersions: []string{"v1"},
						Resources:   []string{"clusterrolebindings"},
					},
				},
				{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{coordinationv1.GroupName},
						APIVersions: []string{"v1"},
						Resources:   []string{"leases"},
					},
				},
				{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{certificatesv1.GroupName},
						APIVersions: []string{"v1"},
						Resources:   []string{"certificatesigningrequests"},
					},
				},
				{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{gardencorev1beta1.GroupName},
						APIVersions: []string{"v1beta1"},
						Resources:   []string{"backupentries", "internalsecrets", "shootstates"},
					},
				},
				{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Delete},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{gardencorev1beta1.GroupName},
						APIVersions: []string{"v1beta1"},
						Resources:   []string{"backupbuckets"},
					},
				},
				{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update, admissionregistrationv1.Delete},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{gardencorev1beta1.GroupName},
						APIVersions: []string{"v1beta1"},
						Resources:   []string{"seeds"},
					},
				},
				{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{operationsv1alpha1.GroupName},
						APIVersions: []string{"v1alpha1"},
						Resources:   []string{"bastions"},
					},
				},
			},
			FailurePolicy: &failurePolicyFail,
			MatchPolicy:   &matchPolicyEquivalent,
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				URL:      ptr.To("https://gardener-admission-controller." + namespace + "/webhooks/admission/seedrestriction"),
				CABundle: caBundle,
			},
			SideEffects: &sideEffectsNone,
		})
	}

	return webhookConfig
}
