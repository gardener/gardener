// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Shoot Care controller tests", func() {
	var (
		seedNamespace        *corev1.Namespace
		secret               *corev1.Secret
		internalDomainSecret *corev1.Secret
		secretBinding        *gardencorev1beta1.SecretBinding
		shoot                *gardencorev1beta1.Shoot
		cluster              *extensionsv1alpha1.Cluster
	)

	BeforeEach(func() {
		seedNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   gardenerutils.ComputeGardenNamespace(seed.Name),
				Labels: map[string]string{testID: testRunID},
			},
		}

		internalDomainSecret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			GenerateName: "secret-",
			Namespace:    seedNamespace.Name,
			Labels: map[string]string{
				"gardener.cloud/role": "internal-domain",
				testID:                testRunID,
			},
			Annotations: map[string]string{
				"dns.gardener.cloud/provider": "test",
				"dns.gardener.cloud/domain":   "example.com",
			},
		}}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret-" + testRunID,
				Namespace: testNamespace.Name,
				Labels:    map[string]string{testID: testRunID},
			},
		}
		secretBinding = &gardencorev1beta1.SecretBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secretbinding-" + testRunID,
				Namespace: testNamespace.Name,
				Labels:    map[string]string{testID: testRunID},
			},
			SecretRef: corev1.SecretReference{Name: secret.Name},
			Provider:  &gardencorev1beta1.SecretBindingProvider{Type: "foo"},
		}
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: testNamespace.Name,
				Labels:    map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ShootSpec{
				SecretBindingName: ptr.To(secretBinding.Name),
				CloudProfileName:  ptr.To("cloudprofile1"),
				SeedName:          ptr.To(seedName),
				Region:            "europe-central-1",
				Provider: gardencorev1beta1.Provider{
					Type: "foo-provider",
					Workers: []gardencorev1beta1.Worker{
						{
							Name:    "cpu-worker",
							Minimum: 3,
							Maximum: 3,
							Machine: gardencorev1beta1.Machine{
								Type: "large",
							},
						},
					},
				},
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.31.1",
				},
				Networking: &gardencorev1beta1.Networking{
					Type:     ptr.To("foo-networking"),
					Services: ptr.To("10.0.0.0/16"),
					Pods:     ptr.To("10.1.0.0/16"),
				},
			},
		}
		cluster = &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{testID: testRunID},
			},
			Spec: extensionsv1alpha1.ClusterSpec{
				Shoot:        runtime.RawExtension{Object: shoot},
				Seed:         runtime.RawExtension{Object: seed},
				CloudProfile: runtime.RawExtension{Object: &gardencorev1beta1.CloudProfile{}},
			},
		}
	})

	JustBeforeEach(func() {
		// Typically, GCM creates the seed-specific namespace, but it doesn't run in this test, hence we have to do it.
		By("Create seed-specific namespace")
		Expect(testClient.Create(ctx, seedNamespace)).To(Succeed())

		By("Wait until the manager cache observes the namespace")
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(seedNamespace), seedNamespace)
		}).Should(Succeed())

		By("Create InternalDomainSecret")
		Expect(testClient.Create(ctx, internalDomainSecret)).To(Succeed())

		By("Wait until the manager cache observes the internal domain secret")
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(internalDomainSecret), internalDomainSecret)
		}).Should(Succeed())

		By("Create Shoot")
		Expect(testClient.Create(ctx, shoot)).To(Succeed())
		log.Info("Created Shoot for test", "shoot", shoot.Name)

		By("Patch shoot status")
		patch := client.MergeFrom(shoot.DeepCopy())
		shoot.Status.SeedName = ptr.To(seedName)
		shoot.Status.Gardener.Version = "1.2.3"
		shoot.Status.TechnicalID = testNamespace.Name
		shoot.Status.UID = "some-uid"
		Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

		By("Ensure manager has observed status patch")
		Eventually(func(g Gomega) string {
			g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			return shoot.Status.Gardener.Version
		}).ShouldNot(BeEmpty())

		DeferCleanup(func() {
			By("Delete seed-specific namespace")
			Expect(testClient.Delete(ctx, seedNamespace)).To(Succeed())

			By("Ensure Namespace is gone")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(seedNamespace), seedNamespace)
			}).Should(BeNotFoundError())

			By("Delete Secret")
			Expect(testClient.Delete(ctx, internalDomainSecret)).To(Succeed())

			By("Ensure Secret is gone")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(internalDomainSecret), internalDomainSecret)
			}).Should(BeNotFoundError())

			By("Delete Shoot")
			Expect(testClient.Delete(ctx, shoot)).To(Succeed())

			By("Ensure Shoot is gone")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
			}).Should(BeNotFoundError())
		})
	})

	Context("when operation cannot be initialized", func() {
		Context("shoot with workers", func() {
			It("should set condition to Unknown", func() {
				By("Expect conditions to be Unknown")
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					return shoot.Status.Conditions
				}).Should(And(
					ContainCondition(OfType(gardencorev1beta1.ShootAPIServerAvailable), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason("ConditionCheckError"), WithMessageSubstrings("operation could not be initialized")),
					ContainCondition(OfType(gardencorev1beta1.ShootControlPlaneHealthy), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason("ConditionCheckError"), WithMessageSubstrings("operation could not be initialized")),
					ContainCondition(OfType(gardencorev1beta1.ShootObservabilityComponentsHealthy), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason("ConditionCheckError"), WithMessageSubstrings("operation could not be initialized")),
					ContainCondition(OfType(gardencorev1beta1.ShootEveryNodeReady), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason("ConditionCheckError"), WithMessageSubstrings("operation could not be initialized")),
					ContainCondition(OfType(gardencorev1beta1.ShootSystemComponentsHealthy), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason("ConditionCheckError"), WithMessageSubstrings("operation could not be initialized")),
				))
			})
		})

		Context("Workerless Shoot", func() {
			BeforeEach(func() {
				shoot.Spec.SecretBindingName = nil
				shoot.Spec.Networking = nil
				shoot.Spec.Provider.Workers = nil
			})

			It("should set condition to Unknown", func() {
				By("Expect conditions to be Unknown")
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					return shoot.Status.Conditions
				}).Should(And(
					ContainCondition(OfType(gardencorev1beta1.ShootAPIServerAvailable), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason("ConditionCheckError"), WithMessageSubstrings("operation could not be initialized")),
					ContainCondition(OfType(gardencorev1beta1.ShootControlPlaneHealthy), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason("ConditionCheckError"), WithMessageSubstrings("operation could not be initialized")),
					ContainCondition(OfType(gardencorev1beta1.ShootObservabilityComponentsHealthy), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason("ConditionCheckError"), WithMessageSubstrings("operation could not be initialized")),
					ContainCondition(OfType(gardencorev1beta1.ShootSystemComponentsHealthy), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason("ConditionCheckError"), WithMessageSubstrings("operation could not be initialized")),
					Not(ContainCondition(OfType(gardencorev1beta1.ShootEveryNodeReady))),
				))
			})
		})
	})

	Context("when operation can be initialized", func() {
		BeforeEach(func() {
			By("Create Secret")
			Expect(testClient.Create(ctx, secret)).To(Succeed())
			log.Info("Created Secret for test", "secret", secret.Name)

			By("Create SecretBinding")
			Expect(testClient.Create(ctx, secretBinding)).To(Succeed())
			log.Info("Created SecretBinding for test", "secretBinding", secretBinding.Name)

			DeferCleanup(func() {
				By("Delete SecretBinding")
				Expect(testClient.Delete(ctx, secretBinding)).To(Succeed())

				By("Ensure SecretBinding is gone")
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(secretBinding), secretBinding)
				}).Should(BeNotFoundError())

				By("Delete Secret")
				Expect(testClient.Delete(ctx, secret)).To(Succeed())

				By("Ensure Secret is gone")
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)
				}).Should(BeNotFoundError())
			})
		})

		// Cluster is created in JustBeforeEach because Shoot is also created in JustBeforeEach, so we need to make sure
		// that the Cluster resource contains the most recent version of the Shoot.
		JustBeforeEach(func() {
			By("Create Cluster")
			cluster.Name = shoot.Status.TechnicalID
			Expect(testClient.Create(ctx, cluster)).To(Succeed())
			log.Info("Created Cluster for test", "cluster", cluster.Name)

			DeferCleanup(func() {
				By("Delete Cluster")
				Expect(testClient.Delete(ctx, cluster)).To(Succeed())

				By("Ensure Cluster is gone")
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)
				}).Should(BeNotFoundError())
			})
		})

		Context("when all control plane deployments for the Shoot are missing", func() {
			Context("Shoot with workers", func() {
				It("should set conditions", func() {
					By("Expect conditions to be set")
					Eventually(func(g Gomega) []gardencorev1beta1.Condition {
						g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
						return shoot.Status.Conditions
					}).Should(And(
						ContainCondition(OfType(gardencorev1beta1.ShootAPIServerAvailable), WithStatus(gardencorev1beta1.ConditionProgressing), WithReason("APIServerDown")),
						ContainCondition(OfType(gardencorev1beta1.ShootControlPlaneHealthy), WithStatus(gardencorev1beta1.ConditionProgressing), WithReason("DeploymentMissing"), WithMessageSubstrings("Missing required deployments: [gardener-resource-manager kube-apiserver kube-controller-manager kube-scheduler machine-controller-manager]")),
						ContainCondition(OfType(gardencorev1beta1.ShootObservabilityComponentsHealthy), WithStatus(gardencorev1beta1.ConditionProgressing), WithReason("DeploymentMissing"), WithMessageSubstrings("Missing required deployments: [kube-state-metrics]")),
						ContainCondition(OfType(gardencorev1beta1.ShootEveryNodeReady), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason("ConditionCheckError"), WithMessageSubstrings("Shoot control plane has not been fully created yet.")),
						ContainCondition(OfType(gardencorev1beta1.ShootSystemComponentsHealthy), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason("ConditionCheckError"), WithMessageSubstrings("Shoot control plane has not been fully created yet.")),
					))
				})
			})

			Context("Workerless Shoot", func() {
				BeforeEach(func() {
					shoot.Spec.Provider.Workers = nil
					shoot.Spec.SecretBindingName = nil
					shoot.Spec.Networking = &gardencorev1beta1.Networking{
						Services: ptr.To("10.0.0.0/16"),
					}
				})

				It("should set conditions", func() {
					By("Expect conditions to be set")
					Eventually(func(g Gomega) []gardencorev1beta1.Condition {
						g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
						return shoot.Status.Conditions
					}).Should(And(
						ContainCondition(OfType(gardencorev1beta1.ShootAPIServerAvailable), WithStatus(gardencorev1beta1.ConditionProgressing), WithReason("APIServerDown")),
						ContainCondition(OfType(gardencorev1beta1.ShootControlPlaneHealthy), WithStatus(gardencorev1beta1.ConditionProgressing), WithReason("DeploymentMissing"), WithMessageSubstrings("Missing required deployments: [gardener-resource-manager kube-apiserver kube-controller-manager]")),
						ContainCondition(OfType(gardencorev1beta1.ShootObservabilityComponentsHealthy), WithStatus(gardencorev1beta1.ConditionTrue), WithReason("ObservabilityComponentsRunning"), WithMessageSubstrings("All observability components are healthy.")),
						ContainCondition(OfType(gardencorev1beta1.ShootSystemComponentsHealthy), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason("ConditionCheckError"), WithMessageSubstrings("Shoot control plane has not been fully created yet.")),
						Not(ContainCondition(OfType(gardencorev1beta1.ShootEveryNodeReady))),
					))
				})
			})
		})

		Context("when some control plane deployments for the Shoot are present", func() {
			JustBeforeEach(func() {
				createDeployment([]string{"gardener-resource-manager", "kube-controller-manager"})
			})

			Context("Shoot with workers", func() {
				It("should set conditions", func() {
					By("Expect conditions to be set")
					Eventually(func(g Gomega) []gardencorev1beta1.Condition {
						g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
						return shoot.Status.Conditions
					}).Should(And(
						ContainCondition(OfType(gardencorev1beta1.ShootAPIServerAvailable), WithStatus(gardencorev1beta1.ConditionProgressing), WithReason("APIServerDown")),
						ContainCondition(OfType(gardencorev1beta1.ShootControlPlaneHealthy), WithStatus(gardencorev1beta1.ConditionProgressing), WithReason("DeploymentMissing"), WithMessageSubstrings("Missing required deployments: [kube-apiserver kube-scheduler machine-controller-manager]")),
						ContainCondition(OfType(gardencorev1beta1.ShootObservabilityComponentsHealthy), WithStatus(gardencorev1beta1.ConditionProgressing), WithReason("DeploymentMissing"), WithMessageSubstrings("Missing required deployments: [kube-state-metrics]")),
						ContainCondition(OfType(gardencorev1beta1.ShootEveryNodeReady), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason("ConditionCheckError"), WithMessageSubstrings("Shoot control plane has not been fully created yet.")),
						ContainCondition(OfType(gardencorev1beta1.ShootSystemComponentsHealthy), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason("ConditionCheckError"), WithMessageSubstrings("Shoot control plane has not been fully created yet.")),
					))
				})
			})

			Context("Workerless Shoot", func() {
				BeforeEach(func() {
					shoot.Spec.Provider.Workers = nil
					shoot.Spec.SecretBindingName = nil
					shoot.Spec.Networking = &gardencorev1beta1.Networking{
						Services: ptr.To("10.0.0.0/16"),
					}
				})

				It("should set conditions", func() {
					By("Expect conditions to be set")
					Eventually(func(g Gomega) []gardencorev1beta1.Condition {
						g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
						return shoot.Status.Conditions
					}).Should(And(
						ContainCondition(OfType(gardencorev1beta1.ShootAPIServerAvailable), WithStatus(gardencorev1beta1.ConditionProgressing), WithReason("APIServerDown")),
						ContainCondition(OfType(gardencorev1beta1.ShootControlPlaneHealthy), WithStatus(gardencorev1beta1.ConditionProgressing), WithReason("DeploymentMissing"), WithMessageSubstrings("Missing required deployments: [kube-apiserver]")),
						ContainCondition(OfType(gardencorev1beta1.ShootObservabilityComponentsHealthy), WithStatus(gardencorev1beta1.ConditionTrue), WithReason("ObservabilityComponentsRunning"), WithMessageSubstrings("All observability components are healthy.")),
						ContainCondition(OfType(gardencorev1beta1.ShootSystemComponentsHealthy), WithStatus(gardencorev1beta1.ConditionUnknown), WithReason("ConditionCheckError"), WithMessageSubstrings("Shoot control plane has not been fully created yet.")),
						Not(ContainCondition(OfType(gardencorev1beta1.ShootEveryNodeReady))),
					))
				})
			})
		})

		Context("when all Managed Resources of spec.class=nil in shoot namespace in seed are healthy except one with spec.class!=nil", func() {
			JustBeforeEach(func() {
				// kube-apiserver deployment is required because care controller doesn't check any other
				// condition if APIServer is down.
				createDeployment([]string{"kube-apiserver"})

				managedResource1 := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-1",
						Namespace: testNamespace.Name,
						Labels: map[string]string{
							testID: testRunID,
						},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						SecretRefs: []corev1.LocalObjectReference{
							{Name: "test-1"},
						},
					},
				}
				Expect(testClient.Create(ctx, managedResource1)).To(Succeed())

				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(managedResource1), managedResource1)
				}).Should(Succeed())

				By("Patch Managed Resource to report healthiness")
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource1), managedResource1)).To(Succeed())

					patch := client.MergeFrom(managedResource1.DeepCopy())
					managedResource1.Status.ObservedGeneration = managedResource1.Generation
					managedResource1.Status.Conditions = []gardencorev1beta1.Condition{
						{
							Type:               resourcesv1alpha1.ResourcesApplied,
							Status:             gardencorev1beta1.ConditionTrue,
							LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
							LastUpdateTime:     metav1.Time{Time: fakeClock.Now()},
						},
						{
							Type:               resourcesv1alpha1.ResourcesHealthy,
							Status:             gardencorev1beta1.ConditionTrue,
							LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
							LastUpdateTime:     metav1.Time{Time: fakeClock.Now()},
						},
						{
							Type:               resourcesv1alpha1.ResourcesProgressing,
							Status:             gardencorev1beta1.ConditionFalse,
							LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
							LastUpdateTime:     metav1.Time{Time: fakeClock.Now()},
						},
					}
					g.Expect(testClient.Status().Patch(ctx, managedResource1, patch)).To(Succeed())
				}).Should(Succeed())

				managedResource2 := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-2",
						Namespace: testNamespace.Name,
						Labels: map[string]string{
							testID: testRunID,
						},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						Class: ptr.To("test"),
						SecretRefs: []corev1.LocalObjectReference{
							{Name: "test-2"},
						},
					},
				}
				Expect(testClient.Create(ctx, managedResource2)).To(Succeed())

				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(managedResource2), managedResource2)
				}).Should(Succeed())

				DeferCleanup(func() {
					By("Delete Managed Resource")
					Expect(testClient.Delete(ctx, managedResource1)).To(Succeed())
					Expect(testClient.Delete(ctx, managedResource2)).To(Succeed())

					By("Ensure Managed Resource is gone")
					Eventually(func() error {
						return mgrClient.Get(ctx, client.ObjectKeyFromObject(managedResource1), managedResource1)
					}).Should(BeNotFoundError())

					Eventually(func() error {
						return mgrClient.Get(ctx, client.ObjectKeyFromObject(managedResource2), managedResource2)
					}).Should(BeNotFoundError())
				})
			})

			Context("Shoot with workers", func() {
				It("SystemComponentsHealthy condition should not fail because all relevant Managed Resources are healthy", func() {
					By("Expect conditions to be set")
					Eventually(func(g Gomega) []gardencorev1beta1.Condition {
						g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
						return shoot.Status.Conditions
					}).Should(And(
						// here SystemComponentsHealthy condition is not healthy because for SystemComponentsHealthy to be healthy a tunnel connection is required
						// which can't be faked, if it would have been failing because of MangedResource is not healthy then the reason will not be `NoTunnelDeployed`.
						ContainCondition(OfType(gardencorev1beta1.ShootSystemComponentsHealthy), WithStatus(gardencorev1beta1.ConditionProgressing), WithReason("NoTunnelDeployed"), WithMessageSubstrings("no tunnels are currently deployed to perform health-check on")),
					))
				})
			})

			Context("Workerless Shoot", func() {
				BeforeEach(func() {
					shoot.Spec.Provider.Workers = nil
					shoot.Spec.SecretBindingName = nil
					shoot.Spec.Networking = &gardencorev1beta1.Networking{
						Services: ptr.To("10.0.0.0/16"),
					}
				})

				It("SystemComponentsHealthy condition should not fail because all relevant Managed Resources are healthy", func() {
					By("Expect conditions to be set")
					Eventually(func(g Gomega) []gardencorev1beta1.Condition {
						g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
						return shoot.Status.Conditions
					}).Should(And(
						ContainCondition(OfType(gardencorev1beta1.ShootSystemComponentsHealthy), WithStatus(gardencorev1beta1.ConditionTrue), WithReason("SystemComponentsRunning"), WithMessageSubstrings("All system components are healthy.")),
					))
				})
			})
		})

		Context("when Managed Resources of spec.class=nil in shoot namespace in seed are not healthy", func() {
			JustBeforeEach(func() {
				// kube-apiserver deployment is required because care controller doesn't check any other
				// condition if APIServer is down.
				createDeployment([]string{"kube-apiserver"})

				managedResource1 := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-1",
						Namespace: testNamespace.Name,
						Labels: map[string]string{
							testID: testRunID,
						},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						SecretRefs: []corev1.LocalObjectReference{
							{Name: "test-1"},
						},
					},
				}
				Expect(testClient.Create(ctx, managedResource1)).To(Succeed())

				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(managedResource1), managedResource1)
				}).Should(Succeed())

				By("Patch Managed Resource status")
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource1), managedResource1)).To(Succeed())

					patch := client.MergeFrom(managedResource1.DeepCopy())
					managedResource1.Status.ObservedGeneration = managedResource1.Generation
					managedResource1.Status.Conditions = []gardencorev1beta1.Condition{
						{
							Type:               resourcesv1alpha1.ResourcesApplied,
							Status:             gardencorev1beta1.ConditionFalse,
							LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
							LastUpdateTime:     metav1.Time{Time: fakeClock.Now()},
							Reason:             "ApplyFailed",
							Message:            "Resources failed to get applied",
						},
						{
							Type:               resourcesv1alpha1.ResourcesHealthy,
							Status:             gardencorev1beta1.ConditionTrue,
							LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
							LastUpdateTime:     metav1.Time{Time: fakeClock.Now()},
						},
						{
							Type:               resourcesv1alpha1.ResourcesProgressing,
							Status:             gardencorev1beta1.ConditionFalse,
							LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
							LastUpdateTime:     metav1.Time{Time: fakeClock.Now()},
						},
					}
					g.Expect(testClient.Status().Patch(ctx, managedResource1, patch)).To(Succeed())
				}).Should(Succeed())

				DeferCleanup(func() {
					By("Delete Managed Resource")
					Expect(testClient.Delete(ctx, managedResource1)).To(Succeed())

					By("Ensure Managed Resource is gone")
					Eventually(func() error {
						return mgrClient.Get(ctx, client.ObjectKeyFromObject(managedResource1), managedResource1)
					}).Should(BeNotFoundError())
				})
			})

			Context("Shoot with workers", func() {
				It("SystemComponentsHealthy condition should fail because of ManagedResource is not healthy", func() {
					By("Expect conditions to be set")
					Eventually(func(g Gomega) []gardencorev1beta1.Condition {
						g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
						return shoot.Status.Conditions
					}).Should(And(
						ContainCondition(OfType(gardencorev1beta1.ShootSystemComponentsHealthy), WithStatus(gardencorev1beta1.ConditionProgressing), WithReason("ApplyFailed"), WithMessageSubstrings("Resources failed to get applied")),
					))
				})
			})

			Context("Workerless Shoot", func() {
				BeforeEach(func() {
					shoot.Spec.Provider.Workers = nil
					shoot.Spec.SecretBindingName = nil
					shoot.Spec.Networking = &gardencorev1beta1.Networking{
						Services: ptr.To("10.0.0.0/16"),
					}
				})

				It("SystemComponentsHealthy condition should fail because of ManagedResource is not healthy", func() {
					By("Expect conditions to be set")
					Eventually(func(g Gomega) []gardencorev1beta1.Condition {
						g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
						return shoot.Status.Conditions
					}).Should(And(
						ContainCondition(OfType(gardencorev1beta1.ShootSystemComponentsHealthy), WithStatus(gardencorev1beta1.ConditionProgressing), WithReason("ApplyFailed"), WithMessageSubstrings("Resources failed to get applied")),
					))
				})
			})
		})
	})
})

func createDeployment(names []string) {
	for _, name := range names {
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testNamespace.Name,
				Labels: map[string]string{
					testID:                      testRunID,
					v1beta1constants.GardenRole: getRole(name),
				},
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
				Replicas: ptr.To[int32](1),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "foo-container",
							Image: "foo",
						}},
					},
				},
			},
		}

		By("Create Deployment " + name)
		ExpectWithOffset(1, testClient.Create(ctx, deployment)).To(Succeed(), "for deployment "+name)
		log.Info("Created Deployment for test", "deployment", client.ObjectKeyFromObject(deployment))

		By("Ensure manager has observed deployment " + name)
		EventuallyWithOffset(1, func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)
		}).Should(Succeed())

		DeferCleanup(func() {
			By("Delete Deployment " + name)
			ExpectWithOffset(1, testClient.Delete(ctx, deployment)).To(Succeed(), "for deployment "+name)

			By("Ensure Deployment " + name + " is gone")
			EventuallyWithOffset(1, func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)
			}).Should(BeNotFoundError(), "for deployment "+name)

			By("Ensure manager has observed deployment deletion " + name)
			EventuallyWithOffset(1, func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)
			}).Should(BeNotFoundError())
		})
	}
}

func getRole(name string) string {
	switch name {
	case "gardener-resource-manager", "kube-controller-manager", "kube-scheduler":
		return v1beta1constants.GardenRoleControlPlane
	case "plutono":
		return v1beta1constants.GardenRoleMonitoring
	}
	return ""
}
