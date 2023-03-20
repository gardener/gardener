// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/utils/shoots/access"
)

// SecretEncryptionVerifier creates and reads secrets in the shoot to verify correct configuration of etcd encryption.
type SecretEncryptionVerifier struct {
	*framework.ShootCreationFramework
}

// Before is called before the rotation is started.
func (v *SecretEncryptionVerifier) Before(ctx context.Context) {
	By("Verify secret encryption before credentials rotation")
	v.verifySecretEncryption(ctx)
}

// ExpectPreparingStatus is called while waiting for the Preparing status.
func (v *SecretEncryptionVerifier) ExpectPreparingStatus(g Gomega) {}

// AfterPrepared is called when the Shoot is in Prepared status.
func (v *SecretEncryptionVerifier) AfterPrepared(ctx context.Context) {
	By("Verify secret encryption after preparing credentials rotation")
	v.verifySecretEncryption(ctx)
}

// ExpectCompletingStatus is called while waiting for the Completing status.
func (v *SecretEncryptionVerifier) ExpectCompletingStatus(g Gomega) {}

// AfterCompleted is called when the Shoot is in Completed status.
func (v *SecretEncryptionVerifier) AfterCompleted(ctx context.Context) {
	By("Verify secret encryption after credentials rotation")
	v.verifySecretEncryption(ctx)
}

func (v *SecretEncryptionVerifier) verifySecretEncryption(ctx context.Context) {
	var (
		shootClient kubernetes.Interface
		err         error
	)

	Eventually(func(g Gomega) {
		shootClient, err = access.CreateShootClientFromAdminKubeconfig(ctx, v.GardenClient, v.Shoot)
		g.Expect(err).NotTo(HaveOccurred())
	}).Should(Succeed())

	Eventually(func(g Gomega) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "test-foo-", Namespace: "default"},
			StringData: map[string]string{
				"content": "foo",
			},
		}
		g.Expect(shootClient.Client().Create(ctx, secret)).To(Succeed())
	}).Should(Succeed(), "creating secret should succeed")

	Eventually(func(g Gomega) {
		g.Expect(shootClient.Client().List(ctx, &corev1.SecretList{})).To(Succeed())
	}).Should(Succeed(), "reading all secrets should succeed")
}
