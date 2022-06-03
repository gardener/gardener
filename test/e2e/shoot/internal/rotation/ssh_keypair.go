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

package rotation

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/test/framework"
)

type SSHKeypairVerifier struct {
	*framework.ShootCreationFramework

	oldKeypairData map[string][]byte
}

func (v *SSHKeypairVerifier) Before(ctx context.Context) {
	By("Verify old ssh-keypair secret")
	Eventually(func(g Gomega) {
		secret := &corev1.Secret{}
		g.Expect(v.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v.Shoot.Namespace, Name: gutil.ComputeShootProjectSecretName(v.Shoot.Name, "ssh-keypair")}, secret)).To(Succeed())
		g.Expect(secret.Data).To(And(
			HaveKeyWithValue("id_rsa", Not(BeEmpty())),
			HaveKeyWithValue("id_rsa.pub", Not(BeEmpty())),
		))
		v.oldKeypairData = secret.Data
	}).Should(Succeed(), "current ssh-keypair secret should be present")

	Eventually(func(g Gomega) {
		secret := &corev1.Secret{}
		err := v.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v.Shoot.Namespace, Name: gutil.ComputeShootProjectSecretName(v.Shoot.Name, "ssh-keypair.old")}, secret)
		if apierrors.IsNotFound(err) {
			return
		}
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(secret.Data).To(And(
			HaveKeyWithValue("id_rsa", Not(Equal(v.oldKeypairData["id_rsa"]))),
			HaveKeyWithValue("id_rsa.pub", Not(Equal(v.oldKeypairData["id_rsa.pub"]))),
		))
	}).Should(Succeed(), "old ssh-keypair secret should not be present or different from current")
}

func (v *SSHKeypairVerifier) ExpectPreparingStatus(g Gomega) {
	g.Expect(time.Now().UTC().Sub(v.Shoot.Status.Credentials.Rotation.SSHKeypair.LastInitiationTime.Time.UTC())).To(BeNumerically("<=", time.Minute))
}

func (v *SSHKeypairVerifier) AfterPrepared(ctx context.Context) {
	sshKeypairRotation := v.Shoot.Status.Credentials.Rotation.SSHKeypair
	Expect(sshKeypairRotation.LastCompletionTime.Time.UTC().After(sshKeypairRotation.LastInitiationTime.Time.UTC())).To(BeTrue())

	By("Verify new ssh-keypair secret")
	Eventually(func(g Gomega) {
		secret := &corev1.Secret{}
		g.Expect(v.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v.Shoot.Namespace, Name: gutil.ComputeShootProjectSecretName(v.Shoot.Name, "ssh-keypair")}, secret)).To(Succeed())
		g.Expect(secret.Data).To(And(
			HaveKeyWithValue("id_rsa", Not(Equal(v.oldKeypairData["id_rsa"]))),
			HaveKeyWithValue("id_rsa.pub", Not(Equal(v.oldKeypairData["id_rsa.pub"]))),
		))

		g.Expect(v.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: v.Shoot.Namespace, Name: gutil.ComputeShootProjectSecretName(v.Shoot.Name, "ssh-keypair.old")}, secret)).To(Succeed())
		g.Expect(secret.Data).To(Equal(v.oldKeypairData))
	}).Should(Succeed(), "ssh-keypair secret should have been rotated")
}

// ssh-keypair rotation is completed after one reconciliation (there is no second phase)
// hence, there is nothing to check in the second part of the credentials rotation

func (v *SSHKeypairVerifier) ExpectCompletingStatus(g Gomega) {}

func (v *SSHKeypairVerifier) AfterCompleted(ctx context.Context) {}
