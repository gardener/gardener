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

package manager_test

import (
	"github.com/gardener/gardener/pkg/utils"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	. "github.com/gardener/gardener/pkg/utils/secrets/manager"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var _ = Describe("Manager", func() {
	Describe("#ObjectMeta", func() {
		var (
			configName              = "config-name"
			namespace               = "some-namespace"
			lastRotationStartedTime = "1646060228"
		)

		It("should generate the expected object meta for a never-rotated CA cert secret", func() {
			config := &secretutils.CertificateSecretConfig{Name: configName}

			meta, err := ObjectMeta(namespace, config, "", nil, nil, nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(meta).To(Equal(metav1.ObjectMeta{
				Name:      configName,
				Namespace: namespace,
				Labels: map[string]string{
					"name":                       configName,
					"managed-by":                 "secrets-manager",
					"checksum-of-config":         "1645436262831067767",
					"last-rotation-started-time": "",
				},
			}))
		})

		It("should generate the expected object meta for a rotated CA cert secret", func() {
			config := &secretutils.CertificateSecretConfig{Name: configName}

			meta, err := ObjectMeta(namespace, config, lastRotationStartedTime, nil, nil, nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(meta).To(Equal(metav1.ObjectMeta{
				Name:      configName + "-76711",
				Namespace: namespace,
				Labels: map[string]string{
					"name":                       configName,
					"managed-by":                 "secrets-manager",
					"checksum-of-config":         "1645436262831067767",
					"last-rotation-started-time": "1646060228",
				},
			}))
		})

		DescribeTable("check different label options",
			func(nameInfix string, signingCAChecksum *string, persist *bool, bundleFor *string, extraLabels map[string]string) {
				config := &secretutils.CertificateSecretConfig{
					Name:      configName,
					SigningCA: &secretutils.Certificate{},
				}

				meta, err := ObjectMeta(namespace, config, lastRotationStartedTime, signingCAChecksum, persist, bundleFor)
				Expect(err).NotTo(HaveOccurred())

				labels := map[string]string{
					"name":                       configName,
					"managed-by":                 "secrets-manager",
					"checksum-of-config":         "17861245496710117091",
					"last-rotation-started-time": "1646060228",
				}

				Expect(meta).To(Equal(metav1.ObjectMeta{
					Name:      configName + "-" + nameInfix + "-76711",
					Namespace: namespace,
					Labels:    utils.MergeStringMaps(labels, extraLabels),
				}))
			},

			Entry("no extras", "a9c2fcb9", nil, nil, nil, nil),
			Entry("with signing ca checksum", "a11a0b2d", pointer.String("checksum"), nil, nil, map[string]string{"checksum-of-signing-ca": "checksum"}),
			Entry("with persist", "a9c2fcb9", nil, pointer.Bool(true), nil, map[string]string{"persist": "true"}),
			Entry("with bundleFor", "a9c2fcb9", nil, nil, pointer.String("bundle-origin"), map[string]string{"bundle-for": "bundle-origin"}),
		)
	})

	DescribeTable("#Secret",
		func(data map[string][]byte, expectedType corev1.SecretType) {
			objectMeta := metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			}

			Expect(Secret(objectMeta, data)).To(Equal(&corev1.Secret{
				ObjectMeta: objectMeta,
				Data:       data,
				Type:       corev1.SecretTypeOpaque,
				Immutable:  pointer.Bool(true),
			}))
		},

		Entry("regular secret", map[string][]byte{"some": []byte("data")}, corev1.SecretTypeOpaque),
		Entry("tls secret", map[string][]byte{"tls.key": nil, "tls.crt": nil}, corev1.SecretTypeTLS),
	)
})
