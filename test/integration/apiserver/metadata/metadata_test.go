// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package metadata_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Metadata subresource", func() {
	Context("cluster-scoped resource (Seed)", func() {
		var seed *gardencorev1beta1.Seed

		BeforeEach(func() {
			seed = &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-",
				},
				Spec: gardencorev1beta1.SeedSpec{
					Provider: gardencorev1beta1.SeedProvider{
						Region: "region",
						Type:   "provider-type",
					},
					Ingress: &gardencorev1beta1.Ingress{
						Domain: "seed.example.com",
						Controller: gardencorev1beta1.IngressController{
							Kind: "nginx",
						},
					},
					DNS: gardencorev1beta1.SeedDNS{
						Provider: &gardencorev1beta1.SeedDNSProvider{
							Type: "provider",
							CredentialsRef: &corev1.ObjectReference{
								APIVersion: "v1",
								Kind:       "Secret",
								Name:       "some-secret",
								Namespace:  "some-namespace",
							},
						},
						Internal: &gardencorev1beta1.SeedDNSProviderConfig{
							Type:   "provider",
							Domain: "local.example.com",
							CredentialsRef: corev1.ObjectReference{
								APIVersion: "v1",
								Kind:       "Secret",
								Name:       "some-secret",
								Namespace:  "some-namespace",
							},
						},
					},
					Networks: gardencorev1beta1.SeedNetworks{
						Pods:     "10.0.0.0/16",
						Services: "10.1.0.0/16",
					},
				},
			}
			Expect(testClient.Create(ctx, seed)).To(Succeed())

			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, seed))).To(Succeed())
			})
		})

		It("should allow changing labels via the metadata subresource", func() {
			patch := client.MergeFrom(seed.DeepCopy())
			metav1.SetMetaDataLabel(&seed.ObjectMeta, "foo", "bar")

			Expect(testClient.SubResource("metadata").Patch(ctx, seed, patch)).To(Succeed())
			Expect(seed.Labels).To(HaveKeyWithValue("foo", "bar"))
		})

		It("should not change the spec when updating via the metadata subresource", func() {
			patch := client.MergeFrom(seed.DeepCopy())
			metav1.SetMetaDataLabel(&seed.ObjectMeta, "foo", "bar")
			seed.Spec.Provider.Region = "other-region"

			Expect(testClient.SubResource("metadata").Patch(ctx, seed, patch)).To(Succeed())

			updatedSeed := &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: seed.Name}}
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(updatedSeed), updatedSeed)).To(Succeed())
			Expect(updatedSeed.Labels).To(HaveKeyWithValue("foo", "bar"))
			Expect(updatedSeed.Spec.Provider.Region).To(Equal("region"))
		})

		It("should run the parent strategy's canonicalization when updating via the metadata subresource", func() {
			generationBeforePatch := seed.Generation

			patch := client.MergeFrom(seed.DeepCopy())
			metav1.SetMetaDataAnnotation(&seed.ObjectMeta, "gardener.cloud/operation", "reconcile")

			Expect(testClient.SubResource("metadata").Patch(ctx, seed, patch)).To(Succeed())

			updatedSeed := &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: seed.Name}}
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(updatedSeed), updatedSeed)).To(Succeed())
			Expect(updatedSeed.Annotations).NotTo(HaveKey("gardener.cloud/operation"))
			Expect(updatedSeed.Generation).To(Equal(generationBeforePatch + 1))
		})
	})

	Context("namespaced resource (Project)", func() {
		var project *gardencorev1beta1.Project

		BeforeEach(func() {
			project = &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-",
				},
			}
			Expect(testClient.Create(ctx, project)).To(Succeed())

			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, project))).To(Succeed())
			})
		})

		It("should allow changing annotations via the metadata subresource", func() {
			patch := client.MergeFrom(project.DeepCopy())
			metav1.SetMetaDataAnnotation(&project.ObjectMeta, "foo", "bar")

			Expect(testClient.SubResource("metadata").Patch(ctx, project, patch)).To(Succeed())
			Expect(project.Annotations).To(HaveKeyWithValue("foo", "bar"))
		})

		It("should not change the spec when updating via the metadata subresource", func() {
			patch := client.MergeFrom(project.DeepCopy())
			metav1.SetMetaDataAnnotation(&project.ObjectMeta, "foo", "bar")
			project.Spec.Description = new("some description")

			Expect(testClient.SubResource("metadata").Patch(ctx, project, patch)).To(Succeed())

			updatedProject := &gardencorev1beta1.Project{ObjectMeta: metav1.ObjectMeta{Name: project.Name}}
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(updatedProject), updatedProject)).To(Succeed())
			Expect(updatedProject.Annotations).To(HaveKeyWithValue("foo", "bar"))
			Expect(updatedProject.Spec.Description).To(BeNil())
		})
	})

	Context("subresource lookup", func() {
		It("should reject the metadata subresource for a non-existing object", func() {
			seed := &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: "non-existing"}}
			patch := client.MergeFrom(seed.DeepCopy())
			metav1.SetMetaDataLabel(&seed.ObjectMeta, "foo", "bar")

			Expect(testClient.SubResource("metadata").Patch(ctx, seed, patch)).To(BeNotFoundError())
		})
	})
})
