// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package namespacedcloudprofile_test

import (
	"context"
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

		updateStrategy := gardencorev1beta1.MachineImageUpdateStrategy("major")

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
							{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "7.8.9"}, CRI: []gardencorev1beta1.CRI{{Name: "containerd"}}, Architectures: []string{"amd64"}},
						},
					},
					{
						Name: "custom-image",
						Versions: []gardencorev1beta1.MachineImageVersion{
							{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.1.2"}, CRI: []gardencorev1beta1.CRI{{Name: "containerd"}}, Architectures: []string{"amd64"}},
						},
						UpdateStrategy: &updateStrategy,
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
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "7.8.9"},
							CRI:              []gardencorev1beta1.CRI{{Name: "containerd"}},
							Architectures:    []string{"amd64"},
						},
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
				{
					Name: "custom-image",
					Versions: []gardencorev1beta1.MachineImageVersion{
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.1.2"},
							CRI:              []gardencorev1beta1.CRI{{Name: "containerd"}},
							Architectures:    []string{"amd64"}},
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
			Eventually(func() error {
				return testClient.Create(ctx, parentCloudProfile)
			}).Should(Succeed())

			By("Create NamespacedCloudProfile")
			namespacedCloudProfile.Spec.Parent = gardencorev1beta1.CloudProfileReference{
				Kind: "CloudProfile",
				Name: parentCloudProfile.Name,
			}
			Eventually(func() error {
				return testClient.Create(ctx, namespacedCloudProfile)
			}).Should(Succeed())
			waitForNamespacedCloudProfileToBeReconciled(ctx, namespacedCloudProfile)

			if shoot != nil {
				By("Create Shoot")
				shoot.Spec.CloudProfile = &gardencorev1beta1.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: namespacedCloudProfile.Name,
				}
				Eventually(func() error {
					return testClient.Create(ctx, shoot)
				}).Should(Succeed())
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
				Expect(namespacedCloudProfile.Finalizers).To(ConsistOf("gardener"))

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
				Expect(namespacedCloudProfile.Finalizers).To(ConsistOf("gardener"))

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
			Eventually(func() error {
				return testClient.Create(ctx, parentCloudProfile)
			}).Should(Succeed())

			By("Create NamespacedCloudProfile")
			namespacedCloudProfile.Spec.Parent = gardencorev1beta1.CloudProfileReference{
				Kind: "CloudProfile",
				Name: parentCloudProfile.Name,
			}
			Eventually(func() error {
				return testClient.Create(ctx, namespacedCloudProfile)
			}).Should(Succeed())
			waitForNamespacedCloudProfileToBeReconciled(ctx, namespacedCloudProfile)

			DeferCleanup(func() {
				By("Delete NamespacedCloudProfile")
				Expect(testClient.Delete(ctx, namespacedCloudProfile)).To(Succeed())

				By("Delete ParentCloudProfile")
				Expect(testClient.Delete(ctx, parentCloudProfile)).To(Succeed())
			})
		})

		It("should merge the NamespacedCloudProfile correctly", func() {
			Expect(withSortedArrays(namespacedCloudProfile.Status.CloudProfileSpec)).To(Equal(*mergedCloudProfileSpec))
		})

		It("should update the NamespacedCloudProfile status on NamespacedCloudProfile spec update", func() {
			namespacedCloudProfilePatch := client.StrategicMergeFrom(namespacedCloudProfile.DeepCopy())
			namespacedCloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{}
			Eventually(func() error {
				return testClient.Patch(ctx, namespacedCloudProfile, namespacedCloudProfilePatch)
			}).Should(Succeed())
			waitForNamespacedCloudProfileToBeReconciled(ctx, namespacedCloudProfile)

			Expect(namespacedCloudProfile.Status.CloudProfileSpec.Kubernetes.Versions).To(ContainElements(
				gardencorev1beta1.ExpirableVersion{Version: "1.2.3"},
				gardencorev1beta1.ExpirableVersion{Version: "1.3.0"},
			))
		})

		It("should update the NamespacedCloudProfile status on CloudProfile spec update", func() {
			cloudProfilePatch := client.StrategicMergeFrom(parentCloudProfile.DeepCopy())
			parentCloudProfile.Spec.Kubernetes.Versions = append(parentCloudProfile.Spec.Kubernetes.Versions, gardencorev1beta1.ExpirableVersion{Version: "1.4.0"})
			Eventually(func() error {
				return testClient.Patch(ctx, parentCloudProfile, cloudProfilePatch)
			}).Should(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)).To(Succeed())
				g.Expect(namespacedCloudProfile.Status.CloudProfileSpec.Kubernetes.Versions).To(ContainElements(
					gardencorev1beta1.ExpirableVersion{Version: "1.2.3", ExpirationDate: &expirationDateFuture},
					gardencorev1beta1.ExpirableVersion{Version: "1.3.0"},
					gardencorev1beta1.ExpirableVersion{Version: "1.4.0"},
				))
			}).Should(Succeed())
		})

		Context("limits.maxNodesTotal", func() {
			It("should update the NamespacedCloudProfile status if the parent CloudProfile sets a limit", func() {
				Expect(namespacedCloudProfile.Status.CloudProfileSpec.Limits).To(BeNil())

				By("Set limit in parent cloud profile")
				cloudProfilePatch := client.StrategicMergeFrom(parentCloudProfile.DeepCopy())
				parentCloudProfile.Spec.Limits = &gardencorev1beta1.Limits{
					MaxNodesTotal: ptr.To(int32(10)),
				}
				Eventually(func() error {
					return testClient.Patch(ctx, parentCloudProfile, cloudProfilePatch)
				}).Should(Succeed())

				By("Wait for NamespacedCloudProfile status to be updated by the controller")
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)).To(Succeed())
					g.Expect(namespacedCloudProfile.Status.CloudProfileSpec.Limits).To(Not(BeNil()))
					g.Expect(namespacedCloudProfile.Status.CloudProfileSpec.Limits.MaxNodesTotal).To(Equal(ptr.To(int32(10))))
				}).Should(Succeed())
			})

			It("should update the NamespacedCloudProfile status with a decreased limit", func() {
				By("Set limit in parent cloud profile")
				cloudProfilePatch := client.StrategicMergeFrom(parentCloudProfile.DeepCopy())
				parentCloudProfile.Spec.Limits = &gardencorev1beta1.Limits{
					MaxNodesTotal: ptr.To(int32(10)),
				}
				Eventually(func() error {
					return testClient.Patch(ctx, parentCloudProfile, cloudProfilePatch)
				}).Should(Succeed())

				By("Patch the NamespacedCloudProfile with a decreased limit")
				namespacedCloudProfilePatch := client.StrategicMergeFrom(namespacedCloudProfile.DeepCopy())
				namespacedCloudProfile.Spec.Limits = &gardencorev1beta1.Limits{
					MaxNodesTotal: ptr.To(int32(5)),
				}
				Eventually(func() error {
					return testClient.Patch(ctx, namespacedCloudProfile, namespacedCloudProfilePatch)
				}).Should(Succeed())
				waitForNamespacedCloudProfileToBeReconciled(ctx, namespacedCloudProfile)

				Expect(namespacedCloudProfile.Status.CloudProfileSpec.Limits.MaxNodesTotal).To(Equal(ptr.To(int32(5))))
			})

			It("should update the NamespacedCloudProfile status with an increased limit", func() {
				By("Set limit in parent cloud profile")
				cloudProfilePatch := client.MergeFrom(parentCloudProfile.DeepCopy())
				parentCloudProfile.Spec.Limits = &gardencorev1beta1.Limits{
					MaxNodesTotal: ptr.To(int32(10)),
				}
				Eventually(func() error {
					return testClient.Patch(ctx, parentCloudProfile, cloudProfilePatch)
				}).Should(Succeed())

				By("Patch the NamespacedCloudProfile with an increased limit")
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)).To(Succeed())
				namespacedCloudProfilePatch := client.StrategicMergeFrom(namespacedCloudProfile.DeepCopy())
				namespacedCloudProfile.Spec.Limits = &gardencorev1beta1.Limits{
					MaxNodesTotal: ptr.To(int32(24)),
				}
				Expect(testClient.Patch(ctx, namespacedCloudProfile, namespacedCloudProfilePatch)).To(Succeed())
			})

			It("should always override the limits value with the value provided in the NamespacedCloudProfile", func() {
				By("Set a limit in the NamespacedCloudProfile")
				namespacedCloudProfilePatch := client.StrategicMergeFrom(namespacedCloudProfile.DeepCopy())
				namespacedCloudProfile.Spec.Limits = &gardencorev1beta1.Limits{
					MaxNodesTotal: ptr.To(int32(10)),
				}
				Eventually(func() error {
					return testClient.Patch(ctx, namespacedCloudProfile, namespacedCloudProfilePatch)
				}).Should(Succeed())
				waitForNamespacedCloudProfileToBeReconciled(ctx, namespacedCloudProfile)

				Expect(namespacedCloudProfile.Status.CloudProfileSpec.Limits.MaxNodesTotal).To(Equal(ptr.To(int32(10))))

				By("Set a lower limit in parent cloud profile")
				cloudProfilePatch := client.MergeFrom(parentCloudProfile.DeepCopy())
				parentCloudProfile.Spec.Limits = &gardencorev1beta1.Limits{
					MaxNodesTotal: ptr.To(int32(9)),
				}
				Eventually(func() error {
					return testClient.Patch(ctx, parentCloudProfile, cloudProfilePatch)
				}).Should(Succeed())

				Consistently(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)).To(Succeed())
					g.Expect(namespacedCloudProfile.Status.CloudProfileSpec.Limits.MaxNodesTotal).To(Equal(ptr.To(int32(10))))
				}).Should(Succeed())

				By("Increase the limit in the parent cloud profile")
				cloudProfilePatch = client.StrategicMergeFrom(parentCloudProfile.DeepCopy())
				parentCloudProfile.Spec.Limits = &gardencorev1beta1.Limits{
					MaxNodesTotal: ptr.To(int32(100)),
				}
				Eventually(func() error {
					return testClient.Patch(ctx, parentCloudProfile, cloudProfilePatch)
				}).Should(Succeed())

				By("Wait for NamespacedCloudProfile status to be updated by the controller")
				Consistently(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)).To(Succeed())
					g.Expect(namespacedCloudProfile.Status.CloudProfileSpec.Limits.MaxNodesTotal).To(Equal(ptr.To(int32(10))))
				}).Should(Succeed())
			})

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
			Eventually(func() error {
				return testClient.Create(ctx, parentCloudProfile)
			}).Should(Succeed())
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
			waitForNamespacedCloudProfileToBeReconciled(ctx, namespacedCloudProfile)

			Expect(namespacedCloudProfile.Spec.Kubernetes.Versions).To(BeEmpty())
			Expect(namespacedCloudProfile.Status.CloudProfileSpec.Kubernetes.Versions).To(ContainElements(
				gardencorev1beta1.ExpirableVersion{Version: "1.2.3"},
				gardencorev1beta1.ExpirableVersion{Version: "1.3.0"},
			))
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
			waitForNamespacedCloudProfileToBeReconciled(ctx, namespacedCloudProfile)

			expectedMachineImages := []gardencorev1beta1.MachineImage{
				{
					Name: "some-image",
					Versions: []gardencorev1beta1.MachineImageVersion{
						{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "4.5.6"}, CRI: []gardencorev1beta1.CRI{{Name: "containerd"}}, Architectures: []string{"amd64"}},
					},
					UpdateStrategy: ptr.To(gardencorev1beta1.UpdateStrategyMajor),
				},
			}
			Expect(namespacedCloudProfile.Spec.MachineImages).To(BeEmpty())
			Expect(namespacedCloudProfile.Status.CloudProfileSpec.MachineImages).To(Equal(expectedMachineImages))
		})

		It("should not allow update with an already expired Kubernetes version", func() {
			Eventually(func() error {
				return testClient.Create(ctx, namespacedCloudProfile)
			}).Should(Succeed())
			waitForNamespacedCloudProfileToBeReconciled(ctx, namespacedCloudProfile)

			namespacedCloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{
				{Version: "1.2.3", ExpirationDate: &expirationDatePast},
			}
			Expect(testClient.Update(ctx, namespacedCloudProfile)).To(MatchError(
				ContainSubstring("expiration date for version \"1.2.3\" is in the past"),
			))
		})

		It("should not allow update with an already expired MachineImage version", func() {
			Eventually(func() error {
				return testClient.Create(ctx, namespacedCloudProfile)
			}).Should(Succeed())
			waitForNamespacedCloudProfileToBeReconciled(ctx, namespacedCloudProfile)

			namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
				{
					Name: "some-image",
					Versions: []gardencorev1beta1.MachineImageVersion{
						{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "4.5.6", ExpirationDate: &expirationDatePast}},
					},
				},
			}
			Expect(testClient.Update(ctx, namespacedCloudProfile)).To(MatchError(
				ContainSubstring("expiration date for version \"4.5.6\" is in the past"),
			))
		})
	})
})

func waitForNamespacedCloudProfileToBeReconciled(ctx context.Context, namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)).To(Succeed())
		g.Expect(namespacedCloudProfile.Status.ObservedGeneration).To(Equal(namespacedCloudProfile.Generation))
	}).Should(Succeed())
}

func withSortedArrays(nscpfl gardencorev1beta1.CloudProfileSpec) gardencorev1beta1.CloudProfileSpec {
	sort.Slice(nscpfl.MachineTypes, func(i, j int) bool {
		return strings.Compare(nscpfl.MachineTypes[i].Name, nscpfl.MachineTypes[j].Name) >= 0
	})
	sort.Slice(nscpfl.MachineImages, func(i, j int) bool {
		return strings.Compare(nscpfl.MachineImages[i].Name, nscpfl.MachineImages[j].Name) >= 0
	})
	for mi := range nscpfl.MachineImages {
		sort.Slice(nscpfl.MachineImages[mi].Versions, func(j, k int) bool {
			return strings.Compare(nscpfl.MachineImages[mi].Versions[j].Version, nscpfl.MachineImages[mi].Versions[k].Version) >= 0
		})
	}
	sort.Slice(nscpfl.Kubernetes.Versions, func(i, j int) bool {
		return strings.Compare(nscpfl.Kubernetes.Versions[i].Version, nscpfl.Kubernetes.Versions[j].Version) >= 0
	})
	return nscpfl
}
