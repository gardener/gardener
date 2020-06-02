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
	"crypto/rand"
	"crypto/rsa"

	"github.com/gardener/gardener/pkg/utils"

	. "github.com/gardener/gardener/pkg/utils/secrets"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RSA Private Key Secrets", func() {
	Describe("RSA Secret Configuration", func() {
		var (
			rsaPrivateKeyConfig *RSASecretConfig
			rsaInfoData         *PrivateKeyInfoData
		)

		BeforeEach(func() {
			rsaPrivateKeyConfig = &RSASecretConfig{
				Bits: 16,
				Name: "rsa-secret",
			}
			rsaInfoData = &PrivateKeyInfoData{
				PrivateKey: []byte("foo"),
			}
		})

		Describe("#Generate", func() {
			It("should properly generate RSAKeys object", func() {
				obj, err := rsaPrivateKeyConfig.Generate()
				Expect(err).NotTo(HaveOccurred())

				rsaSecret, ok := obj.(*RSAKeys)
				Expect(ok).To(BeTrue())

				Expect(rsaSecret.PrivateKey).NotTo(BeNil())
				Expect(*rsaSecret.PublicKey).To(Equal(rsaSecret.PrivateKey.PublicKey))

			})
			It("should generate ssh public key if specified in the config", func() {
				rsaPrivateKeyConfig.UsedForSSH = true
				obj, err := rsaPrivateKeyConfig.Generate()
				Expect(err).NotTo(HaveOccurred())

				rsaSecret, ok := obj.(*RSAKeys)
				Expect(ok).To(BeTrue())
				Expect(rsaSecret.OpenSSHAuthorizedKey).NotTo(BeNil())
			})
		})

		Describe("#GenerateInfoData", func() {
			It("should generate correct PrivateKey InfoData", func() {
				obj, err := rsaPrivateKeyConfig.GenerateInfoData()
				Expect(err).NotTo(HaveOccurred())

				Expect(obj.TypeVersion()).To(Equal(PrivateKeyDataType))

				privateKeyInfoData, ok := obj.(*PrivateKeyInfoData)
				Expect(ok).To(BeTrue())

				Expect(privateKeyInfoData.PrivateKey).NotTo(BeNil())
			})
		})

		Describe("#GenerateFromInfoData", func() {
			It("should properly load RSAkeys object from PrivateKeyInfoData", func() {
				base64Key := "LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb2dJQkFBS0NBUUVBMUtRQ2o4dXk4RWRRT1lRVmNBQ1VDQVBVdjJzNS9pUlZrQ2VBY3Y5eXhGTWJOUk5jCnFUTEx4YmRRK0lRcGVzcmcrQWFra3FORjJONHFsRXU2bUdlcnhJaWVLT1JCMVlYaFE3NlZGUnB2NVBpWVVkZDgKdXNCMjcwUlpyeFduSytCZVpFYkY1bS8xNHlqYmF1S2pHS0NRWDFWOCsyNDRGUlFoSkkxSUhPVU05ZmxXM0FHOQprOU95dFVCcnB2dkEwK0FiazU3VEZNcGp0Ny9OK2dZbC9yNkZ0SExXczk2YmdnMHdiM1E3M3M5bmlzalFhR1dkCmY0MzNNR2ZMTjdpVVRyR0hUN05KSEZxNlhjNVFLdHNmaUNFV0pzM2Rmc2JlK1dUeWMwcExXenpobFVLNGpyZHEKZFZIcVFoL0x1MU5NWkNuRjc1T2hFaEw5STRyeGdWUzJORTlUNVFJREFRQUJBb0lCQUdjckVBY2VhTG9GajVubwpTTkpucFdyaUdQV3FtNTkvbDNmeWduTEpGN0RETlo4aHZzSmszOU1VNXhjOFlEZXdlVWc5U05uUWw5SHBzTFQwCnJSckdxZU1YK2N5VE9wSFRmQUQzVmJQQWVPdVo1YVZIckwrYkk5bGd5emFVaGVCVzR0VTZOVWhocCtaSDYzVkgKY3FRL091eldPR1p4Q29ySGtuRCtqeTlkdmxVVWVucUxXUzh1VjB4M3MyR0hsMzZiUCt4VUhIQWhleFJuRHgzdQo3QjA5R2FPYjF3aUVzVXFZdmtBdm5MbXJ5RDB0bmwvYndmSGZuZ0dLeXo2WDQvcmdSd1lrcGxJVkVhbXhMbjZVCkE0djBKWUx0b1NiblIxTHF5R2pLL2dSRzdFeFJPb0RJKy85SXJmeHV1aStHd1VBalJKdHlWRXFLcS9RUVB5OUcKc01GL3RtRUNnWUVBMTBiaHJYOHRZUzI1MEwvT0xDQThybEJiUjR2c0xqUTRDVm5Pbytsc21aUXh3OStlNFBWNwpVN2RndmpWTWlITGlOWnZFaEU0NGw2dHhhbHBWcHFzSHRFcDMwUW5zUkFsL1djSElibWJVU0xtWkk2a3Bmd2tmCldyd29pVUd3U21hcVlKenZ2YVNCSy9MblIwOVd1Ny9YK1J0eUdmVWtXTVdoaE1sSG0xaElYYjBDZ1lFQS9OMTMKTFhpTERiMnJWZGJOYVJ5UjhaUFd0bWVYOUNQWG16dy9EMFlQT203ZWU2U2xLRjBKTkZqMk8rWVJPelM5cXlmQQpFbzIwL1JmK0pDNUVOWFR1MEpQY0ZZK3J2YklyUzNiRGlHSkVQNXMzZW82SFFiSjRqN1U4ays3S1dJQ2huSnlvCi9ISmEwVEExMk82UlF4aEVWUzBGL0xkUjVFZEZSUE5TL3Z3eURVa0NnWUJ6WjhkQjJDUytyT0dwRzdudUE5WWoKNkdZV285Y1lLZHhFZndWODczek5sQmxkbFBxNlJEODU4TnVHL0ZHcjhGSitSS1FEL1Y3dlIvUkQvR3RnTHQyeApkQjVwVExXQS84cHFscXpaS256eEE3WXAzTnluQW4veGgxNy92ZHhBOW1xdDRsUFBTV29KNG16RDJLOTVkTzNWCjJEWEIzcDMrak92Nm9HQ24wWnJ4elFLQmdFMmtSc0s4ZjUzaGZpbG1RajRqR3FEZHJ4RGs4Q0J6blBFNlozWnUKSWFEa2lBWFpBU2xLbjlmbDlQMWhZQ3NZdjZBOUhWblZEeHlqY0ZKMXJsWG5xS2g4cmhna3ZDd0wrQVU4Mno4VwpSVFJ1bVhOVkxpeTYrdy9OSzJPVTc2YUxJSlJ3K2VaQnlxYnVzYW9CWHJNR1VYMEJ6UlBTeWg5WXp1a2orWGozCndQcVpBb0dBSnBKS05Qb0hhaytiQ1ovcVQ0V3NiS1I4TUVsNTMxYmpyaVdXWjhHMVN1VFZmenVmaWNibThMYTEKd3VSK0p6NWp4QnkyS21FM2owTFh6RUthOHpJOFJqRHN6bkowekgvc091bEh6b3pENDYzc1lsOEdHcnpFU3M5NAo1RTdPMHZ3TUFUL1lWck5aNGJMdFdIS2tCNzJ4K3Y4aEF3RDcrWGJhaXk0cHJiRHJjclk9Ci0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg=="
				privateKey, err := utils.DecodeBase64(base64Key)
				Expect(err).NotTo(HaveOccurred())

				rsaInfoData.PrivateKey = privateKey
				obj, err := rsaPrivateKeyConfig.GenerateFromInfoData(rsaInfoData)
				Expect(err).NotTo(HaveOccurred())

				rsaSecret, ok := obj.(*RSAKeys)
				Expect(ok).To(BeTrue())

				expectedPrivateKey, err := utils.DecodePrivateKey(rsaInfoData.PrivateKey)
				Expect(err).NotTo(HaveOccurred())

				Expect(rsaSecret.PrivateKey).To(Equal(expectedPrivateKey))
				Expect(*rsaSecret.PublicKey).To(Equal(rsaSecret.PrivateKey.PublicKey))
			})
		})

		Describe("#LoadFromSecretData", func() {
			It("should properly load PrivateKeyInfoData from secret data", func() {
				secretData := map[string][]byte{
					DataKeyRSAPrivateKey: []byte("foo"),
				}
				obj, err := rsaPrivateKeyConfig.LoadFromSecretData(secretData)
				Expect(err).NotTo(HaveOccurred())

				currentRSAInfoData, ok := obj.(*PrivateKeyInfoData)
				Expect(ok).To(BeTrue())
				Expect(currentRSAInfoData).To(Equal(rsaInfoData))
			})
		})
	})

	Describe("RSAKeys Object", func() {
		var (
			rsaKeys *RSAKeys
			key     *rsa.PrivateKey
		)
		BeforeEach(func() {
			var err error
			key, err = rsa.GenerateKey(rand.Reader, 16)
			Expect(err).NotTo(HaveOccurred())

			rsaKeys = &RSAKeys{
				PrivateKey:           key,
				OpenSSHAuthorizedKey: []byte("bar"),
			}
		})

		Describe("#SecretData", func() {
			It("should properly return secret data", func() {
				secretData := map[string][]byte{
					DataKeyRSAPrivateKey:     utils.EncodePrivateKey(key),
					DataKeySSHAuthorizedKeys: []byte("bar"),
				}
				Expect(rsaKeys.SecretData()).To(Equal(secretData))
			})
		})
	})
})
