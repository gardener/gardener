// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operator_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	authenticationv1alpha1 "github.com/gardener/gardener/pkg/apis/authentication/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	settingsv1alpha1 "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/gardener/operator"
)

var _ = Describe("Garden", func() {
	DescribeTable("#IsServedByGardenerAPIServer",
		func(resource string, expected bool) {
			Expect(IsServedByGardenerAPIServer(resource)).To(Equal(expected))
		},
		Entry("gardener core resource", gardencorev1beta1.Resource("shoots").String(), true),
		Entry("operations resource", operationsv1alpha1.Resource("bastions").String(), true),
		Entry("settings resource", settingsv1alpha1.Resource("openidconnectpresets").String(), true),
		Entry("seedmanagement resource", seedmanagementv1alpha1.Resource("managedseeds").String(), true),
		Entry("authentication resource", authenticationv1alpha1.Resource("adminkubeconfigrequests").String(), true),
		Entry("security resource", securityv1alpha1.Resource("workloadidentities").String(), true),
		Entry("any other resource", "foo", false),
	)

	DescribeTable("#IsServedByKubeAPIServer",
		func(resource string, expected bool) {
			Expect(IsServedByKubeAPIServer(resource)).To(Equal(expected))
		},
		Entry("kubernetes core resource", corev1.Resource("secrets").String(), true),
		Entry("gardener core resource", gardencorev1beta1.Resource("shoots").String(), false),
		Entry("operations resource", operationsv1alpha1.Resource("bastions").String(), false),
		Entry("settings resource", settingsv1alpha1.Resource("openidconnectpresets").String(), false),
		Entry("seedmanagement resource", seedmanagementv1alpha1.Resource("managedseeds").String(), false),
		Entry("authentication resource", authenticationv1alpha1.Resource("adminkubeconfigrequests").String(), false),
		Entry("security resource", securityv1alpha1.Resource("workloadidentities").String(), false),
		Entry("any other resource", "foo", true),
	)

	Describe("#ComputeRequiredExtensionsForGarden", func() {
		var garden *operatorv1alpha1.Garden

		BeforeEach(func() {
			garden = &operatorv1alpha1.Garden{}
		})

		It("should return no extension types", func() {
			Expect(ComputeRequiredExtensionsForGarden(garden).UnsortedList()).To(BeEmpty())
		})

		It("should return required BackupBucket extension type", func() {
			garden.Spec.VirtualCluster.ETCD = &operatorv1alpha1.ETCD{
				Main: &operatorv1alpha1.ETCDMain{
					Backup: &operatorv1alpha1.Backup{
						Provider: "local-infrastructure",
					},
				},
			}

			Expect(ComputeRequiredExtensionsForGarden(garden).UnsortedList()).To(ConsistOf(
				"BackupBucket/local-infrastructure",
			))
		})

		It("should return required DNSRecord extension types", func() {
			garden.Spec.DNS = &operatorv1alpha1.DNSManagement{
				Providers: []operatorv1alpha1.DNSProvider{
					{Type: "local-dns-1"},
					{Type: "local-dns-2"},
				},
			}

			Expect(ComputeRequiredExtensionsForGarden(garden).UnsortedList()).To(ConsistOf(
				"DNSRecord/local-dns-1",
				"DNSRecord/local-dns-2",
			))
		})

		It("should return required Extension extension types", func() {
			garden.Spec.Extensions = []operatorv1alpha1.GardenExtension{
				{Type: "local-extension-1"},
				{Type: "local-extension-2"},
			}

			Expect(ComputeRequiredExtensionsForGarden(garden).UnsortedList()).To(ConsistOf(
				"Extension/local-extension-1",
				"Extension/local-extension-2",
			))
		})

		It("should return all required extensions", func() {
			garden.Spec.DNS = &operatorv1alpha1.DNSManagement{
				Providers: []operatorv1alpha1.DNSProvider{
					{Type: "local-dns"},
				},
			}
			garden.Spec.VirtualCluster.ETCD = &operatorv1alpha1.ETCD{
				Main: &operatorv1alpha1.ETCDMain{
					Backup: &operatorv1alpha1.Backup{
						Provider: "local-infrastructure",
					},
				},
			}
			garden.Spec.Extensions = []operatorv1alpha1.GardenExtension{
				{Type: "local-extension-1"},
				{Type: "local-extension-2"},
			}

			Expect(ComputeRequiredExtensionsForGarden(garden).UnsortedList()).To(ConsistOf(
				"BackupBucket/local-infrastructure",
				"DNSRecord/local-dns",
				"Extension/local-extension-1",
				"Extension/local-extension-2",
			))
		})
	})

	Describe("#IsRuntimeExtensionInstallationSuccessful", func() {
		var (
			ctx        context.Context
			fakeClient client.Client

			extensionName   string
			gardenNamespace string
			managedResource *resourcesv1alpha1.ManagedResource
		)

		BeforeEach(func() {
			ctx = context.Background()
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

			extensionName = "test"
			gardenNamespace = "test-namespace"
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "extension-test-garden",
					Namespace: gardenNamespace,
				},
			}

			Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())
		})

		It("should return an error if no managed resource status is available", func() {
			Expect(IsRuntimeExtensionInstallationSuccessful(ctx, fakeClient, gardenNamespace, extensionName)).To(MatchError("condition ResourcesApplied for managed resource test-namespace/extension-test-garden has not been reported yet"))
		})

		It("should return an error if managed resource applied condition is false", func() {
			managedResource.Status.Conditions = []gardencorev1beta1.Condition{
				{Type: resourcesv1alpha1.ResourcesApplied, Status: gardencorev1beta1.ConditionFalse, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
			}
			Expect(fakeClient.Update(ctx, managedResource)).To(Succeed())

			Expect(IsRuntimeExtensionInstallationSuccessful(ctx, fakeClient, gardenNamespace, extensionName)).To(MatchError("condition ResourcesApplied of managed resource test-namespace/extension-test-garden is False: "))
		})

		It("should return an error if managed resource healthy condition is false", func() {
			managedResource.Status.Conditions = []gardencorev1beta1.Condition{
				{Type: resourcesv1alpha1.ResourcesApplied, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
				{Type: resourcesv1alpha1.ResourcesHealthy, Status: gardencorev1beta1.ConditionFalse, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
			}
			Expect(fakeClient.Update(ctx, managedResource)).To(Succeed())

			Expect(IsRuntimeExtensionInstallationSuccessful(ctx, fakeClient, gardenNamespace, extensionName)).To(MatchError("condition ResourcesHealthy of managed resource test-namespace/extension-test-garden is False: "))
		})

		It("should return an error if managed resource is progressing", func() {
			managedResource.Status.Conditions = []gardencorev1beta1.Condition{
				{Type: resourcesv1alpha1.ResourcesApplied, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
				{Type: resourcesv1alpha1.ResourcesHealthy, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
				{Type: resourcesv1alpha1.ResourcesProgressing, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
			}
			Expect(fakeClient.Update(ctx, managedResource)).To(Succeed())

			Expect(IsRuntimeExtensionInstallationSuccessful(ctx, fakeClient, gardenNamespace, extensionName)).To(MatchError("condition ResourcesProgressing of managed resource test-namespace/extension-test-garden is True: "))
		})

		It("should succeed if managed resource is healthy", func() {
			managedResource.Status.Conditions = []gardencorev1beta1.Condition{
				{Type: resourcesv1alpha1.ResourcesApplied, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
				{Type: resourcesv1alpha1.ResourcesHealthy, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
				{Type: resourcesv1alpha1.ResourcesProgressing, Status: gardencorev1beta1.ConditionFalse, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
			}
			Expect(fakeClient.Update(ctx, managedResource)).To(Succeed())

			Expect(IsRuntimeExtensionInstallationSuccessful(ctx, fakeClient, gardenNamespace, extensionName)).To(Succeed())
		})
	})

	Describe("#RequiredGardenExtensionsReady", func() {
		var (
			ctx        context.Context
			log        logr.Logger
			fakeClient client.Client

			extensionName   string
			gardenNamespace string

			extension       *operatorv1alpha1.Extension
			managedResource *resourcesv1alpha1.ManagedResource
		)

		BeforeEach(func() {
			ctx = context.Background()
			log = logr.Discard()
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

			extensionName = "test"
			gardenNamespace = "test-namespace"

			extension = &operatorv1alpha1.Extension{
				ObjectMeta: metav1.ObjectMeta{
					Name: extensionName,
				},
				Spec: operatorv1alpha1.ExtensionSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: "BackupBucket", Type: "local-infrastructure"},
						{Kind: "DNSRecord", Type: "local-dns"},
						{Kind: "Extension", Type: "local-ext"},
					},
				},
			}
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "extension-test-garden",
					Namespace: gardenNamespace,
				},
			}

			Expect(fakeClient.Create(ctx, extension)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())
		})

		It("should return an error if required extension does not exist", func() {
			Expect(RequiredGardenExtensionsReady(ctx, log, fakeClient, gardenNamespace, sets.New("BackupBucket/foo"))).To(MatchError("extension controllers missing or unready: map[BackupBucket/foo:{}]"))
		})

		It("should return an error if required extension is not ready", func() {
			Expect(RequiredGardenExtensionsReady(ctx, log, fakeClient, gardenNamespace, sets.New("BackupBucket/local-infrastructure"))).To(MatchError("extension controllers missing or unready: map[BackupBucket/local-infrastructure:{}]"))
		})

		It("should succeed if required extension is ready", func() {
			managedResource.Status.Conditions = []gardencorev1beta1.Condition{
				{Type: resourcesv1alpha1.ResourcesApplied, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
				{Type: resourcesv1alpha1.ResourcesHealthy, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
				{Type: resourcesv1alpha1.ResourcesProgressing, Status: gardencorev1beta1.ConditionFalse, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
			}
			Expect(fakeClient.Update(ctx, managedResource)).To(Succeed())

			Expect(RequiredGardenExtensionsReady(ctx, log, fakeClient, gardenNamespace, sets.New("BackupBucket/local-infrastructure", "DNSRecord/local-dns"))).To(Succeed())
		})
	})
})
