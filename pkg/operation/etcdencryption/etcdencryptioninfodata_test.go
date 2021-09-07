// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package etcdencryption_test

import (
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/operation/common"
	. "github.com/gardener/gardener/pkg/operation/etcdencryption"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const yamlString = `apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
- providers:
  - aescbc:
      keys:
      - name: bar
        secret: foo
  - identity: {}
  resources:
  - secrets`

const jsonData = "{\"encryptionKeys\":[{\"key\":\"foo\",\"name\":\"bar\"}],\"forcePlainTextResources\":false,\"rewriteResources\":false}"

var _ = Describe("ETCD Encryption InfoData", func() {
	Describe("EncryptionConfig", func() {
		var (
			etcdEncryptionDataJson []byte
			etcdEncryptionConfig   *EncryptionConfig
		)

		BeforeEach(func() {
			etcdEncryptionDataJson = []byte(jsonData)
			etcdEncryptionConfig = &EncryptionConfig{
				EncryptionKeys: []EncryptionKey{
					{
						Key:  "foo",
						Name: "bar",
					},
				},
				ForcePlainTextResources: false,
				RewriteResources:        false,
			}
		})
		Describe("#Marshal", func() {
			It("should marshal EncryptionConfig InfoData into correct json format", func() {
				data, err := etcdEncryptionConfig.Marshal()

				Expect(err).NotTo(HaveOccurred())
				Expect(data).To(Equal(etcdEncryptionDataJson))
			})
		})
		Describe("#SetForcePlainTextResources", func() {
			It("should set ForcePlainTextResources and RewriteResources fields to true", func() {
				etcdEncryptionConfig.SetForcePlainTextResources(true)
				Expect(etcdEncryptionConfig.ForcePlainTextResources).To(Equal(true))
				Expect(etcdEncryptionConfig.RewriteResources).To(Equal(true))
			})
		})
	})

	Context("ETCD Encryptyion InfoData Utility functions", func() {
		var (
			gardenerResourceDataList gardencorev1alpha1helper.GardenerResourceDataList
			secret                   *corev1.Secret
			expectedEncryptionConfig *EncryptionConfig
			expectedEncryptionKey    EncryptionKey
		)

		BeforeEach(func() {
			gardenerResourceDataList = gardencorev1alpha1helper.GardenerResourceDataList{
				{
					Name: common.ETCDEncryptionConfigDataName,
					Type: string(ETCDEncryptionDataType),
					Data: runtime.RawExtension{Raw: []byte(jsonData)},
				},
			}

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						common.EtcdEncryptionForcePlaintextAnnotationName: "true",
					},
				},
				Data: map[string][]byte{
					"encryption-configuration.yaml": []byte(yamlString),
				},
			}

			expectedEncryptionKey = EncryptionKey{
				Key:  "foo",
				Name: "bar",
			}

			expectedEncryptionConfig = &EncryptionConfig{
				EncryptionKeys: []EncryptionKey{
					expectedEncryptionKey,
				},
				ForcePlainTextResources: false,
				RewriteResources:        false,
			}
		})

		Describe("#GetETCDEncryption", func() {
			It("should retrieve EncryptionConfig when it exists in the gardener resource data list", func() {
				etcdEncryption, err := GetEncryptionConfig(gardenerResourceDataList)
				Expect(err).NotTo(HaveOccurred())
				Expect(etcdEncryption).To(Equal(expectedEncryptionConfig))
			})
			It("should return nil when EncryptionConfig does not exists in the gardener resource data list", func() {
				etcdEncryption, err := GetEncryptionConfig(gardencorev1alpha1helper.GardenerResourceDataList{})
				Expect(err).NotTo(HaveOccurred())
				Expect(etcdEncryption).To(BeNil())
			})
		})
		Describe("Generate Encryption Key", func() {
			var etcdEncryptionConfig *EncryptionConfig

			BeforeEach(func() {
				etcdEncryptionConfig = &EncryptionConfig{}
			})
			It("should sync the encryption key from an already existing secret", func() {
				err := etcdEncryptionConfig.AddEncryptionKeyFromSecret(secret)
				Expect(err).NotTo(HaveOccurred())
				Expect(etcdEncryptionConfig.EncryptionKeys[0]).To(Equal(expectedEncryptionKey))
			})
			It("should generate a new encryption key", func() {
				err := etcdEncryptionConfig.AddNewEncryptionKey()
				Expect(err).NotTo(HaveOccurred())
				Expect(etcdEncryptionConfig.EncryptionKeys[0]).ToNot(BeNil())
			})
		})
	})
})
