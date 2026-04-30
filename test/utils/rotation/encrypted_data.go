// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	NewTargetClientFunc func(ctx context.Context) (kubernetes.Interface, error)
	Resources           []EncryptedResource
}

// Before is called before the rotation is started.
func (v *EncryptedDataVerifier) Before(ctx context.Context) {
	By("Verify encrypted data before credentials rotation")
	targetClient, err := v.NewTargetClientFunc(ctx)
	Expect(err).NotTo(HaveOccurred())
	VerifyEncryptedData(ctx, targetClient.Client(), v.Resources)
}

// ExpectPreparingStatus is called while waiting for the Preparing status.
func (v *EncryptedDataVerifier) ExpectPreparingStatus(_ Gomega) {}

// ExpectPreparingWithoutWorkersRolloutStatus is called while waiting for the PreparingWithoutWorkersRollout status.
func (v *EncryptedDataVerifier) ExpectPreparingWithoutWorkersRolloutStatus(_ Gomega) {}

// ExpectWaitingForWorkersRolloutStatus is called while waiting for the WaitingForWorkersRollout status.
func (v *EncryptedDataVerifier) ExpectWaitingForWorkersRolloutStatus(_ Gomega) {}

// AfterPrepared is called when the Shoot is in Prepared status.
func (v *EncryptedDataVerifier) AfterPrepared(ctx context.Context) {
	By("Verify encrypted data after preparing credentials rotation")
	targetClient, err := v.NewTargetClientFunc(ctx)
	Expect(err).NotTo(HaveOccurred())
	VerifyEncryptedData(ctx, targetClient.Client(), v.Resources)
}

// ExpectCompletingStatus is called while waiting for the Completing status.
func (v *EncryptedDataVerifier) ExpectCompletingStatus(_ Gomega) {}

// AfterCompleted is called when the Shoot is in Completed status.
func (v *EncryptedDataVerifier) AfterCompleted(ctx context.Context) {
	By("Verify encrypted data after credentials rotation")
	targetClient, err := v.NewTargetClientFunc(ctx)
	Expect(err).NotTo(HaveOccurred())
	VerifyEncryptedData(ctx, targetClient.Client(), v.Resources)
}
