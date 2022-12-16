// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package upgrade

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gardener/etcd-druid/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/utils/test/matchers"
	e2e "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/framework"
	shootupdatesuite "github.com/gardener/gardener/test/utils/shoots/update"
	"github.com/gardener/gardener/test/utils/shoots/update/highavailability"
)

var _ = Describe("Gardener upgrade Tests for", func() {
	var (
		gardenerPreviousVersion    = os.Getenv("GARDENER_PREVIOUS_VERSION")
		gardenerPreviousGitVersion = os.Getenv("GARDENER_PREVIOUS_RELEASE")
		gardenerCurrentVersion     = os.Getenv("GARDENER_NEXT_VERSION")
		gardenerCurrentGitVersion  = os.Getenv("GARDENER_NEXT_RELEASE")
		projectNamespace           = "garden-local"
	)

	Context("Shoot::e2e-upgrade", func() {
		var (
			parentCtx       = context.Background()
			job             *batchv1.Job
			err             error
			shootTest       = e2e.DefaultShoot("e2e-upgrade")
			f               = framework.NewShootCreationFramework(&framework.ShootCreationConfig{GardenerConfig: e2e.DefaultGardenConfig(projectNamespace)})
			etcdMainPodName = getEtcdMainMemberLastOrdinalPodName(shootTest)
		)

		shootTest.Namespace = projectNamespace
		f.Shoot = shootTest

		When("Pre-Upgrade (Gardener version:'"+gardenerPreviousVersion+"', Git version:'"+gardenerPreviousGitVersion+"')", Ordered, Label("pre-upgrade"), func() {
			var (
				ctx    context.Context
				cancel context.CancelFunc
			)

			BeforeAll(func() {
				ctx, cancel = context.WithTimeout(parentCtx, 30*time.Minute)
				DeferCleanup(cancel)
			})

			It("should create a shoot", func() {
				By("create shoot")
				Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
				f.Verify()
				By("create a sample configMap")
				Expect(f.ShootFramework.ShootClient.Client().Create(ctx, getSampleConfigMap())).To(Succeed())
			})

			It("deploying zero-downtime validator job to ensure no downtime while after upgrading gardener", func() {
				shootSeedNamespace := f.Shoot.Status.TechnicalID
				job, err = highavailability.DeployZeroDownTimeValidatorJob(
					ctx,
					f.ShootFramework.SeedClient.Client(), "update", shootSeedNamespace,
					shootupdatesuite.GetKubeAPIServerAuthToken(
						ctx,
						f.ShootFramework.SeedClient,
						shootSeedNamespace,
					),
				)
				Expect(err).NotTo(HaveOccurred())
				shootupdatesuite.WaitForJobToBeReady(ctx, f.ShootFramework.SeedClient.Client(), job)
			})
		})

		When("Post-Upgrade (Gardener version:'"+gardenerCurrentVersion+"', Git version:'"+gardenerCurrentGitVersion+"')", Ordered, Label("post-upgrade"), func() {
			var (
				ctx        context.Context
				cancel     context.CancelFunc
				seedClient client.Client
			)

			BeforeAll(func() {
				ctx, cancel = context.WithTimeout(parentCtx, 20*time.Minute)
				DeferCleanup(cancel)
				Expect(f.GetShoot(ctx, shootTest)).To(Succeed())
				f.ShootFramework, err = f.NewShootFramework(ctx, shootTest)
				Expect(err).NotTo(HaveOccurred())
				seedClient = f.ShootFramework.SeedClient.Client()
			})

			It("should be able to restore etcd when etcd pod's pvc is corrupted in previous gardener release:", Label("etcd"), Label("high-availability"), func() {
				By("Scaling down ETCD StatefulSet shoot")
				expectedReplicas := 1
				if v1beta1helper.IsHAControlPlaneConfigured(f.Shoot) {
					expectedReplicas = 3
				}

				scaleDownOrUpStsEtcdMain(ctx, seedClient, shootTest.Status.TechnicalID, int32(expectedReplicas-1))
				deletePVC(ctx, seedClient, &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "main-etcd-" + etcdMainPodName,
						Namespace: shootTest.Status.TechnicalID,
					},
				})
				etcd := &v1alpha1.Etcd{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "etcd-main",
						Namespace: shootTest.Status.TechnicalID,
					}}
				Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(etcd), etcd)).Should(Succeed())
				Eventually(func() bool {
					return seedClient.Get(ctx, client.ObjectKeyFromObject(etcd), etcd) == nil && etcd.Status.CurrentReplicas == int32(expectedReplicas-1)
				}, time.Minute*5, time.Millisecond*500).Should(BeTrue())

				By("Scaling up ETCD StatefulSet shoot")
				scaleDownOrUpStsEtcdMain(ctx, seedClient, shootTest.Status.TechnicalID, int32(expectedReplicas))

				By("Check ETCD cluster all members are ready")
				checkEtcdReady(ctx, seedClient, etcd)

				By("Verifying etcd-main restored PVC or not")
				cm := &corev1.ConfigMap{}
				Expect(f.ShootFramework.ShootClient.Client().Get(ctx, client.ObjectKeyFromObject(getSampleConfigMap()), cm)).To(Succeed())
				Expect(cm.Data).To(Equal(getSampleConfigMap().Data))
			})

			It("verifying no downtime while upgrading gardener", func() {
				job = &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "zero-down-time-validator-update",
						Namespace: shootTest.Status.TechnicalID,
					}}
				Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(job), job)).To(Succeed())
				Expect(job.Status.Failed).Should(BeZero())
				Expect(seedClient.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())
			})

			It("should able to delete a shoot which was created in previous release", func() {
				Expect(f.Shoot.Status.Gardener.Version).Should(Equal(gardenerPreviousVersion))
				Expect(f.GardenerFramework.DeleteShootAndWaitForDeletion(ctx, shootTest)).To(Succeed())
			})
		})
	})

	// This test will create a non-HA control plane shoot in Gardener version vX.X.X
	// and then upgrades shoot's control plane to HA once successfully upgraded Gardener version to vY.Y.Y.
	Context("Shoot::e2e-upgrade-ha", Label("high-availability"), func() {
		var (
			parentCtx = context.Background()
			f         = framework.NewShootCreationFramework(&framework.ShootCreationConfig{GardenerConfig: e2e.DefaultGardenConfig(projectNamespace)})
			shootTest = e2e.DefaultShoot("e2e-upgrade-ha")
			err       error
		)

		shootTest.Namespace = projectNamespace
		shootTest.Spec.ControlPlane = nil
		f.Shoot = shootTest

		When("(Gardener version:'"+gardenerPreviousVersion+"', Git version:'"+gardenerPreviousGitVersion+"')", Ordered, Label("pre-upgrade"), func() {
			var (
				ctx    context.Context
				cancel context.CancelFunc
			)

			BeforeAll(func() {
				ctx, cancel = context.WithTimeout(parentCtx, 30*time.Minute)
				DeferCleanup(cancel)
			})

			It("should create a shoot", func() {
				Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
				f.Verify()
				Expect(f.ShootFramework.ShootClient.Client().Create(ctx, getSampleConfigMap())).To(Succeed())
			})
		})

		When("Post-Upgrade (Gardener version:'"+gardenerCurrentVersion+"', Git version:'"+gardenerCurrentGitVersion+"')", Ordered, Label("post-upgrade"), func() {
			var (
				ctx    context.Context
				cancel context.CancelFunc
			)

			BeforeAll(func() {
				ctx, cancel = context.WithTimeout(parentCtx, 60*time.Minute)
				DeferCleanup(cancel)
				Expect(f.GetShoot(ctx, shootTest)).To(Succeed())
				f.ShootFramework, err = f.NewShootFramework(ctx, shootTest)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should be able to upgrade a non-HA shoot which was created in previous gardener release to HA with failure tolerance type '"+
				os.Getenv("SHOOT_FAILURE_TOLERANCE_TYPE")+"'", func() {
				highavailability.UpgradeAndVerify(ctx, f.ShootFramework, getFailureToleranceType())
			})

			It("should be able to delete a shoot which was created in previous gardener release", func() {

				Expect(f.Shoot.Status.Gardener.Version).Should(Equal(gardenerPreviousVersion))
				Expect(f.GardenerFramework.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
			})
		})
	})
})

// scaleDownOrUpStsEtcdMain scales down or scales up replica size of etcd main for given shoot.
func scaleDownOrUpStsEtcdMain(ctx context.Context, seedClient client.Client, namespace string, replicaSize int32) {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "etcd-main",
			Namespace: namespace,
		},
	}
	Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(sts), sts)).To(Succeed())
	sts.Spec.Replicas = pointer.Int32Ptr(replicaSize)
	Expect(seedClient.Update(ctx, sts)).To(Succeed())
	Eventually(func() error {
		if err := seedClient.Get(ctx, client.ObjectKeyFromObject(sts), sts); err != nil {
			return fmt.Errorf("error occurred while getting statefulset %q error: %v", sts.Name, err)
		}

		if sts.Status.Replicas != replicaSize {
			return fmt.Errorf("statefulset replicas %v, is not same as the expected replica size %v", sts.Status.Replicas, replicaSize)
		}
		return nil
	}, time.Minute*5, time.Millisecond*500).Should(Succeed())
}

func getEtcdMainMemberLastOrdinalPodName(shoot *gardencorev1beta1.Shoot) string {
	etcdMainPodName := "etcd-main-0"
	if v1beta1helper.IsHAControlPlaneConfigured(shoot) {
		etcdMainPodName = "etcd-main-2"
	}
	return etcdMainPodName
}

func getSampleConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
		Data: map[string]string{
			"now": time.Now().Local().Format("01-02-2006"),
		},
	}
}

func deletePVC(ctx context.Context, seedClient client.Client, pvc *corev1.PersistentVolumeClaim) {
	By("Delete PVC: " + pvc.Name)
	Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(pvc), pvc)).To(Succeed())
	Expect(seedClient.Delete(ctx, pvc, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())
	Eventually(func() error {
		return seedClient.Get(ctx, client.ObjectKeyFromObject(pvc), pvc)
	}, time.Minute*5, time.Millisecond*500).Should(matchers.BeNotFoundError())
}

// checkEtcdReady checks ETCD cluster members are ready or not
func checkEtcdReady(ctx context.Context, cl client.Client, etcd *v1alpha1.Etcd) {
	EventuallyWithOffset(1, func() error {
		err := cl.Get(ctx, client.ObjectKeyFromObject(etcd), etcd)
		if err != nil {
			return err
		}

		if etcd.Status.Ready == nil || !*etcd.Status.Ready {
			return fmt.Errorf("etcd %s is not ready", etcd.Name)
		}

		if etcd.Status.ClusterSize == nil {
			return fmt.Errorf("etcd %s cluster size is empty", etcd.Name)
		}

		if *etcd.Status.ClusterSize != etcd.Spec.Replicas {
			return fmt.Errorf("etcd %s cluster size %v, is not same as the expected cluster size %v",
				etcd.Name, etcd.Status.ClusterSize, etcd.Spec.Replicas)
		}

		if len(etcd.Status.Conditions) == 0 {
			return fmt.Errorf("etcd %s status conditions is empty", etcd.Name)
		}

		for _, c := range etcd.Status.Conditions {
			// skip BackupReady status check if etcd.Spec.Backup.Store is not configured.
			if etcd.Spec.Backup.Store == nil && c.Type == v1alpha1.ConditionTypeBackupReady {
				continue
			}

			if c.Status != v1alpha1.ConditionTrue {
				return fmt.Errorf("etcd %q status %q condition %s is not True",
					etcd.Name, c.Type, c.Status)
			}
		}
		return nil
	}, time.Minute*5, time.Second*2).Should(BeNil())
}

// getFailureToleranceType returns a failureToleranceType based on env variable SHOOT_FAILURE_TOLERANCE_TYPE value
func getFailureToleranceType() gardencorev1beta1.FailureToleranceType {
	var failureToleranceType gardencorev1beta1.FailureToleranceType
	switch os.Getenv("SHOOT_FAILURE_TOLERANCE_TYPE") {
	case "zone":
		failureToleranceType = gardencorev1beta1.FailureToleranceTypeZone
	case "node":
		failureToleranceType = gardencorev1beta1.FailureToleranceTypeNode
	}
	return failureToleranceType
}
