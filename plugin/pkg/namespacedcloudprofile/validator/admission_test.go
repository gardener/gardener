// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/ptr"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	. "github.com/gardener/gardener/plugin/pkg/namespacedcloudprofile/validator"
)

var _ = DescribeTableSubtree("ParentCloudProfile has CapabilitiesDefinition", func(useCapabilitiesDefinition bool) {

	Describe("Admission", func() {
		Describe("#Validate", func() {
			var (
				ctx                 context.Context
				admissionHandler    *ValidateNamespacedCloudProfile
				coreInformerFactory gardencoreinformers.SharedInformerFactory

				parentCloudProfile           *gardencorev1beta1.CloudProfile
				namespacedCloudProfileParent gardencore.CloudProfileReference
				namespacedCloudProfile       *gardencore.NamespacedCloudProfile

				machineType             gardencorev1beta1.MachineType
				machineTypeCore         gardencore.MachineType
				imageName               = "test-image"
				expiredExpirationDate   *metav1.Time
				validExpirationDate     *metav1.Time
				capabilitiesDefinition  gardencorev1beta1.Capabilities
				machineCapabilitiesCore gardencore.Capabilities
				machineCapabilities     gardencorev1beta1.Capabilities
				capabilitiesSet         []apiextensionsv1.JSON
				machineArchitecture     string
				imageArchitectures      []string
			)

			BeforeEach(func() {
				ctx = context.Background()

				if useCapabilitiesDefinition {
					capabilitiesDefinition = gardencorev1beta1.Capabilities{
						v1beta1constants.ArchitectureKey: {
							Values: []string{"amd64"},
						},
						"hypervisorType": {
							Values: []string{"gen1", "gen2"},
						},
					}
					machineCapabilitiesCore = gardencore.Capabilities{
						v1beta1constants.ArchitectureKey: {
							Values: []string{"amd64"},
						},
					}
					machineCapabilities = gardencorev1beta1.Capabilities{
						v1beta1constants.ArchitectureKey: {
							Values: []string{"amd64"},
						},
					}
					capabilitiesSet = []apiextensionsv1.JSON{{Raw: []byte(`{"` + v1beta1constants.ArchitectureKey + `":"amd64", "hypervisorType":"gen1"}`)}}
				} else {
					machineArchitecture = "amd64"
					imageArchitectures = []string{"amd64"}
				}

				parentCloudProfile = &gardencorev1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "parent-profile",
					},
					Spec: gardencorev1beta1.CloudProfileSpec{
						CapabilitiesDefinition: capabilitiesDefinition,
						Kubernetes: gardencorev1beta1.KubernetesSettings{Versions: []gardencorev1beta1.ExpirableVersion{
							{Version: "1.30.0", Classification: ptr.To(gardencorev1beta1.ClassificationPreview)},
							{Version: "1.29.0", Classification: ptr.To(gardencorev1beta1.ClassificationSupported)},
							{Version: "1.28.0"},
						}},
						MachineImages: []gardencorev1beta1.MachineImage{
							{
								Name: imageName,
								Versions: []gardencorev1beta1.MachineImageVersion{{
									ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.0.0"},
									CRI:              []gardencorev1beta1.CRI{{Name: "containerd"}},
									Architectures:    imageArchitectures,
									CapabilitiesSet:  capabilitiesSet,
								}},
							},
						},
					},
				}
				gardencorev1beta1.SetObjectDefaults_CloudProfile(parentCloudProfile)
				namespacedCloudProfileParent = gardencore.CloudProfileReference{
					Kind: "CloudProfile",
					Name: parentCloudProfile.Name,
				}
				namespacedCloudProfile = &gardencore.NamespacedCloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "profile",
					},
					Spec: gardencore.NamespacedCloudProfileSpec{
						Parent: namespacedCloudProfileParent,
					},
				}

				machineType = gardencorev1beta1.MachineType{
					Name:         "my-machine",
					Architecture: &machineArchitecture,
					Capabilities: machineCapabilities,
				}
				machineTypeCore = gardencore.MachineType{
					Name:         "my-machine",
					Architecture: &machineArchitecture,
					Capabilities: machineCapabilitiesCore,
				}

				admissionHandler, _ = New()
				admissionHandler.AssignReadyFunc(func() bool { return true })
				coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
				admissionHandler.SetCoreInformerFactory(coreInformerFactory)

				expiredExpirationDate = &metav1.Time{Time: time.Now().Add(-24 * time.Hour)}
				validExpirationDate = &metav1.Time{Time: time.Now().Add(24 * time.Hour)}
			})

			Describe("parent", func() {
				It("should not allow creating a NamespacedCloudProfile with an invalid parent reference", func() {
					namespacedCloudProfile.Spec.Parent = gardencore.CloudProfileReference{Kind: "CloudProfile", Name: "idontexist"}

					attrs := admission.NewAttributesRecord(namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(ContainSubstring("parent CloudProfile could not be found")))
				})

				It("should allow creating a NamespacedCloudProfile with a valid parent reference", func() {
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())

					attrs := admission.NewAttributesRecord(namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})
			})

			Describe("Kubernetes versions", func() {
				It("should not allow creating a (Namespaced)CloudProfile if the resulting Kubernetes versions are empty", func() {
					parentCloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{}
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())

					attrs := admission.NewAttributesRecord(namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(ContainSubstring("must provide at least one Kubernetes version")))
				})

				It("should fail if the latest Kubernetes version has an expiration date", func() {
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())

					namespacedCloudProfile.Spec.Kubernetes = &gardencore.KubernetesSettings{Versions: []gardencore.ExpirableVersion{
						{Version: "1.30.0", ExpirationDate: validExpirationDate},
					}}

					attrs := admission.NewAttributesRecord(namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(ContainSubstring("expiration date of latest kubernetes version ('1.30.0') must not be set")))
				})

				It("should allow creating a NamespacedCloudProfile that specifies a Kubernetes version from the parent CloudProfile and extends the expiration date", func() {
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())

					namespacedCloudProfile.Spec.Kubernetes = &gardencore.KubernetesSettings{Versions: []gardencore.ExpirableVersion{
						{Version: "1.29.0", ExpirationDate: validExpirationDate},
					}}

					attrs := admission.NewAttributesRecord(namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})

				It("should fail for creating a NamespacedCloudProfile that specifies a Kubernetes version not in the parent CloudProfile", func() {
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())

					namespacedCloudProfile.Spec.Kubernetes = &gardencore.KubernetesSettings{Versions: []gardencore.ExpirableVersion{
						{Version: "1.25.0", ExpirationDate: validExpirationDate},
					}}

					attrs := admission.NewAttributesRecord(namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(ContainSubstring("invalid kubernetes version specified: '1.25.0' does not exist in parent")))
				})

				It("should fail for creating a NamespacedCloudProfile that specifies a Kubernetes version without an expiration date", func() {
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())

					namespacedCloudProfile.Spec.Kubernetes = &gardencore.KubernetesSettings{Versions: []gardencore.ExpirableVersion{
						{Version: "1.29.0"},
					}}

					attrs := admission.NewAttributesRecord(namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(ContainSubstring("specified version '1.29.0' does not set expiration date")))
				})

				It("should allow creation with past expiration dates", func() {
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())

					namespacedCloudProfile.Spec.Kubernetes = &gardencore.KubernetesSettings{Versions: []gardencore.ExpirableVersion{
						{Version: "1.29.0", ExpirationDate: expiredExpirationDate},
					}}

					attrs := admission.NewAttributesRecord(namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})

				It("should allow updates to a NamespacedCloudProfile even if one unchanged overridden Kubernetes version is already expired", func() {
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())

					namespacedCloudProfile.Spec.Kubernetes = &gardencore.KubernetesSettings{Versions: []gardencore.ExpirableVersion{
						{Version: "1.28.0", ExpirationDate: expiredExpirationDate},
					}}
					updatedNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()

					namespacedCloudProfile.Status.CloudProfileSpec.Kubernetes.Versions = []gardencore.ExpirableVersion{
						{Version: "1.28.0", ExpirationDate: expiredExpirationDate},
						{Version: "1.29.0"},
						{Version: "1.30.0"},
					}
					updatedNamespacedCloudProfile.Spec.Kubernetes = &gardencore.KubernetesSettings{Versions: []gardencore.ExpirableVersion{
						{Version: "1.28.0", ExpirationDate: expiredExpirationDate},
						{Version: "1.29.0", ExpirationDate: validExpirationDate},
					}}

					attrs := admission.NewAttributesRecord(updatedNamespacedCloudProfile, namespacedCloudProfile, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Update, &metav1.CreateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})

				It("should fail for updating a expiration date to a still invalid value", func() {
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())

					namespacedCloudProfile.Spec.Kubernetes = &gardencore.KubernetesSettings{Versions: []gardencore.ExpirableVersion{
						{Version: "1.28.0", ExpirationDate: expiredExpirationDate},
					}}
					updatedNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()

					namespacedCloudProfile.Status.CloudProfileSpec.Kubernetes.Versions = []gardencore.ExpirableVersion{
						{Version: "1.28.0", ExpirationDate: expiredExpirationDate}, {Version: "1.29.0"}, {Version: "1.30.0"},
					}
					stillExpiredDate := &metav1.Time{Time: time.Now().Add(-30 * time.Minute)}
					updatedNamespacedCloudProfile.Spec.Kubernetes = &gardencore.KubernetesSettings{Versions: []gardencore.ExpirableVersion{
						{Version: "1.28.0", ExpirationDate: stillExpiredDate},
					}}

					attrs := admission.NewAttributesRecord(updatedNamespacedCloudProfile, namespacedCloudProfile, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Update, &metav1.CreateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(ContainSubstring("expiration date for version \"1.28.0\" is in the past")))
				})
			})

			Describe("machineType", func() {
				It("should not allow creating a NamespacedCloudProfile that defines a machineType of the parent CloudProfile", func() {
					parentCloudProfile.Spec.MachineTypes = []gardencorev1beta1.MachineType{machineType}
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())

					namespacedCloudProfile.Spec.MachineTypes = []gardencore.MachineType{machineTypeCore}

					attrs := admission.NewAttributesRecord(namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(And(
						ContainSubstring("NamespacedCloudProfile attempts to overwrite parent CloudProfile with machineType"),
						ContainSubstring("my-machine"),
					)))
				})

				It("should allow creating a NamespacedCloudProfile that defines a machineType of the parent CloudProfile if it was added to the NamespacedCloudProfile first", func() {
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())

					namespacedCloudProfile.Spec.MachineTypes = []gardencore.MachineType{machineTypeCore}

					attrs := admission.NewAttributesRecord(namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())

					parentCloudProfile.Spec.MachineTypes = []gardencorev1beta1.MachineType{machineType}

					attrs = admission.NewAttributesRecord(namespacedCloudProfile, namespacedCloudProfile, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Update, &metav1.CreateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})

				It("should allow creating a NamespacedCloudProfile that defines a machineType of the parent CloudProfile if it was added to the NamespacedCloudProfile first but is changed and the parent changes", func() {
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())

					namespacedCloudProfile.Spec.MachineTypes = []gardencore.MachineType{machineTypeCore}

					attrs := admission.NewAttributesRecord(namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())

					oldNamespacedCloudProfile := *namespacedCloudProfile.DeepCopy()
					machineType.Usable = ptr.To(false)
					parentCloudProfile.Spec.MachineTypes = []gardencorev1beta1.MachineType{machineType}

					attrs = admission.NewAttributesRecord(namespacedCloudProfile, &oldNamespacedCloudProfile, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Update, &metav1.CreateOptions{}, false, nil)
					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})

				It("should allow creating a NamespacedCloudProfile that defines a different machineType than the parent CloudProfile", func() {
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())
					// Relevant checks are only executed if parentCloudProfile hast a machineType
					parentCloudProfile.Spec.MachineTypes = []gardencorev1beta1.MachineType{machineType}

					namespacedCloudProfile.Spec.MachineTypes = []gardencore.MachineType{{Name: "my-other-machine", Architecture: &machineArchitecture, Capabilities: machineCapabilitiesCore}}

					attrs := admission.NewAttributesRecord(namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})
			})

			Describe("volumeType", func() {
				It("should allow a NamespacedCloudProfile to specify a VolumeType, if it has been added to the parent CloudProfile only afterwards", func() {
					volumeName, volumeClass := "volume-type-1", "super-premium"
					parentCloudProfile.Spec.VolumeTypes = []gardencorev1beta1.VolumeType{{Name: volumeName, Class: volumeClass}}
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())

					namespacedCloudProfile.Spec.VolumeTypes = []gardencore.VolumeType{{Name: volumeName, Class: volumeClass}}
					oldNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()
					namespacedCloudProfile.Generation++ // Increase generation to trigger full validation check.

					attrs := admission.NewAttributesRecord(namespacedCloudProfile, oldNamespacedCloudProfile, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})
			})

			Describe("machineImages", func() {
				It("should allow creating a NamespacedCloudProfile that specifies a MachineImage version from the parent CloudProfile, overriding the updateStrategy and the expiration date", func() {
					parentCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
						{Name: imageName, Versions: []gardencorev1beta1.MachineImageVersion{{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.0.0"}, CRI: []gardencorev1beta1.CRI{{Name: "containerd"}}}}},
					}
					gardencorev1beta1.SetObjectDefaults_CloudProfile(parentCloudProfile)
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())

					namespacedCloudProfile.Spec.MachineImages = []gardencore.MachineImage{
						{
							Name:           imageName,
							UpdateStrategy: ptr.To(gardencore.UpdateStrategyPatch),
							Versions:       []gardencore.MachineImageVersion{{ExpirableVersion: gardencore.ExpirableVersion{Version: "1.0.0", ExpirationDate: ptr.To(metav1.Now())}}},
						}}

					attrs := admission.NewAttributesRecord(namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})

				It("should allow creating a NamespacedCloudProfile that specifies a new MachineImage entry not in the parent CloudProfile", func() {
					parentCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
						{Name: imageName, Versions: []gardencorev1beta1.MachineImageVersion{{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.0.0"}, CRI: []gardencorev1beta1.CRI{{Name: "containerd"}}}}},
					}
					gardencorev1beta1.SetObjectDefaults_CloudProfile(parentCloudProfile)
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())

					namespacedCloudProfile.Spec.MachineImages = []gardencore.MachineImage{
						{
							Name: "another-image",
							Versions: []gardencore.MachineImageVersion{
								{ExpirableVersion: gardencore.ExpirableVersion{Version: "1.0.0", ExpirationDate: ptr.To(metav1.Now())}, CRI: []gardencore.CRI{{Name: "containerd"}}, Architectures: imageArchitectures, CapabilitiesSet: capabilitiesSet},
							},
							UpdateStrategy: ptr.To(gardencore.UpdateStrategyMajor),
						},
					}

					attrs := admission.NewAttributesRecord(namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})

				It("should fail creating a NamespacedCloudProfile that specifies a new MachineImage entry not in the parent CloudProfile without image versions", func() {
					parentCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
						{Name: imageName, Versions: []gardencorev1beta1.MachineImageVersion{{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.0.0"}, CRI: []gardencorev1beta1.CRI{{Name: "containerd"}}}}},
					}
					gardencorev1beta1.SetObjectDefaults_CloudProfile(parentCloudProfile)
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())

					namespacedCloudProfile.Spec.MachineImages = []gardencore.MachineImage{
						{
							Name:           "another-image",
							Versions:       []gardencore.MachineImageVersion{},
							UpdateStrategy: ptr.To(gardencore.UpdateStrategyMajor),
						},
					}

					attrs := admission.NewAttributesRecord(namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(ContainSubstring("must provide at least one version for the machine image 'another-image'")))
				})

				It("should succeed for creating a NamespacedCloudProfile that specifies a new version to an existing MachineImage from the parent CloudProfile", func() {
					parentCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
						{Name: imageName, Versions: []gardencorev1beta1.MachineImageVersion{{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.0.0"}, CRI: []gardencorev1beta1.CRI{{Name: "containerd"}}}}},
					}
					gardencorev1beta1.SetObjectDefaults_CloudProfile(parentCloudProfile)
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())

					machineImage := gardencore.MachineImage{Name: imageName, Versions: []gardencore.MachineImageVersion{{ExpirableVersion: gardencore.ExpirableVersion{Version: "1.2.0", ExpirationDate: ptr.To(metav1.Now())}, CRI: []gardencore.CRI{{Name: "containerd"}}, Architectures: imageArchitectures, CapabilitiesSet: capabilitiesSet}}}

					namespacedCloudProfile.Spec.MachineImages = []gardencore.MachineImage{machineImage}

					attrs := admission.NewAttributesRecord(namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})

				It("should fail for creating a NamespacedCloudProfile that overrides an existing MachineImage version without specifying an expiration date", func() {
					parentCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
						{Name: imageName, Versions: []gardencorev1beta1.MachineImageVersion{
							{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.2.0"}, CRI: []gardencorev1beta1.CRI{{Name: "containerd"}}},
							{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.0.0"}, CRI: []gardencorev1beta1.CRI{{Name: "containerd"}}},
						}},
					}
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())

					namespacedCloudProfile.Spec.MachineImages = []gardencore.MachineImage{
						{Name: imageName, Versions: []gardencore.MachineImageVersion{{ExpirableVersion: gardencore.ExpirableVersion{Version: "1.0.0"}}}},
					}

					attrs := admission.NewAttributesRecord(namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(ContainSubstring("expiration date for version \"1.0.0\" must be set")))
				})

				It("should fail for updating a NamespacedCloudProfile that specifies an already expired MachineImage version", func() {
					parentCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
						{Name: imageName, Versions: []gardencorev1beta1.MachineImageVersion{
							{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.1.0"}, CRI: []gardencorev1beta1.CRI{{Name: "containerd"}}},
							{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.2.0"}, CRI: []gardencorev1beta1.CRI{{Name: "containerd"}}},
						}},
					}
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())

					oldNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()
					namespacedCloudProfile.Spec.MachineImages = []gardencore.MachineImage{
						{Name: imageName, Versions: []gardencore.MachineImageVersion{
							{ExpirableVersion: gardencore.ExpirableVersion{Version: "1.1.0", ExpirationDate: expiredExpirationDate}},
						}},
					}

					attrs := admission.NewAttributesRecord(namespacedCloudProfile, oldNamespacedCloudProfile, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(ContainSubstring("expiration date for version \"1.1.0\" is in the past")))
				})

				It("should allow creating a NamespacedCloudProfile that specifies an already expired MachineImage version", func() {
					parentCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
						{Name: imageName, Versions: []gardencorev1beta1.MachineImageVersion{
							{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.1.0"}, CRI: []gardencorev1beta1.CRI{{Name: "containerd"}}},
							{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.2.0"}, CRI: []gardencorev1beta1.CRI{{Name: "containerd"}}},
						}},
					}
					gardencorev1beta1.SetObjectDefaults_CloudProfile(parentCloudProfile)
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())

					namespacedCloudProfile.Spec.MachineImages = []gardencore.MachineImage{
						{Name: imageName, Versions: []gardencore.MachineImageVersion{
							{ExpirableVersion: gardencore.ExpirableVersion{Version: "1.1.0", ExpirationDate: expiredExpirationDate}},
						}},
					}

					attrs := admission.NewAttributesRecord(namespacedCloudProfile, nil, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})

				It("should allow a NamespacedCloudProfile to specify a MachineImage, if it has been added to the parent CloudProfile only afterwards", func() {
					parentCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
						{Name: imageName, Versions: []gardencorev1beta1.MachineImageVersion{
							{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.1.0"}, CRI: []gardencorev1beta1.CRI{{Name: "containerd"}}},
						}},
					}
					gardencorev1beta1.SetObjectDefaults_CloudProfile(parentCloudProfile)
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())

					namespacedCloudProfile.Spec.MachineImages = []gardencore.MachineImage{
						{Name: imageName, Versions: []gardencore.MachineImageVersion{
							{ExpirableVersion: gardencore.ExpirableVersion{Version: "1.1.0"}, CRI: []gardencore.CRI{{Name: "containerd"}}, Architectures: imageArchitectures, CapabilitiesSet: capabilitiesSet},
						}},
					}
					oldNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()
					namespacedCloudProfile.Generation++ // Increase generation to trigger full validation check.

					attrs := admission.NewAttributesRecord(namespacedCloudProfile, oldNamespacedCloudProfile, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})

				It("should not allow any changes to a MachineImage in a NamespacedCloudProfile, if it has been added to the parent CloudProfile in the meantime", func() {
					parentCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
						{Name: imageName, Versions: []gardencorev1beta1.MachineImageVersion{
							{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.1.0"}, CRI: []gardencorev1beta1.CRI{{Name: "containerd"}}},
						}},
					}
					gardencorev1beta1.SetObjectDefaults_CloudProfile(parentCloudProfile)
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())

					namespacedCloudProfile.Spec.MachineImages = []gardencore.MachineImage{
						{Name: imageName, Versions: []gardencore.MachineImageVersion{
							{ExpirableVersion: gardencore.ExpirableVersion{Version: "1.1.0"}, CRI: []gardencore.CRI{{Name: "containerd"}}},
						}},
					}
					oldNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()
					namespacedCloudProfile.Spec.MachineImages[0].UpdateStrategy = ptr.To(gardencore.UpdateStrategyMajor)
					namespacedCloudProfile.Spec.MachineImages[0].Versions[0].Architectures = imageArchitectures
					namespacedCloudProfile.Spec.MachineImages[0].Versions[0].CapabilitiesSet = capabilitiesSet

					attrs := admission.NewAttributesRecord(namespacedCloudProfile, oldNamespacedCloudProfile, gardencorev1beta1.Kind("NamespacedCloudProfile").WithVersion("version"), "", namespacedCloudProfile.Name, gardencorev1beta1.Resource("namespacedcloudprofile").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.machineImages[0].updateStrategy"),
						"Detail": ContainSubstring("cannot update the machine image update strategy of \"test-image\", as this version has been added to the parent CloudProfile by now"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.machineImages[0].versions[0]"),
						"Detail": ContainSubstring("cannot update the machine image version spec of \"test-image@1.1.0\", as this version has been added to the parent CloudProfile by now"),
					}))))
				})
			})

			Context("simulated NamespacedCloudProfile status", func() {
				var (
					parentCloudProfileName     string
					namespacedCloudProfileName string
					namespaceName              string

					parentCloudProfile     *gardencorev1beta1.CloudProfile
					namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile

					machineTypesConstraint []gardencorev1beta1.MachineType
					volumeTypesConstraint  []gardencorev1beta1.VolumeType

					supportedClassification  = gardencorev1beta1.ClassificationSupported
					previewClassification    = gardencorev1beta1.ClassificationPreview
					deprecatedClassification = gardencorev1beta1.ClassificationDeprecated
				)

				BeforeEach(func() {
					parentCloudProfileName = "cloudprofile1"
					namespacedCloudProfileName = "namespaced-profile"
					namespaceName = "garden-test"

					machineType = gardencorev1beta1.MachineType{
						Name:         "machine-type-1",
						CPU:          resource.MustParse("2"),
						GPU:          resource.MustParse("0"),
						Memory:       resource.MustParse("100Gi"),
						Architecture: &machineArchitecture,
						Capabilities: machineCapabilities,
					}
					machineTypesConstraint = []gardencorev1beta1.MachineType{
						machineType,
					}
					volumeTypesConstraint = []gardencorev1beta1.VolumeType{
						{
							Name:  "volume-type-1",
							Class: "super-premium",
						},
					}

					parentCloudProfile = &gardencorev1beta1.CloudProfile{
						ObjectMeta: metav1.ObjectMeta{
							Name: parentCloudProfileName,
						},
						Spec: gardencorev1beta1.CloudProfileSpec{
							Kubernetes: gardencorev1beta1.KubernetesSettings{
								Versions: []gardencorev1beta1.ExpirableVersion{{Version: "1.11.4"}},
							},
							CapabilitiesDefinition: capabilitiesDefinition,
							MachineImages: []gardencorev1beta1.MachineImage{
								{
									Name: imageName,
									Versions: []gardencorev1beta1.MachineImageVersion{
										{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.0.0"}},
										{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.1.2"}},
									},
								},
							},
						},
					}
					gardencorev1beta1.SetObjectDefaults_CloudProfile(parentCloudProfile)

					namespacedCloudProfile = &gardencorev1beta1.NamespacedCloudProfile{
						ObjectMeta: metav1.ObjectMeta{
							Name:      namespacedCloudProfileName,
							Namespace: namespaceName,
						},
						Spec: gardencorev1beta1.NamespacedCloudProfileSpec{
							Parent: gardencorev1beta1.CloudProfileReference{
								Kind: "CloudProfile",
								Name: parentCloudProfileName,
							},
							Kubernetes: &gardencorev1beta1.KubernetesSettings{
								Versions: []gardencorev1beta1.ExpirableVersion{{
									Version: "1.11.4",
								}},
							},
							MachineImages: []gardencorev1beta1.MachineImage{
								{
									Name: imageName,
									Versions: []gardencorev1beta1.MachineImageVersion{
										{
											ExpirableVersion: gardencorev1beta1.ExpirableVersion{
												Version: "1.0.0",
											},
										},
									},
								},
							},
							MachineTypes: machineTypesConstraint,
							VolumeTypes:  volumeTypesConstraint,
						},
					}
				})

				It("should not return any errors", func() {
					errorList := ValidateSimulatedNamespacedCloudProfileStatus(parentCloudProfile, namespacedCloudProfile)

					Expect(errorList).To(BeEmpty())
				})

				Context("kubernetes version constraints", func() {
					It("should enforce that at least one version has been defined", func() {
						parentCloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{}
						namespacedCloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{}

						errorList := ValidateSimulatedNamespacedCloudProfileStatus(parentCloudProfile, namespacedCloudProfile)

						Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal("status.cloudProfileSpec.kubernetes.versions"),
						})), PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("status.cloudProfileSpec.kubernetes.versions"),
						}))))
					})

					It("should forbid expiration date on latest kubernetes version", func() {
						expirationDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
						parentCloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{
							{
								Version:        "1.1.0",
								Classification: &supportedClassification,
							},
							{
								Version:        "1.2.0",
								Classification: &deprecatedClassification,
							},
						}
						namespacedCloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{
							{
								Version:        "1.1.0",
								Classification: &supportedClassification,
							},
							{
								Version:        "1.2.0",
								Classification: &deprecatedClassification,
								ExpirationDate: expirationDate,
							},
						}

						errorList := ValidateSimulatedNamespacedCloudProfileStatus(parentCloudProfile, namespacedCloudProfile)

						Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("status.cloudProfileSpec.kubernetes.versions[].expirationDate"),
						}))))
					})

					It("only allow one supported version per minor version", func() {
						parentCloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{
							{
								Version:        "1.1.0",
								Classification: &supportedClassification,
							},
							{
								Version:        "1.1.1",
								Classification: &supportedClassification,
							},
						}
						namespacedCloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{
							{
								Version:        "1.1.0",
								Classification: &supportedClassification,
							},
							{
								Version:        "1.1.1",
								Classification: &supportedClassification,
							},
						}
						errorList := ValidateSimulatedNamespacedCloudProfileStatus(parentCloudProfile, namespacedCloudProfile)

						Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("status.cloudProfileSpec.kubernetes.versions[1]"),
						})), PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("status.cloudProfileSpec.kubernetes.versions[0]"),
						}))))
					})
				})

				Context("machine image validation", func() {
					It("should allow an empty list of machine images", func() {
						namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{}

						errorList := ValidateSimulatedNamespacedCloudProfileStatus(parentCloudProfile, namespacedCloudProfile)

						Expect(errorList).To(BeEmpty())
					})

					It("should allow expiration date on latest machine image version within NamespacedCloudProfile spec", func() {
						expirationDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
						namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
							{
								Name: imageName,
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version:        "0.1.2",
											ExpirationDate: expirationDate,
											Classification: &previewClassification,
										},
										CRI:             []gardencorev1beta1.CRI{{Name: "containerd"}},
										Architectures:   imageArchitectures,
										CapabilitiesSet: capabilitiesSet,
									},
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version: "1.0.0",
										},
									},
								},
							},
							{
								Name: "xy",
								Versions: []gardencorev1beta1.MachineImageVersion{
									{
										ExpirableVersion: gardencorev1beta1.ExpirableVersion{
											Version:        "0.1.1",
											ExpirationDate: expirationDate,
											Classification: &supportedClassification,
										},
										CRI:           []gardencorev1beta1.CRI{{Name: "containerd"}},
										Architectures: imageArchitectures,
									},
								},
								UpdateStrategy: ptr.To(gardencorev1beta1.UpdateStrategyMajor),
							},
						}

						errorList := ValidateSimulatedNamespacedCloudProfileStatus(parentCloudProfile, namespacedCloudProfile)
						Expect(errorList).To(BeEmpty())
					})
				})

				Context("machine types validation", func() {
					It("should allow an empty machine type list", func() {
						namespacedCloudProfile.Spec.MachineTypes = []gardencorev1beta1.MachineType{}

						errorList := ValidateSimulatedNamespacedCloudProfileStatus(parentCloudProfile, namespacedCloudProfile)

						Expect(errorList).To(BeEmpty())
					})
				})
			})
		})

		Describe("#Register", func() {
			It("should register the plugin", func() {
				plugins := admission.NewPlugins()
				Register(plugins)

				registered := plugins.Registered()
				Expect(registered).To(HaveLen(1))
				Expect(registered).To(ContainElement("NamespacedCloudProfileValidator"))
			})
		})

		Describe("#New", func() {
			It("should only handle CREATE and UPDATE operations", func() {
				dr, err := New()
				Expect(err).ToNot(HaveOccurred())
				Expect(dr.Handles(admission.Create)).To(BeTrue())
				Expect(dr.Handles(admission.Update)).To(BeTrue())
				Expect(dr.Handles(admission.Connect)).To(BeFalse())
				Expect(dr.Handles(admission.Delete)).To(BeFalse())
			})
		})
	})
},
	Entry("CapabilitiesDefinition is used", true),
	Entry("CapabilitiesDefinition is NOT used", false),
)
