// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package required_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Extension Required controller tests", func() {
	var (
		providerExtension, dnsExtension *operatorv1alpha1.Extension

		backupBucket *extensionsv1alpha1.BackupBucket
		dnsRecord    *extensionsv1alpha1.DNSRecord
		extension    *extensionsv1alpha1.Extension

		backupBucketProvider string
		dnsProvider          string
		extensionProvider    string
	)

	BeforeEach(func() {
		backupBucketProvider = "local"
		extensionProvider = "ext-local"
		dnsProvider = "dns-local"

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
					{
						Kind: "Extension",
						Type: extensionProvider,
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

		backupBucket = &extensionsv1alpha1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: extensionPrefix + "-bucket",
			},
			Spec: extensionsv1alpha1.BackupBucketSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: backupBucketProvider,
				},
				Region: "region",
				SecretRef: corev1.SecretReference{
					Name: "test-bar-bucket",
				},
			},
		}

		dnsRecord = &extensionsv1alpha1.DNSRecord{
			ObjectMeta: metav1.ObjectMeta{
				Name:      extensionPrefix + "-record",
				Namespace: testNamespace.Name,
			},
			Spec: extensionsv1alpha1.DNSRecordSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: dnsProvider + "-foo",
				},
				SecretRef: corev1.SecretReference{
					Name: "test-foo-dns",
				},
				Name:       "test.example.com",
				RecordType: extensionsv1alpha1.DNSRecordTypeA,
				Values:     []string{"1.2.3.4"},
			},
		}

		extension = &extensionsv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name:      extensionPrefix + "-record",
				Namespace: testNamespace.Name,
			},
			Spec: extensionsv1alpha1.ExtensionSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: extensionProvider,
				},
			},
		}
	})

	It("should reconcile the extensions and calculate the expected required status", func() {
		By("Create extensions")

		for _, ext := range []client.Object{providerExtension, dnsExtension} {
			Expect(testClient.Create(ctx, ext)).To(Succeed())
			log.Info("Created extension", "garden", ext.GetName())
			DeferCleanup(func() {
				By("Delete extension")
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, ext))).To(Succeed())
				By("Ensure extension is gone")
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(ext), ext)
				}).Should(BeNotFoundError())
			})
		}

		By("Check provider extensions is reported as not required")
		Eventually(func(g Gomega) []gardencorev1beta1.Condition {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(providerExtension), providerExtension)).To(Succeed())
			return providerExtension.Status.Conditions
		}).Should(ContainCondition(
			OfType(operatorv1alpha1.ExtensionRequiredRuntime),
			WithStatus(gardencorev1beta1.ConditionFalse),
			WithReason("ExtensionNotRequired"),
		))

		By("Check dns extensions is reported as not required")
		Eventually(func(g Gomega) []gardencorev1beta1.Condition {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(providerExtension), providerExtension)).To(Succeed())
			return providerExtension.Status.Conditions
		}).Should(ContainCondition(
			OfType(operatorv1alpha1.ExtensionRequiredRuntime),
			WithStatus(gardencorev1beta1.ConditionFalse),
			WithReason("ExtensionNotRequired"),
		))

		By("Deploy BackupBucket for provider extension")
		Expect(testClient.Create(ctx, backupBucket)).To(Succeed())

		By("Check provider extension is reported as required")
		Eventually(func(g Gomega) []gardencorev1beta1.Condition {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(providerExtension), providerExtension)).To(Succeed())
			return providerExtension.Status.Conditions
		}).Should(ContainCondition(
			OfType(operatorv1alpha1.ExtensionRequiredRuntime),
			WithStatus(gardencorev1beta1.ConditionTrue),
			WithReason("ExtensionRequired"),
		))

		By("Deploy DNSRecord with different type")
		Expect(testClient.Create(ctx, dnsRecord)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, dnsRecord))).To(Succeed())
		})

		By("Check DNS extension is still not reported as required")
		Consistently(func(g Gomega) []gardencorev1beta1.Condition {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dnsExtension), dnsExtension)).To(Succeed())
			return dnsExtension.Status.Conditions
		}).Should(ContainCondition(
			OfType(operatorv1alpha1.ExtensionRequiredRuntime),
			WithStatus(gardencorev1beta1.ConditionFalse),
			WithReason("ExtensionNotRequired"),
		))

		By("Deploy Extension for provider extension")
		Expect(testClient.Create(ctx, extension)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, extension))).To(Succeed())
		})
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(extension), &extensionsv1alpha1.Extension{})
		}).Should(Succeed())

		By("Delete BackupBucket for provider extension and check extension is still required")
		Expect(testClient.Delete(ctx, backupBucket)).To(Succeed())

		Eventually(func(g Gomega) []gardencorev1beta1.Condition {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(providerExtension), providerExtension)).To(Succeed())
			return providerExtension.Status.Conditions
		}).Should(ContainCondition(
			OfType(operatorv1alpha1.ExtensionRequiredRuntime),
			WithStatus(gardencorev1beta1.ConditionTrue),
			WithReason("ExtensionRequired"),
		))

		By("Delete Extension for provider extension and check extension is not required anymore")
		Expect(testClient.Delete(ctx, extension)).To(Succeed())

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
