// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrappers_test

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/cmd/gardenlet/app/bootstrappers"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gardenletbootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("GardenKubeconfig", func() {
	var (
		ctx                = context.TODO()
		log                = logr.Discard()
		seedName           = "seed"
		kubeconfigValidity = time.Hour

		fakeClient client.Client
		cfg        *gardenletconfigv1alpha1.GardenletConfiguration
		result     *KubeconfigBootstrapResult
		runner     *GardenKubeconfig
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		cfg = &gardenletconfigv1alpha1.GardenletConfiguration{
			GardenClientConnection: &gardenletconfigv1alpha1.GardenClientConnection{
				KubeconfigValidity: &gardenletconfigv1alpha1.KubeconfigValidity{
					Validity: &metav1.Duration{Duration: kubeconfigValidity},
				},
			},
			SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
				SeedTemplate: gardencorev1beta1.SeedTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name: seedName,
					},
				},
			},
		}
		result = &KubeconfigBootstrapResult{}
		runner = &GardenKubeconfig{
			SeedClient: fakeClient,
			Log:        log,
			Config:     cfg,
			Result:     result,
		}
	})

	Describe("#Start", func() {
		Context("when kubeconfigSecret is nil", func() {
			BeforeEach(func() {
				runner.Config.GardenClientConnection.KubeconfigSecret = nil
			})

			It("should return an error because .gardenClientConnection.kubeconfig is nil", func() {
				runner.Config.GardenClientConnection.Kubeconfig = ""

				Expect(runner.Start(ctx)).To(MatchError(ContainSubstring("the configuration file needs to either specify a Garden API Server kubeconfig under `.gardenClientConnection.kubeconfig` or provide bootstrapping information.")))
				Expect(result.Kubeconfig).To(BeNil())
			})

			It("should return nil", func() {
				kubeconfigPath := "/some/path/to/a/kubeconfig/file"
				runner.Config.GardenClientConnection.Kubeconfig = kubeconfigPath

				Expect(runner.Start(ctx)).To(Succeed())
				Expect(result.Kubeconfig).To(BeNil())
			})
		})

		Context("when kubeconfigSecret is set", func() {
			var (
				secretName      = "gardenlet-kubeconfig"
				secretNamespace = "garden"
			)

			BeforeEach(func() {
				runner.Config.GardenClientConnection.KubeconfigSecret = &corev1.SecretReference{
					Name:      secretName,
					Namespace: secretNamespace,
				}
			})

			Context("when kubeconfig already exists", func() {
				var (
					restConfig         *rest.Config
					existingKubeconfig []byte
				)

				BeforeEach(func() {
					restConfig = &rest.Config{
						Host: "testhost",
						TLSClientConfig: rest.TLSClientConfig{
							Insecure: false,
							CAData:   []byte("foo"),
						},
					}

					var err error
					existingKubeconfig, err = gardenletbootstraputil.CreateGardenletKubeconfigWithClientCertificate(restConfig, nil, nil)
					Expect(err).ToNot(HaveOccurred())

					secret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      secretName,
							Namespace: secretNamespace,
						},
						Data: map[string][]byte{"kubeconfig": existingKubeconfig},
					}
					Expect(fakeClient.Create(ctx, secret)).To(Succeed())
				})

				It("should return the existing kubeconfig", func() {
					Expect(runner.Start(ctx)).To(Succeed())
					Expect(result.Kubeconfig).To(Equal(existingKubeconfig))
				})

				It("should update the CA bundle if it changed", func() {
					newCABundle := []byte("bar")
					runner.Config.GardenClientConnection.GardenClusterCACert = newCABundle

					restConfig.CAData = newCABundle
					updatedKubeconfig, err := gardenletbootstraputil.CreateGardenletKubeconfigWithClientCertificate(restConfig, nil, nil)
					Expect(err).ToNot(HaveOccurred())

					Expect(runner.Start(ctx)).To(Succeed())
					Expect(result.Kubeconfig).To(Equal(updatedKubeconfig))
				})
			})

			Context("when kubeconfig does not yet exist", func() {
				Context("when bootstrapKubeconfig is nil", func() {
					BeforeEach(func() {
						runner.Config.GardenClientConnection.BootstrapKubeconfig = nil
					})

					It("should return an error when the bootstrapKubeconfig is not set", func() {
						Expect(runner.Start(ctx)).To(MatchError(ContainSubstring("the configuration file needs to either specify a Garden API Server kubeconfig under `.gardenClientConnection.kubeconfig` or provide bootstrapping information.")))
						Expect(result.Kubeconfig).To(BeNil())
					})
				})

				Context("when bootstrapKubeconfig is set", func() {
					var (
						bootstrapSecretName      = "bootstrap-secret-name"
						bootstrapSecretNamespace = "bootstrap-secret-namespace"
					)

					BeforeEach(func() {
						runner.Config.GardenClientConnection.BootstrapKubeconfig = &corev1.SecretReference{
							Name:      bootstrapSecretName,
							Namespace: bootstrapSecretNamespace,
						}
					})

					It("should return an error when the bootstrap kubeconfig secret does not exist", func() {
						Expect(runner.Start(ctx)).To(MatchError(ContainSubstring("bootstrap secret does not contain a kubeconfig, cannot bootstrap")))
						Expect(result.Kubeconfig).To(BeNil())
					})

					It("should return an error when the bootstrap kubeconfig secret exists but does not contain a kubeconfig", func() {
						secret := &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      bootstrapSecretName,
								Namespace: bootstrapSecretNamespace,
							},
						}
						Expect(fakeClient.Create(ctx, secret)).To(Succeed())

						Expect(runner.Start(ctx)).To(MatchError(ContainSubstring("bootstrap secret does not contain a kubeconfig, cannot bootstrap")))
						Expect(result.Kubeconfig).To(BeNil())
					})

					It("should request a kubeconfig with the bootstrap kubeconfig", func() {
						var (
							requestedKubeconfig = []byte("requested-kubeconfig-" + kubeconfigValidity.String())
							csrName             = "created-csr"
						)

						secret := &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      bootstrapSecretName,
								Namespace: bootstrapSecretNamespace,
							},
							Data: map[string][]byte{"kubeconfig": []byte("bootstrap-kubeconfig")},
						}
						Expect(fakeClient.Create(ctx, secret)).To(Succeed())

						DeferCleanup(test.WithVars(
							&NewClientFromBytes, func(_ []byte, _ ...kubernetes.ConfigFunc) (kubernetes.Interface, error) {
								return nil, nil
							},
							&RequestKubeconfigWithBootstrapClient, func(_ context.Context, _ logr.Logger, _ client.Client, _ kubernetes.Interface, _, _ client.ObjectKey, seedName string, validity *metav1.Duration) ([]byte, string, string, error) {
								return []byte("requested-kubeconfig-" + validity.Duration.String()), csrName, seedName, nil
							},
						))

						Expect(runner.Start(ctx)).To(Succeed())
						Expect(result.Kubeconfig).To(Equal(requestedKubeconfig))
						Expect(result.CSRName).To(Equal(csrName))
						Expect(result.SeedName).To(Equal(seedName))
					})
				})
			})
		})
	})
})
