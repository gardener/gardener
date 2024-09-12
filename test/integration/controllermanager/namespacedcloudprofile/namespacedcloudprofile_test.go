// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package namespacedcloudprofile_test

import (
	"sort"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("NamespacedCloudProfile controller tests", func() {
	var (
		parentCloudProfile     *gardencorev1beta1.CloudProfile
		namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile

		mergedCloudProfileSpec *gardencorev1beta1.CloudProfileSpec

		expirationDateFuture metav1.Time
	)

	BeforeEach(func() {
		dateNow, _ := time.Parse(time.DateOnly, time.Now().Format(time.DateOnly))
		expirationDateFuture = metav1.Time{Time: dateNow.Local().Add(48 * time.Hour)}

		parentCloudProfile = &gardencorev1beta1.CloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-",
			},
			Spec: gardencorev1beta1.CloudProfileSpec{
				Type: "some-type",
				Kubernetes: gardencorev1beta1.KubernetesSettings{
					Versions: []gardencorev1beta1.ExpirableVersion{{Version: "1.3.0"}, {Version: "1.2.3"}},
				},
				MachineImages: []gardencorev1beta1.MachineImage{
					{
						Name: "some-image",
						Versions: []gardencorev1beta1.MachineImageVersion{
							{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "4.5.6"}},
						},
					},
				},
				MachineTypes: []gardencorev1beta1.MachineType{{
					Name:   "some-type",
					CPU:    resource.MustParse("1"),
					GPU:    resource.MustParse("0"),
					Memory: resource.MustParse("1Gi"),
				}},
				Regions: []gardencorev1beta1.Region{
					{Name: "some-region"},
				},
			},
		}

		namespacedCloudProfile = &gardencorev1beta1.NamespacedCloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.NamespacedCloudProfileSpec{
				Kubernetes: &gardencorev1beta1.KubernetesSettings{
					Versions: []gardencorev1beta1.ExpirableVersion{{Version: "1.2.3", ExpirationDate: &expirationDateFuture}},
				},
				MachineImages: []gardencorev1beta1.MachineImage{
					{
						Name: "some-image",
						Versions: []gardencorev1beta1.MachineImageVersion{
							{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "4.5.6", ExpirationDate: &expirationDateFuture}},
						},
					},
				},
				MachineTypes: []gardencorev1beta1.MachineType{{
					Name:   "some-other-type",
					CPU:    resource.MustParse("2"),
					GPU:    resource.MustParse("0"),
					Memory: resource.MustParse("2Gi"),
				}},
			},
		}

		updateStrategy := gardencorev1beta1.MachineImageUpdateStrategy("major")
		usable := true
		architecture := "amd64"

		mergedCloudProfileSpec = &gardencorev1beta1.CloudProfileSpec{
			Kubernetes: gardencorev1beta1.KubernetesSettings{
				Versions: []gardencorev1beta1.ExpirableVersion{
					{Version: "1.3.0"},
					{
						Version:        "1.2.3",
						ExpirationDate: &expirationDateFuture,
					},
				},
			},
			MachineImages: []gardencorev1beta1.MachineImage{
				{
					Name: "some-image",
					Versions: []gardencorev1beta1.MachineImageVersion{
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "4.5.6", ExpirationDate: &expirationDateFuture},
							CRI: []gardencorev1beta1.CRI{
								{
									Name:              "containerd",
									ContainerRuntimes: nil,
								},
							},
							Architectures: []string{
								"amd64",
							},
						},
					},
					UpdateStrategy: &updateStrategy,
				},
			},
			MachineTypes: []gardencorev1beta1.MachineType{
				{
					Name:         "some-type",
					CPU:          resource.MustParse("1"),
					GPU:          resource.MustParse("0"),
					Memory:       resource.MustParse("1Gi"),
					Usable:       &usable,
					Architecture: &architecture,
				},
				{
					Name:         "some-other-type",
					CPU:          resource.MustParse("2"),
					GPU:          resource.MustParse("0"),
					Memory:       resource.MustParse("2Gi"),
					Usable:       &usable,
					Architecture: &architecture,
				}},
			Regions: []gardencorev1beta1.Region{
				{Name: "some-region"},
			},
			Type: "some-type",
		}
	})

	Context("deletion of NamespacedCloudProfile", func() {
		var (
			shoot *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: testID + "-",
					Namespace:    testNamespace.Name,
				},
				Spec: gardencorev1beta1.ShootSpec{
					SecretBindingName: ptr.To("my-provider-account"),
					Region:            "foo-region",
					Provider: gardencorev1beta1.Provider{
						Type: "aws",
						Workers: []gardencorev1beta1.Worker{
							{
								Name:    "cpu-worker",
								Minimum: 2,
								Maximum: 2,
								Machine: gardencorev1beta1.Machine{Type: "large"},
							},
						},
					},
					Kubernetes: gardencorev1beta1.Kubernetes{Version: "1.26.1"},
					Networking: &gardencorev1beta1.Networking{Type: ptr.To("foo-networking")},
				},
			}
		})

		JustBeforeEach(func() {
			By("Create parent CloudProfile")
			Expect(testClient.Create(ctx, parentCloudProfile)).To(Succeed())
			log.Info("Created parent CloudProfile for test", "parentCloudProfile", client.ObjectKeyFromObject(parentCloudProfile))

			By("Create NamespacedCloudProfile")
			namespacedCloudProfile.Spec.Parent = gardencorev1beta1.CloudProfileReference{
				Kind: "CloudProfile",
				Name: parentCloudProfile.Name,
			}
			Eventually(func() error {
				return testClient.Create(ctx, namespacedCloudProfile)
			}).Should(Succeed())
			Eventually(func(g Gomega) {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)).To(Succeed())
				g.Expect(namespacedCloudProfile.Status.ObservedGeneration).To(Equal(namespacedCloudProfile.Generation))
			}).Should(Succeed())
			log.Info("Created NamespacedCloudProfile for test", "namespacedCloudProfile", client.ObjectKeyFromObject(namespacedCloudProfile))

			if shoot != nil {
				By("Create Shoot")
				shoot.Spec.CloudProfile = &gardencorev1beta1.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: namespacedCloudProfile.Name,
				}
				Expect(testClient.Create(ctx, shoot)).To(Succeed())
				log.Info("Created shoot for test", "shoot", client.ObjectKeyFromObject(shoot))

				By("Wait until manager has observed Shoot")
				// Use the manager's cache to ensure it has observed the Shoot.
				// Otherwise, the controller might clean up the NamespacedCloudProfile too early because it thinks all referencing
				// Shoots are gone. Similar to https://github.com/gardener/gardener/issues/6486 and
				// https://github.com/gardener/gardener/issues/6607.
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(shoot), &gardencorev1beta1.Shoot{})
				}).Should(Succeed())

				DeferCleanup(func() {
					By("Delete Shoot")
					Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot))).To(Succeed())
				})
			}

			DeferCleanup(func() {
				By("Delete NamespacedCloudProfile")
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, namespacedCloudProfile))).To(Succeed())

				By("Delete ParentCloudProfile")
				Expect(testClient.Delete(ctx, parentCloudProfile)).To(Succeed())
			})
		})

		Context("with shoots referencing the NamespacedCloudProfile", func() {
			JustBeforeEach(func() {
				By("Ensure finalizer got added")
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)).To(Succeed())
					g.Expect(namespacedCloudProfile.Finalizers).To(ConsistOf("gardener"))
				}).Should(Succeed())

				By("Delete NamespacedCloudProfile")
				Expect(testClient.Delete(ctx, namespacedCloudProfile)).To(Succeed())
			})

			It("should add the finalizer and not release it on deletion since there still is a referencing shoot", func() {
				By("Ensure NamespacedCloudProfile is not released")
				Consistently(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)
				}).Should(Succeed())
			})

			It("should add the finalizer and release it on deletion after the shoot got deleted", func() {
				By("Delete Shoot")
				Expect(testClient.Delete(ctx, shoot)).To(Succeed())

				By("Ensure NamespacedCloudProfile is released")
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)
				}).Should(BeNotFoundError())
			})
		})

		Context("with no shoot referencing the NamespacedCloudProfile", func() {
			BeforeEach(func() {
				shoot = nil
			})

			It("should add the finalizer and release it on deletion", func() {
				By("Ensure finalizer got added")
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)).To(Succeed())
					g.Expect(namespacedCloudProfile.Finalizers).To(ConsistOf("gardener"))
				}).Should(Succeed())

				By("Delete NamespacedCloudProfile")
				Expect(testClient.Delete(ctx, namespacedCloudProfile)).To(Succeed())

				By("Ensure NamespacedCloudProfile is released")
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)
				}).Should(BeNotFoundError())
			})
		})
	})

	Context("merging the CloudProfiles", func() {
		JustBeforeEach(func() {
			By("Create parent CloudProfile")
			Expect(testClient.Create(ctx, parentCloudProfile)).To(Succeed())
			log.Info("Created parent CloudProfile for test", "parentCloudProfile", client.ObjectKeyFromObject(parentCloudProfile))

			By("Create NamespacedCloudProfile")
			namespacedCloudProfile.Spec.Parent = gardencorev1beta1.CloudProfileReference{
				Kind: "CloudProfile",
				Name: parentCloudProfile.Name,
			}
			Eventually(func() error {
				return testClient.Create(ctx, namespacedCloudProfile)
			}).Should(Succeed())
			log.Info("Created NamespacedCloudProfile for test", "namespacedCloudProfile", client.ObjectKeyFromObject(namespacedCloudProfile))

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)).To(Succeed())
				g.Expect(namespacedCloudProfile.Status.ObservedGeneration).To(Equal(namespacedCloudProfile.Generation))
			}).Should(Succeed())

			DeferCleanup(func() {
				By("Delete NamespacedCloudProfile")
				Expect(testClient.Delete(ctx, namespacedCloudProfile)).To(Succeed())

				By("Delete ParentCloudProfile")
				Expect(testClient.Delete(ctx, parentCloudProfile)).To(Succeed())
			})
		})

		It("should merge the NamespacedCloudProfile correctly", func() {
			Eventually(func(g Gomega) {
				err := testClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(withSortedArrays(namespacedCloudProfile.Status.CloudProfileSpec)).To(Equal(*mergedCloudProfileSpec))
				g.Expect(namespacedCloudProfile.Status.ObservedGeneration).To(Equal(namespacedCloudProfile.Generation))
			}).Should(Succeed())
		})

		It("should update the NamespacedCloudProfile status on NamespacedCloudProfile spec update", func() {
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)).To(Succeed())
			namespacedCloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{}
			namespacedCloudProfile.ResourceVersion = ""

			err := testClient.Update(ctx, namespacedCloudProfile)
			Expect(err).NotTo(HaveOccurred())

			expectedKubernetesVersions := gardencorev1beta1.KubernetesSettings{
				Versions: []gardencorev1beta1.ExpirableVersion{
					{Version: "1.2.3"},
					{Version: "1.3.0"},
				},
			}

			Eventually(func(g Gomega) {
				err := testClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(namespacedCloudProfile.Status.CloudProfileSpec.Kubernetes.Versions).To(ContainElements(expectedKubernetesVersions.Versions))
				g.Expect(namespacedCloudProfile.Status.ObservedGeneration).To(Equal(namespacedCloudProfile.Generation))
			}).Should(Succeed())
		})

		It("should update the NamespacedCloudProfile status on CloudProfile spec update", func() {
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(parentCloudProfile), parentCloudProfile)).To(Succeed())
			parentCloudProfile.Spec.Kubernetes.Versions = append(parentCloudProfile.Spec.Kubernetes.Versions, gardencorev1beta1.ExpirableVersion{Version: "1.4.0"})

			err := testClient.Update(ctx, parentCloudProfile)
			Expect(err).NotTo(HaveOccurred())

			expectedKubernetesVersions := gardencorev1beta1.KubernetesSettings{
				Versions: []gardencorev1beta1.ExpirableVersion{
					{Version: "1.2.3", ExpirationDate: &expirationDateFuture},
					{Version: "1.3.0"},
					{Version: "1.4.0"},
				},
			}

			Eventually(func(g Gomega) {
				err := testClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(namespacedCloudProfile.Status.CloudProfileSpec.Kubernetes.Versions).To(ContainElements(expectedKubernetesVersions.Versions))
				g.Expect(namespacedCloudProfile.Status.ObservedGeneration).To(Equal(namespacedCloudProfile.Generation))
				g.Expect(namespacedCloudProfile.Status.ObservedGeneration).NotTo(BeEquivalentTo(0))
			}).Should(Succeed())
		})
	})

	Context("handling NamespacedCloudProfile expiration date overrides", func() {
		var (
			expirationDatePast metav1.Time
		)

		BeforeEach(func() {
			dateNow, _ := time.Parse(time.DateOnly, time.Now().Format(time.DateOnly))
			expirationDatePast = metav1.Time{Time: dateNow.Local().Add(-96 * time.Hour)}
		})

		JustBeforeEach(func() {
			By("Create parent CloudProfile")
			Expect(testClient.Create(ctx, parentCloudProfile)).To(Succeed())
			log.Info("Created parent CloudProfile for test", "parentCloudProfile", client.ObjectKeyFromObject(parentCloudProfile))

			namespacedCloudProfile.Spec.Parent = gardencorev1beta1.CloudProfileReference{
				Kind: "CloudProfile",
				Name: parentCloudProfile.Name,
			}

			DeferCleanup(func() {
				By("Delete ParentCloudProfile")
				Expect(testClient.Delete(ctx, parentCloudProfile)).To(Succeed())
			})
		})

		It("should allow creation with an already expired Kubernetes version but remove it from the persisted spec and not render it into the status", func() {
			namespacedCloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{
				{Version: "1.2.3", ExpirationDate: &expirationDatePast},
			}

			Eventually(func() error {
				return testClient.Create(ctx, namespacedCloudProfile)
			}).Should(Succeed())
			log.Info("Created NamespacedCloudProfile for test", "namespacedCloudProfile", client.ObjectKeyFromObject(namespacedCloudProfile))

			expectedKubernetesVersions := gardencorev1beta1.KubernetesSettings{
				Versions: []gardencorev1beta1.ExpirableVersion{
					{Version: "1.2.3"},
					{Version: "1.3.0"},
				},
			}

			Eventually(func(g Gomega) {
				err := testClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(namespacedCloudProfile.Spec.Kubernetes.Versions).To(BeEmpty())
				g.Expect(namespacedCloudProfile.Status.CloudProfileSpec.Kubernetes.Versions).To(ContainElements(expectedKubernetesVersions.Versions))
				g.Expect(namespacedCloudProfile.Status.ObservedGeneration).To(Equal(namespacedCloudProfile.Generation))
			}).Should(Succeed())
		})

		It("should allow creation with an already expired MachineImage version but remove it from persisted spec and not render it into the status", func() {
			namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
				{
					Name: "some-image",
					Versions: []gardencorev1beta1.MachineImageVersion{
						{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "4.5.6", ExpirationDate: &expirationDatePast}},
					},
				},
			}

			Eventually(func() error {
				return testClient.Create(ctx, namespacedCloudProfile)
			}).Should(Succeed())
			log.Info("Created NamespacedCloudProfile for test", "namespacedCloudProfile", client.ObjectKeyFromObject(namespacedCloudProfile))

			expectedMachineImages := []gardencorev1beta1.MachineImage{
				{
					Name: "some-image",
					Versions: []gardencorev1beta1.MachineImageVersion{
						{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "4.5.6"}, CRI: []gardencorev1beta1.CRI{{Name: "containerd"}}, Architectures: []string{"amd64"}},
					},
					UpdateStrategy: ptr.To(gardencorev1beta1.UpdateStrategyMajor),
				},
			}

			Eventually(func(g Gomega) {
				err := testClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(namespacedCloudProfile.Spec.MachineImages).To(BeEmpty())
				g.Expect(namespacedCloudProfile.Status.CloudProfileSpec.MachineImages).To(Equal(expectedMachineImages))
				g.Expect(namespacedCloudProfile.Status.ObservedGeneration).To(Equal(namespacedCloudProfile.Generation))
			}).Should(Succeed())
		})

		It("should not allow update with an already expired Kubernetes version", func() {
			Eventually(func() error {
				return testClient.Create(ctx, namespacedCloudProfile)
			}).Should(Succeed())
			log.Info("Created NamespacedCloudProfile for test", "namespacedCloudProfile", client.ObjectKeyFromObject(namespacedCloudProfile))
			Eventually(func(g Gomega) {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)).To(Succeed())
				g.Expect(namespacedCloudProfile.Status.ObservedGeneration).To(Equal(namespacedCloudProfile.Generation))
			}).Should(Succeed())

			namespacedCloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{
				{Version: "1.2.3", ExpirationDate: &expirationDatePast},
			}
			Expect(testClient.Update(ctx, namespacedCloudProfile)).To(MatchError(
				ContainSubstring("expiration date of version '1.2.3' is in the past"),
			))
		})

		It("should not allow update with an already expired MachineImage version", func() {
			Eventually(func() error {
				return testClient.Create(ctx, namespacedCloudProfile)
			}).Should(Succeed())
			log.Info("Created NamespacedCloudProfile for test", "namespacedCloudProfile", client.ObjectKeyFromObject(namespacedCloudProfile))
			Eventually(func(g Gomega) {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)).To(Succeed())
				g.Expect(namespacedCloudProfile.Status.ObservedGeneration).To(Equal(namespacedCloudProfile.Generation))
			}).Should(Succeed())

			namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
				{
					Name: "some-image",
					Versions: []gardencorev1beta1.MachineImageVersion{
						{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "4.5.6", ExpirationDate: &expirationDatePast}},
					},
				},
			}
			Expect(testClient.Update(ctx, namespacedCloudProfile)).To(MatchError(
				ContainSubstring("expiration date of version '4.5.6' is in the past"),
			))
		})
	})
})

func withSortedArrays(nscpfl gardencorev1beta1.CloudProfileSpec) gardencorev1beta1.CloudProfileSpec {
	sort.Slice(nscpfl.MachineTypes, func(i, j int) bool {
		return strings.Compare(nscpfl.MachineTypes[i].Name, nscpfl.MachineTypes[j].Name) >= 0
	})
	sort.Slice(nscpfl.MachineImages, func(i, j int) bool {
		return strings.Compare(nscpfl.MachineImages[i].Name, nscpfl.MachineImages[j].Name) >= 0
	})
	sort.Slice(nscpfl.Kubernetes.Versions, func(i, j int) bool {
		return strings.Compare(nscpfl.Kubernetes.Versions[i].Version, nscpfl.Kubernetes.Versions[j].Version) >= 0
	})
	return nscpfl
}
