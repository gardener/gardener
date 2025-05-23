// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package webhook_test

import (
	"github.com/coreos/go-systemd/v22/unit"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/extensions/pkg/webhook"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

var _ = Describe("Utils", func() {
	Describe("#Ensure*", func() {
		Describe("#EnsureStringWithPrefix", func() {
			var flags []string

			BeforeEach(func() {
				flags = []string{
					"--flag1=key2=value2,key3=value3",
					"--flag2=value1",
					"--flag3=value3,value1",
					"--flag1=value4",
				}
			})

			It("should replace all strings with a given prefix with prefix+value", func() {
				result := webhook.EnsureStringWithPrefix(flags, "--flag1=", "key1=value1")
				Expect(result).To(Equal([]string{
					"--flag1=key1=value1",
					"--flag2=value1",
					"--flag3=value3,value1",
					"--flag1=key1=value1",
				}))
			})

			It("should add prefix+value if there is no string with a given prefix", func() {
				result := webhook.EnsureStringWithPrefix(flags, "--flag4=", "key1=value1")
				Expect(result).To(Equal(append(flags, "--flag4=key1=value1")))
			})
		})

		Describe("#EnsureStringWithPrefixContains", func() {
			var flags []string

			BeforeEach(func() {
				flags = []string{
					"--flag1=key1=value1,key2=value2",
					"--flag2=value1",
					"--flag3=value3,value1",
					"--flag1=key1=value1",
				}
			})

			It("should ensure the specified value is in all strings with a given prefix", func() {
				result := webhook.EnsureStringWithPrefixContains(flags, "--flag1=", "key2=value2", ",")
				Expect(result).To(Equal([]string{
					"--flag1=key1=value1,key2=value2",
					"--flag2=value1",
					"--flag3=value3,value1",
					"--flag1=key1=value1,key2=value2",
				}))
			})

			It("should add prefix+value if there is no string with a given prefix", func() {
				result := webhook.EnsureStringWithPrefixContains(flags, "--flag4=", "value4", ",")
				Expect(result).To(Equal(append(flags, "--flag4=value4")))
			})
		})

		Describe("#EnsureEnvVarWithName", func() {
			var envVars []corev1.EnvVar

			BeforeEach(func() {
				envVars = []corev1.EnvVar{
					{
						Name:  "envVar1",
						Value: "value1",
					},
					{
						Name:  "envVar2",
						Value: "value2",
					},
				}
			})

			It("should add a new EnvVar if not present", func() {
				newEnvVar := corev1.EnvVar{
					Name:  "envVar3",
					Value: "value3",
				}
				result := webhook.EnsureEnvVarWithName(envVars, newEnvVar)
				Expect(result).To(Equal(append(envVars, newEnvVar)))
			})

			It("should replace the existing EnvVar if it's not identical", func() {
				existingEnvVar := corev1.EnvVar{
					Name:  "envVar1",
					Value: "value3",
				}
				result := webhook.EnsureEnvVarWithName(envVars, existingEnvVar)
				Expect(result).To(Equal([]corev1.EnvVar{
					{
						Name:  "envVar1",
						Value: "value3",
					},
					{
						Name:  "envVar2",
						Value: "value2",
					},
				}))
			})

			It("should do nothing to the existing EnvVar if it's identical", func() {
				identicalEnvVar := envVars[0]
				result := webhook.EnsureEnvVarWithName(envVars, identicalEnvVar)
				Expect(result).To(Equal(envVars))
			})
		})

		Describe("#EnsureVolumeMountWithName", func() {
			var volumeMounts []corev1.VolumeMount

			BeforeEach(func() {
				volumeMounts = []corev1.VolumeMount{
					{
						Name:     "volumeMount1",
						ReadOnly: true,
					},
					{
						Name:     "volumeMount2",
						ReadOnly: false,
					},
				}
			})

			It("should add a new VolumeMount if not present", func() {
				newVolumeMount := corev1.VolumeMount{
					Name:     "volumeMount3",
					ReadOnly: true,
				}
				result := webhook.EnsureVolumeMountWithName(volumeMounts, newVolumeMount)
				Expect(result).To(Equal(append(volumeMounts, newVolumeMount)))
			})

			It("should replace the existing VolumeMount if it's not identical", func() {
				existingVolumeMount := corev1.VolumeMount{
					Name:     "volumeMount1",
					ReadOnly: false,
				}
				result := webhook.EnsureVolumeMountWithName(volumeMounts, existingVolumeMount)
				Expect(result).To(Equal([]corev1.VolumeMount{
					{
						Name:     "volumeMount1",
						ReadOnly: false,
					},
					{
						Name:     "volumeMount2",
						ReadOnly: false,
					},
				}))
			})

			It("should do nothing to the existing VolumeMount if it's identical", func() {
				identicalVolumeMount := volumeMounts[0]
				result := webhook.EnsureVolumeMountWithName(volumeMounts, identicalVolumeMount)
				Expect(result).To(Equal(volumeMounts))
			})
		})

		Describe("#EnsureVolumeWithName", func() {
			var volumes []corev1.Volume

			BeforeEach(func() {
				volumes = []corev1.Volume{
					{
						Name: "volume1",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{Medium: ""},
						},
					},
					{
						Name: "volume2",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				}
			})

			It("should add a new Volume if not present", func() {
				newVolume := corev1.Volume{
					Name: "volume3",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}
				result := webhook.EnsureVolumeWithName(volumes, newVolume)
				Expect(result).To(Equal(append(volumes, newVolume)))
			})

			It("should replace the existing Volume if it's not identical", func() {
				existingVolume := corev1.Volume{
					Name: "volume1",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{Medium: "Memory"},
					},
				}
				result := webhook.EnsureVolumeWithName(volumes, existingVolume)
				Expect(result).To(Equal([]corev1.Volume{
					{
						Name: "volume1",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{Medium: "Memory"},
						},
					},
					{
						Name: "volume2",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				}))
			})

			It("should do nothing to the existing Volume if it's identical", func() {
				identicalVolume := volumes[0]
				result := webhook.EnsureVolumeWithName(volumes, identicalVolume)
				Expect(result).To(Equal(volumes))
			})
		})

		Describe("#EnsureContainerWithName", func() {
			var containers []corev1.Container

			BeforeEach(func() {
				containers = []corev1.Container{
					{
						Name:  "container1",
						Image: "image1",
					},
					{
						Name:  "container2",
						Image: "image2",
					},
				}
			})

			It("should add a new Container if not present", func() {
				newContainer := corev1.Container{
					Name:  "container3",
					Image: "image3",
				}
				result := webhook.EnsureContainerWithName(containers, newContainer)
				Expect(result).To(Equal(append(containers, newContainer)))
			})

			It("should replace the existing Container if it's not identical", func() {
				existingContainer := corev1.Container{
					Name:  "container1",
					Image: "image3",
				}
				result := webhook.EnsureContainerWithName(containers, existingContainer)
				Expect(result).To(Equal(
					[]corev1.Container{
						{
							Name:  "container1",
							Image: "image3",
						},
						{
							Name:  "container2",
							Image: "image2",
						},
					},
				))
			})

			It("should do nothing to the existing Container if it's identical", func() {
				identicalContainer := containers[0]
				result := webhook.EnsureContainerWithName(containers, identicalContainer)
				Expect(result).To(Equal(containers))
			})
		})

		Describe("#EnsureVPAContainerResourcePolicyWithName", func() {
			var (
				policies []vpaautoscalingv1.ContainerResourcePolicy
				modeAuto = vpaautoscalingv1.ContainerScalingModeAuto
				modeOff  = vpaautoscalingv1.ContainerScalingModeOff
			)

			BeforeEach(func() {
				policies = []vpaautoscalingv1.ContainerResourcePolicy{
					{
						ContainerName: "container1",
						Mode:          &modeAuto,
					},
					{
						ContainerName: "container2",
						Mode:          &modeOff,
					},
				}
			})

			It("should add a new ContainerResourcePolicy if not present", func() {
				newPolicy := vpaautoscalingv1.ContainerResourcePolicy{
					ContainerName: "container3",
					Mode:          &modeAuto,
				}
				result := webhook.EnsureVPAContainerResourcePolicyWithName(policies, newPolicy)
				Expect(result).To(Equal(append(policies, newPolicy)))
			})

			It("should replace the existing ContainerResourcePolicy if it's not identical", func() {
				existingPolicy := vpaautoscalingv1.ContainerResourcePolicy{
					ContainerName: "container1",
					Mode:          &modeOff,
				}
				result := webhook.EnsureVPAContainerResourcePolicyWithName(policies, existingPolicy)
				Expect(result).To(Equal([]vpaautoscalingv1.ContainerResourcePolicy{
					{
						ContainerName: "container1",
						Mode:          &modeOff,
					},
					{
						ContainerName: "container2",
						Mode:          &modeOff,
					},
				}))
			})

			It("should do nothing to the existing ContainerResourcePolicy if it's identical", func() {
				identicalPolicy := policies[0]
				result := webhook.EnsureVPAContainerResourcePolicyWithName(policies, identicalPolicy)
				Expect(result).To(Equal(policies))
			})
		})

		Describe("#EnsurePVCWithName", func() {
			var pvcs []corev1.PersistentVolumeClaim

			BeforeEach(func() {
				pvcs = []corev1.PersistentVolumeClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc1",
						},
						Spec: corev1.PersistentVolumeClaimSpec{
							Resources: corev1.VolumeResourceRequirements{
								Requests: map[corev1.ResourceName]resource.Quantity{
									"storage": resource.MustParse("10Gi"),
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc2",
						},
						Spec: corev1.PersistentVolumeClaimSpec{
							Resources: corev1.VolumeResourceRequirements{
								Requests: map[corev1.ResourceName]resource.Quantity{
									"storage": resource.MustParse("20Gi"),
								},
							},
						},
					},
				}
			})

			It("should add a new PersistentVolumeClaim if not present", func() {
				newPVC := corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pvc3",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						Resources: corev1.VolumeResourceRequirements{
							Requests: map[corev1.ResourceName]resource.Quantity{
								"storage": resource.MustParse("30Gi"),
							},
						},
					},
				}
				result := webhook.EnsurePVCWithName(pvcs, newPVC)
				Expect(result).To(Equal(append(pvcs, newPVC)))
			})

			It("should replace the existing PersistentVolumeClaim if it's not identical", func() {
				existingPVC := corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pvc1",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						Resources: corev1.VolumeResourceRequirements{
							Requests: map[corev1.ResourceName]resource.Quantity{
								"storage": resource.MustParse("30Gi"),
							},
						},
					},
				}
				result := webhook.EnsurePVCWithName(pvcs, existingPVC)
				Expect(result).To(Equal([]corev1.PersistentVolumeClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc1",
						},
						Spec: corev1.PersistentVolumeClaimSpec{
							Resources: corev1.VolumeResourceRequirements{
								Requests: map[corev1.ResourceName]resource.Quantity{
									"storage": resource.MustParse("30Gi"),
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc2",
						},
						Spec: corev1.PersistentVolumeClaimSpec{
							Resources: corev1.VolumeResourceRequirements{
								Requests: map[corev1.ResourceName]resource.Quantity{
									"storage": resource.MustParse("20Gi"),
								},
							},
						},
					},
				}))
			})

			It("should do nothing to the existing PersistentVolumeClaim if it's identical", func() {
				identicalPVC := pvcs[0]
				result := webhook.EnsurePVCWithName(pvcs, identicalPVC)
				Expect(result).To(Equal(pvcs))
			})
		})

		Describe("#EnsureUnitOption", func() {
			var unitOptions []*unit.UnitOption

			BeforeEach(func() {
				unitOptions = []*unit.UnitOption{
					{
						Section: "Unit",
						Name:    "Description",
						Value:   "Test Unit 1",
					},
					{
						Section: "Service",
						Name:    "ExecStart",
						Value:   "/usr/bin/test1",
					},
				}
			})

			It("should add a new UnitOption if not present", func() {
				newOption := &unit.UnitOption{
					Section: "Install",
					Name:    "WantedBy",
					Value:   "multi-user.target",
				}
				result := webhook.EnsureUnitOption(unitOptions, newOption)
				Expect(result).To(Equal(append(unitOptions, newOption)))
			})

			It("should not replace the existing UnitOption if it's not identical but add it", func() {
				existingOption := &unit.UnitOption{
					Section: "Unit",
					Name:    "Description",
					Value:   "Test Unit 2",
				}
				result := webhook.EnsureUnitOption(unitOptions, existingOption)
				Expect(result).To(Equal(append(unitOptions, existingOption)))
			})

			It("should do nothing to the existing UnitOption if it's identical", func() {
				identicalOption := unitOptions[0]
				result := webhook.EnsureUnitOption(unitOptions, identicalOption)
				Expect(result).To(Equal(unitOptions))
			})
		})

		Describe("#EnsureFileWithPath", func() {
			var files []extensionsv1alpha1.File

			BeforeEach(func() {
				files = []extensionsv1alpha1.File{
					{
						Path: "/foo.txt",
						Content: extensionsv1alpha1.FileContent{
							Inline: &extensionsv1alpha1.FileContentInline{
								Data: "foo",
							},
						},
					},
					{
						Path: "/bar.txt",
						Content: extensionsv1alpha1.FileContent{
							Inline: &extensionsv1alpha1.FileContentInline{
								Data: "bar",
							},
						},
					},
				}
			})

			It("should append file when file with such path does not exist", func() {
				newFile := extensionsv1alpha1.File{
					Path: "/baz.txt",
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Data: "baz",
						},
					},
				}

				actual := webhook.EnsureFileWithPath(files, newFile)
				Expect(actual).To(Equal(append(files, newFile)))
			})

			It("should update file when file with such path exists", func() {
				newFile := extensionsv1alpha1.File{
					Path: "/foo.txt",
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Data: "baz",
						},
					},
				}

				actual := webhook.EnsureFileWithPath(files, newFile)
				Expect(actual).To(Equal([]extensionsv1alpha1.File{
					{
						Path: "/foo.txt",
						Content: extensionsv1alpha1.FileContent{
							Inline: &extensionsv1alpha1.FileContentInline{
								Data: "baz",
							},
						},
					},
					{
						Path: "/bar.txt",
						Content: extensionsv1alpha1.FileContent{
							Inline: &extensionsv1alpha1.FileContentInline{
								Data: "bar",
							},
						},
					},
				}))
			})

			It("should do nothing when the new file is exactly the same as the existing one", func() {
				newFile := files[0]

				actual := webhook.EnsureFileWithPath(files, newFile)
				Expect(actual).To(Equal(files))
			})
		})

		Describe("#EnsureUnitWithName", func() {
			var units []extensionsv1alpha1.Unit

			BeforeEach(func() {
				units = []extensionsv1alpha1.Unit{
					{
						Name:    "foo.service",
						Content: ptr.To("foo"),
					},
					{
						Name:    "bar.service",
						Content: ptr.To("bar"),
					},
				}
			})

			It("should append unit when unit with such name does not exist", func() {
				newUnit := extensionsv1alpha1.Unit{
					Name:    "baz.service",
					Content: ptr.To("bar"),
				}

				actual := webhook.EnsureUnitWithName(units, newUnit)
				Expect(actual).To(Equal(append(units, newUnit)))
			})

			It("should update unit when unit with such name exists", func() {
				newUnit := extensionsv1alpha1.Unit{
					Name:    "foo.service",
					Content: ptr.To("baz"),
				}

				actual := webhook.EnsureUnitWithName(units, newUnit)
				Expect(actual).To(Equal([]extensionsv1alpha1.Unit{
					{
						Name:    "foo.service",
						Content: ptr.To("baz"),
					},
					{
						Name:    "bar.service",
						Content: ptr.To("bar"),
					},
				}))
			})

			It("should do nothing when the new unit is exactly the same as the existing one", func() {
				newUnit := units[0]

				actual := webhook.EnsureUnitWithName(units, newUnit)
				Expect(actual).To(Equal(units))

			})
		})

		Describe("#EnsureAnnotationOrLabel", func() {
			var annotations map[string]string

			BeforeEach(func() {
				annotations = map[string]string{
					"annotation1": "value1",
					"annotation2": "value2",
				}
			})

			It("should ensure the specified annotation or label exists", func() {
				result := webhook.EnsureAnnotationOrLabel(annotations, "annotation3", "value3")
				Expect(result).To(Equal(map[string]string{
					"annotation1": "value1",
					"annotation2": "value2",
					"annotation3": "value3",
				}))
			})

			It("should overwrite the value of an existing annotation or label", func() {
				result := webhook.EnsureAnnotationOrLabel(annotations, "annotation1", "newvalue1")
				Expect(result).To(Equal(map[string]string{
					"annotation1": "newvalue1",
					"annotation2": "value2",
				}))
			})

			It("should create a new map if the input map is nil", func() {
				result := webhook.EnsureAnnotationOrLabel(nil, "annotation1", "value1")
				Expect(result).To(Equal(map[string]string{
					"annotation1": "value1",
				}))
			})
		})
	})

	Describe("#EnsureNo*", func() {
		Describe("#EnsureNoStringWithPrefix", func() {
			var flags []string

			BeforeEach(func() {
				flags = []string{
					"--prefix1-flag1=value1",
					"--flag2=value2",
					"--prefix1-flag3=value3",
				}
			})

			It("should delete all strings with a given prefix", func() {
				result := webhook.EnsureNoStringWithPrefix(flags, "--prefix1")
				Expect(result).To(Equal([]string{"--flag2=value2"}))
			})
		})

		Describe("#EnsureNoStringWithPrefixContains", func() {
			var flags []string

			BeforeEach(func() {
				flags = []string{
					"--flag1=key2=value2,key3=value3",
					"--flag2=value1",
					"--flag3=value3,value1",
					"--flag1=key3=value3,key1=value1",
				}
			})

			It("should delete the specified value from all strings with a given prefix", func() {
				result := webhook.EnsureNoStringWithPrefixContains(flags, "--flag1=", "key3=value3", ",")
				Expect(result).To(Equal([]string{
					"--flag1=key2=value2",
					"--flag2=value1",
					"--flag3=value3,value1",
					"--flag1=key1=value1",
				}))
			})
		})

		Describe("#EnsureNoEnvVarWithName", func() {
			var envVars []corev1.EnvVar

			BeforeEach(func() {
				envVars = []corev1.EnvVar{
					{Name: "envVar1"},
					{Name: "envVar2"},
					{Name: "envVar1"},
				}
			})

			It("should delete all environment variables with a given name", func() {
				result := webhook.EnsureNoEnvVarWithName(envVars, "envVar1")
				Expect(result).To(Equal([]corev1.EnvVar{{Name: "envVar2"}}))
			})
		})

		Describe("#EnsureNoVolumeMountWithName", func() {
			var volumeMounts []corev1.VolumeMount

			BeforeEach(func() {
				volumeMounts = []corev1.VolumeMount{
					{Name: "mount1"},
					{Name: "mount2"},
					{Name: "mount1"},
				}
			})

			It("should delete all volume mounts with a given name", func() {
				result := webhook.EnsureNoVolumeMountWithName(volumeMounts, "mount1")
				Expect(result).To(Equal([]corev1.VolumeMount{{Name: "mount2"}}))
			})
		})

		Describe("#EnsureNoVolumeWithName", func() {
			var volumes []corev1.Volume

			BeforeEach(func() {
				volumes = []corev1.Volume{
					{
						Name: "volume1",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
					{
						Name: "volume2",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
					{
						Name: "volume1",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				}
			})

			It("should delete all volumes with a given name", func() {
				result := webhook.EnsureNoVolumeWithName(volumes, "volume1")
				Expect(result).To(Equal([]corev1.Volume{{
					Name: "volume2",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}}))
			})
		})

		Describe("#EnsureNoContainerWithName", func() {
			var containers []corev1.Container

			BeforeEach(func() {
				containers = []corev1.Container{
					{Name: "container1"},
					{Name: "container2"},
					{Name: "container1"},
				}
			})

			It("should delete all containers with a given name", func() {
				result := webhook.EnsureNoContainerWithName(containers, "container1")
				Expect(result).To(Equal([]corev1.Container{{Name: "container2"}}))
			})
		})

		Describe("#EnsureNoPVCWithName", func() {
			var pvcs []corev1.PersistentVolumeClaim

			BeforeEach(func() {
				pvcs = []corev1.PersistentVolumeClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc1",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc2",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc1",
						},
					},
				}
			})

			It("should delete all Persistent Volume Claims with a given name", func() {
				result := webhook.EnsureNoPVCWithName(pvcs, "pvc1")
				Expect(result).To(Equal([]corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "pvc2"}}}))
			})
		})
	})
})
