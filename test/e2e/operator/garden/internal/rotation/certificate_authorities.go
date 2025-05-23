// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/operator/v1alpha1/helper"
	"github.com/gardener/gardener/test/utils/rotation"
)

// CAVerifier verifies the certificate authorities rotation.
type CAVerifier struct {
	RuntimeClient client.Client
	Garden        *operatorv1alpha1.Garden

	secretsBefore    rotation.SecretConfigNamesToSecrets
	secretsPrepared  rotation.SecretConfigNamesToSecrets
	secretsCompleted rotation.SecretConfigNamesToSecrets
}

var allCAs = []string{
	caCluster,
	caClient,
	caETCD,
	caETCDPeer,
	caFrontProxy,
	caGardener,
}

const (
	caCluster    = "ca"
	caClient     = "ca-client"
	caETCD       = "ca-etcd"
	caETCDPeer   = "ca-etcd-peer"
	caFrontProxy = "ca-front-proxy"
	caGardener   = "ca-gardener"
)

// Before is called before the rotation is started.
func (v *CAVerifier) Before(ctx context.Context) {
	By("Verify CA secrets of gardener-operator before rotation")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(v.RuntimeClient.List(ctx, secretList, client.InNamespace(v1beta1constants.GardenNamespace), ManagedByGardenerOperatorSecretsManager)).To(Succeed())

		grouped := rotation.GroupByName(secretList.Items)
		for _, ca := range allCAs {
			bundle := ca + "-bundle"
			g.Expect(grouped[ca]).To(HaveLen(1), ca+" secret should get created, but not rotated yet")
			g.Expect(grouped[bundle]).To(HaveLen(1), ca+" bundle secret should get created, but not rotated yet")
		}
		v.secretsBefore = grouped
	}).Should(Succeed())
}

// ExpectPreparingStatus is called while waiting for the Preparing status.
func (v *CAVerifier) ExpectPreparingStatus(g Gomega) {
	g.Expect(helper.GetCARotationPhase(v.Garden.Status.Credentials)).To(Equal(gardencorev1beta1.RotationPreparing))
	g.Expect(time.Now().UTC().Sub(v.Garden.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationTime.Time.UTC())).To(BeNumerically("<=", time.Minute))
	g.Expect(v.Garden.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationFinishedTime).To(BeNil())
	g.Expect(v.Garden.Status.Credentials.Rotation.CertificateAuthorities.LastCompletionTriggeredTime).To(BeNil())
}

// ExpectPreparingWithoutWorkersRolloutStatus is called while waiting for the PreparingWithoutWorkersRollout status.
func (v *CAVerifier) ExpectPreparingWithoutWorkersRolloutStatus(_ Gomega) {}

// ExpectWaitingForWorkersRolloutStatus is called while waiting for the WaitingForWorkersRollout status.
func (v *CAVerifier) ExpectWaitingForWorkersRolloutStatus(_ Gomega) {}

// AfterPrepared is called when the Shoot is in Prepared status.
func (v *CAVerifier) AfterPrepared(ctx context.Context) {
	Expect(v.Garden.Status.Credentials.Rotation.CertificateAuthorities.Phase).To(Equal(gardencorev1beta1.RotationPrepared), "ca rotation phase should be 'Prepared'")
	Expect(v.Garden.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationFinishedTime).NotTo(BeNil())
	Expect(v.Garden.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationFinishedTime.After(v.Garden.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationTime.Time)).To(BeTrue())

	By("Verify CA secrets of gardener-operator after preparation")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(v.RuntimeClient.List(ctx, secretList, client.InNamespace(v1beta1constants.GardenNamespace), ManagedByGardenerOperatorSecretsManager)).To(Succeed())

		grouped := rotation.GroupByName(secretList.Items)
		for _, ca := range allCAs {
			bundle := ca + "-bundle"
			g.Expect(grouped[ca]).To(HaveLen(2), ca+" secret should get rotated, but old CA is kept")
			g.Expect(grouped[bundle]).To(HaveLen(1), ca+" bundle secret should have changed")
			g.Expect(grouped[ca]).To(ContainElement(v.secretsBefore[ca][0]), "old "+ca+" secret should be kept")
			g.Expect(grouped[bundle]).To(Not(ContainElement(v.secretsBefore[bundle][0])), "old "+ca+" bundle should get cleaned up")
		}
		v.secretsPrepared = grouped
	}).Should(Succeed())
}

// ExpectCompletingStatus is called while waiting for the Completing status.
func (v *CAVerifier) ExpectCompletingStatus(g Gomega) {
	g.Expect(helper.GetCARotationPhase(v.Garden.Status.Credentials)).To(Equal(gardencorev1beta1.RotationCompleting))
	Expect(v.Garden.Status.Credentials.Rotation.CertificateAuthorities.LastCompletionTriggeredTime).NotTo(BeNil())
	Expect(v.Garden.Status.Credentials.Rotation.CertificateAuthorities.LastCompletionTriggeredTime.Time.Equal(v.Garden.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationFinishedTime.Time) ||
		v.Garden.Status.Credentials.Rotation.CertificateAuthorities.LastCompletionTriggeredTime.After(v.Garden.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationFinishedTime.Time)).To(BeTrue())
}

// AfterCompleted is called when the Shoot is in Completed status.
func (v *CAVerifier) AfterCompleted(ctx context.Context) {
	caRotation := v.Garden.Status.Credentials.Rotation.CertificateAuthorities
	Expect(helper.GetCARotationPhase(v.Garden.Status.Credentials)).To(Equal(gardencorev1beta1.RotationCompleted))
	Expect(caRotation.LastCompletionTime.Time.UTC().After(caRotation.LastInitiationTime.Time.UTC())).To(BeTrue())
	Expect(caRotation.LastInitiationFinishedTime).To(BeNil())
	Expect(caRotation.LastCompletionTriggeredTime).To(BeNil())

	By("Verify CA secrets of gardener-operator after completion")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(v.RuntimeClient.List(ctx, secretList, client.InNamespace(v1beta1constants.GardenNamespace), ManagedByGardenerOperatorSecretsManager)).To(Succeed())

		grouped := rotation.GroupByName(secretList.Items)
		for _, ca := range allCAs {
			bundle := ca + "-bundle"
			g.Expect(grouped[ca]).To(HaveLen(1), "old "+ca+" secret should get cleaned up")
			g.Expect(grouped[bundle]).To(HaveLen(1), ca+" bundle secret should have changed")
			g.Expect(grouped[ca]).To(ContainElement(v.secretsPrepared[ca][1]), "new "+ca+" secret should be kept")
			g.Expect(grouped[bundle]).To(Not(ContainElement(v.secretsPrepared[bundle][0])), "combined "+ca+" bundle should get cleaned up")
		}
		v.secretsCompleted = grouped
	}).Should(Succeed())
}
