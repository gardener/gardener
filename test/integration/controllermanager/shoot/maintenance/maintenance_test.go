// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package maintenance_test

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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
		shoot127     *gardencorev1beta1.Shoot
		shoot129     *gardencorev1beta1.Shoot
		shoot130     *gardencorev1beta1.Shoot
		shoot131     *gardencorev1beta1.Shoot
		shoot132     *gardencorev1beta1.Shoot

		// Test Machine Image
		machineImageName             = "foo-image"
		highestVersionNextMajorARM   = "2.1.1"
		highestVersionNextMajorAMD64 = "2.1.3"

		highestVersionForCurrentMajorARM   = "0.4.1"
		highestVersionForCurrentMajorAMD64 = "0.3.0"

		highestPatchNextMinorARM   = "0.2.4"
		highestPatchNextMinorAMD64 = "0.2.3"

		highestSupportedARMVersion = "2.0.0"

		highestPatchSameMinorARM   = "0.0.4"
		highestPatchSameMinorAMD64 = "0.0.3"
		overallLatestVersionARM    = "3.0.0"
		overallLatestVersionAMD64  = "3.0.1"

		testMachineImageVersion = "0.0.1-beta"
		testMachineImage        = gardencorev1beta1.ShootMachineImage{
			Name:    machineImageName,
			Version: &testMachineImageVersion,
		}

		// other
		deprecatedClassification = gardencorev1beta1.ClassificationDeprecated
		supportedClassification  = gardencorev1beta1.ClassificationSupported
		expirationDateInThePast  = metav1.Date(2012, 1, 1, 0, 0, 0, 0, time.UTC)

		testKubernetesVersionLowPatchLowMinor             gardencorev1beta1.ExpirableVersion
		testKubernetesVersionHighestPatchLowMinor         gardencorev1beta1.ExpirableVersion
		testKubernetesVersionLowPatchConsecutiveMinor     gardencorev1beta1.ExpirableVersion
		testKubernetesVersionHighestPatchConsecutiveMinor gardencorev1beta1.ExpirableVersion
	)

	BeforeEach(func() {
		testKubernetesVersionLowPatchLowMinor = gardencorev1beta1.ExpirableVersion{Version: "0.0.1", Classification: &deprecatedClassification}
		testKubernetesVersionHighestPatchLowMinor = gardencorev1beta1.ExpirableVersion{Version: "0.0.5", Classification: &deprecatedClassification}
		testKubernetesVersionLowPatchConsecutiveMinor = gardencorev1beta1.ExpirableVersion{Version: "0.1.1", Classification: &deprecatedClassification}
		testKubernetesVersionHighestPatchConsecutiveMinor = gardencorev1beta1.ExpirableVersion{Version: "0.1.5", Classification: &deprecatedClassification}

		fakeClock.SetTime(time.Now().Round(time.Second))

		cloudProfile = &gardencorev1beta1.CloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-",
			},
			Spec: gardencorev1beta1.CloudProfileSpec{
				Kubernetes: gardencorev1beta1.KubernetesSettings{
					Versions: []gardencorev1beta1.ExpirableVersion{
						{
							Version: "1.27.0",
						},
						{
							Version: "1.28.0",
						},
						{
							Version: "1.29.0",
						},
						{
							Version: "1.30.0",
						},
						{
							Version: "1.31.0",
						},
						{
							Version: "1.32.0",
						},
						{
							Version: "1.33.0",
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
									// ARM overall latest version: 3.0.0
									Version:        overallLatestVersionARM,
									Classification: &deprecatedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"arm64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// AMD64 overall latest version: 3.0.1
									Version:        overallLatestVersionAMD64,
									Classification: &supportedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// latest ARM {patch, minor} of next major: 2.1.1
									Version:        highestVersionNextMajorARM,
									Classification: &deprecatedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"arm64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// latest AMD64 {patch, minor} of next major: 2.1.3
									Version:        highestVersionNextMajorAMD64,
									Classification: &supportedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// patch update should never be forcefully updated to the next major version
									Version:        highestSupportedARMVersion,
									Classification: &supportedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"amd64", "arm64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// latest patch for minor: 0.4.1
									Version:        highestVersionForCurrentMajorARM,
									Classification: &supportedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"arm64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// latest patch for minor: 0.3.0
									Version:        highestVersionForCurrentMajorAMD64,
									Classification: &supportedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// ARM: highest patch version for next higher minor version
									// should be force-updated to this version: 0.2.4
									Version:        highestPatchNextMinorARM,
									Classification: &deprecatedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"arm64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// AMD64: highest patch version for next higher minor version
									// should be force-updated to this version: 0.2.3
									Version:        highestPatchNextMinorAMD64,
									Classification: &deprecatedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        "0.2.0",
									Classification: &deprecatedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"amd64", "arm64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// ARM highest patch version for Shoot's minor: 0.0.4
									Version:        highestPatchSameMinorARM,
									Classification: &supportedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"arm64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// AMD64 highest patch version for Shoot's minor: 0.0.3
									Version:        highestPatchSameMinorAMD64,
									Classification: &deprecatedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        "0.0.2",
									Classification: &deprecatedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
									{
										Name: gardencorev1beta1.CRINameContainerD,
									},
								},
								Architectures: []string{"amd64", "arm64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        testMachineImageVersion,
									Classification: &deprecatedClassification,
								},
								CRI: []gardencorev1beta1.CRI{
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
				SecretBindingName: ptr.To("my-provider-account"),
				CloudProfile:      &gardencorev1beta1.CloudProfileReference{Name: cloudProfile.Name},
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
								Architecture: ptr.To("arm64"),
							},
						},
					},
				},
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.31.1",
				},
				Networking: &gardencorev1beta1.Networking{
					Type: ptr.To("foo-networking"),
				},
				Maintenance: &gardencorev1beta1.Maintenance{
					AutoUpdate: &gardencorev1beta1.MaintenanceAutoUpdate{
						KubernetesVersion:   false,
						MachineImageVersion: ptr.To(false),
					},
					TimeWindow: &gardencorev1beta1.MaintenanceTimeWindow{
						Begin: timewindow.NewMaintenanceTime(time.Now().Add(2*time.Hour).Hour(), 0, 0).Formatted(),
						End:   timewindow.NewMaintenanceTime(time.Now().Add(4*time.Hour).Hour(), 0, 0).Formatted(),
					},
				},
			},
		}

		shoot127 = shoot.DeepCopy()
		shoot129 = shoot.DeepCopy()
		shoot130 = shoot.DeepCopy()
		shoot131 = shoot.DeepCopy()
		shoot132 = shoot.DeepCopy()
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
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "maintenance.gardener.cloud/operation", "rotate-ssh-keypair")
				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				By("Trigger maintenance")
				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				waitForShootToBeMaintained(shoot)

				By("Ensure proper operation annotation handling")
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				Expect(shoot.Generation).To(Equal(oldGeneration + 1))
				Expect(shoot.Annotations["gardener.cloud/operation"]).To(Equal("rotate-ssh-keypair"))
				Expect(shoot.Annotations["maintenance.gardener.cloud/operation"]).To(BeEmpty())
			})

			It("should not set the maintenance operation annotation if it's invalid and use the reconcile operation instead", func() {
				By("Prepare shoot")
				patch := client.MergeFrom(shoot.DeepCopy())
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "maintenance.gardener.cloud/operation", "foo-bar-does-not-exist")
				err := testClient.Patch(ctx, shoot, patch)
				Expect(apierrors.IsInvalid(err)).To(BeTrue())

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

	Context("failed last maintenance operation", func() {
		BeforeEach(func() {
			By("Prepare shoot")
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Status.LastMaintenance = &gardencorev1beta1.LastMaintenance{State: gardencorev1beta1.LastOperationStateFailed}
			Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())
		})

		It("should remove the failed state of maintenance in case no changes were made during maintenance", func() {
			By("Trigger maintenance")
			Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

			waitForShootToBeMaintained(shoot)

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
				g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("Maintenance succeeded"))
				g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
			}).Should(Succeed())
		})

		It("should update the maintenance status according to the changes made during maintenance", func() {
			// expire the Shoot's Kubernetes version because autoupdate is set to false
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: ptr.To(testKubernetesVersionLowPatchLowMinor.Version)}
			Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

			By("Expire Shoot's kubernetes version in the CloudProfile")
			Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, *shoot.Spec.CloudProfileName, testKubernetesVersionLowPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

			By("Wait until manager has observed the CloudProfile update")
			waitKubernetesVersionToBeExpiredInCloudProfile(*shoot.Spec.CloudProfileName, testKubernetesVersionLowPatchLowMinor.Version, &expirationDateInThePast)

			Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

			Eventually(func(g Gomega) string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
				g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"0.0.1\" to \"0.0.5\". Reason: Kubernetes version expired - force update required, Worker pool \"cpu-worker1\": Updated Kubernetes version from \"0.0.1\" to \"0.0.5\". Reason: Kubernetes version expired - force update required"))
				g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
				g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
				return *shoot.Spec.Provider.Workers[0].Kubernetes.Version
			}).Should(Equal(testKubernetesVersionHighestPatchLowMinor.Version))
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

		Describe("AutoUpdateStrategy: major (default)", func() {
			BeforeEach(func() {
				By("Set updateStrategy: major for Shoot's machine image in the CloudProfile")
				patch := client.MergeFrom(cloudProfile.DeepCopy())
				updateStrategyMajor := gardencorev1beta1.UpdateStrategyMajor
				cloudProfile.Spec.MachineImages[0].UpdateStrategy = &updateStrategyMajor
				Expect(testClient.Patch(ctx, cloudProfile, patch)).To(Succeed())
			})

			It("auto update to latest overall version (update strategy: major)", func() {
				By("Set autoupdate=true and Shoot's machine version to be the latest in the minor in the CloudProfile")
				cloneShoot := &gardencorev1beta1.Shoot{}
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), cloneShoot)).ToNot(HaveOccurred())
				patch := client.StrategicMergeFrom(cloneShoot.DeepCopy())

				cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version = &highestPatchSameMinorAMD64
				cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version = &highestPatchSameMinorARM
				cloneShoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = ptr.To(true)
				Expect(testClient.Patch(ctx, cloneShoot, patch)).ToNot(HaveOccurred())

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[0].Machine.Image.Name, Version: &overallLatestVersionAMD64}))
					g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[1].Machine.Image.Name, Version: &highestSupportedARMVersion}))
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("Reason: Automatic update of the machine image version is configured (image update strategy: major)"))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
				}).Should(Succeed())
			})

			It("force update to latest overall version because the machine image is expired (update strategy: major)", func() {
				By("Set Shoot's machine version to be the latest in the minor in the CloudProfile")
				cloneShoot := &gardencorev1beta1.Shoot{}
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), cloneShoot)).ToNot(HaveOccurred())
				patch := client.StrategicMergeFrom(cloneShoot.DeepCopy())

				cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version = &highestPatchSameMinorAMD64
				cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version = &highestPatchSameMinorARM
				Expect(testClient.Patch(ctx, cloneShoot, patch)).ToNot(HaveOccurred())

				By("Expire Shoot worker 1's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, *shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[0].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Expire Shoot worker 2's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, *shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[1].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update for the first worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(*shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[0].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version, &expirationDateInThePast)

				By("Wait until manager has observed the CloudProfile update for the second worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(*shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[1].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[0].Machine.Image.Name, Version: &overallLatestVersionAMD64}))
					// updates for strategy major prefer upgrading to a supported version. Here, the latest ARM supported version is version "2.0.0" and the ARM latest overall version
					// is a deprecated version "3.0.0"
					g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[1].Machine.Image.Name, Version: &highestSupportedARMVersion}))
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("Reason: Machine image version expired - force update required (image update strategy: major)"))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
				}).Should(Succeed())
			})

			It("updating one worker pool fails, while the other one succeeds (update strategy: major)", func() {
				By("Set Shoot's machine version to be the latest in the minor in the CloudProfile")
				cloneShoot := &gardencorev1beta1.Shoot{}
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), cloneShoot)).ToNot(HaveOccurred())
				patch := client.StrategicMergeFrom(cloneShoot.DeepCopy())

				cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version = &highestPatchSameMinorAMD64
				cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version = &overallLatestVersionARM
				Expect(testClient.Patch(ctx, cloneShoot, patch)).ToNot(HaveOccurred())

				cpPatch := client.MergeFrom(cloudProfile.DeepCopy())
				updateStrategyMajor := gardencorev1beta1.UpdateStrategyMajor
				cloudProfile.Spec.MachineImages[0].UpdateStrategy = &updateStrategyMajor
				Expect(testClient.Patch(ctx, cloudProfile, cpPatch)).To(Succeed())

				By("Expire Shoot worker 1's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, *shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[0].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Expire Shoot worker 2's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, *shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[1].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update for the first worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(*shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[0].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version, &expirationDateInThePast)

				By("Wait until manager has observed the CloudProfile update for the second worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(*shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[1].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[0].Machine.Image.Name, Version: &overallLatestVersionAMD64}))
					g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[1].Machine.Image.Name, Version: &overallLatestVersionARM}))
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("(1/2) maintenance operations successful"))
					g.Expect(shoot.Status.LastMaintenance.FailureReason).ToNot(BeNil())
					g.Expect(*shoot.Status.LastMaintenance.FailureReason).To(ContainSubstring("Worker pool \"cpu-worker2\": failed to update machine image"))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateFailed))
				}).Should(Succeed())
			})

			It("force update to latest patch version in minor, as the current version is not the latest for the current minor (update strategy: major)", func() {
				By("Expire Shoot's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, *shoot.Spec.CloudProfileName, testMachineImage, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitMachineImageVersionToBeExpiredInCloudProfile(*shoot.Spec.CloudProfileName, testMachineImage.Name, *testMachineImage.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[0].Machine.Image.Name, Version: &highestPatchSameMinorAMD64}))
					g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[1].Machine.Image.Name, Version: &highestPatchSameMinorARM}))
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("Reason: Machine image version expired - force update required (image update strategy: major)"))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
				}).Should(Succeed())
			})

		})

		Describe("AutoUpdateStrategy: patch", func() {
			BeforeEach(func() {
				By("Set updateStrategy: major for Shoot's machine image in the CloudProfile")
				patch := client.MergeFrom(cloudProfile.DeepCopy())
				updateStrategyPatch := gardencorev1beta1.UpdateStrategyPatch
				cloudProfile.Spec.MachineImages[0].UpdateStrategy = &updateStrategyPatch
				Expect(testClient.Patch(ctx, cloudProfile, patch)).To(Succeed())
			})

			It("auto update to latest patch version in minor (update strategy: patch)", func() {
				// set test specific shoot settings
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = ptr.To(true)
				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: machineImageName, Version: ptr.To(highestPatchSameMinorAMD64)}))
					g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: machineImageName, Version: ptr.To(highestPatchSameMinorARM)}))
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring(fmt.Sprintf("Worker pool \"cpu-worker1\": Updated machine image \"foo-image\" from \"%s\" to \"%s\". Reason: Automatic update of the machine image version is configured (image update strategy: patch)", testMachineImageVersion, highestPatchSameMinorAMD64)))
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring(fmt.Sprintf("Worker pool \"cpu-worker2\": Updated machine image \"foo-image\" from \"%s\" to \"%s\". Reason: Automatic update of the machine image version is configured (image update strategy: patch)", testMachineImageVersion, highestPatchSameMinorARM)))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
				}).Should(Succeed())
			})

			It("force update to latest patch version in minor - not on latest patch version yet (update strategy: patch)", func() {
				By("Expire Shoot's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, *shoot.Spec.CloudProfileName, testMachineImage, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitMachineImageVersionToBeExpiredInCloudProfile(*shoot.Spec.CloudProfileName, testMachineImage.Name, *testMachineImage.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[0].Machine.Image.Name, Version: &highestPatchSameMinorAMD64}))
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring(fmt.Sprintf("Worker pool \"cpu-worker1\": Updated machine image \"foo-image\" from \"%s\" to \"%s\". Reason: Machine image version expired - force update required (image update strategy: patch)", testMachineImageVersion, highestPatchSameMinorAMD64)))
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring(fmt.Sprintf("Worker pool \"cpu-worker2\": Updated machine image \"foo-image\" from \"%s\" to \"%s\". Reason: Machine image version expired - force update required (image update strategy: patch)", testMachineImageVersion, highestPatchSameMinorARM)))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
				}).Should(Succeed())
			})

			It("force update to latest version in next minor because the machine image is expired (update strategy: patch)", func() {
				By("Set Shoot's machine version in the CloudProfile to be the latest in the minor")
				cloneShoot := &gardencorev1beta1.Shoot{}
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), cloneShoot)).ToNot(HaveOccurred())
				patch := client.StrategicMergeFrom(cloneShoot.DeepCopy())

				cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version = &highestPatchSameMinorAMD64
				cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version = &highestPatchSameMinorARM
				Expect(testClient.Patch(ctx, cloneShoot, patch)).ToNot(HaveOccurred())

				By("Expire Shoot worker 1's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, *shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[0].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Expire Shoot worker 2's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, *shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[1].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update for the first worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(*shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[0].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version, &expirationDateInThePast)

				By("Wait until manager has observed the CloudProfile update for the second worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(*shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[1].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[0].Machine.Image.Name, Version: &highestPatchNextMinorAMD64}))
					g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[1].Machine.Image.Name, Version: &highestPatchNextMinorARM}))
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring(fmt.Sprintf("Worker pool \"cpu-worker1\": Updated machine image \"foo-image\" from \"%s\" to \"%s\". Reason: Machine image version expired - force update required (image update strategy: patch)", highestPatchSameMinorAMD64, highestPatchNextMinorAMD64)))
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring(fmt.Sprintf("Worker pool \"cpu-worker2\": Updated machine image \"foo-image\" from \"%s\" to \"%s\". Reason: Machine image version expired - force update required (image update strategy: patch)", highestPatchSameMinorARM, highestPatchNextMinorARM)))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
				}).Should(Succeed())
			})

			It("fail to force update because already on latest patch in minor (update strategy: patch)", func() {
				By("Set Shoot's machine version in the CloudProfile to be the latest in the current major")
				cloneShoot := &gardencorev1beta1.Shoot{}
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), cloneShoot)).ToNot(HaveOccurred())
				patch := client.StrategicMergeFrom(cloneShoot.DeepCopy())

				cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version = &highestVersionForCurrentMajorAMD64
				cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version = &highestVersionForCurrentMajorARM
				Expect(testClient.Patch(ctx, cloneShoot, patch)).ToNot(HaveOccurred())

				By("Expire Shoot worker 1's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, *shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[0].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Expire Shoot worker 2's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, *shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[1].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update for the first worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(*shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[0].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version, &expirationDateInThePast)

				By("Wait until manager has observed the CloudProfile update for the second worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(*shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[1].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[0].Machine.Image.Name, Version: &highestVersionForCurrentMajorAMD64}))
					g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[1].Machine.Image.Name, Version: &highestVersionForCurrentMajorARM}))
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateFailed))
				}).Should(Succeed())
			})
		})

		Describe("AutoUpdateStrategy: minor", func() {
			BeforeEach(func() {
				By("Set updateStrategy: minor for Shoot's machine image in the CloudProfile")
				patch := client.MergeFrom(cloudProfile.DeepCopy())
				updateStrategyMinor := gardencorev1beta1.UpdateStrategyMinor
				cloudProfile.Spec.MachineImages[0].UpdateStrategy = &updateStrategyMinor
				Expect(testClient.Patch(ctx, cloudProfile, patch)).To(Succeed())
			})

			It("auto update to latest patch version in major (update strategy: minor)", func() {
				By("Set Shoot's machine version in the CloudProfile to be the latest in the minor (so does not upgrade to latest in minor first)")
				cloneShoot := &gardencorev1beta1.Shoot{}
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), cloneShoot)).ToNot(HaveOccurred())
				patch := client.StrategicMergeFrom(cloneShoot.DeepCopy())

				cloneShoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = ptr.To(true)
				cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version = &highestPatchNextMinorAMD64
				cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version = &highestPatchNextMinorARM
				Expect(testClient.Patch(ctx, cloneShoot, patch)).ToNot(HaveOccurred())

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: machineImageName, Version: ptr.To(highestVersionForCurrentMajorAMD64)}))
					g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: machineImageName, Version: ptr.To(highestVersionForCurrentMajorARM)}))
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring(fmt.Sprintf("Worker pool \"cpu-worker1\": Updated machine image \"foo-image\" from \"%s\" to \"%s\". Reason: Automatic update of the machine image version is configured (image update strategy: minor)", highestPatchNextMinorAMD64, highestVersionForCurrentMajorAMD64)))
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring(fmt.Sprintf("Worker pool \"cpu-worker2\": Updated machine image \"foo-image\" from \"%s\" to \"%s\". Reason: Automatic update of the machine image version is configured (image update strategy: minor)", highestPatchNextMinorARM, highestVersionForCurrentMajorARM)))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
				}).Should(Succeed())
			})

			It("force update to latest version in current major - not on latest in current major yet (update strategy: minor)", func() {
				By("Set Shoot's machine version in the CloudProfile to be the latest in the current major (so does not upgrade to latest in current major first)")
				cloneShoot := &gardencorev1beta1.Shoot{}
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), cloneShoot)).ToNot(HaveOccurred())
				patch := client.StrategicMergeFrom(cloneShoot.DeepCopy())

				cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version = &highestPatchSameMinorAMD64
				cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version = &highestPatchSameMinorARM
				Expect(testClient.Patch(ctx, cloneShoot, patch)).ToNot(HaveOccurred())

				By("Expire Shoot worker 1's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, *shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[0].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Expire Shoot worker 2's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, *shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[1].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update for the first worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(*shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[0].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version, &expirationDateInThePast)

				By("Wait until manager has observed the CloudProfile update for the second worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(*shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[1].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[0].Machine.Image.Name, Version: &highestVersionForCurrentMajorAMD64}))
					g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[1].Machine.Image.Name, Version: &highestVersionForCurrentMajorARM}))
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring(fmt.Sprintf("Worker pool \"cpu-worker1\": Updated machine image \"foo-image\" from \"%s\" to \"%s\". Reason: Machine image version expired - force update required (image update strategy: minor)", highestPatchSameMinorAMD64, highestVersionForCurrentMajorAMD64)))
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring(fmt.Sprintf("Worker pool \"cpu-worker2\": Updated machine image \"foo-image\" from \"%s\" to \"%s\". Reason: Machine image version expired - force update required (image update strategy: minor)", highestPatchSameMinorARM, highestVersionForCurrentMajorARM)))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
				}).Should(Succeed())
			})

			It("force update to latest version in the next major (update strategy: minor)", func() {
				By("Set Shoot's machine version in the CloudProfile to be the latest in the current major (so does not upgrade to latest in current major first)")
				cloneShoot := &gardencorev1beta1.Shoot{}
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), cloneShoot)).ToNot(HaveOccurred())
				patch := client.StrategicMergeFrom(cloneShoot.DeepCopy())

				cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version = &highestVersionForCurrentMajorAMD64
				cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version = &highestVersionForCurrentMajorARM
				Expect(testClient.Patch(ctx, cloneShoot, patch)).ToNot(HaveOccurred())

				By("Expire Shoot worker 1's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, *shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[0].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Expire Shoot worker 2's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, *shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[1].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update for the first worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(*shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[0].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version, &expirationDateInThePast)

				By("Wait until manager has observed the CloudProfile update for the second worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(*shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[1].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[0].Machine.Image.Name, Version: &highestVersionNextMajorAMD64}))
					g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[1].Machine.Image.Name, Version: &highestVersionNextMajorARM}))
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring(fmt.Sprintf("Worker pool \"cpu-worker1\": Updated machine image \"foo-image\" from \"%s\" to \"%s\". Reason: Machine image version expired - force update required (image update strategy: minor)", highestVersionForCurrentMajorAMD64, highestVersionNextMajorAMD64)))
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring(fmt.Sprintf("Worker pool \"cpu-worker2\": Updated machine image \"foo-image\" from \"%s\" to \"%s\". Reason: Machine image version expired - force update required (image update strategy: minor)", highestVersionForCurrentMajorARM, highestVersionNextMajorARM)))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
				}).Should(Succeed())
			})

			It("fail to force update because already on overall latest version (update strategy: minor)", func() {
				By("Set Shoot's machine version in the CloudProfile to be the latest in the current major")
				cloneShoot := &gardencorev1beta1.Shoot{}
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), cloneShoot)).ToNot(HaveOccurred())
				patch := client.StrategicMergeFrom(cloneShoot.DeepCopy())

				cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version = &overallLatestVersionAMD64
				cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version = &overallLatestVersionARM
				Expect(testClient.Patch(ctx, cloneShoot, patch)).ToNot(HaveOccurred())

				By("Expire Shoot worker 1's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, *shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[0].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Expire Shoot worker 2's machine image in the CloudProfile")
				Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, *shoot.Spec.CloudProfileName, *cloneShoot.Spec.Provider.Workers[1].Machine.Image, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update for the first worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(*shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[0].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[0].Machine.Image.Version, &expirationDateInThePast)

				By("Wait until manager has observed the CloudProfile update for the second worker")
				waitMachineImageVersionToBeExpiredInCloudProfile(*shoot.Spec.CloudProfileName, cloneShoot.Spec.Provider.Workers[1].Machine.Image.Name, *cloneShoot.Spec.Provider.Workers[1].Machine.Image.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(*shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[0].Machine.Image.Name, Version: &overallLatestVersionAMD64}))
					g.Expect(*shoot.Spec.Provider.Workers[1].Machine.Image).To(Equal(gardencorev1beta1.ShootMachineImage{Name: shoot.Spec.Provider.Workers[1].Machine.Image.Name, Version: &overallLatestVersionARM}))
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateFailed))
				}).Should(Succeed())
			})
		})
	})

	Describe("Kubernetes version maintenance tests", func() {
		var test = func() {
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
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"0.0.1\" to \"0.0.5\". Reason: Automatic update of Kubernetes version configured"))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					return shoot.Spec.Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchLowMinor.Version))
			})

			It("Kubernetes version should be updated: force update patch version", func() {
				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, *shoot.Spec.CloudProfileName, testKubernetesVersionLowPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(*shoot.Spec.CloudProfileName, testKubernetesVersionLowPatchLowMinor.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"0.0.1\" to \"0.0.5\". Reason: Kubernetes version expired - force update required"))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					return shoot.Spec.Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchLowMinor.Version))
			})

			It("Kubernetes version should be updated: force update minor version(>= v1.31) and set spec.kubernetes.kubeAPIServer.oidcConfig.clientAuthentication to nil", func() {
				shoot130.Spec.Kubernetes.Version = "1.30.0"
				shoot130.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
					OIDCConfig: &gardencorev1beta1.OIDCConfig{
						ClientAuthentication: &gardencorev1beta1.OpenIDConnectClientAuthentication{
							ExtraConfig: map[string]string{"foo": "bar"},
							Secret:      ptr.To("foo-secret"),
						},
					},
				}

				By("Create k8s v1.30 Shoot")
				Expect(testClient.Create(ctx, shoot130)).To(Succeed())
				log.Info("Created shoot with k8s v1.30 for test", "shoot", client.ObjectKeyFromObject(shoot))

				DeferCleanup(func() {
					By("Delete Shoot with k8s v1.30")
					Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot130))).To(Succeed())
				})

				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, *shoot130.Spec.CloudProfileName, "1.30.0", &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(*shoot130.Spec.CloudProfileName, "1.30.0", &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot130, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot130), shoot130)).To(Succeed())
					g.Expect(shoot130.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot130.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"1.30.0\" to \"1.31.0\". Reason: Kubernetes version expired - force update required, .spec.kubernetes.kubeAPIServer.oidcConfig.clientAuthentication is set to nil. Reason: The field was no-op since its introduction and can no longer be enabled for Shoot clusters using Kubernetes version 1.31+"))
					g.Expect(shoot130.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot130.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot130.Spec.Kubernetes.KubeAPIServer.OIDCConfig.ClientAuthentication).To(BeNil())
					return shoot130.Spec.Kubernetes.Version
				}).Should(Equal("1.31.0"))
			})

			It("Kubernetes version should be updated: force update minor version(>= v1.32) and set spec.kubernetes.kubeAPIServer.oidcConfig to nil", func() {
				shoot131.Spec.Kubernetes.Version = "1.31.0"
				shoot131.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
					OIDCConfig: &gardencorev1beta1.OIDCConfig{
						ClientID:  ptr.To("client-id"),
						IssuerURL: ptr.To("https://foo.bar"),
					},
				}

				By("Create k8s v1.31 Shoot")
				Expect(testClient.Create(ctx, shoot131)).To(Succeed())
				log.Info("Created shoot with k8s v1.31 for test", "shoot", client.ObjectKeyFromObject(shoot))

				DeferCleanup(func() {
					By("Delete Shoot with k8s v1.31")
					Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot131))).To(Succeed())
				})

				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, *shoot131.Spec.CloudProfileName, "1.31.0", &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(*shoot131.Spec.CloudProfileName, "1.31.0", &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot131, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot131), shoot131)).To(Succeed())
					g.Expect(shoot131.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot131.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"1.31.0\" to \"1.32.0\". Reason: Kubernetes version expired - force update required, .spec.kubernetes.kubeAPIServer.oidcConfig is set to nil. Reason: The field has been deprecated in favor of structured authentication and can no longer be enabled for Shoot clusters using Kubernetes version 1.32+"))
					g.Expect(shoot131.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot131.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot131.Spec.Kubernetes.KubeAPIServer.OIDCConfig).To(BeNil())
					return shoot131.Spec.Kubernetes.Version
				}).Should(Equal("1.32.0"))
			})

			It("Kubernetes version should be updated: force update minor version", func() {
				// set the shoots Kubernetes version to be the highest patch version of the minor version
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.Kubernetes.Version = testKubernetesVersionHighestPatchLowMinor.Version
				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, *shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(*shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				// expect shoot to have updated to latest patch version of next minor version
				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"0.0.5\" to \"0.1.5\". Reason: Kubernetes version expired - force update required"))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					return shoot.Spec.Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchConsecutiveMinor.Version))
			})

			It("Kubernetes version should be updated: force update minor version and maintain feature gates and admission plugins", func() {
				testKubernetesVersionLowPatchLowMinor = gardencorev1beta1.ExpirableVersion{Version: "1.27.1", Classification: &deprecatedClassification}
				testKubernetesVersionHighestPatchLowMinor = gardencorev1beta1.ExpirableVersion{Version: "1.27.5", Classification: &deprecatedClassification}
				testKubernetesVersionLowPatchConsecutiveMinor = gardencorev1beta1.ExpirableVersion{Version: "1.28.1", Classification: &deprecatedClassification}
				testKubernetesVersionHighestPatchConsecutiveMinor = gardencorev1beta1.ExpirableVersion{Version: "1.28.5", Classification: &deprecatedClassification}

				var (
					// Use two actual feature gates from pkg/utils/validation/features/featuregates.go
					// which are supported in testKubernetesVersionHighestPatchLowMinor.Version
					// but not in testKubernetesVersionHighestPatchConsecutiveMinor.Version
					unsupportedfeatureGate1 = "AdvancedAuditing"
					unsupportedfeatureGate2 = "CSIStorageCapacity"
					// Use two feature gates which are supported in both versions
					supportedfeatureGate1 = "AppArmor"
					supportedfeatureGate2 = "AllBeta"

					supportedAdmissionPlugin1 = "AlwaysPullImages"
					supportedAdmissionPlugin2 = "AlwaysDeny"
					// No unsupported admission plugins are present as of now
				)

				patch := client.MergeFrom(cloudProfile.DeepCopy())
				cloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{
					testKubernetesVersionLowPatchLowMinor,
					testKubernetesVersionHighestPatchLowMinor,
					testKubernetesVersionLowPatchConsecutiveMinor,
					testKubernetesVersionHighestPatchConsecutiveMinor,
				}

				Expect(testClient.Patch(ctx, cloudProfile, patch)).To(Succeed())

				// set the shoots Kubernetes version to be the highest patch version of the minor version
				shoot127.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
					Version: testKubernetesVersionHighestPatchLowMinor.Version,
					KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
						KubernetesConfig: gardencorev1beta1.KubernetesConfig{
							FeatureGates: map[string]bool{
								supportedfeatureGate1:   true,
								unsupportedfeatureGate1: true,
								supportedfeatureGate2:   false,
							},
						},
						AdmissionPlugins: []gardencorev1beta1.AdmissionPlugin{
							{Name: supportedAdmissionPlugin1},
							{Name: supportedAdmissionPlugin2},
						},
					},
					KubeControllerManager: &gardencorev1beta1.KubeControllerManagerConfig{
						KubernetesConfig: gardencorev1beta1.KubernetesConfig{
							FeatureGates: map[string]bool{
								supportedfeatureGate1:   true,
								unsupportedfeatureGate1: true,
							},
						},
					},
					KubeScheduler: &gardencorev1beta1.KubeSchedulerConfig{
						KubernetesConfig: gardencorev1beta1.KubernetesConfig{
							FeatureGates: map[string]bool{
								supportedfeatureGate2:   true,
								unsupportedfeatureGate1: true,
								unsupportedfeatureGate2: true,
							},
						},
					},
					KubeProxy: &gardencorev1beta1.KubeProxyConfig{
						KubernetesConfig: gardencorev1beta1.KubernetesConfig{
							FeatureGates: map[string]bool{
								supportedfeatureGate1:   true,
								supportedfeatureGate2:   false,
								unsupportedfeatureGate2: true,
							},
						},
					},
					Kubelet: &gardencorev1beta1.KubeletConfig{
						KubernetesConfig: gardencorev1beta1.KubernetesConfig{
							FeatureGates: map[string]bool{
								supportedfeatureGate1:   true,
								unsupportedfeatureGate1: true,
								unsupportedfeatureGate2: true,
							},
						},
					},
				}

				By("Create k8s v1.27 Shoot")
				Expect(testClient.Create(ctx, shoot127)).To(Succeed())
				log.Info("Created shoot with k8s v1.27 for test", "shoot", client.ObjectKeyFromObject(shoot127))

				DeferCleanup(func() {
					By("Delete Shoot with k8s v1.27")
					Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot127))).To(Succeed())
				})

				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, *shoot127.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(*shoot127.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot127, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				// expect shoot to have updated to latest patch version of next minor version
				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot127), shoot127)).To(Succeed())
					g.Expect(shoot127.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot127.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"1.27.5\" to \"1.28.5\". Reason: Kubernetes version expired - force update required"))

					g.Expect(shoot127.Status.LastMaintenance.Description).To(ContainSubstring("Removed feature gates from \"spec.kubernetes.kubeAPIServer.featureGates\" because they are not supported in Kubernetes version \"1.28.5\": AdvancedAuditing"))
					g.Expect(shoot127.Spec.Kubernetes.KubeAPIServer.FeatureGates).To(Equal(map[string]bool{
						supportedfeatureGate1: true,
						supportedfeatureGate2: false,
					}))
					g.Expect(shoot127.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins).To(ConsistOf(
						HaveField("Name", Equal(supportedAdmissionPlugin1)),
						HaveField("Name", Equal(supportedAdmissionPlugin2)),
					))

					g.Expect(shoot127.Status.LastMaintenance.Description).To(ContainSubstring("Removed feature gates from \"spec.kubernetes.kubeControllerManager.featureGates\" because they are not supported in Kubernetes version \"1.28.5\": AdvancedAuditing"))
					g.Expect(shoot127.Spec.Kubernetes.KubeControllerManager.FeatureGates).To(Equal(map[string]bool{
						supportedfeatureGate1: true,
					}))

					g.Expect(shoot127.Status.LastMaintenance.Description).To(ContainSubstring("Removed feature gates from \"spec.kubernetes.kubeScheduler.featureGates\" because they are not supported in Kubernetes version \"1.28.5\": AdvancedAuditing, CSIStorageCapacity"))
					g.Expect(shoot127.Spec.Kubernetes.KubeScheduler.FeatureGates).To(Equal(map[string]bool{
						supportedfeatureGate2: true,
					}))

					g.Expect(shoot127.Status.LastMaintenance.Description).To(ContainSubstring("Removed feature gates from \"spec.kubernetes.kubeProxy.featureGates\" because they are not supported in Kubernetes version \"1.28.5\": CSIStorageCapacity"))
					g.Expect(shoot127.Spec.Kubernetes.KubeProxy.FeatureGates).To(Equal(map[string]bool{
						supportedfeatureGate1: true,
						supportedfeatureGate2: false,
					}))

					g.Expect(shoot127.Status.LastMaintenance.Description).To(ContainSubstring("Removed feature gates from \"spec.kubernetes.kubelet.featureGates\" because they are not supported in Kubernetes version \"1.28.5\": AdvancedAuditing, CSIStorageCapacity"))
					g.Expect(shoot127.Spec.Kubernetes.Kubelet.FeatureGates).To(Equal(map[string]bool{
						supportedfeatureGate1: true,
					}))

					g.Expect(shoot127.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot127.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))

					return shoot127.Spec.Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchConsecutiveMinor.Version))
			})
		}

		Context("Shoot with worker", func() {
			test()

			It("Kubernetes version should be updated: force update minor version (>= 1.30) and change swap behaviour", func() {
				testKubernetesVersionLowPatchLowMinor = gardencorev1beta1.ExpirableVersion{Version: "1.29.1", Classification: &deprecatedClassification}
				testKubernetesVersionHighestPatchLowMinor = gardencorev1beta1.ExpirableVersion{Version: "1.29.5", Classification: &deprecatedClassification}
				testKubernetesVersionLowPatchConsecutiveMinor = gardencorev1beta1.ExpirableVersion{Version: "1.30.1", Classification: &deprecatedClassification}
				testKubernetesVersionHighestPatchConsecutiveMinor = gardencorev1beta1.ExpirableVersion{Version: "1.30.5", Classification: &deprecatedClassification}

				patch := client.MergeFrom(cloudProfile.DeepCopy())
				cloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{
					testKubernetesVersionLowPatchLowMinor,
					testKubernetesVersionHighestPatchLowMinor,
					testKubernetesVersionLowPatchConsecutiveMinor,
					testKubernetesVersionHighestPatchConsecutiveMinor,
				}

				Expect(testClient.Patch(ctx, cloudProfile, patch)).To(Succeed())

				// set the shoots Kubernetes version to be the highest patch version of the minor version
				shoot129.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
					Version: testKubernetesVersionHighestPatchLowMinor.Version,
					Kubelet: &gardencorev1beta1.KubeletConfig{
						KubernetesConfig: gardencorev1beta1.KubernetesConfig{
							FeatureGates: map[string]bool{
								"NodeSwap": true,
							},
						},
						FailSwapOn: ptr.To(false),
						MemorySwap: &gardencorev1beta1.MemorySwapConfiguration{
							SwapBehavior: ptr.To(gardencorev1beta1.UnlimitedSwap),
						},
					},
				}

				By("Create k8s v1.29 Shoot")
				Expect(testClient.Create(ctx, shoot129)).To(Succeed())
				log.Info("Created shoot with k8s v1.29 for test", "shoot", client.ObjectKeyFromObject(shoot129))

				DeferCleanup(func() {
					By("Delete Shoot with k8s v1.29")
					Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot129))).To(Succeed())
				})

				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, *shoot129.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(*shoot129.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot129, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				// expect shoot to have updated to latest patch version of next minor version
				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot129), shoot129)).To(Succeed())
					g.Expect(shoot129.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot129.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"1.29.5\" to \"1.30.5\". Reason: Kubernetes version expired - force update required"))
					g.Expect(shoot129.Status.LastMaintenance.Description).To(ContainSubstring("spec.kubernetes.kubelet.memorySwap.swapBehavior is set to 'LimitedSwap'. Reason: 'UnlimitedSwap' cannot be used for Kubernetes version 1.30 and higher."))

					g.Expect(shoot129.Spec.Kubernetes.Kubelet.MemorySwap.SwapBehavior).To(Equal(ptr.To(gardencorev1beta1.LimitedSwap)))

					g.Expect(shoot129.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot129.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))

					return shoot129.Spec.Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchConsecutiveMinor.Version))
			})

			It("Kubernetes version should be updated: force update minor version (>= 1.30) and change swap behaviour for a worker pool", func() {
				testKubernetesVersionLowPatchLowMinor = gardencorev1beta1.ExpirableVersion{Version: "1.29.1", Classification: &deprecatedClassification}
				testKubernetesVersionHighestPatchLowMinor = gardencorev1beta1.ExpirableVersion{Version: "1.29.5", Classification: &deprecatedClassification}
				testKubernetesVersionLowPatchConsecutiveMinor = gardencorev1beta1.ExpirableVersion{Version: "1.30.1", Classification: &deprecatedClassification}
				testKubernetesVersionHighestPatchConsecutiveMinor = gardencorev1beta1.ExpirableVersion{Version: "1.30.5", Classification: &deprecatedClassification}

				patch := client.MergeFrom(cloudProfile.DeepCopy())
				cloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{
					testKubernetesVersionLowPatchLowMinor,
					testKubernetesVersionHighestPatchLowMinor,
					testKubernetesVersionLowPatchConsecutiveMinor,
					testKubernetesVersionHighestPatchConsecutiveMinor,
				}

				Expect(testClient.Patch(ctx, cloudProfile, patch)).To(Succeed())

				// set the shoots Kubernetes version to be the highest patch version of the minor version
				shoot129.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
					Version: testKubernetesVersionHighestPatchLowMinor.Version,
				}
				shoot129.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{
					Kubelet: &gardencorev1beta1.KubeletConfig{
						KubernetesConfig: gardencorev1beta1.KubernetesConfig{
							FeatureGates: map[string]bool{
								"NodeSwap": true,
							},
						},
						FailSwapOn: ptr.To(false),
						MemorySwap: &gardencorev1beta1.MemorySwapConfiguration{
							SwapBehavior: ptr.To(gardencorev1beta1.UnlimitedSwap),
						},
					},
				}

				By("Create k8s v1.29 Shoot")
				Expect(testClient.Create(ctx, shoot129)).To(Succeed())
				log.Info("Created shoot with k8s v1.29 for test", "shoot", client.ObjectKeyFromObject(shoot129))

				DeferCleanup(func() {
					By("Delete Shoot with k8s v1.29")
					Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot129))).To(Succeed())
				})

				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, *shoot129.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(*shoot129.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot129, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				// expect shoot to have updated to latest patch version of next minor version
				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot129), shoot129)).To(Succeed())
					g.Expect(shoot129.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot129.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"1.29.5\" to \"1.30.5\". Reason: Kubernetes version expired - force update required"))
					g.Expect(shoot129.Status.LastMaintenance.Description).To(ContainSubstring("spec.provider.workers[0].kubernetes.kubelet.memorySwap.swapBehavior is set to 'LimitedSwap'. Reason: 'UnlimitedSwap' cannot be used for Kubernetes version 1.30 and higher."))

					g.Expect(shoot129.Spec.Provider.Workers[0].Kubernetes.Kubelet.MemorySwap.SwapBehavior).To(Equal(ptr.To(gardencorev1beta1.LimitedSwap)))

					g.Expect(shoot129.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot129.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))

					return shoot129.Spec.Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchConsecutiveMinor.Version))
			})

			It("Kubernetes version should be updated: force update minor version (>= 1.31) and move systemReserved to kubeReserved", func() {
				shoot130.Spec.Kubernetes.Version = "1.30.0"
				shoot130.Spec.Kubernetes.Kubelet = &gardencorev1beta1.KubeletConfig{
					SystemReserved: &gardencorev1beta1.KubeletConfigReserved{
						CPU: resource.NewQuantity(50, resource.DecimalSI), Memory: resource.NewQuantity(55, resource.DecimalSI), EphemeralStorage: resource.NewQuantity(60, resource.DecimalSI),
					},
					KubeReserved: &gardencorev1beta1.KubeletConfigReserved{
						CPU: resource.NewQuantity(100, resource.DecimalSI), Memory: resource.NewQuantity(105, resource.DecimalSI), PID: resource.NewQuantity(10, resource.DecimalSI),
					},
				}

				By("Create k8s v1.30 Shoot")
				Expect(testClient.Create(ctx, shoot130)).To(Succeed())
				log.Info("Created shoot with k8s v1.30 for test", "shoot", client.ObjectKeyFromObject(shoot))

				DeferCleanup(func() {
					By("Delete Shoot with k8s v1.30")
					Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot130))).To(Succeed())
				})

				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, *shoot130.Spec.CloudProfileName, "1.30.0", &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(*shoot130.Spec.CloudProfileName, "1.30.0", &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot130, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot130), shoot130)).To(Succeed())
					g.Expect(shoot130.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot130.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"1.30.0\" to \"1.31.0\". Reason: Kubernetes version expired - force update required, .spec.kubernetes.kubelet.systemReserved is added to .spec.kubernetes.kubelet.kubeReserved. Reason: The systemReserved field is forbidden for Shoot clusters using Kubernetes version 1.31+, its value has to be added to kubeReserved"))
					g.Expect(shoot130.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot130.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot130.Spec.Kubernetes.Kubelet.KubeReserved).To(Equal(&gardencorev1beta1.KubeletConfigReserved{CPU: ptr.To(resource.MustParse("150")), Memory: ptr.To(resource.MustParse("160")), EphemeralStorage: ptr.To(resource.MustParse("60")), PID: ptr.To(resource.MustParse("10"))}))
					g.Expect(shoot130.Spec.Kubernetes.Kubelet.SystemReserved).To(BeNil())
					return shoot130.Spec.Kubernetes.Version
				}).Should(Equal("1.31.0"))
			})

			It("Kubernetes version should be updated: force update minor version (>= 1.31) and move systemReserved to kubeReserved for a worker pool", func() {
				shoot130.Spec.Kubernetes.Version = "1.30.0"
				shoot130.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{
					Kubelet: &gardencorev1beta1.KubeletConfig{
						SystemReserved: &gardencorev1beta1.KubeletConfigReserved{
							CPU: resource.NewQuantity(50, resource.DecimalSI), Memory: resource.NewQuantity(55, resource.DecimalSI), EphemeralStorage: resource.NewQuantity(60, resource.DecimalSI),
						},
						KubeReserved: &gardencorev1beta1.KubeletConfigReserved{
							CPU: resource.NewQuantity(100, resource.DecimalSI), Memory: resource.NewQuantity(105, resource.DecimalSI), PID: resource.NewQuantity(10, resource.DecimalSI),
						},
					},
				}

				By("Create k8s v1.30 Shoot")
				Expect(testClient.Create(ctx, shoot130)).To(Succeed())
				log.Info("Created shoot with k8s v1.30 for test", "shoot", client.ObjectKeyFromObject(shoot))

				DeferCleanup(func() {
					By("Delete Shoot with k8s v1.30")
					Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot130))).To(Succeed())
				})

				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, *shoot130.Spec.CloudProfileName, "1.30.0", &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(*shoot130.Spec.CloudProfileName, "1.30.0", &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot130, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot130), shoot130)).To(Succeed())
					g.Expect(shoot130.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot130.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"1.30.0\" to \"1.31.0\". Reason: Kubernetes version expired - force update required, .spec.provider.workers[0].kubernetes.kubelet.systemReserved is added to .spec.provider.workers[0].kubernetes.kubelet.kubeReserved. Reason: The systemReserved field is forbidden for Shoot clusters using Kubernetes version 1.31+, its value has to be added to kubeReserved"))
					g.Expect(shoot130.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot130.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot130.Spec.Provider.Workers[0].Kubernetes.Kubelet.KubeReserved).To(Equal(&gardencorev1beta1.KubeletConfigReserved{CPU: ptr.To(resource.MustParse("150")), Memory: ptr.To(resource.MustParse("160")), EphemeralStorage: ptr.To(resource.MustParse("60")), PID: ptr.To(resource.MustParse("10"))}))
					g.Expect(shoot130.Spec.Provider.Workers[0].Kubernetes.Kubelet.SystemReserved).To(BeNil())
					return shoot130.Spec.Kubernetes.Version
				}).Should(Equal("1.31.0"))
			})

			It("Kubernetes version should be updated: force update minor version(>= v1.33) and set spec.kubernetes.kubeControllerManager.podEvictionTimeout to nil", func() {
				shoot132.Spec.Kubernetes.Version = "1.32.0"
				shoot132.Spec.Kubernetes.KubeControllerManager = &gardencorev1beta1.KubeControllerManagerConfig{
					PodEvictionTimeout: &metav1.Duration{Duration: time.Minute},
				}

				By("Create k8s v1.32 Shoot")
				Expect(testClient.Create(ctx, shoot132)).To(Succeed())
				log.Info("Created shoot with k8s v1.32 for test", "shoot", client.ObjectKeyFromObject(shoot))

				DeferCleanup(func() {
					By("Delete Shoot with k8s v1.32")
					Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot132))).To(Succeed())
				})

				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, *shoot132.Spec.CloudProfileName, "1.32.0", &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(*shoot132.Spec.CloudProfileName, "1.32.0", &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot132, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot132), shoot132)).To(Succeed())
					g.Expect(shoot132.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot132.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"1.32.0\" to \"1.33.0\". Reason: Kubernetes version expired - force update required, .spec.kubernetes.kubeControllerManager.podEvictionTimeout is set to nil. Reason: The field was deprecated in favour of `spec.kubernetes.kubeAPIServer.defaultNotReadyTolerationSeconds` and `spec.kubernetes.kubeAPIServer.defaultUnreachableTolerationSeconds` and can no longer be enabled for Shoot clusters using Kubernetes version 1.33+"))
					g.Expect(shoot132.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot132.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot132.Spec.Kubernetes.KubeControllerManager.PodEvictionTimeout).To(BeNil())
					return shoot132.Spec.Kubernetes.Version
				}).Should(Equal("1.33.0"))
			})
		})

		Context("Workerless Shoot", func() {
			BeforeEach(func() {
				shoot.Spec.Provider.Workers = nil
			})

			test()
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
				shoot.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: ptr.To(testKubernetesVersionLowPatchLowMinor.Version)}
				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("All maintenance operations successful. Control Plane: Updated Kubernetes version from \"0.0.1\" to \"0.0.5\". Reason: Automatic update of Kubernetes version configured, Worker pool \"cpu-worker1\": Updated Kubernetes version from \"0.0.1\" to \"0.0.5\". Reason: Automatic update of Kubernetes version configured"))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					return *shoot.Spec.Provider.Workers[0].Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchLowMinor.Version))
			})

			It("Kubernetes version should be updated: force update patch version", func() {
				// expire the Shoot's Kubernetes version because autoupdate is set to false
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: ptr.To(testKubernetesVersionLowPatchLowMinor.Version)}
				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, *shoot.Spec.CloudProfileName, testKubernetesVersionLowPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(*shoot.Spec.CloudProfileName, testKubernetesVersionLowPatchLowMinor.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"0.0.1\" to \"0.0.5\". Reason: Kubernetes version expired - force update required, Worker pool \"cpu-worker1\": Updated Kubernetes version from \"0.0.1\" to \"0.0.5\". Reason: Kubernetes version expired - force update required"))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					return *shoot.Spec.Provider.Workers[0].Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchLowMinor.Version))
			})

			It("Kubernetes version should be updated: force update minor version", func() {
				// set the shoots Kubernetes version to be the highest patch version of the minor version
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.Kubernetes.Version = testKubernetesVersionHighestPatchLowMinor.Version
				shoot.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: ptr.To(testKubernetesVersionHighestPatchLowMinor.Version)}

				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, *shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(*shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				// expect worker pool to have updated to latest patch version of next minor version
				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"0.0.5\" to \"0.1.5\". Reason: Kubernetes version expired - force update required, Worker pool \"cpu-worker1\": Updated Kubernetes version from \"0.0.5\" to \"0.1.5\". Reason: Kubernetes version expired - force update required"))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					return *shoot.Spec.Provider.Workers[0].Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchConsecutiveMinor.Version))
			})

			It("Worker Pool Kubernetes version should be updated: force update minor version and maintain feature gates", func() {
				testKubernetesVersionLowPatchLowMinor = gardencorev1beta1.ExpirableVersion{Version: "1.27.1", Classification: &deprecatedClassification}
				testKubernetesVersionHighestPatchLowMinor = gardencorev1beta1.ExpirableVersion{Version: "1.27.5", Classification: &deprecatedClassification}
				testKubernetesVersionLowPatchConsecutiveMinor = gardencorev1beta1.ExpirableVersion{Version: "1.28.1", Classification: &deprecatedClassification}
				testKubernetesVersionHighestPatchConsecutiveMinor = gardencorev1beta1.ExpirableVersion{Version: "1.28.5", Classification: &deprecatedClassification}

				var (
					// Use two actual feature gates from pkg/utils/validation/features/featuregates.go
					// which are supported in testKubernetesVersionHighestPatchLowMinor.Version
					// but not in testKubernetesVersionHighestPatchConsecutiveMinor.Version
					unsupportedfeatureGate1 = "AdvancedAuditing"
					unsupportedfeatureGate2 = "CSIStorageCapacity"
					// Use two feature gates which are supported in both versions
					supportedfeatureGate1 = "AppArmor"
					supportedfeatureGate2 = "AllBeta"
				)

				patch := client.MergeFrom(cloudProfile.DeepCopy())
				cloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{
					testKubernetesVersionLowPatchLowMinor,
					testKubernetesVersionHighestPatchLowMinor,
					testKubernetesVersionLowPatchConsecutiveMinor,
					testKubernetesVersionHighestPatchConsecutiveMinor,
				}

				Expect(testClient.Patch(ctx, cloudProfile, patch)).To(Succeed())

				shoot127.Spec.Kubernetes.Version = testKubernetesVersionHighestPatchLowMinor.Version
				shoot127.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{
					Version: ptr.To(testKubernetesVersionHighestPatchLowMinor.Version),
					Kubelet: &gardencorev1beta1.KubeletConfig{
						KubernetesConfig: gardencorev1beta1.KubernetesConfig{
							FeatureGates: map[string]bool{
								supportedfeatureGate1:   true,
								supportedfeatureGate2:   false,
								unsupportedfeatureGate1: true,
								unsupportedfeatureGate2: true,
							},
						},
					},
				}
				shoot127.Spec.Provider.Workers[1].Kubernetes = &gardencorev1beta1.WorkerKubernetes{
					Kubelet: &gardencorev1beta1.KubeletConfig{
						KubernetesConfig: gardencorev1beta1.KubernetesConfig{
							FeatureGates: map[string]bool{
								supportedfeatureGate1:   true,
								unsupportedfeatureGate1: true,
							},
						},
					},
				}

				By("Create k8s v1.27 Shoot")
				Expect(testClient.Create(ctx, shoot127)).To(Succeed())
				log.Info("Created shoot with k8s v1.27 for test", "shoot", client.ObjectKeyFromObject(shoot127))

				DeferCleanup(func() {
					By("Delete Shoot with k8s v1.27")
					Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot127))).To(Succeed())
				})

				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, *shoot127.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(*shoot127.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot127, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				// expect worker pool to have updated to latest patch version of next minor version
				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot127), shoot127)).To(Succeed())
					g.Expect(shoot127.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot127.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"1.27.5\" to \"1.28.5\". Reason: Kubernetes version expired - force update required"))
					g.Expect(shoot127.Status.LastMaintenance.Description).To(ContainSubstring("Worker pool \"cpu-worker1\": Updated Kubernetes version from \"1.27.5\" to \"1.28.5\". Reason: Kubernetes version expired - force update required"))
					g.Expect(shoot127.Status.LastMaintenance.Description).To(ContainSubstring(" Removed feature gates from \"spec.provider.workers[0].kubernetes.kubelet.featureGates\" because they are not supported in Kubernetes version \"1.28.5\": AdvancedAuditing, CSIStorageCapacity"))
					g.Expect(shoot127.Spec.Provider.Workers[0].Kubernetes.Kubelet.FeatureGates).To(Equal(map[string]bool{
						supportedfeatureGate1: true,
						supportedfeatureGate2: false,
					}))

					g.Expect(shoot127.Status.LastMaintenance.Description).To(ContainSubstring(" Removed feature gates from \"spec.provider.workers[1].kubernetes.kubelet.featureGates\" because they are not supported in Kubernetes version \"1.28.5\": AdvancedAuditing"))
					g.Expect(shoot127.Spec.Provider.Workers[1].Kubernetes.Kubelet.FeatureGates).To(Equal(map[string]bool{
						supportedfeatureGate1: true,
					}))

					g.Expect(shoot127.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot127.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))

					g.Expect(shoot127.Spec.Kubernetes.Version).To(Equal(testKubernetesVersionHighestPatchConsecutiveMinor.Version))

					return *shoot127.Spec.Provider.Workers[0].Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchConsecutiveMinor.Version))
			})

			It("Worker Pool Kubernetes version should be updated, but control plane version stays: force update minor of worker pool version", func() {
				// set the shoots Kubernetes version to be the highest patch version of the minor version
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.Kubernetes.Version = testKubernetesVersionLowPatchConsecutiveMinor.Version
				shoot.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: ptr.To(testKubernetesVersionHighestPatchLowMinor.Version)}

				Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

				By("Expire Shoot's worker pool kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, *shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(*shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				// expect worker pool to have updated to latest patch version of next minor version
				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
					g.Expect(shoot.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot.Status.LastMaintenance.Description).To(ContainSubstring("Worker pool \"cpu-worker1\": Updated Kubernetes version from \"0.0.5\" to \"0.1.1\". Reason: Kubernetes version expired - force update required"))
					g.Expect(shoot.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					return *shoot.Spec.Provider.Workers[0].Kubernetes.Version
				}).Should(Equal(testKubernetesVersionLowPatchConsecutiveMinor.Version))
			})

			It("Worker Pool Kubernetes version should be updated, but control plane version stays: force update minor version and maintain feature gates", func() {
				testKubernetesVersionLowPatchLowMinor = gardencorev1beta1.ExpirableVersion{Version: "1.27.1", Classification: &deprecatedClassification}
				testKubernetesVersionHighestPatchLowMinor = gardencorev1beta1.ExpirableVersion{Version: "1.27.5", Classification: &deprecatedClassification}
				testKubernetesVersionLowPatchConsecutiveMinor = gardencorev1beta1.ExpirableVersion{Version: "1.28.1", Classification: &deprecatedClassification}
				testKubernetesVersionHighestPatchConsecutiveMinor = gardencorev1beta1.ExpirableVersion{Version: "1.28.5", Classification: &deprecatedClassification}

				var (
					// Use two actual feature gates from pkg/utils/validation/features/featuregates.go
					// which are supported in testKubernetesVersionHighestPatchLowMinor.Version
					// but not in testKubernetesVersionHighestPatchConsecutiveMinor.Version
					unsupportedfeatureGate1 = "AdvancedAuditing"
					unsupportedfeatureGate2 = "CSIStorageCapacity"
					// Use two feature gates which are supported in both versions
					supportedfeatureGate1 = "AppArmor"
					supportedfeatureGate2 = "AllBeta"
				)

				patch := client.MergeFrom(cloudProfile.DeepCopy())
				cloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{
					testKubernetesVersionLowPatchLowMinor,
					testKubernetesVersionHighestPatchLowMinor,
					testKubernetesVersionLowPatchConsecutiveMinor,
					testKubernetesVersionHighestPatchConsecutiveMinor,
				}

				Expect(testClient.Patch(ctx, cloudProfile, patch)).To(Succeed())

				shoot127.Spec.Kubernetes.Version = testKubernetesVersionLowPatchConsecutiveMinor.Version
				shoot127.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{
					Version: ptr.To(testKubernetesVersionHighestPatchLowMinor.Version),
					Kubelet: &gardencorev1beta1.KubeletConfig{
						KubernetesConfig: gardencorev1beta1.KubernetesConfig{
							FeatureGates: map[string]bool{
								supportedfeatureGate1:   true,
								supportedfeatureGate2:   false,
								unsupportedfeatureGate1: true,
								unsupportedfeatureGate2: true,
							},
						},
					},
				}
				shoot127.Spec.Provider.Workers[1].Kubernetes = &gardencorev1beta1.WorkerKubernetes{
					Version: ptr.To(testKubernetesVersionHighestPatchLowMinor.Version),
					Kubelet: &gardencorev1beta1.KubeletConfig{
						KubernetesConfig: gardencorev1beta1.KubernetesConfig{
							FeatureGates: map[string]bool{
								supportedfeatureGate1:   true,
								unsupportedfeatureGate1: true,
							},
						},
					},
				}

				By("Create k8s v1.27 Shoot")
				Expect(testClient.Create(ctx, shoot127)).To(Succeed())
				log.Info("Created shoot with k8s v1.27 for test", "shoot", client.ObjectKeyFromObject(shoot127))

				DeferCleanup(func() {
					By("Delete Shoot with k8s v1.27")
					Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot127))).To(Succeed())
				})

				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, *shoot127.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(*shoot127.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot127, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				// expect worker pool to have updated to latest patch version of next minor version
				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot127), shoot127)).To(Succeed())
					g.Expect(shoot127.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot127.Status.LastMaintenance.Description).To(ContainSubstring("Worker pool \"cpu-worker1\": Updated Kubernetes version from \"1.27.5\" to \"1.28.1\". Reason: Kubernetes version expired - force update required"))
					g.Expect(shoot127.Status.LastMaintenance.Description).To(ContainSubstring(" Removed feature gates from \"spec.provider.workers[0].kubernetes.kubelet.featureGates\" because they are not supported in Kubernetes version \"1.28.1\": AdvancedAuditing, CSIStorageCapacity"))
					g.Expect(shoot127.Spec.Provider.Workers[0].Kubernetes.Kubelet.FeatureGates).To(Equal(map[string]bool{
						supportedfeatureGate1: true,
						supportedfeatureGate2: false,
					}))

					g.Expect(shoot127.Status.LastMaintenance.Description).To(ContainSubstring(" Removed feature gates from \"spec.provider.workers[1].kubernetes.kubelet.featureGates\" because they are not supported in Kubernetes version \"1.28.1\": AdvancedAuditing"))
					g.Expect(shoot127.Spec.Provider.Workers[1].Kubernetes.Kubelet.FeatureGates).To(Equal(map[string]bool{
						supportedfeatureGate1: true,
					}))

					g.Expect(shoot127.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot127.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					return *shoot127.Spec.Provider.Workers[0].Kubernetes.Version
				}).Should(Equal(testKubernetesVersionLowPatchConsecutiveMinor.Version))
			})

			It("Worker Pool Kubernetes version should be updated: force update minor version (>= 1.30) and set swap behavior", func() {
				testKubernetesVersionLowPatchLowMinor = gardencorev1beta1.ExpirableVersion{Version: "1.29.1", Classification: &deprecatedClassification}
				testKubernetesVersionHighestPatchLowMinor = gardencorev1beta1.ExpirableVersion{Version: "1.29.5", Classification: &deprecatedClassification}
				testKubernetesVersionLowPatchConsecutiveMinor = gardencorev1beta1.ExpirableVersion{Version: "1.30.1", Classification: &deprecatedClassification}
				testKubernetesVersionHighestPatchConsecutiveMinor = gardencorev1beta1.ExpirableVersion{Version: "1.30.5", Classification: &deprecatedClassification}

				patch := client.MergeFrom(cloudProfile.DeepCopy())
				cloudProfile.Spec.Kubernetes.Versions = []gardencorev1beta1.ExpirableVersion{
					testKubernetesVersionLowPatchLowMinor,
					testKubernetesVersionHighestPatchLowMinor,
					testKubernetesVersionLowPatchConsecutiveMinor,
					testKubernetesVersionHighestPatchConsecutiveMinor,
				}

				Expect(testClient.Patch(ctx, cloudProfile, patch)).To(Succeed())

				shoot129.Spec.Kubernetes.Version = testKubernetesVersionHighestPatchLowMinor.Version
				shoot129.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{
					Version: ptr.To(testKubernetesVersionHighestPatchLowMinor.Version),
					Kubelet: &gardencorev1beta1.KubeletConfig{
						KubernetesConfig: gardencorev1beta1.KubernetesConfig{
							FeatureGates: map[string]bool{
								"NodeSwap": true,
							},
						},
						FailSwapOn: ptr.To(false),
						MemorySwap: &gardencorev1beta1.MemorySwapConfiguration{
							SwapBehavior: ptr.To(gardencorev1beta1.UnlimitedSwap),
						},
					},
				}
				shoot129.Spec.Provider.Workers[1].Kubernetes = &gardencorev1beta1.WorkerKubernetes{
					Kubelet: &gardencorev1beta1.KubeletConfig{
						KubernetesConfig: gardencorev1beta1.KubernetesConfig{
							FeatureGates: map[string]bool{
								"NodeSwap": true,
							},
						},
						FailSwapOn: ptr.To(false),
						MemorySwap: &gardencorev1beta1.MemorySwapConfiguration{
							SwapBehavior: ptr.To(gardencorev1beta1.LimitedSwap),
						},
					},
				}

				By("Create k8s v1.29 Shoot")
				Expect(testClient.Create(ctx, shoot129)).To(Succeed())
				log.Info("Created shoot with k8s v1.29 for test", "shoot", client.ObjectKeyFromObject(shoot129))

				DeferCleanup(func() {
					By("Delete Shoot with k8s v1.29")
					Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot129))).To(Succeed())
				})

				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, *shoot129.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(*shoot129.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot129, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				// expect worker pool to have updated to latest patch version of next minor version
				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot129), shoot129)).To(Succeed())
					g.Expect(shoot129.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot129.Status.LastMaintenance.Description).To(ContainSubstring("Control Plane: Updated Kubernetes version from \"1.29.5\" to \"1.30.5\". Reason: Kubernetes version expired - force update required"))
					g.Expect(shoot129.Status.LastMaintenance.Description).To(ContainSubstring("Worker pool \"cpu-worker1\": Updated Kubernetes version from \"1.29.5\" to \"1.30.5\". Reason: Kubernetes version expired - force update required"))
					g.Expect(shoot129.Status.LastMaintenance.Description).To(ContainSubstring("spec.provider.workers[0].kubernetes.kubelet.memorySwap.swapBehavior is set to 'LimitedSwap'. Reason: 'UnlimitedSwap' cannot be used for Kubernetes version 1.30 and higher."))
					g.Expect(shoot129.Status.LastMaintenance.Description).NotTo(ContainSubstring("spec.provider.workers[1]"))

					g.Expect(shoot129.Spec.Provider.Workers[0].Kubernetes.Kubelet.MemorySwap.SwapBehavior).To(Equal(ptr.To(gardencorev1beta1.LimitedSwap)))
					g.Expect(shoot129.Spec.Provider.Workers[1].Kubernetes.Kubelet.MemorySwap.SwapBehavior).To(Equal(ptr.To(gardencorev1beta1.LimitedSwap)))

					g.Expect(shoot129.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot129.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))

					g.Expect(shoot129.Spec.Kubernetes.Version).To(Equal(testKubernetesVersionHighestPatchConsecutiveMinor.Version))

					return *shoot129.Spec.Provider.Workers[0].Kubernetes.Version
				}).Should(Equal(testKubernetesVersionHighestPatchConsecutiveMinor.Version))
			})

			It("Kubernetes version should be updated: force update minor version (>= 1.31) and move systemReserved to kubeReserved", func() {
				shoot130.Spec.Kubernetes.Version = "1.30.0"
				shoot130.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{
					Version: ptr.To("1.30.0"),
					Kubelet: &gardencorev1beta1.KubeletConfig{
						SystemReserved: &gardencorev1beta1.KubeletConfigReserved{
							CPU: resource.NewQuantity(50, resource.DecimalSI), Memory: resource.NewQuantity(55, resource.DecimalSI), EphemeralStorage: resource.NewQuantity(60, resource.DecimalSI),
						},
						KubeReserved: &gardencorev1beta1.KubeletConfigReserved{
							CPU: resource.NewQuantity(100, resource.DecimalSI), Memory: resource.NewQuantity(105, resource.DecimalSI), PID: resource.NewQuantity(10, resource.DecimalSI),
						},
					},
				}

				By("Create k8s v1.30 Shoot")
				Expect(testClient.Create(ctx, shoot130)).To(Succeed())
				log.Info("Created shoot with k8s v1.30 for test", "shoot", client.ObjectKeyFromObject(shoot))

				DeferCleanup(func() {
					By("Delete Shoot with k8s v1.30")
					Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot130))).To(Succeed())
				})

				By("Expire Shoot's kubernetes version in the CloudProfile")
				Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, *shoot130.Spec.CloudProfileName, "1.30.0", &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

				By("Wait until manager has observed the CloudProfile update")
				waitKubernetesVersionToBeExpiredInCloudProfile(*shoot130.Spec.CloudProfileName, "1.30.0", &expirationDateInThePast)

				Expect(kubernetesutils.SetAnnotationAndUpdate(ctx, testClient, shoot130, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

				Eventually(func(g Gomega) string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot130), shoot130)).To(Succeed())
					g.Expect(shoot130.Status.LastMaintenance).NotTo(BeNil())
					g.Expect(shoot130.Status.LastMaintenance.Description).To(ContainSubstring("Worker pool \"cpu-worker1\": Updated Kubernetes version from \"1.30.0\" to \"1.31.0\". Reason: Kubernetes version expired - force update required, .spec.provider.workers[0].kubernetes.kubelet.systemReserved is added to .spec.provider.workers[0].kubernetes.kubelet.kubeReserved. Reason: The systemReserved field is forbidden for Shoot clusters using Kubernetes version 1.31+, its value has to be added to kubeReserved"))
					g.Expect(shoot130.Status.LastMaintenance.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					g.Expect(shoot130.Status.LastMaintenance.TriggeredTime).To(Equal(metav1.Time{Time: fakeClock.Now()}))
					g.Expect(shoot130.Spec.Provider.Workers[0].Kubernetes.Kubelet.KubeReserved).To(Equal(&gardencorev1beta1.KubeletConfigReserved{CPU: ptr.To(resource.MustParse("150")), Memory: ptr.To(resource.MustParse("160")), EphemeralStorage: ptr.To(resource.MustParse("60")), PID: ptr.To(resource.MustParse("10"))}))
					g.Expect(shoot130.Spec.Provider.Workers[0].Kubernetes.Kubelet.SystemReserved).To(BeNil())
					g.Expect(shoot130.Spec.Kubernetes.Version).To(Equal("1.31.0"))
					return *shoot130.Spec.Provider.Workers[0].Kubernetes.Version
				}).Should(Equal("1.31.0"))
			})
		})
	})
})
