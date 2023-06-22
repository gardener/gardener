// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package maintenance_test

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/timewindow"
)

var _ = Describe("Shoot Maintenance controller tests", func() {
	var (
		cloudProfile *gardencorev1beta1.CloudProfile
		shoot        *gardencorev1beta1.Shoot
		shoot126     *gardencorev1beta1.Shoot

		// Test Machine Image
		machineImageName                = "foo-image"
		highestAMD64MachineImageVersion = "1.1.1"
		highestAMD64MachineImage        = gardencorev1beta1.ShootMachineImage{
			Name:    machineImageName,
			Version: &highestAMD64MachineImageVersion,
		}
		highestARM64MachineImageVersion = "1.2.0"
		highestARM64MachineImage        = gardencorev1beta1.ShootMachineImage{
			Name:    machineImageName,
			Version: &highestARM64MachineImageVersion,
		}
		testMachineImageVersion = "0.0.1-beta"
		testMachineImage        = gardencorev1beta1.ShootMachineImage{
			Name:    machineImageName,
			Version: &testMachineImageVersion,
		}

		// other
		deprecatedClassification = gardencorev1beta1.ClassificationDeprecated
		supportedClassification  = gardencorev1beta1.ClassificationSupported
		expirationDateInThePast  = metav1.Date(2012, 1, 1, 0, 0, 0, 0, time.UTC)

		// Test Kubernetes versions
		testKubernetesVersionLowPatchLowMinor             = gardencorev1beta1.ExpirableVersion{Version: "0.0.1", Classification: &deprecatedClassification}
		testKubernetesVersionHighestPatchLowMinor         = gardencorev1beta1.ExpirableVersion{Version: "0.0.5", Classification: &deprecatedClassification}
		testKubernetesVersionLowPatchConsecutiveMinor     = gardencorev1beta1.ExpirableVersion{Version: "0.1.1", Classification: &deprecatedClassification}
		testKubernetesVersionHighestPatchConsecutiveMinor = gardencorev1beta1.ExpirableVersion{Version: "0.1.5", Classification: &deprecatedClassification}
	)

	BeforeEach(func() {
		fakeClock.SetTime(time.Now().Round(time.Second))

		cloudProfile = &gardencorev1beta1.CloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-",
			},
			Spec: gardencorev1beta1.CloudProfileSpec{
				Kubernetes: gardencorev1beta1.KubernetesSettings{
					Versions: []gardencorev1beta1.ExpirableVersion{
						{
							Version: "1.25.1",
						},
						{
							Version: "1.26.0",
						},
						{
							Version: "1.27.0",
						},
						testKubernetesVersionLowPatchLowMinor,
						testKubernetesVersionHighestPatchLowMinor,
						testKubernetesVersionLowPatchConsecutiveMinor,
						testKubernetesVersionHighestPatchConsecutiveMinor,
					},
				},
				MachineImages: []gardencorev1beta1.MachineImage{
					{
						Name: machineImageName,
						Versions: []gardencorev1beta1.MachineImageVersion{
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        highestAMD64MachineImageVersion,
									Classification: &supportedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameDocker,
									},
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        highestARM64MachineImageVersion,
									Classification: &supportedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameDocker,
									},
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"arm64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        testMachineImageVersion,
									Classification: &deprecatedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameDocker,
									},
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"amd64", "arm64"},
							},
						},
					},
				},
				MachineTypes: []gardencorev1beta1.MachineType{
					{
						Name: "large",
					},
				},
				Regions: []gardencorev1beta1.Region{
					{
						Name: "foo-region",
					},
				},
				Type: "foo-type",
			},
		}

		By("Create CloudProfile")
		Expect(testClient.Create(ctx, cloudProfile)).To(Succeed())
		log.Info("Created CloudProfile for test", "cloudProfile", client.ObjectKeyFromObject(cloudProfile))

		DeferCleanup(func() {
			By("Delete CloudProfile")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, cloudProfile))).To(Succeed())
		})

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "test-", Namespace: testNamespace.Name},
			Spec: gardencorev1beta1.ShootSpec{
				SecretBindingName: pointer.String("my-provider-account"),
				CloudProfileName:  cloudProfile.Name,
				Region:            "foo-region",
				Provider: gardencorev1beta1.Provider{
					Type: "foo-provider",
					Workers: []gardencorev1beta1.Worker{
						{
							Name:    "cpu-worker1",
							Minimum: 2,
							Maximum: 2,
							Machine: gardencorev1beta1.Machine{
								Image: &testMachineImage,
								Type:  "large",
							},
						},
						{
							Name:    "cpu-worker2",
							Minimum: 2,
							Maximum: 2,
							Machine: gardencorev1beta1.Machine{
								Image:        &testMachineImage,
								Type:         "large",
								Architecture: pointer.String("arm64"),
							},
						},
					},
				},
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.25.1",
				},
				Networking: &gardencorev1beta1.Networking{
					Type: pointer.String("foo-networking"),
				},
				Maintenance: &gardencorev1beta1.Maintenance{
					AutoUpdate: &gardencorev1beta1.MaintenanceAutoUpdate{
						KubernetesVersion:   false,
						MachineImageVersion: pointer.Bool(false),
					},
					TimeWindow: &gardencorev1beta1.MaintenanceTimeWindow{
						Begin: timewindow.NewMaintenanceTime(time.Now().Add(2*time.Hour).Hour(), 0, 0).Formatted(),
						End:   timewindow.NewMaintenanceTime(time.Now().Add(4*time.Hour).Hour(), 0, 0).Formatted(),
					},
				},
			},
		}

		shoot126 = shoot.DeepCopy()
		// set dummy kubernetes version to shoot
		shoot.Spec.Kubernetes.Version = testKubernetesVersionLowPatchLowMinor.Version

		By("Create Shoot")
		Expect(testClient.Create(ctx, shoot)).To(Succeed())
		log.Info("Created shoot for test", "shoot", client.ObjectKeyFromObject(shoot))

		DeferCleanup(func() {
			By("Delete Shoot")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot))).To(Succeed())
		})
	})

	It("should add task annotations", func() {
		By("Trigger maintenance")
		Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

		waitForShootToBeMaintained(shoot)

		By("Ensure task annotations are present")
		Expect(shoot.Annotations).To(HaveKey("shoot.gardener.cloud/tasks"))
		Expect(strings.Split(shoot.Annotations["shoot.gardener.cloud/tasks"], ",")).To(And(
			ContainElement("deployInfrastructure"),
			ContainElement("deployDNSRecordInternal"),
			ContainElement("deployDNSRecordExternal"),
			ContainElement("deployDNSRecordIngress"),
		))
	})

	Context("operation annotations", func() {
		var oldGeneration int64

		BeforeEach(func() {
			oldGeneration = shoot.Generation
		})

		Context("failed last operation state", func() {
			BeforeEach(func() {
				By("Prepare shoot")
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateFailed}
				Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())
			})

			It("should not set the retry operation annotation due to missing 'needs-retry-operation' annotation", func() {
				By("Trigger maintenance")
				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				waitForShootToBeMaintained(shoot)

				By("Ensure proper operation annotation handling")
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				Expect(shoot.Generation).To(Equal(oldGeneration))
				Expect(shoot.Annotations["gardener.cloud/operation"]).To(BeEmpty())
			})

			It("should set the retry operation annotation due to 'needs-retry-operation' annotation (implicitly increasing the generation)", func() {
				By("Prepare shoot")
				patch := client.MergeFrom(shoot.DeepCopy())
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "maintenance.shoot.gardener.cloud/needs-retry-operation", "true")
				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				By("Trigger maintenance")
				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				waitForShootToBeMaintained(shoot)

				By("Ensure proper operation annotation handling")
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				Expect(shoot.Generation).To(Equal(oldGeneration + 1))
				Expect(shoot.Annotations["gardener.cloud/operation"]).To(BeEmpty())
				Expect(shoot.Annotations["maintenance.shoot.gardener.cloud/needs-retry-operation"]).To(BeEmpty())
			})
		})

		Context("non-failed last operation states", func() {
			BeforeEach(func() {
				By("Prepare shoot")
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{}
				Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())
			})

			It("should set the reconcile operation annotation (implicitly increasing the generation)", func() {
				By("Trigger maintenance")
				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				waitForShootToBeMaintained(shoot)

				By("Ensure proper operation annotation handling")
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				Expect(shoot.Generation).To(Equal(oldGeneration + 1))
				Expect(shoot.Annotations["gardener.cloud/operation"]).To(BeEmpty())
			})

			It("should set the maintenance operation annotation if it's valid", func() {
				By("Prepare shoot")
				patch := client.MergeFrom(shoot.DeepCopy())
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "maintenance.gardener.cloud/operation", "rotate-kubeconfig-credentials")
				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				By("Trigger maintenance")
				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				waitForShootToBeMaintained(shoot)

				By("Ensure proper operation annotation handling")
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				Expect(shoot.Generation).To(Equal(oldGeneration + 1))
				Expect(shoot.Annotations["gardener.cloud/operation"]).To(Equal("rotate-kubeconfig-credentials"))
				Expect(shoot.Annotations["maintenance.gardener.cloud/operation"]).To(BeEmpty())
			})

			It("should not set the maintenance operation annotation if it's invalid and use the reconcile operation instead", func() {
				By("Prepare shoot")
				patch := client.MergeFrom(shoot.DeepCopy())
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "maintenance.gardener.cloud/operation", "foo-bar-does-not-exist")
				err := testClient.Patch(ctx, shoot, patch)
				Expect(apierrors.IsInvalid(err)).To(Equal(true))

				By("Trigger maintenance")
				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				waitForShootToBeMaintained(shoot)

				By("Ensure proper operation annotation handling")
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				Expect(shoot.Generation).To(Equal(oldGeneration + 1))
				Expect(shoot.Annotations["gardener.cloud/operation"]).To(BeEmpty())
				Expect(shoot.Annotations["maintenance.gardener.cloud/operation"]).To(BeEmpty())
			})
		})
	})

	Describe("Machine image maintenance tests", func() {
		It("Do not update Shoot machine image in maintenance time: AutoUpdate.MachineImageVersion == false && expirationDate does not apply", func() {
			Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

			Consistently(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: testMachineImage.Name, Version: testMachineImage.Version}))
				g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: testMachineImage.Name, Version: testMachineImage.Version}))
			}).Should(Succeed())
		})

		It("Shoot machine image must be updated in maintenance time: AutoUpdate.MachineImageVersion == true && expirationDate does not apply", func() {
			// set test specific shoot settings
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = pointer.Bool(true)
			Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

			Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: highestAMD64MachineImage.Name, Version: highestAMD64MachineImage.Version}))
				g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: highestARM64MachineImage.Name, Version: highestARM64MachineImage.Version}))
				g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
				g.Expect(*shoot.Status.LastMaintenance).To(Equal(gardencorev1beta1.LastMaintenance{
					Description: "Machine image of worker-pool \"cpu-worker1\" upgraded from \"foo-image\" version \"0.0.1-beta\" to version \"1.1.1\". Reason: AutoUpdate of MachineImage configured" + ", " +
						"Machine image of worker-pool \"cpu-worker2\" upgraded from \"foo-image\" version \"0.0.1-beta\" to version \"1.2.0\". Reason: AutoUpdate of MachineImage configured",
					TriggeredTime: metav1.Time{Time: fakeClock.Now()},
					State:         gardencorev1beta1.LastOperationStateSucceeded,
				}))
			}).Should(Succeed())
		})

		It("Shoot machine image must be updated in maintenance time: AutoUpdate.MachineImageVersion == false && expirationDate applies", func() {
			By("Expire Shoot's machine image in the CloudProfile")
			Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, testMachineImage, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

			By("Wait until manager has observed the CloudProfile update")
			waitMachineImageVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, testMachineImage.Name, *testMachineImage.Version, &expirationDateInThePast)

			Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: highestAMD64MachineImage.Name, Version: highestAMD64MachineImage.Version}))
				g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: highestARM64MachineImage.Name, Version: highestARM64MachineImage.Version}))
				g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
				g.Expect(*shoot.Status.LastMaintenance).To(Equal(gardencorev1beta1.LastMaintenance{
					Description: "Machine image of worker-pool \"cpu-worker1\" upgraded from \"foo-image\" version \"0.0.1-beta\" to version \"1.1.1\". Reason: MachineImage expired - force update required" + ", " +
						"Machine image of worker-pool \"cpu-worker2\" upgraded from \"foo-image\" version \"0.0.1-beta\" to version \"1.2.0\". Reason: MachineImage expired - force update required",
					TriggeredTime: metav1.Time{Time: fakeClock.Now()},
					State:         gardencorev1beta1.LastOperationStateSucceeded,
				}))
			}).Should(Succeed())
		})
	})

	Describe("Kubernetes version maintenance tests", func() {
		BeforeEach(func() {
			shoot126.Spec.Kubernetes.Version = "1.26.0"
			shoot126.Spec.Kubernetes.EnableStaticTokenKubeconfig = pointer.BoolPtr(true)

			By("Create k8s v1.26 Shoot")
			Expect(testClient.Create(ctx, shoot126)).To(Succeed())
			log.Info("Created shoot with k8s v1.26 for test", "shoot", client.ObjectKeyFromObject(shoot))

			DeferCleanup(func() {
				By("Delete Shoot with k8s v1.26")
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot126))).To(Succeed())
			})
		})

		Context("Shoot with worker", func() {
			It("Kubernetes version should not be updated: auto update not enabled", func() {
				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Consistently(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					return shoot.Spec.Kubernetes.Version
				}).Should(Equal(testKubernetesVersionLowPatchLowMinor.Version))
			})

			It("Kubernetes version should be updated: auto update enabled", func() {
				// set test specific shoot settings
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = true
				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(*shoot.Status.LastMaintenance).To(Equal(gardencorev1beta1.LastMaintenance{
						Description:   "For \"Control Plane\": Kubernetes version upgraded \"0.0.1\" to version \"0.0.5\". Reason: AutoUpdate of Kubernetes version configured",
						TriggeredTime: metav1.Time{Time: fakeClock.Now()},
						State:         gardencorev1beta1.LastOperationStateSucceeded,
					}))
					return shoot.Spec.Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchLowMinor.Version))
			})

			It("Kubernetes version should be updated: force update patch version", func() {
				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, testKubernetesVersionLowPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, testKubernetesVersionLowPatchLowMinor.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(*shoot.Status.LastMaintenance).To(Equal(gardencorev1beta1.LastMaintenance{
						Description:   "For \"Control Plane\": Kubernetes version upgraded \"0.0.1\" to version \"0.0.5\". Reason: Kubernetes version expired - force update required",
						TriggeredTime: metav1.Time{Time: fakeClock.Now()},
						State:         gardencorev1beta1.LastOperationStateSucceeded,
					}))
					return shoot.Spec.Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchLowMinor.Version))
			})

			It("Kubernetes version should be updated: force update minor version(>= v1.27) and set EnableStaticTokenKubeconfig value to false", func() {
				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, shoot126.Spec.CloudProfileName, "1.26.0", &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(shoot126.Spec.CloudProfileName, "1.26.0", &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot126, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot126), shoot126)).To(Succeed())
					g.Expect(shoot126.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(*shoot126.Status.LastMaintenance).To(Equal(gardencorev1beta1.LastMaintenance{
						Description: "EnableStaticTokenKubeconfig is set to false. Reason: The static token kubeconfig can no longer be enabled for Shoot clusters using Kubernetes version 1.27 and higher" + ", " +
							"For \"Control Plane\": Kubernetes version upgraded \"1.26.0\" to version \"1.27.0\". Reason: Kubernetes version expired - force update required",
						TriggeredTime: metav1.Time{Time: fakeClock.Now()},
						State:         gardencorev1beta1.LastOperationStateSucceeded,
					}))
					g.Expect(shoot126.Spec.Kubernetes.EnableStaticTokenKubeconfig).To(Equal(pointer.BoolPtr(false)))
					return shoot126.Spec.Kubernetes.Version
				}).Should(Equal("1.27.0"))
			})

			It("Kubernetes version should be updated: force update minor version", func() {
				// set the shoots Kubernetes version to be the highest patch version of the minor version
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.Kubernetes.Version = testKubernetesVersionHighestPatchLowMinor.Version
				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				// expect shoot to have updated to latest patch version of next minor version
				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(*shoot.Status.LastMaintenance).To(Equal(gardencorev1beta1.LastMaintenance{
						Description:   "For \"Control Plane\": Kubernetes version upgraded \"0.0.5\" to version \"0.1.5\". Reason: Kubernetes version expired - force update required",
						TriggeredTime: metav1.Time{Time: fakeClock.Now()},
						State:         gardencorev1beta1.LastOperationStateSucceeded,
					}))
					return shoot.Spec.Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchConsecutiveMinor.Version))
			})
		})

		Describe("Worker Pool Kubernetes version maintenance tests", func() {
			It("Kubernetes version should not be updated: auto update not enabled", func() {
				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Consistently(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					return shoot.Spec.Kubernetes.Version
				}).Should(Equal(testKubernetesVersionLowPatchLowMinor.Version))
			})

			It("Kubernetes version should be updated: auto update enabled", func() {
				// set test specific shoot settings
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = true
				shoot.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: pointer.String(testKubernetesVersionLowPatchLowMinor.Version)}
				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(*shoot.Status.LastMaintenance).To(Equal(gardencorev1beta1.LastMaintenance{
						Description: "For \"Control Plane\": Kubernetes version upgraded \"0.0.1\" to version \"0.0.5\". Reason: AutoUpdate of Kubernetes version configured" + ", " +
							"For \"Worker Pool cpu-worker1\": Kubernetes version upgraded \"0.0.1\" to version \"0.0.5\". Reason: AutoUpdate of Kubernetes version configured",
						TriggeredTime: metav1.Time{Time: fakeClock.Now()},
						State:         gardencorev1beta1.LastOperationStateSucceeded,
					}))
					return *shoot.Spec.Provider.Workers[0].Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchLowMinor.Version))
			})

			It("Kubernetes version should be updated: force update patch version", func() {
				// expire the Shoot's Kubernetes version because autoupdate is set to false
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: pointer.String(testKubernetesVersionLowPatchLowMinor.Version)}
				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, testKubernetesVersionLowPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, testKubernetesVersionLowPatchLowMinor.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(*shoot.Status.LastMaintenance).To(Equal(gardencorev1beta1.LastMaintenance{
						Description: "For \"Control Plane\": Kubernetes version upgraded \"0.0.1\" to version \"0.0.5\". Reason: Kubernetes version expired - force update required" + ", " +
							"For \"Worker Pool cpu-worker1\": Kubernetes version upgraded \"0.0.1\" to version \"0.0.5\". Reason: Kubernetes version expired - force update required",
						TriggeredTime: metav1.Time{Time: fakeClock.Now()},
						State:         gardencorev1beta1.LastOperationStateSucceeded,
					}))
					return *shoot.Spec.Provider.Workers[0].Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchLowMinor.Version))
			})

			It("Kubernetes version should be updated: force update minor version", func() {
				// set the shoots Kubernetes version to be the highest patch version of the minor version
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.Kubernetes.Version = testKubernetesVersionHighestPatchLowMinor.Version
				shoot.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: pointer.String(testKubernetesVersionHighestPatchLowMinor.Version)}

				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				// expect worker pool to have updated to latest patch version of next minor version
				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(*shoot.Status.LastMaintenance).To(Equal(gardencorev1beta1.LastMaintenance{
						Description: "For \"Control Plane\": Kubernetes version upgraded \"0.0.5\" to version \"0.1.5\". Reason: Kubernetes version expired - force update required" + ", " +
							"For \"Worker Pool cpu-worker1\": Kubernetes version upgraded \"0.0.5\" to version \"0.1.5\". Reason: Kubernetes version expired - force update required",
						TriggeredTime: metav1.Time{Time: fakeClock.Now()},
						State:         gardencorev1beta1.LastOperationStateSucceeded,
					}))
					return *shoot.Spec.Provider.Workers[0].Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchConsecutiveMinor.Version))
			})

			It("Worker Pool Kubernetes version should be updated, but control plane version stays: force update minor of worker pool version", func() {
				// set the shoots Kubernetes version to be the highest patch version of the minor version
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.Kubernetes.Version = testKubernetesVersionLowPatchConsecutiveMinor.Version
				shoot.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: pointer.String(testKubernetesVersionHighestPatchLowMinor.Version)}

				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				By("Expire Shoot's worker pool kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				// expect worker pool to have updated to latest patch version of next minor version
				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(*shoot.Status.LastMaintenance).To(Equal(gardencorev1beta1.LastMaintenance{
						Description:   "For \"Worker Pool cpu-worker1\": Kubernetes version upgraded \"0.0.5\" to version \"0.1.5\". Reason: Kubernetes version expired - force update required",
						TriggeredTime: metav1.Time{Time: fakeClock.Now()},
						State:         gardencorev1beta1.LastOperationStateSucceeded,
					}))
					return *shoot.Spec.Provider.Workers[0].Kubernetes.Version
				}).Should(Equal(testKubernetesVersionLowPatchConsecutiveMinor.Version))
			})
		})

		Context("Workerless Shoot", func() {
			BeforeEach(func() {
				shoot.Spec.Provider.Workers = nil
			})

			It("Kubernetes version should not be updated: auto update not enabled", func() {
				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Consistently(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					return shoot.Spec.Kubernetes.Version
				}).Should(Equal(testKubernetesVersionLowPatchLowMinor.Version))
			})

			It("Kubernetes version should be updated: auto update enabled", func() {
				// set test specific shoot settings
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = true
				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(*shoot.Status.LastMaintenance).To(Equal(gardencorev1beta1.LastMaintenance{
						Description:   "For \"Control Plane\": Kubernetes version upgraded \"0.0.1\" to version \"0.0.5\". Reason: AutoUpdate of Kubernetes version configured",
						TriggeredTime: metav1.Time{Time: fakeClock.Now()},
						State:         gardencorev1beta1.LastOperationStateSucceeded,
					}))
					return shoot.Spec.Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchLowMinor.Version))
			})

			It("Kubernetes version should be updated: force update patch version", func() {
				// expire the Shoot's Kubernetes version because autoupdate is set to false
				patch := client.MergeFrom(shoot.DeepCopy())
				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, testKubernetesVersionLowPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, testKubernetesVersionLowPatchLowMinor.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(*shoot.Status.LastMaintenance).To(Equal(gardencorev1beta1.LastMaintenance{
						Description:   "For \"Control Plane\": Kubernetes version upgraded \"0.0.1\" to version \"0.0.5\". Reason: Kubernetes version expired - force update required",
						TriggeredTime: metav1.Time{Time: fakeClock.Now()},
						State:         gardencorev1beta1.LastOperationStateSucceeded,
					}))
					return shoot.Spec.Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchLowMinor.Version))
			})

			It("Kubernetes version should be updated: force update minor version", func() {
				// set the shoots Kubernetes version to be the highest patch version of the minor version
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.Kubernetes.Version = testKubernetesVersionHighestPatchLowMinor.Version

				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				// expect worker pool to have updated to latest patch version of next minor version
				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(*shoot.Status.LastMaintenance).To(Equal(gardencorev1beta1.LastMaintenance{
						Description:   "For \"Control Plane\": Kubernetes version upgraded \"0.0.5\" to version \"0.1.5\". Reason: Kubernetes version expired - force update required",
						TriggeredTime: metav1.Time{Time: fakeClock.Now()},
						State:         gardencorev1beta1.LastOperationStateSucceeded,
					}))
					return shoot.Spec.Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchConsecutiveMinor.Version))
			})

			It("Kubernetes version should be updated: force update minor version(>= v1.27) and set EnableStaticTokenKubeconfig value to false", func() {
				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, shoot126.Spec.CloudProfileName, "1.26.0", &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(shoot126.Spec.CloudProfileName, "1.26.0", &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot126, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot126), shoot126)).To(Succeed())
					g.Expect(shoot126.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(*shoot126.Status.LastMaintenance).To(Equal(gardencorev1beta1.LastMaintenance{
						Description: "EnableStaticTokenKubeconfig is set to false. Reason: The static token kubeconfig can no longer be enabled for Shoot clusters using Kubernetes version 1.27 and higher" + ", " +
							"For \"Control Plane\": Kubernetes version upgraded \"1.26.0\" to version \"1.27.0\". Reason: Kubernetes version expired - force update required",
						TriggeredTime: metav1.Time{Time: fakeClock.Now()},
						State:         gardencorev1beta1.LastOperationStateSucceeded,
					}))
					g.Expect(shoot126.Spec.Kubernetes.EnableStaticTokenKubeconfig).To(Equal(pointer.BoolPtr(false)))
					return shoot126.Spec.Kubernetes.Version
				}).Should(Equal("1.27.0"))
			})
		})
	})
})
