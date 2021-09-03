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

package secrets_test

import (
	. "github.com/gardener/gardener/pkg/utils/secrets"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Static Token Secrets", func() {
	Describe("StaticToken Secret Configuration", func() {
		var (
			staticTokenConfig   *StaticTokenSecretConfig
			staticTokenInfoData *StaticTokenInfoData
			username            = "bar"
		)

		BeforeEach(func() {
			staticTokenConfig = &StaticTokenSecretConfig{
				Name: "static-token",
				Tokens: map[string]TokenConfig{
					username: {
						Username: username,
						UserID:   "foo",
						Groups:   []string{"group"},
					},
				},
			}

			staticTokenInfoData = &StaticTokenInfoData{
				Tokens: map[string]string{
					username: "foo",
				},
			}
		})

		Describe("#Generate", func() {
			It("should properly generate RSAKeys object", func() {
				obj, err := staticTokenConfig.Generate()
				Expect(err).NotTo(HaveOccurred())

				staticToken, ok := obj.(*StaticToken)
				Expect(ok).To(BeTrue())

				Expect(len(staticToken.Tokens)).To(Equal(1))
				Expect(staticToken.Tokens[0].Token).ToNot(Equal(""))
				Expect(staticToken.Tokens[0].Username).To(Equal(staticTokenConfig.Tokens[username].Username))
				Expect(staticToken.Tokens[0].UserID).To(Equal(staticTokenConfig.Tokens[username].UserID))
				Expect(staticToken.Tokens[0].Groups).To(Equal(staticTokenConfig.Tokens[username].Groups))
			})
		})

		Describe("#GenerateInfoData", func() {
			It("should generate correct StaticToken InfoData", func() {
				obj, err := staticTokenConfig.GenerateInfoData()
				Expect(err).NotTo(HaveOccurred())

				Expect(obj.TypeVersion()).To(Equal(StaticTokenDataType))

				currentStaticTokenInfoData, ok := obj.(*StaticTokenInfoData)
				Expect(ok).To(BeTrue())

				Expect(len(currentStaticTokenInfoData.Tokens)).To(Equal(1))
				Expect(currentStaticTokenInfoData.Tokens[username]).ToNot(Equal(""))
			})
		})

		Describe("#GenerateFromInfoData", func() {
			It("should properly load StaticToken object from StaticTokenInfoData", func() {
				obj, err := staticTokenConfig.GenerateFromInfoData(staticTokenInfoData)
				Expect(err).NotTo(HaveOccurred())

				staticToken, ok := obj.(*StaticToken)
				Expect(ok).To(BeTrue())

				Expect(len(staticToken.Tokens)).To(Equal(1))
				Expect(staticToken.Tokens[0].Token).To(Equal(staticTokenInfoData.Tokens[username]))
				Expect(staticToken.Tokens[0].Username).To(Equal(staticTokenConfig.Tokens[username].Username))
				Expect(staticToken.Tokens[0].UserID).To(Equal(staticTokenConfig.Tokens[username].UserID))
				Expect(staticToken.Tokens[0].Groups).To(Equal(staticTokenConfig.Tokens[username].Groups))
			})
		})

		Describe("#LoadFromSecretData", func() {
			It("should properly load StaticTokenInfoData from secret data", func() {
				secretData := map[string][]byte{
					DataKeyStaticTokenCSV: []byte("foo,bar,foo,group"),
				}
				obj, err := staticTokenConfig.LoadFromSecretData(secretData)
				Expect(err).NotTo(HaveOccurred())

				currentStaticTokenInfoData, ok := obj.(*StaticTokenInfoData)
				Expect(ok).To(BeTrue())

				Expect(currentStaticTokenInfoData).To(Equal(staticTokenInfoData))
			})
		})
	})

	Describe("StaticToken Object", func() {
		var staticToken *StaticToken

		BeforeEach(func() {
			staticToken = &StaticToken{
				Tokens: []Token{
					{
						Token:    "foo",
						Username: "foo",
						UserID:   "bar",
						Groups:   []string{"group"},
					},
				},
			}
		})

		Describe("#SecretData", func() {
			It("should properly return secret data", func() {
				secretData := map[string][]byte{
					DataKeyStaticTokenCSV: []byte("foo,foo,bar,group"),
				}
				Expect(staticToken.SecretData()).To(Equal(secretData))
			})
		})
	})
})
