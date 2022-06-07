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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/gardener/gardener/test/framework"
)

// ETCDEncryptionKeyVerifier verifies the etcd encryption key rotation.
type ETCDEncryptionKeyVerifier struct {
	*framework.ShootCreationFramework

	secretsBefore   secretConfigNamesToSecrets
	secretsPrepared secretConfigNamesToSecrets
}

const etcdEncryptionKey = "kube-apiserver-etcd-encryption-key"

var (
	decoder runtime.Decoder

	labelSelectorEncryptionConfig = client.MatchingLabels{v1beta1constants.LabelRole: v1beta1constants.SecretNamePrefixETCDEncryptionConfiguration}
)

func init() {
	scheme := runtime.NewScheme()
	utilruntime.Must(apiserverconfigv1.AddToScheme(scheme))
	decoder = serializer.NewCodecFactory(scheme).UniversalDeserializer()
}

// Before is called before the rotation is started.
func (v *ETCDEncryptionKeyVerifier) Before(ctx context.Context) {
	seedClient := v.ShootFramework.SeedClient.Client()

	By("Verify old etcd encryption key secret")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(seedClient.List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), managedByGardenletSecretsManager)).To(Succeed())

		v.secretsBefore = groupByName(secretList.Items)
		g.Expect(v.secretsBefore).To(HaveKeyWithValue(etcdEncryptionKey, HaveLen(1)), "etcd encryption key should get created, but not rotated yet")
	}).Should(Succeed())

	By("Verify old etcd encryption config secret")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(seedClient.List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), labelSelectorEncryptionConfig)).To(Succeed())
		g.Expect(secretList.Items).To(HaveLen(1))

		encryptionConfiguration := &apiserverconfigv1.EncryptionConfiguration{}
		g.Expect(runtime.DecodeInto(decoder, secretList.Items[0].Data["encryption-configuration.yaml"], encryptionConfiguration)).To(Succeed())

		g.Expect(encryptionConfiguration.Resources).To(HaveLen(1))
		g.Expect(encryptionConfiguration.Resources[0].Providers).To(DeepEqual([]apiserverconfigv1.ProviderConfiguration{
			{
				AESCBC: &apiserverconfigv1.AESConfiguration{
					Keys: []apiserverconfigv1.Key{{
						// old key
						Name:   string(v.secretsBefore[etcdEncryptionKey][0].Data["key"]),
						Secret: string(v.secretsBefore[etcdEncryptionKey][0].Data["secret"]),
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
	g.Expect(v1beta1helper.GetShootETCDEncryptionKeyRotationPhase(v.Shoot.Status.Credentials)).To(Equal(gardencorev1beta1.RotationPreparing))
	g.Expect(time.Now().UTC().Sub(v.Shoot.Status.Credentials.Rotation.ETCDEncryptionKey.LastInitiationTime.Time.UTC())).To(BeNumerically("<=", time.Minute))
}

// AfterPrepared is called when the Shoot is in Prepared status.
func (v *ETCDEncryptionKeyVerifier) AfterPrepared(ctx context.Context) {
	seedClient := v.ShootFramework.SeedClient.Client()

	Expect(v.Shoot.Status.Credentials.Rotation.ETCDEncryptionKey.Phase).To(Equal(gardencorev1beta1.RotationPrepared), "rotation phase should be 'Prepared'")

	By("Verify etcd encryption key secrets")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(seedClient.List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), managedByGardenletSecretsManager)).To(Succeed())

		v.secretsPrepared = groupByName(secretList.Items)
		g.Expect(v.secretsPrepared).To(HaveKeyWithValue(etcdEncryptionKey, HaveLen(2)), "etcd encryption key should get rotated")
		g.Expect(v.secretsPrepared).To(HaveKeyWithValue(etcdEncryptionKey, ContainElement(v.secretsBefore[etcdEncryptionKey][0])), "old etcd encryption key secret should be kept")
	}).Should(Succeed())

	By("Verify combined etcd encryption config secret")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(seedClient.List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), labelSelectorEncryptionConfig)).To(Succeed())
		g.Expect(secretList.Items).To(HaveLen(1))

		encryptionConfiguration := &apiserverconfigv1.EncryptionConfiguration{}
		_, err := runtime.Decode(decoder, secretList.Items[0].Data["encryption-configuration.yaml"])
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(encryptionConfiguration.Resources).To(HaveLen(1))
		g.Expect(encryptionConfiguration.Resources[0].Providers).To(DeepEqual([]apiserverconfigv1.ProviderConfiguration{
			{
				AESCBC: &apiserverconfigv1.AESConfiguration{
					Keys: []apiserverconfigv1.Key{{
						// new key
						Name:   string(v.secretsPrepared[etcdEncryptionKey][1].Data["key"]),
						Secret: string(v.secretsPrepared[etcdEncryptionKey][1].Data["secret"]),
					}, {
						// old key
						Name:   string(v.secretsPrepared[etcdEncryptionKey][0].Data["key"]),
						Secret: string(v.secretsPrepared[etcdEncryptionKey][0].Data["secret"]),
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
	g.Expect(v1beta1helper.GetShootETCDEncryptionKeyRotationPhase(v.Shoot.Status.Credentials)).To(Equal(gardencorev1beta1.RotationCompleting))
}

// AfterCompleted is called when the Shoot is in Completed status.
func (v *ETCDEncryptionKeyVerifier) AfterCompleted(ctx context.Context) {
	seedClient := v.ShootFramework.SeedClient.Client()

	etcdEncryptionKeyRotation := v.Shoot.Status.Credentials.Rotation.ETCDEncryptionKey
	Expect(v1beta1helper.GetShootETCDEncryptionKeyRotationPhase(v.Shoot.Status.Credentials)).To(Equal(gardencorev1beta1.RotationCompleted))
	Expect(etcdEncryptionKeyRotation.LastCompletionTime.Time.UTC().After(etcdEncryptionKeyRotation.LastInitiationTime.Time.UTC())).To(BeTrue())

	By("Verify new etcd encryption key secret")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(seedClient.List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), managedByGardenletSecretsManager)).To(Succeed())

		grouped := groupByName(secretList.Items)
		g.Expect(grouped).To(HaveKeyWithValue(etcdEncryptionKey, HaveLen(1)), "old etcd encryption key should get cleaned up")
		g.Expect(grouped).To(HaveKeyWithValue(etcdEncryptionKey, ContainElement(v.secretsPrepared[etcdEncryptionKey][1])), "new etcd encryption key secret should be kept")
	}).Should(Succeed())

	By("Verify new etcd encryption config secret")
	Eventually(func(g Gomega) {
		secretList := &corev1.SecretList{}
		g.Expect(seedClient.List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), labelSelectorEncryptionConfig)).To(Succeed())
		g.Expect(secretList.Items).To(HaveLen(1))

		encryptionConfiguration := &apiserverconfigv1.EncryptionConfiguration{}
		_, err := runtime.Decode(decoder, secretList.Items[0].Data["encryption-configuration.yaml"])
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(encryptionConfiguration.Resources).To(HaveLen(1))
		g.Expect(encryptionConfiguration.Resources[0].Providers).To(DeepEqual([]apiserverconfigv1.ProviderConfiguration{
			{
				AESCBC: &apiserverconfigv1.AESConfiguration{
					Keys: []apiserverconfigv1.Key{{
						// new key
						Name:   string(v.secretsPrepared[etcdEncryptionKey][1].Data["key"]),
						Secret: string(v.secretsPrepared[etcdEncryptionKey][1].Data["secret"]),
					}},
				},
			},
			{
				Identity: &apiserverconfigv1.IdentityConfiguration{},
			},
		}))
	}).Should(Succeed(), "etcd encryption config should only have new key")
}
