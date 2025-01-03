// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rotation

import (
	"context"
	"fmt"
	"io"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/test/framework"
)

// SSHKeypairVerifier verifies the ssh keypair rotation.
type SSHKeypairVerifier struct {
	*framework.ShootCreationFramework

	oldKeypairData  map[string][]byte
	old2KeypairData map[string][]byte
}

// Before is called before the rotation is started.
func (v *SSHKeypairVerifier) Before(ctx context.Context) {
	By("Verify old ssh-keypair secret")
	Eventually(func(g Gomega) {
		secret := &corev1.Secret{}
		g.Expect(v.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v.Shoot.Namespace, Name: gardenerutils.ComputeShootProjectResourceName(v.Shoot.Name, "ssh-keypair")}, secret)).To(Succeed())
		g.Expect(secret.Data).To(And(
			HaveKeyWithValue("id_rsa", Not(BeEmpty())),
			HaveKeyWithValue("id_rsa.pub", Not(BeEmpty())),
		))
		v.oldKeypairData = secret.Data
	}).Should(Succeed(), "current ssh-keypair secret should be present")

	Eventually(func(g Gomega) {
		secret := &corev1.Secret{}
		err := v.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v.Shoot.Namespace, Name: gardenerutils.ComputeShootProjectResourceName(v.Shoot.Name, "ssh-keypair.old")}, secret)
		if apierrors.IsNotFound(err) {
			return
		}
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(secret.Data).To(And(
			HaveKeyWithValue("id_rsa", Not(Equal(v.oldKeypairData["id_rsa"]))),
			HaveKeyWithValue("id_rsa.pub", Not(Equal(v.oldKeypairData["id_rsa.pub"]))),
		))
		v.old2KeypairData = secret.Data
	}).Should(Succeed(), "old ssh-keypair secret should not be present or different from current")

	By("Verify that old SSH key(s) are accepted")
	Eventually(func(_ Gomega) {
		authorizedKeys, err := v.readAuthorizedKeysFile(ctx)
		Expect(err).NotTo(HaveOccurred())

		Expect(authorizedKeys).To(ContainSubstring(string(v.oldKeypairData["id_rsa.pub"])))
		if v.old2KeypairData != nil {
			Expect(authorizedKeys).To(ContainSubstring(string(v.old2KeypairData["id_rsa.pub"])))
		}
	}).Should(Succeed())
}

// ExpectPreparingStatus is called while waiting for the Preparing status.
func (v *SSHKeypairVerifier) ExpectPreparingStatus(g Gomega) {
	g.Expect(time.Now().UTC().Sub(v.Shoot.Status.Credentials.Rotation.SSHKeypair.LastInitiationTime.Time.UTC())).To(BeNumerically("<=", time.Minute))
}

// ExpectPreparingWithoutWorkersRolloutStatus is called while waiting for the PreparingWithoutWorkersRollout status.
func (v *SSHKeypairVerifier) ExpectPreparingWithoutWorkersRolloutStatus(_ Gomega) {}

// ExpectWaitingForWorkersRolloutStatus is called while waiting for the WaitingForWorkersRollout status.
func (v *SSHKeypairVerifier) ExpectWaitingForWorkersRolloutStatus(_ Gomega) {}

// AfterPrepared is called when the Shoot is in Prepared status.
func (v *SSHKeypairVerifier) AfterPrepared(ctx context.Context) {
	sshKeypairRotation := v.Shoot.Status.Credentials.Rotation.SSHKeypair
	Expect(sshKeypairRotation.LastCompletionTime.Time.UTC().After(sshKeypairRotation.LastInitiationTime.Time.UTC())).To(BeTrue())

	By("Verify new ssh-keypair secret")
	secret := &corev1.Secret{}
	Eventually(func(g Gomega) {
		g.Expect(v.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v.Shoot.Namespace, Name: gardenerutils.ComputeShootProjectResourceName(v.Shoot.Name, "ssh-keypair")}, secret)).To(Succeed())
		g.Expect(secret.Data).To(And(
			HaveKeyWithValue("id_rsa", Not(Equal(v.oldKeypairData["id_rsa"]))),
			HaveKeyWithValue("id_rsa.pub", Not(Equal(v.oldKeypairData["id_rsa.pub"]))),
		))

		g.Expect(v.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v.Shoot.Namespace, Name: gardenerutils.ComputeShootProjectResourceName(v.Shoot.Name, "ssh-keypair.old")}, secret)).To(Succeed())
		g.Expect(secret.Data).To(Equal(v.oldKeypairData))
	}).Should(Succeed(), "ssh-keypair secret should have been rotated")

	By("Verify that new SSH keys are accepted")
	Eventually(func(_ Gomega) {
		authorizedKeys, err := v.readAuthorizedKeysFile(ctx)
		Expect(err).NotTo(HaveOccurred())

		Expect(authorizedKeys).To(ContainSubstring(string(secret.Data["id_rsa.pub"])))
		Expect(authorizedKeys).To(ContainSubstring(string(v.oldKeypairData["id_rsa.pub"])))
		if v.old2KeypairData != nil {
			Expect(authorizedKeys).NotTo(ContainSubstring(string(v.old2KeypairData["id_rsa.pub"])))
		}
	}).Should(Succeed())
}

// ssh-keypair rotation is completed after one reconciliation (there is no second phase)
// hence, there is nothing to check in the second part of the credentials rotation

// ExpectCompletingStatus is called while waiting for the Completing status.
func (v *SSHKeypairVerifier) ExpectCompletingStatus(_ Gomega) {}

// AfterCompleted is called when the Shoot is in Completed status.
func (v *SSHKeypairVerifier) AfterCompleted(_ context.Context) {}

// Since we can't (and do not want ;-)) trying to really SSH into the machine pods from our test environment, we can
// only check whether the `.ssh/authorized_keys` file on the worker nodes has the expected content.
func (v *SSHKeypairVerifier) readAuthorizedKeysFile(ctx context.Context) (string, error) {
	podList := &corev1.PodList{}
	if err := v.ShootFramework.SeedClient.Client().List(ctx, podList, client.InNamespace(v.Shoot.Status.TechnicalID), client.MatchingLabels{
		"app":              "machine",
		"machine-provider": "local",
	}); err != nil {
		return "", err
	}

	if len(podList.Items) != 1 {
		return "", fmt.Errorf("expected exactly one result when listing all machine pods: %+v", podList.Items)
	}

	stdout, _, err := v.ShootFramework.SeedClient.PodExecutor().Execute(
		ctx,
		v.Shoot.Status.TechnicalID,
		podList.Items[0].Name,
		"node",
		"cat", "/home/gardener/.ssh/authorized_keys",
	)
	if err != nil {
		return "", err
	}

	result, err := io.ReadAll(stdout)
	if err != nil {
		return "", err
	}

	return string(result), nil
}
