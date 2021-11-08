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

/**
	Overview
		- Tests ssh-keypair rotation

	Test: Annotate Shoot with "gardener.cloud/operation" = "rotate-ssh-keypair"
	Expected Output
		- Current ssh-keypair should be rotated
		- Current ssh-keypair should become "ssh-keypair.old" post rotation
 **/

package sshrotation

import (
	"context"
	"time"

	corev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/test/framework"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	reconcileTimeout = 20 * time.Minute
)

var _ = ginkgo.Describe("SSH Keypair Rotation", func() {
	var f = framework.NewShootFramework(nil)

	f.Beta().Serial().CIt("ssh-keypair", func(ctx context.Context) {
		secret := &corev1.Secret{}
		gomega.Expect(f.SeedClient.Client().Get(ctx, client.ObjectKey{Namespace: f.ShootSeedNamespace(), Name: "ssh-keypair"}, secret)).To(gomega.Succeed())
		preRotationPrivateKey := getKeyAndValidate(secret, "id_rsa")
		preRotationPublicKey := getKeyAndValidate(secret, "id_rsa.pub")
		err := f.UpdateShoot(ctx, func(s *corev1beta1.Shoot) error {
			metav1.SetMetaDataAnnotation(&s.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationRotateSSHKeypair)
			return nil
		})
		gomega.Expect(err).To(gomega.BeNil())

		gomega.Expect(f.GetShoot(ctx, f.Shoot)).To(gomega.BeNil())
		v, ok := f.Shoot.Annotations[v1beta1constants.GardenerOperation]
		if ok {
			gomega.Expect(v).NotTo(gomega.Equal(v1beta1constants.ShootOperationRotateSSHKeypair))
		}
		gomega.Expect(f.SeedClient.Client().Get(ctx, client.ObjectKey{Namespace: f.ShootSeedNamespace(), Name: "ssh-keypair"}, secret)).To(gomega.Succeed())
		postRotationPrivateKey := getKeyAndValidate(secret, "id_rsa")
		postRotationPublicKey := getKeyAndValidate(secret, "id_rsa.pub")
		gomega.Expect(f.SeedClient.Client().Get(ctx, client.ObjectKey{Namespace: f.ShootSeedNamespace(), Name: "ssh-keypair.old"}, secret)).To(gomega.Succeed())
		postRotationOldPrivateKey := getKeyAndValidate(secret, "id_rsa")
		postRotationOldPublicKey := getKeyAndValidate(secret, "id_rsa.pub")

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
