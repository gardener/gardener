// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package runtime_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Extension Required Runtime controller tests", Ordered, func() {
	var (
		providerExtension, dnsExtension, extension *operatorv1alpha1.Extension

		backupBucketProvider string
		dnsProvider          string
		extensionProvider    string

		garden *operatorv1alpha1.Garden
	)

	BeforeAll(func() {
		backupBucketProvider = "local"
		dnsProvider = "dns-local"
		extensionProvider = "ext-local"

		providerExtension = &operatorv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name: extensionPrefix + "-provider",
				Labels: map[string]string{
					testID: testRunID,
				},
			},
			Spec: operatorv1alpha1.ExtensionSpec{
				Resources: []gardencorev1beta1.ControllerResource{
					{
						Kind: "BackupBucket",
						Type: backupBucketProvider,
					},
				},
			},
		}

		dnsExtension = &operatorv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name: extensionPrefix + "-dns",
				Labels: map[string]string{
					testID: testRunID,
				},
			},
			Spec: operatorv1alpha1.ExtensionSpec{
				Resources: []gardencorev1beta1.ControllerResource{
					{
						Kind: "DNSRecord",
						Type: dnsProvider,
					},
				},
			},
		}

		extension = &operatorv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name: extensionPrefix + "-generic",
				Labels: map[string]string{
					testID: testRunID,
				},
			},
			Spec: operatorv1alpha1.ExtensionSpec{
				Resources: []gardencorev1beta1.ControllerResource{
					{
						Kind: "Extension",
						Type: extensionProvider,
					},
				},
			},
		}

		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace.Name,
				Labels: map[string]string{
					testID: testRunID,
				},
				Finalizers: []string{"gardener"},
			},
			Spec: operatorv1alpha1.GardenSpec{
				DNS: &operatorv1alpha1.DNSManagement{
					Providers: []operatorv1alpha1.DNSProvider{
						{Type: dnsProvider, Name: dnsProvider},
					},
				},
				VirtualCluster: operatorv1alpha1.VirtualCluster{
					Gardener: operatorv1alpha1.Gardener{
						ClusterIdentity: testRunID,
					},
					Kubernetes: operatorv1alpha1.Kubernetes{
						Version: "1.32.0",
					},
					Networking: operatorv1alpha1.Networking{
						Services: []string{"172.0.0.0/16"},
					},
					Maintenance: operatorv1alpha1.Maintenance{
						TimeWindow: gardencorev1beta1.MaintenanceTimeWindow{
							Begin: "220000+0100",
							End:   "230000+0100",
						},
					},
					ETCD: &operatorv1alpha1.ETCD{
						Main: &operatorv1alpha1.ETCDMain{
							Backup: &operatorv1alpha1.Backup{
								Provider: backupBucketProvider,
							},
						},
					},
				},
				RuntimeCluster: operatorv1alpha1.RuntimeCluster{
					Networking: operatorv1alpha1.RuntimeNetworking{
						Pods:     []string{"10.0.0.0/16"},
						Nodes:    []string{"10.1.0.0/16"},
						Services: []string{"10.2.0.0/16"},
					},
					Ingress: operatorv1alpha1.Ingress{
						Controller: gardencorev1beta1.IngressController{
							Kind: "nginx",
						},
					},
				},
			},
		}

		DeferCleanup(func() {
			for _, ext := range []client.Object{providerExtension, dnsExtension, extension} {
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, ext))).To(Succeed(), fmt.Sprintf("failed to delete extension %s", ext.GetName()))
			}

			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
			garden.Finalizers = nil
			Expect(testClient.Update(ctx, garden)).To(Succeed())
			Expect(testClient.Delete(ctx, garden)).To(Or(Succeed(), BeNotFoundError()))
		})
	})

	It("should successfully create extension resources", func() {
		for _, ext := range []client.Object{providerExtension, dnsExtension, extension} {
			Expect(testClient.Create(ctx, ext)).To(Succeed())
			log.Info("Created extension", "garden", ext.GetName())
		}
	})

	It("should ensure extensions are not reported as required", func() {
		for _, ext := range []*operatorv1alpha1.Extension{providerExtension, dnsExtension, extension} {
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(ext), ext)).To(Succeed())
				return ext.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.ExtensionRequiredRuntime),
				WithStatus(gardencorev1beta1.ConditionFalse),
				WithReason("ExtensionNotRequired"),
			), fmt.Sprintf("extension %s is expected to be reported as not required", ext.GetName()))
		}
	})

	It("should report extensions as required after garden was created", func() {
		Expect(testClient.Create(ctx, garden)).To(Succeed())

		for _, ext := range []client.Object{providerExtension, dnsExtension} {
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(ext), ext)).To(Succeed())
				return providerExtension.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.ExtensionRequiredRuntime),
				WithStatus(gardencorev1beta1.ConditionTrue),
				WithReason("ExtensionRequired"),
			), fmt.Sprintf("extension %s/%s is expected to be reported as required", ext.GetNamespace(), ext.GetName()))
		}

		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extension), extension)).To(Succeed())
		Expect(extension.Status.Conditions).Should(ContainCondition(
			OfType(operatorv1alpha1.ExtensionRequiredRuntime),
			WithStatus(gardencorev1beta1.ConditionFalse),
			WithReason("ExtensionNotRequired"),
		))
	})

	It("should report generic extension as required after garden spec was changed", func() {
		garden.Spec.Extensions = []operatorv1alpha1.GardenExtension{
			{Type: extensionProvider},
		}

		Expect(testClient.Update(ctx, garden)).To(Succeed())

		Eventually(func(g Gomega) []gardencorev1beta1.Condition {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extension), extension)).To(Succeed())
			return extension.Status.Conditions
		}).Should(ContainCondition(
			OfType(operatorv1alpha1.ExtensionRequiredRuntime),
			WithStatus(gardencorev1beta1.ConditionTrue),
			WithReason("ExtensionRequired"),
		))
	})

	It("should report generic extension as not required after garden removed it", func() {
		garden.Spec.Extensions = []operatorv1alpha1.GardenExtension{
			{Type: extensionProvider + "-1"},
		}

		Expect(testClient.Update(ctx, garden)).To(Succeed())

		Eventually(func(g Gomega) []gardencorev1beta1.Condition {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extension), extension)).To(Succeed())
			return extension.Status.Conditions
		}).Should(ContainCondition(
			OfType(operatorv1alpha1.ExtensionRequiredRuntime),
			WithStatus(gardencorev1beta1.ConditionFalse),
			WithReason("ExtensionNotRequired"),
		))
	})

	It("should report dns extension as not required during garden deletion", func() {
		backupBucket := &extensionsv1alpha1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: extensionPrefix + "-bucket",
			},
			Spec: extensionsv1alpha1.BackupBucketSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Class: ptr.To[extensionsv1alpha1.ExtensionClass]("garden"),
					Type:  backupBucketProvider,
				},
				Region: "region",
				SecretRef: corev1.SecretReference{
					Name: "test-bar-bucket",
				},
			},
		}

		Expect(testClient.Create(ctx, backupBucket)).To(Succeed())
		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, backupBucket)).To(Succeed())
		})

		Expect(testClient.Delete(ctx, garden)).To(Succeed())

		Eventually(func(g Gomega) []gardencorev1beta1.Condition {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dnsExtension), dnsExtension)).To(Succeed())
			return dnsExtension.Status.Conditions
		}).Should(ContainCondition(
			OfType(operatorv1alpha1.ExtensionRequiredRuntime),
			WithStatus(gardencorev1beta1.ConditionFalse),
			WithReason("ExtensionNotRequired"),
		))

		Consistently(func(g Gomega) []gardencorev1beta1.Condition {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(providerExtension), providerExtension)).To(Succeed())
			return providerExtension.Status.Conditions
		}).Should(ContainCondition(
			OfType(operatorv1alpha1.ExtensionRequiredRuntime),
			WithStatus(gardencorev1beta1.ConditionTrue),
			WithReason("ExtensionRequired"),
		))
	})

	It("should report provider extension as not required during garden deletion after backupbucket is gone", func() {
		Eventually(func(g Gomega) []gardencorev1beta1.Condition {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(providerExtension), providerExtension)).To(Succeed())
			return providerExtension.Status.Conditions
		}).Should(ContainCondition(
			OfType(operatorv1alpha1.ExtensionRequiredRuntime),
			WithStatus(gardencorev1beta1.ConditionFalse),
			WithReason("ExtensionNotRequired"),
		))
	})
})
