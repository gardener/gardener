// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/yaml"
)

var _ = Describe("utils", func() {
	Describe("secrets", func() {
		Describe("#GenerateKubeconfig", func() {
			var (
				kubecfg clientcmdv1.Config

				clusterName   = "test-cluster"
				apiServerURL  = "kube-apiserver"
				caCert        = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURCekNDQWUrZ0F3SUJBZ0lKQUpMN2JKT01pajd2TUEwR0NTcUdTSWIzRFFFQkJRVUFNQm94R0RBV0JnTlYKQkFNTUQzZDNkeTVsZUdGdGNHeGxMbU52YlRBZUZ3MHhOekE1TVRnd05qRTFOVEZhRncweU56QTVNVFl3TmpFMQpOVEZhTUJveEdEQVdCZ05WQkFNTUQzZDNkeTVsZUdGdGNHeGxMbU52YlRDQ0FTSXdEUVlKS29aSWh2Y05BUUVCCkJRQURnZ0VQQURDQ0FRb0NnZ0VCQUs0OHZGVW9SMytJS2lUYTYzdEUrcE95WW9iNHdjeklDNWNvMlBXUlZoUHUKMkZLTmhRdUQ3Nk1ETmY4eVhJUTh4TzZRTlQxQlBKQ2RnM3FqQWpkU0QwcUlkeUc2L3ZoMVZaeWVCWHJYdFR6bQpKR21LSVg4K1IzVzVVS3RXSUtXclJjMzFERVVGb1Urakp5U2QyakllQWNOdWM0ZEZnZGhhblYvRkxDaHJJbTNRClBXeHRlS1QwZU52bkJFZEg2a3dqNU9uWE9XUlgraGpMNEdIcTM3M3k0S2RXclNGNGxaa2RGQVdFZFd3cFFDNXEKOFByVTdPUHcwMW1WZUN5dm1nZGF4THhsVzNTZ0Q5RS9TU1dGOU10QWYwM2s1RkdYT0xIZFk0ZEwzdTVvV1dkegpVVUtCL05aUG5vaGY0L2VPc09LVThyU08waVkrNzk4Si9yNk5YMW9KNTBjQ0F3RUFBYU5RTUU0d0hRWURWUjBPCkJCWUVGSUREMDFZTXJML2VWMmZRZlF2aWQ5U2ZacncyTUI4R0ExVWRJd1FZTUJhQUZJREQwMVlNckwvZVYyZlEKZlF2aWQ5U2ZacncyTUF3R0ExVWRFd1FGTUFNQkFmOHdEUVlKS29aSWh2Y05BUUVGQlFBRGdnRUJBR0Y2M2loSAp2MXQyLzBSanlWbUJlbEdJaWZXbTlObGdjVi9XS1QvWkF1ejMzK090cjRIMkt6Y0FIYVNadWFOYVFxL0RLUTkyCm9HeEE5WDl4cG5DYzlhTWZiZ2dDc21DdnpESWtiRUovVTJTeUdiWXU0Vm96Z3d2WGd3SCtKU2hGQmZEeWVwT3EKSUh3d0habVNSVXFDRlRZeENVU1dKcko0QUsrOGJJNDdSUmNxSGE0UDBBN2grUDYzc1M1SXl5SzM3MVEyQU5nYQpnbW5VSytIcHpEZkhuVnV2NUZWcjNmbDd2czRnUDRLeVE3NCtXRzNVWDd0OUdvcWoxRFJmUlJjY1J6TmgvY0M4CllqeHVUdFg1VzdGaVVUWExkdmliMlJ2ZTQ2UE1scHJPS0FCcjBEMGFqbzA1U3ZrREJUWnBJbGUxQ1RjcDBmbWsKa25yakN1NFdYK2NKeEprPQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg=="
				clientCert    = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURCekNDQWUrZ0F3SUJBZ0lKQUpMN2JKT01pajd2TUEwR0NTcUdTSWIzRFFFQkJRVUFNQm94R0RBV0JnTlYKQkFNTUQzZDNkeTVsZUdGdGNHeGxMbU52YlRBZUZ3MHhOekE1TVRnd05qRTFOVEZhRncweU56QTVNVFl3TmpFMQpOVEZhTUJveEdEQVdCZ05WQkFNTUQzZDNkeTVsZUdGdGNHeGxMbU52YlRDQ0FTSXdEUVlKS29aSWh2Y05BUUVCCkJRQURnZ0VQQURDQ0FRb0NnZ0VCQUs0OHZGVW9SMytJS2lUYTYzdEUrcE95WW9iNHdjeklDNWNvMlBXUlZoUHUKMkZLTmhRdUQ3Nk1ETmY4eVhJUTh4TzZRTlQxQlBKQ2RnM3FqQWpkU0QwcUlkeUc2L3ZoMVZaeWVCWHJYdFR6bQpKR21LSVg4K1IzVzVVS3RXSUtXclJjMzFERVVGb1Urakp5U2QyakllQWNOdWM0ZEZnZGhhblYvRkxDaHJJbTNRClBXeHRlS1QwZU52bkJFZEg2a3dqNU9uWE9XUlgraGpMNEdIcTM3M3k0S2RXclNGNGxaa2RGQVdFZFd3cFFDNXEKOFByVTdPUHcwMW1WZUN5dm1nZGF4THhsVzNTZ0Q5RS9TU1dGOU10QWYwM2s1RkdYT0xIZFk0ZEwzdTVvV1dkegpVVUtCL05aUG5vaGY0L2VPc09LVThyU08waVkrNzk4Si9yNk5YMW9KNTBjQ0F3RUFBYU5RTUU0d0hRWURWUjBPCkJCWUVGSUREMDFZTXJML2VWMmZRZlF2aWQ5U2ZacncyTUI4R0ExVWRJd1FZTUJhQUZJREQwMVlNckwvZVYyZlEKZlF2aWQ5U2ZacncyTUF3R0ExVWRFd1FGTUFNQkFmOHdEUVlKS29aSWh2Y05BUUVGQlFBRGdnRUJBR0Y2M2loSAp2MXQyLzBSanlWbUJlbEdJaWZXbTlObGdjVi9XS1QvWkF1ejMzK090cjRIMkt6Y0FIYVNadWFOYVFxL0RLUTkyCm9HeEE5WDl4cG5DYzlhTWZiZ2dDc21DdnpESWtiRUovVTJTeUdiWXU0Vm96Z3d2WGd3SCtKU2hGQmZEeWVwT3EKSUh3d0habVNSVXFDRlRZeENVU1dKcko0QUsrOGJJNDdSUmNxSGE0UDBBN2grUDYzc1M1SXl5SzM3MVEyQU5nYQpnbW5VSytIcHpEZkhuVnV2NUZWcjNmbDd2czRnUDRLeVE3NCtXRzNVWDd0OUdvcWoxRFJmUlJjY1J6TmgvY0M4CllqeHVUdFg1VzdGaVVUWExkdmliMlJ2ZTQ2UE1scHJPS0FCcjBEMGFqbzA1U3ZrREJUWnBJbGUxQ1RjcDBmbWsKa25yakN1NFdYK2NKeEprPQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg=="
				clientKey     = "LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBcmp5OFZTaEhmNGdxSk5ycmUwVDZrN0ppaHZqQnpNZ0xseWpZOVpGV0UrN1lVbzJGCkM0UHZvd00xL3pKY2hEekU3cEExUFVFOGtKMkRlcU1DTjFJUFNvaDNJYnIrK0hWVm5KNEZldGUxUE9Za2FZb2gKZno1SGRibFFxMVlncGF0RnpmVU1SUVdoVDZNbkpKM2FNaDRCdzI1emgwV0IyRnFkWDhVc0tHc2liZEE5YkcxNApwUFI0MitjRVIwZnFUQ1BrNmRjNVpGZjZHTXZnWWVyZnZmTGdwMWF0SVhpVm1SMFVCWVIxYkNsQUxtcncrdFRzCjQvRFRXWlY0TEsrYUIxckV2R1ZiZEtBUDBUOUpKWVgweTBCL1RlVGtVWmM0c2QxamgwdmU3bWhaWjNOUlFvSDgKMWsrZWlGL2o5NDZ3NHBUeXRJN1NKajd2M3duK3ZvMWZXZ25uUndJREFRQUJBb0lCQUNWM3dCUWI1a3doYnRhZwpFUlZmL3ZaMTNNUWppQ0ZPUDFmUkc5NnEwelRVSHNhWjBpdG93c1p1TVZWZ2NnNnB0cnVLWFRoLzU5TTlYQUdxClBoYkJXYkp3YUJYanpXS0djTk9PRTZXWDcweGFQU2hJNE8wbzZsc0JiN3g0ZCtySVN1bUFNWlJDSXE4cWFZZHgKUG5PYWlreUpXdVNTNk5vcW1qNzIrb2p5aU9QT0VnOG9OeW1Ya3hIZVlRVCszU2hkYUxpRjJXTUF3dG9oS1YyRAo4VnozRG8yR0NzczN0aTZscVUvS1Q1a2VCUm8wVUZtY00vWnBGY3JrSWNDVGwxRWJockZtMUxWL1prRjZ4dnRkCmM0OWY1c2tFK20wMklVbUJoU0xUc3k3RzRkMUZDUGczRmlXWUd2dElLNk9QTjRORHU1eE5hejJIZnZnRmNUbVAKZDJsODYvRUNnWUVBMitSQnlTWnlhSUM0bXhVMVNiV0JLWWNaVTBjL0VPZG9CUTZGaFpTN0R6SlhJRkRJVkJxQwpDTVMvcERXdVFKV21LYWRsZ1NaQXNyVTE0NW5uUWcwTHhNM254TDZEUU1yNlJsU0RIbXVtQTlPSEc5ZGdIWTI1CkNEanluTEpRdWp1L1hQeHlBa2lMblI5aE96K0h2VWVqdWcrMEdLdVA0WmxrbjRHNDVTOWQ0K2tDZ1lFQXl0bEYKeUd1TWJKSnpDbDRFSVBtTlhrcUVkbkFKUmdBSkR4RFF2d1BZVEFZeTVabnBaVlBpWVB2YmhEaTR3VzZRVHRJRQpkR1VCWUVaUTdYcDR0Y1QxOEt5ZFZ0Wk1qUWgzaWF2UndiMjU4UGlRTHRvbkplYklzOHpESDl1WkxtK0lkblZCCkVvQklXSGZaMjYwOVgyeVpTVnJzRC9JZnNDcnZsd1c5c3VENFk2OENnWUJNTmU0L0F4NC83ZTBOZ3VvM0k3c2kKWTNwNWpJWGxHKzdIWWVNUkN4MVNCUWFCWXI0cnVBdzljY05oN0dENmJXTnJxR0xid2lCR1Q5dmZpR1hJVkxFeApncFBEY3F3VzlzS0xRWnM0SGVNcURGUVZhQzRkMEJMRE1NbVZXWS8xRytRVkhFRi9YUmxXV1p2Zlp3TnFyTHVvCkx1MGlaOE8wVXUrM0FNVE9XZjVXa1FLQmdRQ0NpVHRzOURqVGpaTFdjeFg1R2w2czlRczFKSGZ6UWdhU1dXSGIKNmwrQTNPUlgrS25IZVNuTys0U1NHK1paSkF0ZGphMHNNZXVteHRsQldYVGdsRFVvZ2d4bVcxVzcxRjBJalRkWQprLzFhWXJwMlRCQ3hSVWlXM0FnZE1qWHJPZjc1TEErS0ZsOTMvdmlGYzRCeExmT2V6eEhtV1F1blZKb0Y5NzNSCnBSQnpKUUtCZ0V3WHM2dVVVRTdBRVN1dTh2aS8zeG1heWVDL3pjOXZod1dpTkNoOTNtZVNlL1YvYmFWYVNCSjgKRG9aVVBVTnc3MzNLUlU5TWpUNzM4L0hKczZsN2Z5U1FXMHozSkRLTDduUTVjb1RDR09zWlNHalNIVEdzUU01bgpLSWREdGEyYm5Vb1hUTU01S0h4OW9KQ0tYYy9mZTdGY3ZsanVRd3hESzk1RkNRVFYwclFoCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg=="
				basicAuthUser = "user"
				basicAuthPass = "pass"

				secret      *ControlPlaneSecretConfig
				certificate *Certificate
			)

			BeforeEach(func() {
				secret = &ControlPlaneSecretConfig{
					BasicAuth: &BasicAuth{
						Username: basicAuthUser,
						Password: basicAuthPass,
					},

					KubeConfigRequests: []KubeConfigRequest{{
						ClusterName:   clusterName,
						APIServerHost: apiServerURL,
						CAData:        []byte(caCert),
					}},
				}

				certificate = &Certificate{
					CA: &Certificate{
						CertificatePEM: []byte(caCert),
					},
					CertificatePEM: []byte(clientCert),
					PrivateKeyPEM:  []byte(clientKey),
				}
			})

			AfterEach(func() {
				kubecfg = clientcmdv1.Config{}
			})

			Context("without Basic Authentication credentials", func() {
				It("should return a kubeconfig with one context and one user", func() {
					secret.BasicAuth = nil

					kubeconfig, err := generateKubeconfig(secret, certificate)
					Expect(err).NotTo(HaveOccurred())

					Expect(yaml.Unmarshal(kubeconfig, &kubecfg)).To(Succeed())

					Expect(kubecfg.CurrentContext).To(Equal(clusterName))
					Expect(kubecfg.Clusters).To(HaveLen(1))
					Expect(kubecfg.Contexts).To(HaveLen(1))
					Expect(kubecfg.AuthInfos).To(HaveLen(1))
					Expect(kubecfg.Clusters[0].Cluster.Server).To(Equal("https://" + apiServerURL))
					Expect(kubecfg.Clusters[0].Cluster.CertificateAuthorityData).To(Equal([]byte(caCert)))
					Expect(kubecfg.AuthInfos[0].AuthInfo.ClientCertificateData).ToNot(BeEmpty())
					Expect(kubecfg.AuthInfos[0].AuthInfo.ClientKeyData).ToNot(BeEmpty())
				})

				It("should return a kubeconfig with two contexts and one user when two requests are passed", func() {
					secret.BasicAuth = nil
					secret.KubeConfigRequests = append(secret.KubeConfigRequests, KubeConfigRequest{
						ClusterName:   "foo",
						APIServerHost: "foo.bar",
					})

					kubeconfig, err := generateKubeconfig(secret, certificate)
					Expect(err).NotTo(HaveOccurred())

					err = yaml.Unmarshal(kubeconfig, &kubecfg)
					Expect(err).NotTo(HaveOccurred())

					Expect(kubecfg).To(Equal(clientcmdv1.Config{
						Kind:       "Config",
						APIVersion: "v1",
						Clusters: []clientcmdv1.NamedCluster{
							{
								Name: clusterName,
								Cluster: clientcmdv1.Cluster{
									Server:                   "https://" + apiServerURL,
									CertificateAuthorityData: []byte(caCert),
								},
							},
							{
								Name: "foo",
								Cluster: clientcmdv1.Cluster{
									Server: "https://foo.bar",
								},
							},
						},
						CurrentContext: clusterName,
						Contexts: []clientcmdv1.NamedContext{
							{
								Name: clusterName,
								Context: clientcmdv1.Context{
									Cluster:  clusterName,
									AuthInfo: clusterName,
								},
							},
							{
								Name: "foo",
								Context: clientcmdv1.Context{
									Cluster:  "foo",
									AuthInfo: clusterName,
								},
							},
						},
						AuthInfos: []clientcmdv1.NamedAuthInfo{
							{
								Name: clusterName,
								AuthInfo: clientcmdv1.AuthInfo{
									ClientCertificateData: []byte(clientCert),
									ClientKeyData:         []byte(clientKey),
								},
							},
						},
					}))
				})
			})

			Context("with Basic Authentication credentials", func() {
				It("should return a kubeconfig with one context and two users", func() {
					kubeconfig, err := generateKubeconfig(secret, certificate)
					Expect(err).NotTo(HaveOccurred())

					Expect(yaml.Unmarshal(kubeconfig, &kubecfg)).To(Succeed())

					Expect(kubecfg.CurrentContext).To(Equal(clusterName))
					Expect(kubecfg.Clusters).To(HaveLen(1))
					Expect(kubecfg.Contexts).To(HaveLen(1))
					Expect(kubecfg.AuthInfos).To(HaveLen(2))
					Expect(kubecfg.Clusters[0].Cluster.Server).To(Equal("https://" + apiServerURL))
					Expect(kubecfg.Clusters[0].Cluster.CertificateAuthorityData).To(Equal([]byte(caCert)))
					Expect(kubecfg.AuthInfos[0].AuthInfo.ClientCertificateData).ToNot(BeEmpty())
					Expect(kubecfg.AuthInfos[0].AuthInfo.ClientKeyData).ToNot(BeEmpty())
					Expect(kubecfg.AuthInfos[1].AuthInfo.Username).To(Equal(basicAuthUser))
					Expect(kubecfg.AuthInfos[1].AuthInfo.Password).To(Equal(basicAuthPass))
				})
			})

			It("should return a kubeconfig with two context and two users when two requests are passed", func() {
				secret.KubeConfigRequests = append(secret.KubeConfigRequests, KubeConfigRequest{
					ClusterName:   "foo",
					APIServerHost: "foo.bar",
				})

				kubeconfig, err := generateKubeconfig(secret, certificate)
				Expect(err).NotTo(HaveOccurred())

				Expect(yaml.Unmarshal(kubeconfig, &kubecfg)).To(Succeed())

				Expect(kubecfg).To(Equal(clientcmdv1.Config{
					Kind:       "Config",
					APIVersion: "v1",
					Clusters: []clientcmdv1.NamedCluster{
						{
							Name: clusterName,
							Cluster: clientcmdv1.Cluster{
								Server:                   "https://" + apiServerURL,
								CertificateAuthorityData: []byte(caCert),
							},
						},
						{
							Name: "foo",
							Cluster: clientcmdv1.Cluster{
								Server: "https://foo.bar",
							},
						},
					},
					CurrentContext: clusterName,
					Contexts: []clientcmdv1.NamedContext{
						{
							Name: clusterName,
							Context: clientcmdv1.Context{
								Cluster:  clusterName,
								AuthInfo: clusterName,
							},
						},
						{
							Name: "foo",
							Context: clientcmdv1.Context{
								Cluster:  "foo",
								AuthInfo: clusterName,
							},
						},
					},
					AuthInfos: []clientcmdv1.NamedAuthInfo{
						{
							Name: clusterName,
							AuthInfo: clientcmdv1.AuthInfo{
								ClientCertificateData: []byte(clientCert),
								ClientKeyData:         []byte(clientKey),
							},
						},
						{
							Name: clusterName + "-basic-auth",
							AuthInfo: clientcmdv1.AuthInfo{
								Username: basicAuthUser,
								Password: basicAuthPass,
							},
						},
					},
				}))
			})

			Context("without certificate", func() {
				It("should return a kubeconfig with one context and one user", func() {
					secret.KubeConfigRequests[0].CAData = []byte(caCert)

					kubeconfig, err := generateKubeconfig(secret, nil)
					Expect(err).NotTo(HaveOccurred())

					Expect(yaml.Unmarshal(kubeconfig, &kubecfg)).To(Succeed())

					Expect(kubecfg.CurrentContext).To(Equal(clusterName))
					Expect(kubecfg.Clusters).To(HaveLen(1))
					Expect(kubecfg.Contexts).To(HaveLen(1))
					Expect(kubecfg.AuthInfos).To(HaveLen(1))
					Expect(kubecfg.Clusters[0].Cluster.Server).To(Equal("https://" + apiServerURL))
					Expect(kubecfg.Clusters[0].Cluster.CertificateAuthorityData).To(Equal([]byte(caCert)))
					Expect(kubecfg.AuthInfos[0].AuthInfo.ClientCertificateData).To(BeEmpty())
					Expect(kubecfg.AuthInfos[0].AuthInfo.ClientKeyData).To(BeEmpty())
					Expect(kubecfg.AuthInfos[0].AuthInfo.Token).To(BeEmpty())
					Expect(kubecfg.AuthInfos[0].AuthInfo.Username).To(Equal(basicAuthUser))
					Expect(kubecfg.AuthInfos[0].AuthInfo.Password).To(Equal(basicAuthPass))
				})
			})
		})
	})
})
