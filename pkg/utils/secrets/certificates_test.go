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
	"github.com/gardener/gardener/pkg/utils"

	. "github.com/gardener/gardener/pkg/utils/secrets"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Certificate Secrets", func() {
	Describe("Certificate Configuration", func() {
		var (
			certificateConfig   *CertificateSecretConfig
			certificateInfoData *CertificateInfoData

			base64Cert = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURCekNDQWUrZ0F3SUJBZ0lKQUpMN2JKT01pajd2TUEwR0NTcUdTSWIzRFFFQkJRVUFNQm94R0RBV0JnTlYKQkFNTUQzZDNkeTVsZUdGdGNHeGxMbU52YlRBZUZ3MHhOekE1TVRnd05qRTFOVEZhRncweU56QTVNVFl3TmpFMQpOVEZhTUJveEdEQVdCZ05WQkFNTUQzZDNkeTVsZUdGdGNHeGxMbU52YlRDQ0FTSXdEUVlKS29aSWh2Y05BUUVCCkJRQURnZ0VQQURDQ0FRb0NnZ0VCQUs0OHZGVW9SMytJS2lUYTYzdEUrcE95WW9iNHdjeklDNWNvMlBXUlZoUHUKMkZLTmhRdUQ3Nk1ETmY4eVhJUTh4TzZRTlQxQlBKQ2RnM3FqQWpkU0QwcUlkeUc2L3ZoMVZaeWVCWHJYdFR6bQpKR21LSVg4K1IzVzVVS3RXSUtXclJjMzFERVVGb1Urakp5U2QyakllQWNOdWM0ZEZnZGhhblYvRkxDaHJJbTNRClBXeHRlS1QwZU52bkJFZEg2a3dqNU9uWE9XUlgraGpMNEdIcTM3M3k0S2RXclNGNGxaa2RGQVdFZFd3cFFDNXEKOFByVTdPUHcwMW1WZUN5dm1nZGF4THhsVzNTZ0Q5RS9TU1dGOU10QWYwM2s1RkdYT0xIZFk0ZEwzdTVvV1dkegpVVUtCL05aUG5vaGY0L2VPc09LVThyU08waVkrNzk4Si9yNk5YMW9KNTBjQ0F3RUFBYU5RTUU0d0hRWURWUjBPCkJCWUVGSUREMDFZTXJML2VWMmZRZlF2aWQ5U2ZacncyTUI4R0ExVWRJd1FZTUJhQUZJREQwMVlNckwvZVYyZlEKZlF2aWQ5U2ZacncyTUF3R0ExVWRFd1FGTUFNQkFmOHdEUVlKS29aSWh2Y05BUUVGQlFBRGdnRUJBR0Y2M2loSAp2MXQyLzBSanlWbUJlbEdJaWZXbTlObGdjVi9XS1QvWkF1ejMzK090cjRIMkt6Y0FIYVNadWFOYVFxL0RLUTkyCm9HeEE5WDl4cG5DYzlhTWZiZ2dDc21DdnpESWtiRUovVTJTeUdiWXU0Vm96Z3d2WGd3SCtKU2hGQmZEeWVwT3EKSUh3d0habVNSVXFDRlRZeENVU1dKcko0QUsrOGJJNDdSUmNxSGE0UDBBN2grUDYzc1M1SXl5SzM3MVEyQU5nYQpnbW5VSytIcHpEZkhuVnV2NUZWcjNmbDd2czRnUDRLeVE3NCtXRzNVWDd0OUdvcWoxRFJmUlJjY1J6TmgvY0M4CllqeHVUdFg1VzdGaVVUWExkdmliMlJ2ZTQ2UE1scHJPS0FCcjBEMGFqbzA1U3ZrREJUWnBJbGUxQ1RjcDBmbWsKa25yakN1NFdYK2NKeEprPQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg=="
			base64Key  = "LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb2dJQkFBS0NBUUVBMUtRQ2o4dXk4RWRRT1lRVmNBQ1VDQVBVdjJzNS9pUlZrQ2VBY3Y5eXhGTWJOUk5jCnFUTEx4YmRRK0lRcGVzcmcrQWFra3FORjJONHFsRXU2bUdlcnhJaWVLT1JCMVlYaFE3NlZGUnB2NVBpWVVkZDgKdXNCMjcwUlpyeFduSytCZVpFYkY1bS8xNHlqYmF1S2pHS0NRWDFWOCsyNDRGUlFoSkkxSUhPVU05ZmxXM0FHOQprOU95dFVCcnB2dkEwK0FiazU3VEZNcGp0Ny9OK2dZbC9yNkZ0SExXczk2YmdnMHdiM1E3M3M5bmlzalFhR1dkCmY0MzNNR2ZMTjdpVVRyR0hUN05KSEZxNlhjNVFLdHNmaUNFV0pzM2Rmc2JlK1dUeWMwcExXenpobFVLNGpyZHEKZFZIcVFoL0x1MU5NWkNuRjc1T2hFaEw5STRyeGdWUzJORTlUNVFJREFRQUJBb0lCQUdjckVBY2VhTG9GajVubwpTTkpucFdyaUdQV3FtNTkvbDNmeWduTEpGN0RETlo4aHZzSmszOU1VNXhjOFlEZXdlVWc5U05uUWw5SHBzTFQwCnJSckdxZU1YK2N5VE9wSFRmQUQzVmJQQWVPdVo1YVZIckwrYkk5bGd5emFVaGVCVzR0VTZOVWhocCtaSDYzVkgKY3FRL091eldPR1p4Q29ySGtuRCtqeTlkdmxVVWVucUxXUzh1VjB4M3MyR0hsMzZiUCt4VUhIQWhleFJuRHgzdQo3QjA5R2FPYjF3aUVzVXFZdmtBdm5MbXJ5RDB0bmwvYndmSGZuZ0dLeXo2WDQvcmdSd1lrcGxJVkVhbXhMbjZVCkE0djBKWUx0b1NiblIxTHF5R2pLL2dSRzdFeFJPb0RJKy85SXJmeHV1aStHd1VBalJKdHlWRXFLcS9RUVB5OUcKc01GL3RtRUNnWUVBMTBiaHJYOHRZUzI1MEwvT0xDQThybEJiUjR2c0xqUTRDVm5Pbytsc21aUXh3OStlNFBWNwpVN2RndmpWTWlITGlOWnZFaEU0NGw2dHhhbHBWcHFzSHRFcDMwUW5zUkFsL1djSElibWJVU0xtWkk2a3Bmd2tmCldyd29pVUd3U21hcVlKenZ2YVNCSy9MblIwOVd1Ny9YK1J0eUdmVWtXTVdoaE1sSG0xaElYYjBDZ1lFQS9OMTMKTFhpTERiMnJWZGJOYVJ5UjhaUFd0bWVYOUNQWG16dy9EMFlQT203ZWU2U2xLRjBKTkZqMk8rWVJPelM5cXlmQQpFbzIwL1JmK0pDNUVOWFR1MEpQY0ZZK3J2YklyUzNiRGlHSkVQNXMzZW82SFFiSjRqN1U4ays3S1dJQ2huSnlvCi9ISmEwVEExMk82UlF4aEVWUzBGL0xkUjVFZEZSUE5TL3Z3eURVa0NnWUJ6WjhkQjJDUytyT0dwRzdudUE5WWoKNkdZV285Y1lLZHhFZndWODczek5sQmxkbFBxNlJEODU4TnVHL0ZHcjhGSitSS1FEL1Y3dlIvUkQvR3RnTHQyeApkQjVwVExXQS84cHFscXpaS256eEE3WXAzTnluQW4veGgxNy92ZHhBOW1xdDRsUFBTV29KNG16RDJLOTVkTzNWCjJEWEIzcDMrak92Nm9HQ24wWnJ4elFLQmdFMmtSc0s4ZjUzaGZpbG1RajRqR3FEZHJ4RGs4Q0J6blBFNlozWnUKSWFEa2lBWFpBU2xLbjlmbDlQMWhZQ3NZdjZBOUhWblZEeHlqY0ZKMXJsWG5xS2g4cmhna3ZDd0wrQVU4Mno4VwpSVFJ1bVhOVkxpeTYrdy9OSzJPVTc2YUxJSlJ3K2VaQnlxYnVzYW9CWHJNR1VYMEJ6UlBTeWg5WXp1a2orWGozCndQcVpBb0dBSnBKS05Qb0hhaytiQ1ovcVQ0V3NiS1I4TUVsNTMxYmpyaVdXWjhHMVN1VFZmenVmaWNibThMYTEKd3VSK0p6NWp4QnkyS21FM2owTFh6RUthOHpJOFJqRHN6bkowekgvc091bEh6b3pENDYzc1lsOEdHcnpFU3M5NAo1RTdPMHZ3TUFUL1lWck5aNGJMdFdIS2tCNzJ4K3Y4aEF3RDcrWGJhaXk0cHJiRHJjclk9Ci0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg=="

			privateKey []byte
			cert       []byte
		)

		BeforeEach(func() {
			certificateConfig = &CertificateSecretConfig{
				Name:       "ca",
				CommonName: "metrics-server",
				CertType:   CACert,
			}

			var err error
			cert, err = utils.DecodeBase64(base64Cert)
			Expect(err).NotTo(HaveOccurred())
			privateKey, err = utils.DecodeBase64(base64Key)
			Expect(err).NotTo(HaveOccurred())

			certificateInfoData = &CertificateInfoData{
				PrivateKey:  privateKey,
				Certificate: cert,
			}
		})

		Describe("#Generate", func() {
			It("should properly generate Certificate Object", func() {
				obj, err := certificateConfig.Generate()
				Expect(err).NotTo(HaveOccurred())

				certificate, ok := obj.(*Certificate)
				Expect(ok).To(BeTrue())

				Expect(certificate.PrivateKeyPEM).NotTo(BeNil())
				Expect(certificate.CertificatePEM).NotTo(BeNil())
				Expect(certificate.PrivateKey).NotTo(BeNil())
				Expect(certificate.Certificate).NotTo(BeNil())
			})
		})

		Describe("#GenerateInfoData", func() {
			It("should generate correct Certificate InfoData", func() {
				obj, err := certificateConfig.GenerateInfoData()
				Expect(err).NotTo(HaveOccurred())

				Expect(obj.TypeVersion()).To(Equal(CertificateDataType))

				certificateInfoData, ok := obj.(*CertificateInfoData)
				Expect(ok).To(BeTrue())

				Expect(certificateInfoData.PrivateKey).NotTo(BeNil())
				Expect(certificateInfoData.Certificate).NotTo(BeNil())
			})
		})

		Describe("#GenerateFromInfoData", func() {
			It("should properly load Certificate object from CertificateInfoData", func() {
				obj, err := certificateConfig.GenerateFromInfoData(certificateInfoData)
				Expect(err).NotTo(HaveOccurred())

				certificate, ok := obj.(*Certificate)
				Expect(ok).To(BeTrue())

				Expect(certificate.PrivateKeyPEM).NotTo(BeNil())
				Expect(certificate.CertificatePEM).NotTo(BeNil())
				Expect(certificate.PrivateKey).NotTo(BeNil())
				Expect(certificate.Certificate).NotTo(BeNil())
			})
		})

		Describe("#LoadFromSecretData", func() {
			It("should properly load CertificateInfoData from secret data with CA keys", func() {
				certificateConfig.CertType = CACert
				secretData := map[string][]byte{
					DataKeyPrivateKeyCA:  privateKey,
					DataKeyCertificateCA: cert,
				}
				obj, err := certificateConfig.LoadFromSecretData(secretData)
				Expect(err).NotTo(HaveOccurred())

				certInfoData, ok := obj.(*CertificateInfoData)
				Expect(ok).To(BeTrue())
				Expect(certInfoData.Certificate).To(Equal(certificateInfoData.Certificate))
				Expect(certInfoData.PrivateKey).To(Equal(certificateInfoData.PrivateKey))
			})
			It("should properly load CertificateInfoData from secret data with TLS cert keys", func() {
				certificateConfig.CertType = ServerCert
				secretData := map[string][]byte{
					DataKeyPrivateKey:  privateKey,
					DataKeyCertificate: cert,
				}
				obj, err := certificateConfig.LoadFromSecretData(secretData)
				Expect(err).NotTo(HaveOccurred())

				certInfoData, ok := obj.(*CertificateInfoData)
				Expect(ok).To(BeTrue())
				Expect(certInfoData.Certificate).To(Equal(certificateInfoData.Certificate))
				Expect(certInfoData.PrivateKey).To(Equal(certificateInfoData.PrivateKey))
			})
		})
	})

	Describe("Certificate Object", func() {
		var (
			certificate *Certificate
		)
		BeforeEach(func() {
			certificate = &Certificate{
				PrivateKeyPEM:  []byte("foo"),
				CertificatePEM: []byte("bar"),
			}
		})

		Describe("#SecretData", func() {
			It("should properly return secret data if certificate type is CA", func() {
				secretData := map[string][]byte{
					DataKeyPrivateKeyCA:  []byte("foo"),
					DataKeyCertificateCA: []byte("bar"),
				}
				Expect(certificate.SecretData()).To(Equal(secretData))
			})
			It("should properly return secret data if certificate type is server, client or both", func() {
				certificate.CA = &Certificate{
					CertificatePEM: []byte("ca"),
				}
				secretData := map[string][]byte{
					DataKeyPrivateKey:    []byte("foo"),
					DataKeyCertificate:   []byte("bar"),
					DataKeyCertificateCA: []byte("ca"),
				}
				Expect(certificate.SecretData()).To(Equal(secretData))
			})
		})
	})
})
