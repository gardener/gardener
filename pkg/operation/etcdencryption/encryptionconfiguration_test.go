// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package encryptionconfiguration_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"

	"github.com/gardener/gardener/pkg/operation/common"

	. "github.com/gardener/gardener/pkg/operation/etcdencryption"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
					Name:   NewEncryptionKeyName(t),
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

	Describe("#ParseEncryptionKeyName", func() {
		It("should parse the creation time of the encryption key", func() {
			keyName := NewEncryptionKeyName(t)

			t, err := ParseEncryptionKeyName(keyName)

			Expect(err).NotTo(HaveOccurred())
			Expect(t).To(Equal(t))
		})
	})

	Describe("#NewPassiveConfiguration", func() {
		It("should create a new encryption configuration with identity and AESCBC", func() {
			actual, err := NewPassiveConfiguration(t, r)

			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal(passiveConf))
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

	Describe("#SetResourceEncryption", func() {
		It("should enable the resource encryption for secrets", func() {
			conf := passiveConf.DeepCopy()
			Expect(SetResourceEncryption(conf, common.EtcdEncryptionEncryptedResourceSecrets, true)).
				To(Succeed())
			Expect(conf).To(Equal(activeConf))
		})

		It("should disable the resource encryption for secrets", func() {
			conf := activeConf.DeepCopy()
			Expect(SetResourceEncryption(conf, common.EtcdEncryptionEncryptedResourceSecrets, false)).
				To(Succeed())
			Expect(conf).To(Equal(passiveConf))
		})

		It("should error if there is no configuration for a resource", func() {
			Expect(SetResourceEncryption(passiveConf, "configmaps", true)).To(HaveOccurred())
		})

		It("should error if there is no encrypting provider and encrypted is true", func() {
			conf := passiveConf.DeepCopy()
			conf.Resources[0].Providers = []apiserverconfigv1.ProviderConfiguration{identityConfiguration}
			Expect(SetResourceEncryption(conf, common.EtcdEncryptionEncryptedResourceSecrets, true)).To(HaveOccurred())
		})

		It("should error if there is no identity provider and encrypted is false", func() {
			conf := passiveConf.DeepCopy()
			conf.Resources[0].Providers = []apiserverconfigv1.ProviderConfiguration{aescbcConfiguration}
			Expect(SetResourceEncryption(conf, common.EtcdEncryptionEncryptedResourceSecrets, false)).To(HaveOccurred())
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
