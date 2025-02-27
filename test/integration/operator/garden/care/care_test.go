// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	druidv1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Garden Care controller tests", func() {
	var (
		requiredControlPlaneDeployments = []string{
			"virtual-garden-" + v1beta1constants.DeploymentNameGardenerResourceManager,
			"virtual-garden-" + v1beta1constants.DeploymentNameKubeAPIServer,
			"virtual-garden-" + v1beta1constants.DeploymentNameKubeControllerManager,
		}

		requiredControlPlaneETCDs = []string{
			"virtual-garden-" + v1beta1constants.ETCDMain,
			"virtual-garden-" + v1beta1constants.ETCDEvents,
		}

		garden *operatorv1alpha1.Garden
	)

	BeforeEach(func() {
		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name:   gardenName,
				Labels: map[string]string{testID: testRunID},
			},
			Spec: operatorv1alpha1.GardenSpec{
				RuntimeCluster: operatorv1alpha1.RuntimeCluster{
					Networking: operatorv1alpha1.RuntimeNetworking{
						Pods:     []string{"10.1.0.0/16"},
						Services: []string{"10.2.0.0/16"},
					},
					Ingress: operatorv1alpha1.Ingress{
						Domains: []operatorv1alpha1.DNSDomain{{Name: "ingress.runtime-garden.local.gardener.cloud"}},
						Controller: gardencorev1beta1.IngressController{
							Kind: "nginx",
						},
					},
					Provider: operatorv1alpha1.Provider{
						Zones: []string{"a", "b", "c"},
					},
					Settings: &operatorv1alpha1.Settings{
						VerticalPodAutoscaler: &operatorv1alpha1.SettingVerticalPodAutoscaler{
							Enabled: ptr.To(true),
						},
					},
				},
				VirtualCluster: operatorv1alpha1.VirtualCluster{
					DNS: operatorv1alpha1.DNS{
						Domains: []operatorv1alpha1.DNSDomain{{Name: "virtual-garden.local.gardener.cloud"}},
					},
					Gardener: operatorv1alpha1.Gardener{
						ClusterIdentity: "test",
					},
					Kubernetes: operatorv1alpha1.Kubernetes{
						Version: "1.31.1",
					},
					Maintenance: operatorv1alpha1.Maintenance{
						TimeWindow: gardencorev1beta1.MaintenanceTimeWindow{
							Begin: "220000+0100",
							End:   "230000+0100",
						},
					},
					Networking: operatorv1alpha1.Networking{
						Services: []string{"100.64.0.0/13"},
					},
				},
			},
		}

		By("Create Garden")
		Expect(testClient.Create(ctx, garden)).To(Succeed())
		log.Info("Created Garden for test", "garden", garden.Name)

		DeferCleanup(func() {
			By("Delete Garden")
			Expect(testClient.Delete(ctx, garden)).To(Succeed())

			By("Ensure Garden is gone")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)
			}).Should(BeNotFoundError())
		})
	})

	Context("when ManagedResources for Runtime Cluster exist", func() {
		managedResourceName := "foo"

		BeforeEach(func() {
			By("Create ManagedResource for runtime cluster")
			managedResource := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceName,
					Namespace: getManagedResourceNamespace(managedResourceName, testNamespace.Name),
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class:      ptr.To("seed"),
					SecretRefs: []corev1.LocalObjectReference{{Name: "foo-secret"}},
				},
			}
			Expect(testClient.Create(ctx, managedResource)).To(Succeed())
			log.Info("Created ManagedResource for test", "managedResource", client.ObjectKeyFromObject(managedResource))

			DeferCleanup(func() {
				By("Delete ManagedResource for runtime cluster")
				Expect(testClient.Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceName, Namespace: getManagedResourceNamespace(managedResourceName, testNamespace.Name)}})).To(Succeed())
			})
		})

		It("should set condition to False because all ManagedResource statuses are outdated", func() {
			By("Expect RuntimeComponentsHealthy condition to be False")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
				return garden.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.RuntimeComponentsHealthy),
				WithStatus(gardencorev1beta1.ConditionFalse),
				WithReason("OutdatedStatus"),
				WithMessageSubstrings("observed generation of managed resource"),
			))
		})

		It("should set condition to True because all ManagedResource statuses are healthy", func() {
			updateManagedResourceStatusToHealthy(managedResourceName)

			By("Expect RuntimeComponentsHealthy condition to be True")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
				return garden.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.RuntimeComponentsHealthy),
				WithStatus(gardencorev1beta1.ConditionTrue),
				WithReason("RuntimeComponentsRunning"),
				WithMessageSubstrings("All runtime components are healthy."),
			))
		})
	})

	Context("when ManagedResources for Virtual Cluster exist", func() {
		managedResourceName := "bar"

		BeforeEach(func() {
			By("Create deployments")
			createDeployments(requiredControlPlaneDeployments, v1beta1constants.GardenRole, v1beta1constants.GardenRoleControlPlane)
			By("Update deployment status to healthy")
			for _, name := range requiredControlPlaneDeployments {
				updateDeploymentStatusToHealthy(name)
			}

			By("Create ETCDs")
			createETCDs(requiredControlPlaneETCDs)
			By("Update ETCD status to healthy")
			for _, name := range requiredControlPlaneETCDs {
				updateETCDStatusToHealthy(name)
			}

			By("Create ManagedResource for virtual Cluster")
			managedResource := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceName,
					Namespace: getManagedResourceNamespace(managedResourceName, testNamespace.Name),
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					SecretRefs: []corev1.LocalObjectReference{{Name: "foo-secret"}},
				},
			}
			Expect(testClient.Create(ctx, managedResource)).To(Succeed())
			log.Info("Created ManagedResource for test", "managedResource", client.ObjectKeyFromObject(managedResource))

			DeferCleanup(func() {
				By("Delete ManagedResource for virtual cluster")
				Expect(testClient.Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceName, Namespace: getManagedResourceNamespace(managedResourceName, testNamespace.Name)}})).To(Succeed())
			})
		})

		It("should set condition to False because all ManagedResource statuses are outdated", func() {
			By("Expect RuntimeComponentsHealthy condition to be False")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
				return garden.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.VirtualComponentsHealthy),
				WithStatus(gardencorev1beta1.ConditionFalse),
				WithReason("OutdatedStatus"),
				WithMessageSubstrings("observed generation of managed resource"),
			))
		})

		It("should set condition to True because all ManagedResource statuses are healthy", func() {
			updateManagedResourceStatusToHealthy(managedResourceName)

			By("Expect VirtualGardenComponentsHealthy condition to be True")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
				return garden.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.VirtualComponentsHealthy),
				WithStatus(gardencorev1beta1.ConditionTrue),
				WithReason("VirtualComponentsRunning"),
				WithMessageSubstrings("All virtual garden components are healthy."),
			))
		})
	})

	Context("when all control-plane components of the Garden are missing", func() {
		It("should set condition to False", func() {
			By("Expect VirtualComponentsHealthy condition to be False")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
				return garden.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.VirtualComponentsHealthy),
				WithStatus(gardencorev1beta1.ConditionFalse),
				WithReason("DeploymentMissing"),
				WithMessageSubstrings("Missing required deployments"),
			))
		})
	})

	Context("when control-plane components of the Garden exist", func() {
		BeforeEach(func() {
			By("Create deployments")
			createDeployments(requiredControlPlaneDeployments, v1beta1constants.GardenRole, v1beta1constants.GardenRoleControlPlane)
		})

		It("should set condition to False because status of all deployments are outdated", func() {
			By("Expect VirtualComponentsHealthy condition to be False")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
				return garden.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.VirtualComponentsHealthy),
				WithStatus(gardencorev1beta1.ConditionFalse),
				WithReason("DeploymentUnhealthy"),
				WithMessageSubstrings("observed generation outdated"),
			))
		})

		It("should set condition to False because status of some deployments are outdated", func() {
			for _, name := range requiredControlPlaneDeployments[1:] {
				updateDeploymentStatusToHealthy(name)
			}

			By("Expect VirtualComponentsHealthy condition to be False")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
				return garden.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.VirtualComponentsHealthy),
				WithStatus(gardencorev1beta1.ConditionFalse),
				WithReason("DeploymentUnhealthy"),
				WithMessageSubstrings("observed generation outdated"),
			))
		})

		It("should set condition to False because required ETCDs are missing", func() {
			for _, name := range requiredControlPlaneDeployments {
				updateDeploymentStatusToHealthy(name)
			}

			By("Expect VirtualComponentsHealthy condition to be False")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
				return garden.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.VirtualComponentsHealthy),
				WithStatus(gardencorev1beta1.ConditionFalse),
				WithReason("EtcdMissing"),
				WithMessageSubstrings("Missing required etcds"),
			))
		})

		It("should set condition to False because status of all ETCDs is outdated ", func() {
			for _, name := range requiredControlPlaneDeployments {
				updateDeploymentStatusToHealthy(name)
			}
			createETCDs(requiredControlPlaneETCDs)

			By("Expect VirtualComponentsHealthy condition to be False")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
				return garden.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.VirtualComponentsHealthy),
				WithStatus(gardencorev1beta1.ConditionFalse),
				WithReason("EtcdUnhealthy"),
				WithMessageSubstrings("is unhealthy"),
			))
		})

		It("should set condition to False because status of some ETCDs is outdated ", func() {
			for _, name := range requiredControlPlaneDeployments {
				updateDeploymentStatusToHealthy(name)
			}
			createETCDs(requiredControlPlaneETCDs)
			for _, name := range requiredControlPlaneETCDs[1:] {
				updateETCDStatusToHealthy(name)
			}

			By("Expect VirtualComponentsHealthy condition to be False")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
				return garden.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.VirtualComponentsHealthy),
				WithStatus(gardencorev1beta1.ConditionFalse),
				WithReason("EtcdUnhealthy"),
				WithMessageSubstrings("is unhealthy"),
			))
		})

		It("should set condition to True because all deployments and all ETCDs are healthy ", func() {
			for _, name := range requiredControlPlaneDeployments {
				updateDeploymentStatusToHealthy(name)
			}
			createETCDs(requiredControlPlaneETCDs)
			for _, name := range requiredControlPlaneETCDs {
				updateETCDStatusToHealthy(name)
			}

			By("Expect VirtualComponentsHealthy condition to be True")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
				return garden.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.VirtualComponentsHealthy),
				WithStatus(gardencorev1beta1.ConditionTrue),
				WithReason("VirtualComponentsRunning"),
				WithMessageSubstrings("All virtual garden components are healthy."),
			))
		})
	})

	Context("virtual garden kube-apiserver is always healthy because it checks the envtest kube-apiserver", func() {
		It("should set condition to True", func() {
			By("Expect VirtualGardenAPIServerAvailable condition to be True")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
				return garden.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.VirtualGardenAPIServerAvailable),
				WithStatus(gardencorev1beta1.ConditionTrue),
				WithReason("HealthzRequestSucceeded"),
				WithMessageSubstrings("API server /healthz endpoint responded with success status code."),
			))
		})
	})

	Context("when observability-related ManagedResources exist", func() {
		managedResourceName := "baz"

		BeforeEach(func() {
			By("Create ManagedResource for observability components")
			managedResource := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceName,
					Namespace: getManagedResourceNamespace(managedResourceName, testNamespace.Name),
					Labels:    map[string]string{"care.gardener.cloud/condition-type": "ObservabilityComponentsHealthy"},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					SecretRefs: []corev1.LocalObjectReference{{Name: "foo-secret"}},
				},
			}
			Expect(testClient.Create(ctx, managedResource)).To(Succeed())
			log.Info("Created ManagedResource for test", "managedResource", client.ObjectKeyFromObject(managedResource))

			DeferCleanup(func() {
				By("Delete ManagedResource for observability components")
				Expect(testClient.Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceName, Namespace: getManagedResourceNamespace(managedResourceName, testNamespace.Name)}})).To(Succeed())
			})
		})

		It("should set condition to False because all ManagedResource statuses are outdated", func() {
			By("Expect ObservabilityComponentsHealthy condition to be False")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
				return garden.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.ObservabilityComponentsHealthy),
				WithStatus(gardencorev1beta1.ConditionFalse),
				WithReason("OutdatedStatus"),
				WithMessageSubstrings("observed generation of managed resource"),
			))
		})

		It("should set condition to True because all ManagedResource statuses are healthy", func() {
			updateManagedResourceStatusToHealthy(managedResourceName)

			By("Expect ObservabilityComponentsHealthy condition to be True")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
				return garden.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.ObservabilityComponentsHealthy),
				WithStatus(gardencorev1beta1.ConditionTrue),
				WithReason("ObservabilityComponentsRunning"),
				WithMessageSubstrings("All observability components are healthy."),
			))
		})
	})
})

func createDeployments(names []string, roleLabel, role string) {
	for _, name := range names {
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testNamespace.Name,
				Labels: map[string]string{
					testID:    testRunID,
					roleLabel: role,
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

func createETCDs(names []string) {
	for _, name := range names {
		etcd := &druidv1alpha1.Etcd{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testNamespace.Name,
				Labels: map[string]string{
					testID:                      testRunID,
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
				},
			},
			Spec: druidv1alpha1.EtcdSpec{
				Labels:   map[string]string{"foo": "bar"},
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
			},
		}

		By("Create ETCD " + name)
		ExpectWithOffset(1, testClient.Create(ctx, etcd)).To(Succeed(), "for etcd "+name)
		log.Info("Created ETCD for test", "etcd", client.ObjectKeyFromObject(etcd))

		By("Ensure manager has observed etcd " + name)
		EventuallyWithOffset(1, func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(etcd), etcd)
		}).Should(Succeed())

		DeferCleanup(func() {
			By("Delete ETCD " + name)
			ExpectWithOffset(1, testClient.Delete(ctx, etcd)).To(Succeed(), "for etcd "+name)

			By("Ensure ETCD " + name + " is gone")
			EventuallyWithOffset(1, func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(etcd), etcd)
			}).Should(BeNotFoundError(), "for etcd "+name)

			By("Ensure manager has observed etcd deletion " + name)
			EventuallyWithOffset(1, func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(etcd), etcd)
			}).Should(BeNotFoundError())
		})
	}
}

func updateDeploymentStatusToHealthy(name string) {
	By("Update status to healthy for Deployment " + name)
	deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace.Name}}
	ExpectWithOffset(1, testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())

	deployment.Status.ObservedGeneration = deployment.Generation
	deployment.Status.Conditions = []appsv1.DeploymentCondition{
		{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
	}
	ExpectWithOffset(1, testClient.Status().Update(ctx, deployment)).To(Succeed())
}

func updateETCDStatusToHealthy(name string) {
	By("Update status to healthy for ETCD " + name)
	etcd := &druidv1alpha1.Etcd{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace.Name}}
	ExpectWithOffset(1, testClient.Get(ctx, client.ObjectKeyFromObject(etcd), etcd)).To(Succeed())

	etcd.Status.ObservedGeneration = &etcd.Generation
	etcd.Status.Conditions = []druidv1alpha1.Condition{
		{Type: druidv1alpha1.ConditionTypeBackupReady, Status: druidv1alpha1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
		{Type: druidv1alpha1.ConditionTypeAllMembersUpdated, Status: druidv1alpha1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
	}
	etcd.Status.Ready = ptr.To(true)
	ExpectWithOffset(1, testClient.Status().Update(ctx, etcd)).To(Succeed())
}

func updateManagedResourceStatusToHealthy(name string) {
	By("Update status to healthy for ManagedResource " + name)
	managedResource := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: getManagedResourceNamespace(name, testNamespace.Name)}}
	ExpectWithOffset(1, testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())

	managedResource.Status.ObservedGeneration = managedResource.Generation
	managedResource.Status.Conditions = []gardencorev1beta1.Condition{
		{Type: resourcesv1alpha1.ResourcesApplied, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
		{Type: resourcesv1alpha1.ResourcesHealthy, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
		{Type: resourcesv1alpha1.ResourcesProgressing, Status: gardencorev1beta1.ConditionFalse, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
	}
	ExpectWithOffset(1, testClient.Status().Update(ctx, managedResource)).To(Succeed())
}

func getManagedResourceNamespace(managedResourceName, gardenNamespace string) string {
	if sets.New("istio-system", "virtual-garden-istio").Has(managedResourceName) {
		return "istio-system"
	}
	return gardenNamespace
}
