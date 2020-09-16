// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package etcdencryption_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/gardener/gardener/pkg/operation/common"
	. "github.com/gardener/gardener/pkg/operation/etcdencryption"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
)

var _ = Describe("Encryption Configuration", func() {
	const (
		kind       = "EncryptionConfiguration"
		apiVersion = "apiserver.config.k8s.io/v1"
	)

	var (
		typeMeta              metav1.TypeMeta
		randomBytes           []byte
		r                     io.Reader
		randomBase64          string
		t                     time.Time
		keyName               string
		yamlString            string
		yamlData              []byte
		identityConfiguration apiserverconfigv1.ProviderConfiguration
		aescbcConfiguration   apiserverconfigv1.ProviderConfiguration
		passiveConf           *apiserverconfigv1.EncryptionConfiguration
		activeConf            *apiserverconfigv1.EncryptionConfiguration
	)
	BeforeEach(func() {
		typeMeta = metav1.TypeMeta{APIVersion: apiVersion, Kind: kind}
		t = time.Unix(10, 0)
		randomBytes = bytes.Repeat([]byte{1}, common.EtcdEncryptionKeySecretLen)
		r = bytes.NewReader(randomBytes)
		randomBase64 = base64.StdEncoding.EncodeToString(randomBytes)
		keyName = NewEncryptionKeyName(t)
		yamlString = fmt.Sprintf(
			`apiVersion: %s
kind: %s
resources:
- providers:
  - identity: {}
  - aescbc:
      keys:
      - name: %s
        secret: %s
  resources:
  - secrets
`, apiVersion, kind, NewEncryptionKeyName(t), randomBase64)
		yamlData = []byte(yamlString)
		identityConfiguration = apiserverconfigv1.ProviderConfiguration{Identity: &apiserverconfigv1.IdentityConfiguration{}}
		aescbcConfiguration = apiserverconfigv1.ProviderConfiguration{AESCBC: &apiserverconfigv1.AESConfiguration{
			Keys: []apiserverconfigv1.Key{
				{
					Name:   keyName,
					Secret: randomBase64,
				},
			},
		}}
		passiveConf = &apiserverconfigv1.EncryptionConfiguration{
			Resources: []apiserverconfigv1.ResourceConfiguration{
				{
					Resources: []string{common.EtcdEncryptionEncryptedResourceSecrets},
					Providers: []apiserverconfigv1.ProviderConfiguration{
						identityConfiguration,
						aescbcConfiguration,
					},
				},
			},
		}
		activeConf = &apiserverconfigv1.EncryptionConfiguration{
			Resources: []apiserverconfigv1.ResourceConfiguration{
				{
					Resources: []string{common.EtcdEncryptionEncryptedResourceSecrets},
					Providers: []apiserverconfigv1.ProviderConfiguration{
						aescbcConfiguration,
						identityConfiguration,
					},
				},
			},
		}
	})

	Describe("#NewEncryptionKey", func() {
		It("should create a new encryption key", func() {
			key, err := NewEncryptionKey(t, r)
			Expect(err).NotTo(HaveOccurred())

			Expect(key.Name).To(Equal(NewEncryptionKeyName(t)))
			Expect(key.Secret).To(Equal(randomBase64))
		})
	})

	Describe("#NewEncryptionKeySecret", func() {
		It("should read data and generate a secret", func() {
			secret, err := NewEncryptionKeySecret(r)

			Expect(err).NotTo(HaveOccurred())
			Expect(secret).To(Equal(randomBase64))
		})
	})

	Describe("#NewEncryptionKeyName", func() {
		It("should create a new encryption key name", func() {
			keyName := NewEncryptionKeyName(t)

			Expect(keyName).To(Equal("key10"))
		})
	})

	Describe("#NewEncryptionConfiguration", func() {
		var etcdEncryption *EncryptionConfig

		BeforeEach(func() {
			etcdEncryption = &EncryptionConfig{
				EncryptionKeys: []EncryptionKey{
					{
						Name: keyName,
						Key:  randomBase64,
					},
				},
			}
		})
		It("should create a new active encryption configuration with AESCBC and identity providers", func() {
			actual := NewEncryptionConfiguration(etcdEncryption)

			Expect(actual).To(Equal(activeConf))
		})

		It("should create a new passive configuration with identity and AESCBC providers", func() {
			etcdEncryption.ForcePlainTextResources = true
			actual := NewEncryptionConfiguration(etcdEncryption)

			Expect(actual).To(Equal(passiveConf))
		})
	})

	Describe("#GetSecretKeyForResources", func() {
		It("should get the secret key and name for the provided list of resources", func() {
			name, key, err := GetSecretKeyForResources(activeConf, common.EtcdEncryptionEncryptedResourceSecrets)

			Expect(err).To(BeNil())
			Expect(name).To(Equal(keyName))
			Expect(key).To(Equal(randomBase64))
		})
	})

	Describe("#Load", func() {
		It("should correctly load the encryption configuration", func() {
			passiveConf.TypeMeta = typeMeta
			actual, err := Load(yamlData)

			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal(passiveConf))
		})
	})

	Describe("#Write", func() {
		It("should correctly write the encryption configuration", func() {
			actual, err := Write(passiveConf)

			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal(yamlData))
		})
	})

	Describe("#ReadSecret", func() {
		It("should read the secret and validate it", func() {
			passiveConf.TypeMeta = typeMeta
			secret := &corev1.Secret{
				Data: map[string][]byte{
					common.EtcdEncryptionSecretFileName: yamlData,
				},
			}

			conf, err := ReadSecret(secret)

			Expect(err).NotTo(HaveOccurred())
			Expect(conf).To(Equal(passiveConf))
		})

		It("should error if there is no data at the expected key", func() {
			secret := &corev1.Secret{}

			_, err := ReadSecret(secret)

			Expect(err).To(HaveOccurred())
			Expect(IsConfigurationNotFoundError(err)).To(BeTrue())
		})
	})

	Describe("#UpdateSecret", func() {
		It("should write the secret configuration at the expected key", func() {
			secret := &corev1.Secret{}

			Expect(UpdateSecret(secret, passiveConf)).To(Succeed())
			Expect(secret.Data[common.EtcdEncryptionSecretFileName]).To(Equal(yamlData))
		})
	})
})
