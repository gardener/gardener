// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/shoot/maintenance/helper"
)

var _ = Describe("Helper Functions", func() {
	var (
		machineImage              *gardencorev1beta1.MachineImage
		machineType               *gardencorev1beta1.MachineType
		capabilityDefinitions     []gardencorev1beta1.CapabilityDefinition
		shootMachineImage         *gardencorev1beta1.ShootMachineImage
		worker                    gardencorev1beta1.Worker
		kubeletVersion            *semver.Version
		expirationDateInTheFuture metav1.Time
		expirationDateInThePast   metav1.Time
	)

	BeforeEach(func() {
		expirationDateInTheFuture = metav1.Time{Time: time.Now().Add(time.Hour * 24)}
		expirationDateInThePast = metav1.Time{Time: time.Now().Add(-time.Hour * 24)}
		kubeletVersion = semver.MustParse("1.30.0")

		machineType = &gardencorev1beta1.MachineType{
			Name: "Standard",
			Capabilities: gardencorev1beta1.Capabilities{
				"someCapability": []string{"supported"},
			},
		}
		capabilityDefinitions = []gardencorev1beta1.CapabilityDefinition{
			{Name: v1beta1constants.ArchitectureName, Values: []string{v1beta1constants.ArchitectureAMD64}},
			{Name: "someCapability", Values: []string{"supported", "unsupported"}},
		}
		machineImage = &gardencorev1beta1.MachineImage{
			Name: "CoreOS",
			Versions: []gardencorev1beta1.MachineImageVersion{
				{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{
						Version: "1.0.0",
					},
					CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
					Architectures: []string{"amd64"},
					Flavors: []gardencorev1beta1.MachineImageFlavor{{
						Capabilities: gardencorev1beta1.Capabilities{"someCapability": []string{"supported"}},
					}},
					KubeletVersionConstraint: ptr.To("< 1.27"),
				},
				{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{
						Version:        "1.1.0",
						ExpirationDate: &expirationDateInTheFuture,
					},
					CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
					Architectures: []string{"amd64"},
					Flavors: []gardencorev1beta1.MachineImageFlavor{{
						Capabilities: gardencorev1beta1.Capabilities{"someCapability": []string{"supported"}},
					}},
					KubeletVersionConstraint: ptr.To(">= 1.30.0"),
					InPlaceUpdates: &gardencorev1beta1.InPlaceUpdates{
						Supported:           true,
						MinVersionForUpdate: ptr.To("1.0.0"),
					},
				},
			},
		}

		worker = gardencorev1beta1.Worker{
			Machine: gardencorev1beta1.Machine{
				Architecture: ptr.To("amd64"),
				Image: &gardencorev1beta1.ShootMachineImage{
					Name:    "CoreOS",
					Version: ptr.To("1.0.0"),
				},
			},
			CRI:            &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRINameContainerD},
			UpdateStrategy: ptr.To(gardencorev1beta1.AutoInPlaceUpdate),
		}

		shootMachineImage = &gardencorev1beta1.ShootMachineImage{
			Name:    "CoreOS",
			Version: ptr.To("1.0.0"),
		}
	})

	Describe("#FilterMachineImageVersions", func() {
		It("should filter machine images which supports worker configuration", func() {
			filteredMachineImages := FilterMachineImageVersions(machineImage, worker, kubeletVersion, machineType, capabilityDefinitions)

			Expect(filteredMachineImages.Versions).ShouldNot(BeEmpty())
		})

		It("should filter machine images which supports worker configuration and include current version also for inplace updates", func() {
			machineImage.Versions[0].KubeletVersionConstraint = nil
			machineImage.Versions[0].InPlaceUpdates = &gardencorev1beta1.InPlaceUpdates{Supported: true}
			machineImage.Versions[1].KubeletVersionConstraint = nil
			filteredMachineImages := FilterMachineImageVersions(machineImage, worker, kubeletVersion, machineType, capabilityDefinitions)

			Expect(filteredMachineImages.Versions).Should(ContainElements(gardencorev1beta1.MachineImageVersion{
				ExpirableVersion: gardencorev1beta1.ExpirableVersion{
					Version:        "1.1.0",
					ExpirationDate: &expirationDateInTheFuture,
				},
				CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
				Architectures: []string{"amd64"},
				CapabilitySets: []gardencorev1beta1.CapabilitySet{{
					Capabilities: gardencorev1beta1.Capabilities{"someCapability": []string{"supported"}},
				}},
				InPlaceUpdates: &gardencorev1beta1.InPlaceUpdates{
					Supported:           true,
					MinVersionForUpdate: ptr.To("1.0.0"),
				},
			},
				gardencorev1beta1.MachineImageVersion{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{
						Version: "1.0.0",
					},
					CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
					Architectures: []string{"amd64"},
					CapabilitySets: []gardencorev1beta1.CapabilitySet{{
						Capabilities: gardencorev1beta1.Capabilities{"someCapability": []string{"supported"}},
					}},
					InPlaceUpdates: &gardencorev1beta1.InPlaceUpdates{Supported: true},
				},
			))
			Expect(filteredMachineImages.Versions).Should(HaveLen(2))
		})

		It("should return an empty machine image if no versions found with matching architecture", func() {
			worker.Machine.Architecture = ptr.To("arm64")
			filteredMachineImages := FilterMachineImageVersions(machineImage, worker, kubeletVersion, machineType, capabilityDefinitions)

			Expect(filteredMachineImages.Versions).Should(BeEmpty())
		})

		It("should return an empty machine image if no versions found with matching kubelet version", func() {
			worker.Machine.Image.Version = ptr.To("1.1.0")
			kubeletVersion = semver.MustParse("1.29.0")
			filteredMachineImages := FilterMachineImageVersions(machineImage, worker, kubeletVersion, machineType, capabilityDefinitions)

			Expect(filteredMachineImages.Versions).Should(BeEmpty())
		})

		It("should return an empty machine image if no versions found with matching CRI", func() {
			worker.CRI = &gardencorev1beta1.CRI{Name: "test-cri"}
			filteredMachineImages := FilterMachineImageVersions(machineImage, worker, kubeletVersion, machineType, capabilityDefinitions)

			Expect(filteredMachineImages.Versions).Should(BeEmpty())
		})

		It("should return an empty machine image if no versions found with in-place update constraint", func() {
			worker.Machine.Image.Version = ptr.To("0.1.0")
			worker.UpdateStrategy = ptr.To(gardencorev1beta1.AutoInPlaceUpdate)
			filteredMachineImages := FilterMachineImageVersions(machineImage, worker, kubeletVersion, machineType, capabilityDefinitions)

			Expect(filteredMachineImages.Versions).Should(BeEmpty())
		})

		It("should return an empty machine image if no versions found with supported capabilities", func() {
			machineType.Capabilities = gardencorev1beta1.Capabilities{"someCapability": []string{"unsupported"}}
			filteredMachineImages := FilterMachineImageVersions(machineImage, worker, kubeletVersion, machineType, capabilityDefinitions)

			Expect(filteredMachineImages.Versions).Should(BeEmpty())
		})
	})

	Describe("#DetermineMachineImage", func() {
		var cloudProfile *gardencorev1beta1.CloudProfile

		BeforeEach(func() {
			cloudProfile = &gardencorev1beta1.CloudProfile{
				Spec: gardencorev1beta1.CloudProfileSpec{
					MachineImages: []gardencorev1beta1.MachineImage{
						{
							Name: "CoreOS",
							Versions: []gardencorev1beta1.MachineImageVersion{
								{
									ExpirableVersion: gardencorev1beta1.ExpirableVersion{
										Version: "1.0.0",
									},
								},
								{
									ExpirableVersion: gardencorev1beta1.ExpirableVersion{
										Version: "1.1.0",
									},
								},
							},
						},
					},
				},
			}
		})

		It("should return an error if no machine image found", func() {
			shootMachineImage.Name = "NonExistentImage"
			_, err := DetermineMachineImage(cloudProfile, shootMachineImage)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failure while determining the default machine image in the CloudProfile"))
		})

		It("should determine the correct machine image from the CloudProfile", func() {
			machineImage, err := DetermineMachineImage(cloudProfile, shootMachineImage)

			Expect(err).NotTo(HaveOccurred())
			Expect(machineImage.Name).To(Equal("CoreOS"))
			Expect(machineImage.Versions).To(HaveLen(2))
		})
	})

	Describe("#DetermineMachineImageVersion", func() {
		It("should determine the correct machine image version for patch strategy", func() {
			machineImage.UpdateStrategy = ptr.To(gardencorev1beta1.UpdateStrategyPatch)
			version, err := DetermineMachineImageVersion(shootMachineImage, machineImage, false)

			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal(""))
		})

		It("should determine the correct machine image version for minor strategy", func() {
			machineImage.UpdateStrategy = ptr.To(gardencorev1beta1.UpdateStrategyMinor)
			version, err := DetermineMachineImageVersion(shootMachineImage, machineImage, false)

			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal("1.1.0"))
		})

		It("should determine the correct machine image version for major strategy", func() {
			machineImage.UpdateStrategy = ptr.To(gardencorev1beta1.UpdateStrategyMajor)
			version, err := DetermineMachineImageVersion(shootMachineImage, machineImage, false)

			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal("1.1.0"))
		})

		It("should return an error if the current version is expired and no force update version is available", func() {
			machineImage.UpdateStrategy = ptr.To(gardencorev1beta1.UpdateStrategyMajor)
			machineImage.Versions = []gardencorev1beta1.MachineImageVersion{
				{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{
						Version:        "1.0.0",
						ExpirationDate: &expirationDateInThePast,
					},
				},
			}

			version, err := DetermineMachineImageVersion(shootMachineImage, machineImage, true)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to determine the target version for maintenance"))
			Expect(version).To(BeEmpty())
		})

		It("should return an error if the machine image is reaching end of life", func() {
			machineImage.UpdateStrategy = ptr.To(gardencorev1beta1.UpdateStrategyMajor)
			machineImage.Versions = []gardencorev1beta1.MachineImageVersion{}

			version, err := DetermineMachineImageVersion(shootMachineImage, machineImage, true)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("either the machine image \"CoreOS\" is reaching end of life"))
			Expect(version).To(BeEmpty())
		})
	})

	Describe("#DetermineVersionForStrategy", func() {
		It("should return the latest version for major if higher qualifying version is found", func() {
			versions := []gardencorev1beta1.ExpirableVersion{
				{Version: "1.0.0"},
				{Version: "1.1.0"},
			}

			version, err := DetermineVersionForStrategy(
				versions,
				"1.0.0",
				func(
					_ []gardencorev1beta1.ExpirableVersion, _ string) (bool, string, error) {
					return true, "1.1.0", nil
				},
				func(
					_ []gardencorev1beta1.ExpirableVersion, _ string) (bool, string, error) {
					return false, "", nil
				},
				false,
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal("1.1.0"))
		})

		It("should return an empty string if the current version is already up-to-date and not expired", func() {
			versions := []gardencorev1beta1.ExpirableVersion{
				{Version: "1.0.0"},
			}

			version, err := DetermineVersionForStrategy(
				versions,
				"1.0.0",
				func(_ []gardencorev1beta1.ExpirableVersion, _ string) (bool, string, error) {
					return false, "", nil
				},
				func(_ []gardencorev1beta1.ExpirableVersion, _ string) (bool, string, error) {
					return false, "", nil
				},
				false,
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(BeEmpty())
		})

		It("should return the force update version if the current version is expired and force update is available", func() {
			versions := []gardencorev1beta1.ExpirableVersion{
				{Version: "1.0.0"},
				{Version: "1.1.0"},
			}

			version, err := DetermineVersionForStrategy(
				versions,
				"1.0.0",
				func(_ []gardencorev1beta1.ExpirableVersion, _ string) (bool, string, error) {
					return false, "", nil
				},
				func(_ []gardencorev1beta1.ExpirableVersion, _ string) (bool, string, error) {
					return true, "1.1.0", nil
				},
				true,
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal("1.1.0"))
		})

		It("should return an error if the current version is expired and no force update version is available", func() {
			versions := []gardencorev1beta1.ExpirableVersion{
				{Version: "1.0.0"},
			}

			version, err := DetermineVersionForStrategy(
				versions,
				"1.0.0",
				func(_ []gardencorev1beta1.ExpirableVersion, _ string) (bool, string, error) {
					return false, "", nil
				},
				func(_ []gardencorev1beta1.ExpirableVersion, _ string) (bool, string, error) {
					return false, "", fmt.Errorf("no suitable version found")
				},
				true,
			)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to determine version for forceful update: no suitable version found"))
			Expect(version).To(BeEmpty())
		})

		It("should return an error if automatic update fails", func() {
			versions := []gardencorev1beta1.ExpirableVersion{
				{Version: "1.0.0"},
			}

			version, err := DetermineVersionForStrategy(
				versions,
				"1.0.0",
				func(_ []gardencorev1beta1.ExpirableVersion, _ string) (bool, string, error) {
					return false, "", fmt.Errorf("failed to determine a higher patch version")
				},
				func(_ []gardencorev1beta1.ExpirableVersion, _ string) (bool, string, error) {
					return false, "", nil
				},
				false,
			)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to determine a higher patch version"))
			Expect(version).To(BeEmpty())
		})

		It("should return an error if force update fails", func() {
			versions := []gardencorev1beta1.ExpirableVersion{
				{Version: "1.0.0"},
			}

			version, err := DetermineVersionForStrategy(
				versions,
				"1.0.0",
				func(_ []gardencorev1beta1.ExpirableVersion, _ string) (bool, string, error) {
					return false, "", nil
				},
				func(_ []gardencorev1beta1.ExpirableVersion, _ string) (bool, string, error) {
					return false, "", fmt.Errorf("failed to determine version for forceful update")
				},
				true,
			)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to determine version for forceful update"))
			Expect(version).To(BeEmpty())
		})
	})
})
