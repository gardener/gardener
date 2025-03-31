// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rotation

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// ServiceAccountKeyVerifier verifies the service account key rotation.
type ServiceAccountKeyVerifier struct {
	SecretsManagerLabelSelector         client.MatchingLabels
	GetServiceAccountKeyRotation        func() *gardencorev1beta1.ServiceAccountKeyRotation
	GetRuntimeClient                    func() client.Client
	GetServiceAccountKeySecretNamespace func() string

	secretsBefore   SecretConfigNamesToSecrets
	secretsPrepared SecretConfigNamesToSecrets
}

const (
	serviceAccountKey       = "service-account-key"
	serviceAccountKeyBundle = "service-account-key-bundle"
)

// Before is called before the rotation is started.
func (v *ServiceAccountKeyVerifier) Before(ctx context.Context) {
	runtimeClient := v.GetRuntimeClient()

	By("Verify old service account key secret")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		Expect(runtimeClient.List(ctx, secretList, client.InNamespace(v.GetServiceAccountKeySecretNamespace()), v.SecretsManagerLabelSelector)).To(Succeed())

		grouped := GroupByName(secretList.Items)
		g.Expect(grouped[serviceAccountKey]).To(HaveLen(1), "service account key secret should get created, but not rotated yet")
		g.Expect(grouped[serviceAccountKeyBundle]).To(HaveLen(1), "service account key bundle secret should get created, but not rotated yet")
		v.secretsBefore = grouped
	}).Should(Succeed())
}

// ExpectPreparingStatus is called while waiting for the Preparing status.
func (v *ServiceAccountKeyVerifier) ExpectPreparingStatus(g Gomega) {
	serviceAccountKeyRotation := v.GetServiceAccountKeyRotation()
	g.Expect(serviceAccountKeyRotation.Phase).To(Equal(gardencorev1beta1.RotationPreparing))
	g.Expect(time.Now().UTC().Sub(serviceAccountKeyRotation.LastInitiationTime.Time.UTC())).To(BeNumerically("<=", time.Minute))
	g.Expect(serviceAccountKeyRotation.LastInitiationFinishedTime).To(BeNil())
	g.Expect(serviceAccountKeyRotation.LastCompletionTriggeredTime).To(BeNil())
}

// ExpectPreparingWithoutWorkersRolloutStatus is called while waiting for the PreparingWithoutWorkersRollout status.
func (v *ServiceAccountKeyVerifier) ExpectPreparingWithoutWorkersRolloutStatus(g Gomega) {
	serviceAccountKeyRotation := v.GetServiceAccountKeyRotation()
	g.Expect(serviceAccountKeyRotation.Phase).To(Equal(gardencorev1beta1.RotationPreparingWithoutWorkersRollout))
	g.Expect(time.Now().UTC().Sub(serviceAccountKeyRotation.LastInitiationTime.Time.UTC())).To(BeNumerically("<=", time.Minute))
	g.Expect(serviceAccountKeyRotation.LastInitiationFinishedTime).To(BeNil())
	g.Expect(serviceAccountKeyRotation.LastCompletionTriggeredTime).To(BeNil())
}

// ExpectWaitingForWorkersRolloutStatus is called while waiting for the WaitingForWorkersRollout status.
func (v *ServiceAccountKeyVerifier) ExpectWaitingForWorkersRolloutStatus(g Gomega) {
	serviceAccountKeyRotation := v.GetServiceAccountKeyRotation()
	g.Expect(serviceAccountKeyRotation.Phase).To(Equal(gardencorev1beta1.RotationWaitingForWorkersRollout))
	g.Expect(serviceAccountKeyRotation.LastInitiationTime).NotTo(BeNil())
	g.Expect(serviceAccountKeyRotation.LastInitiationFinishedTime).To(BeNil())
	g.Expect(serviceAccountKeyRotation.LastCompletionTriggeredTime).To(BeNil())
}

// AfterPrepared is called when the Shoot is in Prepared status.
func (v *ServiceAccountKeyVerifier) AfterPrepared(ctx context.Context) {
	serviceAccountKeyRotation := v.GetServiceAccountKeyRotation()
	Expect(serviceAccountKeyRotation.Phase).To(Equal(gardencorev1beta1.RotationPrepared), "rotation phase should be 'Prepared'")
	Expect(serviceAccountKeyRotation.LastInitiationFinishedTime).NotTo(BeNil())
	Expect(serviceAccountKeyRotation.LastInitiationFinishedTime.After(serviceAccountKeyRotation.LastInitiationTime.Time)).To(BeTrue())

	runtimeClient := v.GetRuntimeClient()
	By("Verify service account key bundle secret")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		Expect(runtimeClient.List(ctx, secretList, client.InNamespace(v.GetServiceAccountKeySecretNamespace()), v.SecretsManagerLabelSelector)).To(Succeed())

		grouped := GroupByName(secretList.Items)
		g.Expect(grouped[serviceAccountKey]).To(HaveLen(2), "service account key secret should get rotated, but old service account key is kept")
		g.Expect(grouped[serviceAccountKeyBundle]).To(HaveLen(1))

		g.Expect(grouped[serviceAccountKey]).To(ContainElement(v.secretsBefore[serviceAccountKey][0]), "old service account key secret should be kept")
		g.Expect(grouped[serviceAccountKeyBundle]).To(Not(ContainElement(v.secretsBefore[serviceAccountKeyBundle][0])), "service account key bundle should have changed")
		v.secretsPrepared = grouped
	}).Should(Succeed())
}

// ExpectCompletingStatus is called while waiting for the Completing status.
func (v *ServiceAccountKeyVerifier) ExpectCompletingStatus(g Gomega) {
	serviceAccountKeyRotation := v.GetServiceAccountKeyRotation()
	g.Expect(serviceAccountKeyRotation.Phase).To(Equal(gardencorev1beta1.RotationCompleting))
	Expect(serviceAccountKeyRotation.LastCompletionTriggeredTime).NotTo(BeNil())
	Expect(serviceAccountKeyRotation.LastCompletionTriggeredTime.Time.Equal(serviceAccountKeyRotation.LastInitiationFinishedTime.Time) ||
		serviceAccountKeyRotation.LastCompletionTriggeredTime.After(serviceAccountKeyRotation.LastInitiationFinishedTime.Time)).To(BeTrue())
}

// AfterCompleted is called when the Shoot is in Completed status.
func (v *ServiceAccountKeyVerifier) AfterCompleted(ctx context.Context) {
	serviceAccountKeyRotation := v.GetServiceAccountKeyRotation()
	Expect(serviceAccountKeyRotation.Phase).To(Equal(gardencorev1beta1.RotationCompleted))
	Expect(serviceAccountKeyRotation.LastCompletionTime.Time.UTC().After(serviceAccountKeyRotation.LastInitiationTime.Time.UTC())).To(BeTrue())
	Expect(serviceAccountKeyRotation.LastInitiationFinishedTime).To(BeNil())
	Expect(serviceAccountKeyRotation.LastCompletionTriggeredTime).To(BeNil())

	runtimeClient := v.GetRuntimeClient()
	By("Verify new service account key secret")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		Expect(runtimeClient.List(ctx, secretList, client.InNamespace(v.GetServiceAccountKeySecretNamespace()), v.SecretsManagerLabelSelector)).To(Succeed())

		grouped := GroupByName(secretList.Items)
		g.Expect(grouped[serviceAccountKey]).To(HaveLen(1), "old service account key secret should get cleaned up")
		g.Expect(grouped[serviceAccountKeyBundle]).To(HaveLen(1))

		g.Expect(grouped[serviceAccountKey]).To(ContainElement(v.secretsPrepared[serviceAccountKey][1]), "new service account key secret should be kept")
		g.Expect(grouped[serviceAccountKeyBundle]).To(Not(ContainElement(v.secretsPrepared[serviceAccountKeyBundle][0])), "service account key bundle should have changed")
	}).Should(Succeed())
}
