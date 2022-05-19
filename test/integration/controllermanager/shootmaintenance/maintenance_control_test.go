// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot_maintenance_test

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	shootcontroller "github.com/gardener/gardener/pkg/controllermanager/controller/shoot"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/timewindow"
)

var _ = Describe("Shoot Maintenance controller tests", func() {

	var (
		project      *gardencorev1beta1.Project
		cloudProfile *gardencorev1beta1.CloudProfile
		shoot        *gardencorev1beta1.Shoot

		// Test Machine Image
		highestShootMachineImage gardencorev1beta1.ShootMachineImage
		testMachineImageVersion  = "0.0.1-beta"
		testMachineImage         = gardencorev1beta1.ShootMachineImage{
			Version: &testMachineImageVersion,
		}

		// other
		deprecatedClassification = gardencorev1beta1.ClassificationDeprecated
		supportedClassification  = gardencorev1beta1.ClassificationSupported
		expirationDateInThePast  = metav1.Time{Time: time.Now().UTC().AddDate(0, 0, -1)}

		// Test Kubernetes versions
		testKubernetesVersionLowPatchLowMinor             = gardencorev1beta1.ExpirableVersion{Version: "0.0.1", Classification: &deprecatedClassification}
		testKubernetesVersionHighestPatchLowMinor         = gardencorev1beta1.ExpirableVersion{Version: "0.0.5", Classification: &deprecatedClassification}
		testKubernetesVersionLowPatchConsecutiveMinor     = gardencorev1beta1.ExpirableVersion{Version: "0.1.1", Classification: &deprecatedClassification}
		testKubernetesVersionHighestPatchConsecutiveMinor = gardencorev1beta1.ExpirableVersion{Version: "0.1.5", Classification: &deprecatedClassification}
	)

	BeforeEach(func() {
		By("create project")
		project = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: "dev",
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: pointer.String("garden-dev"),
			},
		}
		Expect(testClient.Create(ctx, project)).To(Succeed())

		By("create cloud profile")
		cloudProfile = &gardencorev1beta1.CloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-cloudprofile",
			},
			Spec: gardencorev1beta1.CloudProfileSpec{
				Kubernetes: gardencorev1beta1.KubernetesSettings{
					Versions: []gardencorev1beta1.ExpirableVersion{
						{
							Version: "1.21.1",
						},
						testKubernetesVersionLowPatchLowMinor,
						testKubernetesVersionHighestPatchLowMinor,
						testKubernetesVersionLowPatchConsecutiveMinor,
						testKubernetesVersionHighestPatchConsecutiveMinor,
					},
				},
				MachineImages: []gardencorev1beta1.MachineImage{
					{
						Name: "foo-image",
						Versions: []gardencorev1beta1.MachineImageVersion{
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        "1.1.1",
									Classification: &supportedClassification,
								},
								CRI: []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        testMachineImageVersion,
									Classification: &deprecatedClassification,
								},
								CRI: []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
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
		Expect(testClient.Create(ctx, cloudProfile)).To(Succeed())

		By("create shoot")
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{Name: "test-shoot", Namespace: "garden-dev"},
			Spec: gardencorev1beta1.ShootSpec{
				SecretBindingName: "my-provider-account",
				CloudProfileName:  "test-cloudprofile",
				Region:            "foo-region",
				Provider: gardencorev1beta1.Provider{
					Type: "foo-provider",
					Workers: []gardencorev1beta1.Worker{
						{
							Name:    "cpu-worker",
							Minimum: 2,
							Maximum: 2,
							Machine: gardencorev1beta1.Machine{
								Image: &gardencorev1beta1.ShootMachineImage{
									Name:    "foo-image",
									Version: pointer.String("1.1.1"),
								},
								Type: "large",
							},
						},
					},
				},
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.21.1",
				},
				Networking: gardencorev1beta1.Networking{
					Type: "foo-networking",
				},
				Maintenance: &gardencorev1beta1.Maintenance{
					AutoUpdate: &gardencorev1beta1.MaintenanceAutoUpdate{
						KubernetesVersion:   false,
						MachineImageVersion: false,
					},
					TimeWindow: &gardencorev1beta1.MaintenanceTimeWindow{
						Begin: timewindow.NewMaintenanceTime(time.Now().Add(2*time.Hour).Hour(), 0, 0).Formatted(),
						End:   timewindow.NewMaintenanceTime(time.Now().Add(4*time.Hour).Hour(), 0, 0).Formatted(),
					},
				},
			},
		}

		// remember highest version of the image.
		highestShootMachineImage = *shoot.Spec.Provider.Workers[0].Machine.Image
		// set dummy kubernetes version to shoot
		shoot.Spec.Kubernetes.Version = testKubernetesVersionLowPatchLowMinor.Version
		// set shoot MachineImage version to testMachineImage
		shoot.Spec.Provider.Workers[0].Machine.Image.Version = testMachineImage.Version
		// remember the test machine image
		// also required to know for which image name the test versions should be added to the CloudProfile
		testMachineImage = *shoot.Spec.Provider.Workers[0].Machine.Image
		Expect(testClient.Create(ctx, shoot)).To(Succeed())
	})

	AfterEach(func() {
		logger.Infof("Delete shoot %s", shoot.Name)
		Expect(deleteShoot(ctx, testClient, shoot)).To(Succeed())

		logger.Infof("Delete CloudProfile %s", cloudProfile.Name)
		Expect(testClient.Delete(ctx, cloudProfile)).To(Succeed())

		logger.Infof("Delete Project %s", project.Name)
		Expect(deleteProject(ctx, testClient, project)).To(Succeed())
	})

	It("should add task annotations", func() {
		By("trigger maintenance")
		Expect(kutil.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

		waitForShootToBeMaintained(shoot)

		By("ensuring task annotations are present")
		Expect(shoot.Annotations).To(HaveKey("shoot.gardener.cloud/tasks"))
		Expect(strings.Split(shoot.Annotations["shoot.gardener.cloud/tasks"], ",")).To(And(
			ContainElement("deployInfrastructure"),
			ContainElement("deployDNSRecordInternal"),
			ContainElement("deployDNSRecordExternal"),
			ContainElement("deployDNSRecordIngress"),
		))
	})

	It("should unset the Shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.ResourceVersion field", func() {
		By("prepare shoot")
		patch := client.MergeFrom(shoot.DeepCopy())
		shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
			AuditConfig: &gardencorev1beta1.AuditConfig{
				AuditPolicy: &gardencorev1beta1.AuditPolicy{
					ConfigMapRef: &corev1.ObjectReference{
						Name:            "auditpolicy",
						ResourceVersion: "123456",
					},
				},
			},
		}
		Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

		By("trigger maintenance")
		Expect(kutil.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

		waitForShootToBeMaintained(shoot)

		By("ensuring resourceVersion is empty")
		Expect(shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.ResourceVersion).To(BeEmpty())
	})

	Describe("Machine image maintenance tests", func() {
		It("Do not update Shoot machine image in maintenance time: AutoUpdate.MachineImageVersion == false && expirationDate does not apply", func() {
			Expect(kutil.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

			Expect(waitForExpectedMachineImageMaintenance(ctx, logger, testClient, shoot, testMachineImage, false, time.Now().Add(time.Second*10))).To(Succeed())
		})

		It("Shoot machine image must be updated in maintenance time: AutoUpdate.MachineImageVersion == true && expirationDate does not apply", func() {
			// set test specific shoot settings
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = true
			Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

			Expect(kutil.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

			Expect(waitForExpectedMachineImageMaintenance(ctx, logger, testClient, shoot, highestShootMachineImage, true, time.Now().Add(time.Second*20))).To(Succeed())
		})

		It("Shoot machine image must be updated in maintenance time: AutoUpdate.MachineImageVersion == false && expirationDate applies", func() {
			// expire the Shoot's machine image
			Expect(patchCloudProfileForMachineImageMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, testMachineImage, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

			Expect(kutil.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

			Expect(waitForExpectedMachineImageMaintenance(ctx, logger, testClient, shoot, highestShootMachineImage, true, time.Now().Add(time.Minute*1))).To(Succeed())
		})
	})

	Describe("Kubernetes version maintenance tests", func() {
		It("Kubernetes version should not be updated: auto update not enabled", func() {
			Expect(kutil.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

			Expect(waitForExpectedKubernetesVersionMaintenance(ctx, logger, testClient, shoot, testKubernetesVersionLowPatchLowMinor.Version, false, time.Now().Add(time.Second*10))).To(Succeed())
		})

		It("Kubernetes version should be updated: auto update enabled", func() {
			// set test specific shoot settings
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = true
			Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

			Expect(kutil.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

			Expect(waitForExpectedKubernetesVersionMaintenance(ctx, logger, testClient, shoot, testKubernetesVersionHighestPatchLowMinor.Version, true, time.Now().Add(time.Second*20))).To(Succeed())
		})

		It("Kubernetes version should be updated: force update patch version", func() {
			// expire the Shoot's Kubernetes version
			Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, testKubernetesVersionLowPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

			Expect(kutil.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

			Expect(waitForExpectedKubernetesVersionMaintenance(ctx, logger, testClient, shoot, testKubernetesVersionHighestPatchLowMinor.Version, true, time.Now().Add(time.Second*20))).To(Succeed())
		})

		It("Kubernetes version should be updated: force update minor version", func() {
			// set the shoots Kubernetes version to be the highest patch version of the minor version
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Spec.Kubernetes.Version = testKubernetesVersionHighestPatchLowMinor.Version
			Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

			// let Shoot's Kubernetes version expire
			Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

			Expect(kutil.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

			// expect shoot to have updated to latest patch version of next minor version
			Expect(waitForExpectedKubernetesVersionMaintenance(ctx, logger, testClient, shoot, testKubernetesVersionHighestPatchConsecutiveMinor.Version, true, time.Now().Add(time.Second*20))).To(Succeed())
		})
	})

	Describe("Worker Pool Kubernetes version maintenance tests", func() {
		It("Kubernetes version should not be updated: auto update not enabled", func() {
			Expect(kutil.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

			Expect(waitForExpectedKubernetesVersionMaintenance(ctx, logger, testClient, shoot, testKubernetesVersionLowPatchLowMinor.Version, false, time.Now().Add(time.Second*10))).To(Succeed())
		})

		It("Kubernetes version should be updated: auto update enabled", func() {
			// set test specific shoot settings
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = true
			shoot.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: pointer.String(testKubernetesVersionLowPatchLowMinor.Version)}
			Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

			Expect(kutil.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

			Eventually(func() string {
				err := testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
				Expect(err).NotTo(HaveOccurred())
				return *shoot.Spec.Provider.Workers[0].Kubernetes.Version
			}).Should(Equal(testKubernetesVersionHighestPatchLowMinor.Version))
		})

		It("Kubernetes version should be updated: force update patch version", func() {
			// expire the Shoot's Kubernetes version because autoupdate is set to false
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: pointer.String(testKubernetesVersionLowPatchLowMinor.Version)}
			Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())
			Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, testKubernetesVersionLowPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

			Expect(kutil.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

			Eventually(func() string {
				err := testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
				Expect(err).NotTo(HaveOccurred())
				return *shoot.Spec.Provider.Workers[0].Kubernetes.Version
			}).Should(Equal(testKubernetesVersionHighestPatchLowMinor.Version))
		})

		It("Kubernetes version should be updated: force update minor version", func() {
			// set the shoots Kubernetes version to be the highest patch version of the minor version
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Spec.Kubernetes.Version = testKubernetesVersionHighestPatchLowMinor.Version
			shoot.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: pointer.String(testKubernetesVersionHighestPatchLowMinor.Version)}

			Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

			// let Shoot's Kubernetes version expire
			Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

			Expect(kutil.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

			// expect worker pool to have updated to latest patch version of next minor version
			Eventually(func() string {
				err := testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
				Expect(err).NotTo(HaveOccurred())
				return *shoot.Spec.Provider.Workers[0].Kubernetes.Version
			}).Should(Equal(testKubernetesVersionHighestPatchConsecutiveMinor.Version))
		})

		It("Worker Pool Kubernetes version should be updated, but control plane version stays: force update minor of worker pool version", func() {
			// set the shoots Kubernetes version to be the highest patch version of the minor version
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Spec.Kubernetes.Version = testKubernetesVersionLowPatchConsecutiveMinor.Version
			shoot.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{Version: pointer.String(testKubernetesVersionHighestPatchLowMinor.Version)}

			Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

			// let Shoot's Kubernetes version expire
			Expect(patchCloudProfileForKubernetesVersionMaintenance(ctx, testClient, shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)).To(Succeed())

			Expect(kutil.SetAnnotationAndUpdate(ctx, testClient, shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)).To(Succeed())

			// expect worker pool to have updated to latest patch version of next minor version
			Eventually(func() string {
				err := testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
				Expect(err).NotTo(HaveOccurred())
				return *shoot.Spec.Provider.Workers[0].Kubernetes.Version
			}).Should(Equal(testKubernetesVersionLowPatchConsecutiveMinor.Version))

			Expect(shoot.Spec.Kubernetes.Version).To(Equal(testKubernetesVersionLowPatchConsecutiveMinor.Version))
		})
	})
})

func addShootMaintenanceControllerToManager(mgr manager.Manager) error {
	recorder := mgr.GetEventRecorderFor("shoot-maintenance-controller")
	concurrentSyncs := 1

	c, err := controller.New(
		"shoot-maintenance-controller",
		mgr,
		controller.Options{
			Reconciler: shootcontroller.NewShootMaintenanceReconciler(logger, testClient, config.ShootMaintenanceControllerConfiguration{ConcurrentSyncs: &concurrentSyncs}, recorder),
		},
	)
	if err != nil {
		return err
	}

	return c.Watch(&source.Kind{Type: &gardencorev1beta1.Shoot{}}, &handler.EnqueueRequestForObject{})
}
