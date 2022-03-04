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

package shootsecrets_test

import (
	"context"
	"fmt"

	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/operation/shootsecrets"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/infodata"
	"github.com/gardener/gardener/pkg/utils/secrets"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("SecretsManager", func() {
	type ExpectedNameType struct {
		Name string
		Type infodata.TypeVersion
	}

	var (
		cas                    map[string]*secrets.Certificate
		secretsConfigGenerator func(certificateAuthorities map[string]*secrets.Certificate) ([]secrets.ConfigInterface, error)

		caName   = "ca1"
		certName = "cert1"
		cacert   = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURCekNDQWUrZ0F3SUJBZ0lKQUpMN2JKT01pajd2TUEwR0NTcUdTSWIzRFFFQkJRVUFNQm94R0RBV0JnTlYKQkFNTUQzZDNkeTVsZUdGdGNHeGxMbU52YlRBZUZ3MHhOekE1TVRnd05qRTFOVEZhRncweU56QTVNVFl3TmpFMQpOVEZhTUJveEdEQVdCZ05WQkFNTUQzZDNkeTVsZUdGdGNHeGxMbU52YlRDQ0FTSXdEUVlKS29aSWh2Y05BUUVCCkJRQURnZ0VQQURDQ0FRb0NnZ0VCQUs0OHZGVW9SMytJS2lUYTYzdEUrcE95WW9iNHdjeklDNWNvMlBXUlZoUHUKMkZLTmhRdUQ3Nk1ETmY4eVhJUTh4TzZRTlQxQlBKQ2RnM3FqQWpkU0QwcUlkeUc2L3ZoMVZaeWVCWHJYdFR6bQpKR21LSVg4K1IzVzVVS3RXSUtXclJjMzFERVVGb1Urakp5U2QyakllQWNOdWM0ZEZnZGhhblYvRkxDaHJJbTNRClBXeHRlS1QwZU52bkJFZEg2a3dqNU9uWE9XUlgraGpMNEdIcTM3M3k0S2RXclNGNGxaa2RGQVdFZFd3cFFDNXEKOFByVTdPUHcwMW1WZUN5dm1nZGF4THhsVzNTZ0Q5RS9TU1dGOU10QWYwM2s1RkdYT0xIZFk0ZEwzdTVvV1dkegpVVUtCL05aUG5vaGY0L2VPc09LVThyU08waVkrNzk4Si9yNk5YMW9KNTBjQ0F3RUFBYU5RTUU0d0hRWURWUjBPCkJCWUVGSUREMDFZTXJML2VWMmZRZlF2aWQ5U2ZacncyTUI4R0ExVWRJd1FZTUJhQUZJREQwMVlNckwvZVYyZlEKZlF2aWQ5U2ZacncyTUF3R0ExVWRFd1FGTUFNQkFmOHdEUVlKS29aSWh2Y05BUUVGQlFBRGdnRUJBR0Y2M2loSAp2MXQyLzBSanlWbUJlbEdJaWZXbTlObGdjVi9XS1QvWkF1ejMzK090cjRIMkt6Y0FIYVNadWFOYVFxL0RLUTkyCm9HeEE5WDl4cG5DYzlhTWZiZ2dDc21DdnpESWtiRUovVTJTeUdiWXU0Vm96Z3d2WGd3SCtKU2hGQmZEeWVwT3EKSUh3d0habVNSVXFDRlRZeENVU1dKcko0QUsrOGJJNDdSUmNxSGE0UDBBN2grUDYzc1M1SXl5SzM3MVEyQU5nYQpnbW5VSytIcHpEZkhuVnV2NUZWcjNmbDd2czRnUDRLeVE3NCtXRzNVWDd0OUdvcWoxRFJmUlJjY1J6TmgvY0M4CllqeHVUdFg1VzdGaVVUWExkdmliMlJ2ZTQ2UE1scHJPS0FCcjBEMGFqbzA1U3ZrREJUWnBJbGUxQ1RjcDBmbWsKa25yakN1NFdYK2NKeEprPQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg=="
		cakey    = "LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBcmp5OFZTaEhmNGdxSk5ycmUwVDZrN0ppaHZqQnpNZ0xseWpZOVpGV0UrN1lVbzJGCkM0UHZvd00xL3pKY2hEekU3cEExUFVFOGtKMkRlcU1DTjFJUFNvaDNJYnIrK0hWVm5KNEZldGUxUE9Za2FZb2gKZno1SGRibFFxMVlncGF0RnpmVU1SUVdoVDZNbkpKM2FNaDRCdzI1emgwV0IyRnFkWDhVc0tHc2liZEE5YkcxNApwUFI0MitjRVIwZnFUQ1BrNmRjNVpGZjZHTXZnWWVyZnZmTGdwMWF0SVhpVm1SMFVCWVIxYkNsQUxtcncrdFRzCjQvRFRXWlY0TEsrYUIxckV2R1ZiZEtBUDBUOUpKWVgweTBCL1RlVGtVWmM0c2QxamgwdmU3bWhaWjNOUlFvSDgKMWsrZWlGL2o5NDZ3NHBUeXRJN1NKajd2M3duK3ZvMWZXZ25uUndJREFRQUJBb0lCQUNWM3dCUWI1a3doYnRhZwpFUlZmL3ZaMTNNUWppQ0ZPUDFmUkc5NnEwelRVSHNhWjBpdG93c1p1TVZWZ2NnNnB0cnVLWFRoLzU5TTlYQUdxClBoYkJXYkp3YUJYanpXS0djTk9PRTZXWDcweGFQU2hJNE8wbzZsc0JiN3g0ZCtySVN1bUFNWlJDSXE4cWFZZHgKUG5PYWlreUpXdVNTNk5vcW1qNzIrb2p5aU9QT0VnOG9OeW1Ya3hIZVlRVCszU2hkYUxpRjJXTUF3dG9oS1YyRAo4VnozRG8yR0NzczN0aTZscVUvS1Q1a2VCUm8wVUZtY00vWnBGY3JrSWNDVGwxRWJockZtMUxWL1prRjZ4dnRkCmM0OWY1c2tFK20wMklVbUJoU0xUc3k3RzRkMUZDUGczRmlXWUd2dElLNk9QTjRORHU1eE5hejJIZnZnRmNUbVAKZDJsODYvRUNnWUVBMitSQnlTWnlhSUM0bXhVMVNiV0JLWWNaVTBjL0VPZG9CUTZGaFpTN0R6SlhJRkRJVkJxQwpDTVMvcERXdVFKV21LYWRsZ1NaQXNyVTE0NW5uUWcwTHhNM254TDZEUU1yNlJsU0RIbXVtQTlPSEc5ZGdIWTI1CkNEanluTEpRdWp1L1hQeHlBa2lMblI5aE96K0h2VWVqdWcrMEdLdVA0WmxrbjRHNDVTOWQ0K2tDZ1lFQXl0bEYKeUd1TWJKSnpDbDRFSVBtTlhrcUVkbkFKUmdBSkR4RFF2d1BZVEFZeTVabnBaVlBpWVB2YmhEaTR3VzZRVHRJRQpkR1VCWUVaUTdYcDR0Y1QxOEt5ZFZ0Wk1qUWgzaWF2UndiMjU4UGlRTHRvbkplYklzOHpESDl1WkxtK0lkblZCCkVvQklXSGZaMjYwOVgyeVpTVnJzRC9JZnNDcnZsd1c5c3VENFk2OENnWUJNTmU0L0F4NC83ZTBOZ3VvM0k3c2kKWTNwNWpJWGxHKzdIWWVNUkN4MVNCUWFCWXI0cnVBdzljY05oN0dENmJXTnJxR0xid2lCR1Q5dmZpR1hJVkxFeApncFBEY3F3VzlzS0xRWnM0SGVNcURGUVZhQzRkMEJMRE1NbVZXWS8xRytRVkhFRi9YUmxXV1p2Zlp3TnFyTHVvCkx1MGlaOE8wVXUrM0FNVE9XZjVXa1FLQmdRQ0NpVHRzOURqVGpaTFdjeFg1R2w2czlRczFKSGZ6UWdhU1dXSGIKNmwrQTNPUlgrS25IZVNuTys0U1NHK1paSkF0ZGphMHNNZXVteHRsQldYVGdsRFVvZ2d4bVcxVzcxRjBJalRkWQprLzFhWXJwMlRCQ3hSVWlXM0FnZE1qWHJPZjc1TEErS0ZsOTMvdmlGYzRCeExmT2V6eEhtV1F1blZKb0Y5NzNSCnBSQnpKUUtCZ0V3WHM2dVVVRTdBRVN1dTh2aS8zeG1heWVDL3pjOXZod1dpTkNoOTNtZVNlL1YvYmFWYVNCSjgKRG9aVVBVTnc3MzNLUlU5TWpUNzM4L0hKczZsN2Z5U1FXMHozSkRLTDduUTVjb1RDR09zWlNHalNIVEdzUU01bgpLSWREdGEyYm5Vb1hUTU01S0h4OW9KQ0tYYy9mZTdGY3ZsanVRd3hESzk1RkNRVFYwclFoCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg=="

		checkIfInfoDataUpsertedSuccessfully = func(resourceDataList gardencorev1alpha1helper.GardenerResourceDataList, expectedResources ...ExpectedNameType) {
			Expect(len(resourceDataList)).To(Equal(len(expectedResources)))
			for _, expectedResource := range expectedResources {
				resourceData := resourceDataList.Get(expectedResource.Name)
				Expect(resourceData).NotTo(BeNil())
				Expect(resourceData.Type).To(BeEquivalentTo(expectedResource.Type))
				Expect(resourceData.Data.Raw).ToNot(BeNil())
			}
		}
	)

	BeforeEach(func() {
		caKey, err := utils.DecodeBase64(cakey)
		Expect(err).NotTo(HaveOccurred())
		caCert, err := utils.DecodeBase64(cacert)
		Expect(err).NotTo(HaveOccurred())
		ca, err := secrets.LoadCertificate(caName, caKey, caCert)
		Expect(err).NotTo(HaveOccurred())

		cas = map[string]*secrets.Certificate{caName: ca}

		secretsConfigGenerator = func(certificateAuthorities map[string]*secrets.Certificate) ([]secrets.ConfigInterface, error) {
			return []secrets.ConfigInterface{
				&secrets.ControlPlaneSecretConfig{
					Name: certName,
					CertificateSecretConfig: &secrets.CertificateSecretConfig{
						CommonName: certName,
						CertType:   secrets.ServerCert,
						SigningCA:  certificateAuthorities[caName],
					},
				},
			}, nil
		}
	})

	Describe("#Generate", func() {
		It("should generate secrets infodata and save it into the gardener resource data list", func() {
			secretsManager := NewSecretsManager(gardencorev1alpha1helper.GardenerResourceDataList{}, secretsConfigGenerator).
				WithCertificateAuthorities(cas)

			err := secretsManager.Generate()
			Expect(err).NotTo(HaveOccurred())

			checkIfInfoDataUpsertedSuccessfully(secretsManager.GardenerResourceDataList,
				ExpectedNameType{certName, secrets.CertificateDataType},
			)
		})

		It("should not overwrite secrets if they already exist in the gardener resource data list", func() {
			resourceDataList := gardencorev1alpha1helper.GardenerResourceDataList{
				{
					Name: caName,
					Type: string(secrets.CertificateDataType),
					Data: runtime.RawExtension{Raw: []byte(fmt.Sprintf(`{"privateKey":"%s","certificate":"%s"}`, cakey, cacert))},
				},
			}

			secretsManager := NewSecretsManager(resourceDataList, secretsConfigGenerator).
				WithCertificateAuthorities(cas)

			err := secretsManager.Generate()
			Expect(err).NotTo(HaveOccurred())

			checkIfInfoDataUpsertedSuccessfully(secretsManager.GardenerResourceDataList,
				ExpectedNameType{caName, secrets.CertificateDataType},
				ExpectedNameType{certName, secrets.CertificateDataType},
			)

			Expect(*secretsManager.GardenerResourceDataList.Get(caName)).To(Equal(resourceDataList[0]))
		})
	})

	Describe("#Deploy", func() {
		var (
			ctrl                     *gomock.Controller
			k8sClient                *mockclient.MockClient
			decodedCert              []byte
			decodedKey               []byte
			gardenerResourceDataList gardencorev1alpha1helper.GardenerResourceDataList
			expectedSecrets          map[string]*corev1.Secret

			controlplaneNS = "controlplane"
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			k8sClient = mockclient.NewMockClient(ctrl)

			decodedCert, _ = utils.DecodeBase64(cacert)
			decodedKey, _ = utils.DecodeBase64(cakey)

			gardenerResourceDataList = gardencorev1alpha1helper.GardenerResourceDataList{
				{
					Name: caName,
					Type: string(secrets.CertificateDataType),
					Data: runtime.RawExtension{Raw: []byte(fmt.Sprintf(`{"privateKey":"%s","certificate":"%s"}`, []byte(cakey), []byte(cacert)))},
				},
				{
					Name: certName,
					Type: string(secrets.CertificateDataType),
					Data: runtime.RawExtension{Raw: []byte(fmt.Sprintf(`{"privateKey":"%s","certificate":"%s"}`, []byte(cakey), []byte(cacert)))},
				},
			}

			expectedSecrets = map[string]*corev1.Secret{
				certName: {
					ObjectMeta: metav1.ObjectMeta{
						Name:      certName,
						Namespace: controlplaneNS,
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{
						secrets.DataKeyCertificateCA:    decodedCert,
						fmt.Sprintf("%s.key", certName): decodedKey,
						fmt.Sprintf("%s.crt", certName): decodedCert,
					},
				},
			}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should deploy all required kubernetes secret objects", func() {
			calls := []*gomock.Call{}
			secretConfigs, err := secretsConfigGenerator(nil)
			Expect(err).NotTo(HaveOccurred())
			for _, secretConfig := range secretConfigs {
				calls = append(calls, k8sClient.EXPECT().Create(context.TODO(), expectedSecrets[secretConfig.GetName()]))
			}

			gomock.InOrder(
				calls...,
			)

			secretsManager := NewSecretsManager(gardenerResourceDataList, secretsConfigGenerator).
				WithExistingSecrets(map[string]*corev1.Secret{}).
				WithCertificateAuthorities(cas)

			err = secretsManager.Deploy(context.TODO(), k8sClient, controlplaneNS)
			Expect(err).NotTo(HaveOccurred())

			Expect(expectedSecrets).To(Equal(secretsManager.DeployedSecrets))
		})
	})
})
