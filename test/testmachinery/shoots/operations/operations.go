// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

/**
	Overview
		- Tests the hibernation of a  shoot.

	Prerequisites
		- A Shoot exists.

	Test:
		Deploys a default application and hibernates the cluster.
		When the cluster is successfully hibernated it is woken up and the deployed application is tested.
	Expected Output
		- Shoot and deployed app is fully functional after hibernation and wakeup.

	Test:
		Fully reconciles a cluster which means that the default reconciliation as well as the maintenance of
		the shoot is triggered.
	Expected Output
		- Shoot is successfully reconciling.

	Test:
		Rotate kubeconfig for a shoot cluster.
	Expected Output
		- The old kubeconfig to be updated and the old file to be no longer autorized.

	Test:
		Rotate ssh keypair for a shoot cluster.
		Annotate Shoot with "gardener.cloud/operation" = "rotate-ssh-keypair".
	Expected Output
		- Current ssh-keypair should be rotated.
		- Current ssh-keypair should be kept in the system post rotation.

 **/

package operations

import (
	"context"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/applications"
)

const (
	hibernationTestTimeout = 1 * time.Hour
	reconcileTimeout       = 40 * time.Minute
)

var _ = ginkgo.Describe("Shoot operation testing", func() {

	f := framework.NewShootFramework(nil)

	var isShootHibernated bool

	f.Default().Serial().CIt("Testing if Shoot can be hibernated successfully", func(ctx context.Context) {
		guestBookTest, err := applications.NewGuestBookTest(f)
		framework.ExpectNoError(err)

		defer guestBookTest.Cleanup(ctx)

		ginkgo.By("Deploy guestbook")
		guestBookTest.DeployGuestBookApp(ctx)
		guestBookTest.Test(ctx)

		ginkgo.By("Hibernate shoot")
		isShootHibernated = true
		err = f.HibernateShoot(ctx)
		framework.ExpectNoError(err)

		ginkgo.By("Wake up shoot")
		err = f.WakeUpShoot(ctx)
		framework.ExpectNoError(err)

		ginkgo.By("Test guestbook")
		guestBookTest.WaitUntilRedisIsReady(ctx)
		guestBookTest.WaitUntilGuestbookDeploymentIsReady(ctx)
		guestBookTest.Test(ctx)

	}, hibernationTestTimeout, framework.WithCAfterTest(func(ctx context.Context) {
		if isShootHibernated && v1beta1helper.HibernationIsEnabled(f.Shoot) {
			ginkgo.By("Wake up shoot")
			err := f.WakeUpShoot(ctx)
			framework.ExpectNoError(err)
		}
	}, 25*time.Minute))

	f.Default().Serial().CIt("should fully maintain and reconcile a shoot cluster", func(ctx context.Context) {
		ginkgo.By("Maintain shoot")
		err := f.UpdateShoot(ctx, func(shoot *gardencorev1beta1.Shoot) error {
			shoot.Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.ShootOperationMaintain
			return nil
		})
		framework.ExpectNoError(err)

		ginkgo.By("Reconcile shoot")
		err = f.UpdateShoot(ctx, func(shoot *gardencorev1beta1.Shoot) error {
			shoot.Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.GardenerOperationReconcile
			return nil
		})
		framework.ExpectNoError(err)
	}, reconcileTimeout)

	f.Beta().Serial().CIt("should rotate the ssh keypair for a shoot cluster", func(ctx context.Context) {
		secret := &corev1.Secret{}
		gomega.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: f.Shoot.Namespace, Name: gardenerutils.ComputeShootProjectResourceName(f.Shoot.Name, gardenerutils.ShootProjectSecretSuffixSSHKeypair)}, secret)).To(gomega.Succeed())
		preRotationPrivateKey := getKeyAndValidate(secret, secrets.DataKeyRSAPrivateKey)
		preRotationPublicKey := getKeyAndValidate(secret, secrets.DataKeySSHAuthorizedKeys)
		err := f.UpdateShoot(ctx, func(s *gardencorev1beta1.Shoot) error {
			metav1.SetMetaDataAnnotation(&s.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationRotateSSHKeypair)
			return nil
		})
		gomega.Expect(err).To(gomega.Succeed())

		gomega.Expect(f.GetShoot(ctx, f.Shoot)).To(gomega.Succeed())
		v, ok := f.Shoot.Annotations[v1beta1constants.GardenerOperation]
		if ok {
			gomega.Expect(v).NotTo(gomega.Equal(v1beta1constants.ShootOperationRotateSSHKeypair))
		}

		gomega.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: f.Shoot.Namespace, Name: gardenerutils.ComputeShootProjectResourceName(f.Shoot.Name, gardenerutils.ShootProjectSecretSuffixSSHKeypair)}, secret)).To(gomega.Succeed())
		postRotationPrivateKey := getKeyAndValidate(secret, secrets.DataKeyRSAPrivateKey)
		postRotationPublicKey := getKeyAndValidate(secret, secrets.DataKeySSHAuthorizedKeys)

		gomega.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: f.Shoot.Namespace, Name: gardenerutils.ComputeShootProjectResourceName(f.Shoot.Name, gardenerutils.ShootProjectSecretSuffixOldSSHKeypair)}, secret)).To(gomega.Succeed())
		postRotationOldPrivateKey := getKeyAndValidate(secret, secrets.DataKeyRSAPrivateKey)
		postRotationOldPublicKey := getKeyAndValidate(secret, secrets.DataKeySSHAuthorizedKeys)

		gomega.Expect(preRotationPrivateKey).NotTo(gomega.Equal(postRotationPrivateKey))
		gomega.Expect(preRotationPublicKey).NotTo(gomega.Equal(postRotationPublicKey))
		gomega.Expect(preRotationPrivateKey).To(gomega.Equal(postRotationOldPrivateKey))
		gomega.Expect(preRotationPublicKey).To(gomega.Equal(postRotationOldPublicKey))

	}, reconcileTimeout)
})

func getKeyAndValidate(s *corev1.Secret, field string) []byte {
	v, ok := s.Data[field]
	gomega.Expect(ok).To(gomega.BeTrue())
	gomega.Expect(v).ToNot(gomega.BeEmpty())
	return v
}
