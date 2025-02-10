// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rotation

import (
	"context"
	"sort"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/apiserver/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

// ETCDEncryptionKeyVerifier verifies the etcd encryption key rotation.
type ETCDEncryptionKeyVerifier struct {
	RuntimeClient                client.Client
	Namespace                    string
	SecretsManagerLabelSelector  client.MatchingLabels
	GetETCDEncryptionKeyRotation func() *gardencorev1beta1.ETCDEncryptionKeyRotation

	EncryptionKey  string
	RoleLabelValue string

	secretsBefore   SecretConfigNamesToSecrets
	secretsPrepared SecretConfigNamesToSecrets
}

var decoder runtime.Decoder

func init() {
	scheme := runtime.NewScheme()
	utilruntime.Must(apiserverconfigv1.AddToScheme(scheme))
	decoder = serializer.NewCodecFactory(scheme).UniversalDeserializer()
}

// Before is called before the rotation is started.
func (v *ETCDEncryptionKeyVerifier) Before(ctx context.Context) {
	By("Verify old etcd encryption key secret")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(v.RuntimeClient.List(ctx, secretList, client.InNamespace(v.Namespace), v.SecretsManagerLabelSelector)).To(Succeed())

		grouped := GroupByName(secretList.Items)
		g.Expect(grouped[v.EncryptionKey]).To(HaveLen(1), "etcd encryption key should get created, but not rotated yet")
		v.secretsBefore = grouped
	}).Should(Succeed())

	By("Verify old etcd encryption config secret")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(v.RuntimeClient.List(ctx, secretList, client.InNamespace(v.Namespace), client.MatchingLabels{v1beta1constants.LabelRole: v.RoleLabelValue})).To(Succeed())
		g.Expect(secretList.Items).NotTo(BeEmpty())
		sort.Sort(sort.Reverse(AgeSorter(secretList.Items)))

		encryptionConfiguration := &apiserverconfigv1.EncryptionConfiguration{}
		g.Expect(runtime.DecodeInto(decoder, secretList.Items[0].Data["encryption-configuration.yaml"], encryptionConfiguration)).To(Succeed())

		g.Expect(encryptionConfiguration.Resources).To(HaveLen(1))

		g.Expect(encryptionConfiguration.Resources[0].Providers).To(DeepEqual([]apiserverconfigv1.ProviderConfiguration{
			{
				AESCBC: &apiserverconfigv1.AESConfiguration{
					Keys: []apiserverconfigv1.Key{{
						// old key
						Name:   string(v.secretsBefore[v.EncryptionKey][0].Data["key"]),
						Secret: getBase64EncodedETCDEncryptionKeyFromSecret(v.secretsBefore[v.EncryptionKey][0]),
					}},
				},
			},
			{
				// identity is always added
				Identity: &apiserverconfigv1.IdentityConfiguration{},
			},
		}))
	}).Should(Succeed(), "etcd encryption config should only have old key")
}

// ExpectPreparingStatus is called while waiting for the Preparing status.
func (v *ETCDEncryptionKeyVerifier) ExpectPreparingStatus(g Gomega) {
	etcdEncryptionKeyRotation := v.GetETCDEncryptionKeyRotation()
	g.Expect(etcdEncryptionKeyRotation.Phase).To(Equal(gardencorev1beta1.RotationPreparing))
	g.Expect(time.Now().UTC().Sub(etcdEncryptionKeyRotation.LastInitiationTime.Time.UTC())).To(BeNumerically("<=", time.Minute))
	g.Expect(etcdEncryptionKeyRotation.LastInitiationFinishedTime).To(BeNil())
	g.Expect(etcdEncryptionKeyRotation.LastCompletionTriggeredTime).To(BeNil())
}

// ExpectPreparingWithoutWorkersRolloutStatus is called while waiting for the PreparingWithoutWorkersRollout status.
func (v *ETCDEncryptionKeyVerifier) ExpectPreparingWithoutWorkersRolloutStatus(_ Gomega) {}

// ExpectWaitingForWorkersRolloutStatus is called while waiting for the WaitingForWorkersRollout status.
func (v *ETCDEncryptionKeyVerifier) ExpectWaitingForWorkersRolloutStatus(_ Gomega) {}

// AfterPrepared is called when the Shoot is in Prepared status.
func (v *ETCDEncryptionKeyVerifier) AfterPrepared(ctx context.Context) {
	etcdEncryptionKeyRotation := v.GetETCDEncryptionKeyRotation()
	Expect(etcdEncryptionKeyRotation.Phase).To(Equal(gardencorev1beta1.RotationPrepared), "rotation phase should be 'Prepared'")
	Expect(etcdEncryptionKeyRotation.LastInitiationFinishedTime).NotTo(BeNil())
	Expect(etcdEncryptionKeyRotation.LastInitiationFinishedTime.After(etcdEncryptionKeyRotation.LastInitiationTime.Time)).To(BeTrue())

	By("Verify etcd encryption key secrets")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(v.RuntimeClient.List(ctx, secretList, client.InNamespace(v.Namespace), v.SecretsManagerLabelSelector)).To(Succeed())

		grouped := GroupByName(secretList.Items)
		g.Expect(grouped[v.EncryptionKey]).To(HaveLen(2), "etcd encryption key should get rotated")
		g.Expect(grouped[v.EncryptionKey]).To(ContainElement(v.secretsBefore[v.EncryptionKey][0]), "old etcd encryption key secret should be kept")
		v.secretsPrepared = grouped
	}).Should(Succeed())

	By("Verify combined etcd encryption config secret")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(v.RuntimeClient.List(ctx, secretList, client.InNamespace(v.Namespace), client.MatchingLabels{v1beta1constants.LabelRole: v.RoleLabelValue})).To(Succeed())
		g.Expect(secretList.Items).NotTo(BeEmpty())
		sort.Sort(sort.Reverse(AgeSorter(secretList.Items)))

		encryptionConfiguration := &apiserverconfigv1.EncryptionConfiguration{}
		err := runtime.DecodeInto(decoder, secretList.Items[0].Data["encryption-configuration.yaml"], encryptionConfiguration)
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(encryptionConfiguration.Resources).To(HaveLen(1))
		g.Expect(encryptionConfiguration.Resources[0].Providers).To(DeepEqual([]apiserverconfigv1.ProviderConfiguration{
			{
				AESCBC: &apiserverconfigv1.AESConfiguration{
					Keys: []apiserverconfigv1.Key{{
						// new key
						Name:   string(v.secretsPrepared[v.EncryptionKey][1].Data["key"]),
						Secret: getBase64EncodedETCDEncryptionKeyFromSecret(v.secretsPrepared[v.EncryptionKey][1]),
					}, {
						// old key
						Name:   string(v.secretsPrepared[v.EncryptionKey][0].Data["key"]),
						Secret: getBase64EncodedETCDEncryptionKeyFromSecret(v.secretsPrepared[v.EncryptionKey][0]),
					}},
				},
			},
			{
				Identity: &apiserverconfigv1.IdentityConfiguration{},
			},
		}))
	}).Should(Succeed(), "etcd encryption config should have both old and new key, with new key as the first one")
}

// ExpectCompletingStatus is called while waiting for the Completing status.
func (v *ETCDEncryptionKeyVerifier) ExpectCompletingStatus(g Gomega) {
	etcdEncryptionKeyRotation := v.GetETCDEncryptionKeyRotation()
	g.Expect(etcdEncryptionKeyRotation.Phase).To(Equal(gardencorev1beta1.RotationCompleting))
	Expect(etcdEncryptionKeyRotation.LastCompletionTriggeredTime).NotTo(BeNil())
	Expect(etcdEncryptionKeyRotation.LastCompletionTriggeredTime.Time.Equal(etcdEncryptionKeyRotation.LastInitiationFinishedTime.Time) ||
		etcdEncryptionKeyRotation.LastCompletionTriggeredTime.After(etcdEncryptionKeyRotation.LastInitiationFinishedTime.Time)).To(BeTrue())
}

// AfterCompleted is called when the Shoot is in Completed status.
func (v *ETCDEncryptionKeyVerifier) AfterCompleted(ctx context.Context) {
	etcdEncryptionKeyRotation := v.GetETCDEncryptionKeyRotation()
	Expect(etcdEncryptionKeyRotation.Phase).To(Equal(gardencorev1beta1.RotationCompleted))
	Expect(etcdEncryptionKeyRotation.LastCompletionTime.Time.UTC().After(etcdEncryptionKeyRotation.LastInitiationTime.Time.UTC())).To(BeTrue())
	Expect(etcdEncryptionKeyRotation.LastInitiationFinishedTime).To(BeNil())
	Expect(etcdEncryptionKeyRotation.LastCompletionTriggeredTime).To(BeNil())

	By("Verify new etcd encryption key secret")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(v.RuntimeClient.List(ctx, secretList, client.InNamespace(v.Namespace), v.SecretsManagerLabelSelector)).To(Succeed())

		grouped := GroupByName(secretList.Items)
		g.Expect(grouped[v.EncryptionKey]).To(HaveLen(1), "old etcd encryption key should get cleaned up")
		g.Expect(grouped[v.EncryptionKey]).To(ContainElement(v.secretsPrepared[v.EncryptionKey][1]), "new etcd encryption key secret should be kept")
	}).Should(Succeed())

	By("Verify new etcd encryption config secret")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(v.RuntimeClient.List(ctx, secretList, client.InNamespace(v.Namespace), client.MatchingLabels{v1beta1constants.LabelRole: v.RoleLabelValue})).To(Succeed())
		g.Expect(secretList.Items).NotTo(BeEmpty())
		sort.Sort(sort.Reverse(AgeSorter(secretList.Items)))

		encryptionConfiguration := &apiserverconfigv1.EncryptionConfiguration{}
		err := runtime.DecodeInto(decoder, secretList.Items[0].Data["encryption-configuration.yaml"], encryptionConfiguration)
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(encryptionConfiguration.Resources).To(HaveLen(1))
		g.Expect(encryptionConfiguration.Resources[0].Providers).To(DeepEqual([]apiserverconfigv1.ProviderConfiguration{
			{
				AESCBC: &apiserverconfigv1.AESConfiguration{
					Keys: []apiserverconfigv1.Key{{
						// new key
						Name:   string(v.secretsPrepared[v.EncryptionKey][1].Data["key"]),
						Secret: getBase64EncodedETCDEncryptionKeyFromSecret(v.secretsPrepared[v.EncryptionKey][1]),
					}},
				},
			},
			{
				Identity: &apiserverconfigv1.IdentityConfiguration{},
			},
		}))
	}).Should(Succeed(), "etcd encryption config should only have new key")
}

func getBase64EncodedETCDEncryptionKeyFromSecret(secret corev1.Secret) string {
	var key string
	if encoding := secret.Data["encoding"]; string(encoding) == "none" {
		key = utils.EncodeBase64(secret.Data["secret"])
	} else {
		key = string(secret.Data["secret"])
	}
	return key
}
