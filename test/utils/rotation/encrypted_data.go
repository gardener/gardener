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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// EncryptedResource contains functions for creating objects and empty lists for encrypted resources.
type EncryptedResource struct {
	NewObject    func() client.Object
	NewEmptyList func() client.ObjectList
}

// EncryptedDataVerifier creates and reads encrypted data in the cluster to verify correct configuration of etcd encryption.
type EncryptedDataVerifier struct {
	NewTargetClientFunc func() (kubernetes.Interface, error)
	Resources           []EncryptedResource
}

// Before is called before the rotation is started.
func (v *EncryptedDataVerifier) Before(ctx context.Context) {
	By("Verify encrypted data before credentials rotation")
	v.verifyEncryptedData(ctx)
}

// ExpectPreparingStatus is called while waiting for the Preparing status.
func (v *EncryptedDataVerifier) ExpectPreparingStatus(g Gomega) {}

// AfterPrepared is called when the Shoot is in Prepared status.
func (v *EncryptedDataVerifier) AfterPrepared(ctx context.Context) {
	By("Verify encrypted data after preparing credentials rotation")
	v.verifyEncryptedData(ctx)
}

// ExpectCompletingStatus is called while waiting for the Completing status.
func (v *EncryptedDataVerifier) ExpectCompletingStatus(g Gomega) {}

// AfterCompleted is called when the Shoot is in Completed status.
func (v *EncryptedDataVerifier) AfterCompleted(ctx context.Context) {
	By("Verify encrypted data after credentials rotation")
	v.verifyEncryptedData(ctx)
}

func (v *EncryptedDataVerifier) verifyEncryptedData(ctx context.Context) {
	var (
		targetClient kubernetes.Interface
		err          error
	)

	Eventually(func(g Gomega) {
		targetClient, err = v.NewTargetClientFunc()
		g.Expect(err).NotTo(HaveOccurred())
	}).Should(Succeed())

	for _, resource := range v.Resources {
		obj := resource.NewObject()
		Eventually(func(g Gomega) {
			g.Expect(targetClient.Client().Create(ctx, obj)).To(Succeed())
		}).Should(Succeed(), "creating resource should succeed for "+client.ObjectKeyFromObject(obj).String())

		Eventually(func(g Gomega) {
			g.Expect(targetClient.Client().List(ctx, resource.NewEmptyList())).To(Succeed())
		}).Should(Succeed(), "reading all encrypted resources should succeed")
	}
}
