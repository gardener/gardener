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

package util_test

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/util"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Shoot", func() {
	var (
		ctrl *gomock.Controller
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#GetOrCreateShootKubeconfig", func() {
		var (
			c         *mockclient.MockClient
			ctx       context.Context
			namespace string

			caName   string
			caSecret *corev1.Secret

			certificateConfig *secrets.CertificateSecretConfig
			certificateSecret *corev1.Secret

			createKubeconfig func() (*corev1.Secret, error)
		)

		BeforeEach(func() {
			c = mockclient.NewMockClient(ctrl)
			ctx = context.TODO()
			namespace = "shoot--foo--bar"

			caName = v1beta1constants.SecretNameCACluster
			caSecret = createNewCA(caName)

			certificateConfig = &secrets.CertificateSecretConfig{
				Name:       "bar",
				CommonName: "foo:bar",
			}
			certificateSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      certificateConfig.Name,
					Namespace: namespace,
				},
			}

			createKubeconfig = func() (*corev1.Secret, error) {
				c.EXPECT().Get(ctx, kutil.Key(namespace, caName), gomock.AssignableToTypeOf(&corev1.Secret{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *corev1.Secret) error {
						*actual = *caSecret
						return nil
					})

				c.EXPECT().Get(ctx, kutil.Key(namespace, certificateConfig.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *corev1.Secret) error {
						*actual = *certificateSecret
						return apierrors.NewNotFound(schema.GroupResource{}, certificateConfig.Name)
					}).
					MaxTimes(2)

				c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).
					DoAndReturn(func(_ context.Context, _ *corev1.Secret, _ ...client.CreateOption) error {
						return nil
					})

				return util.GetOrCreateShootKubeconfig(ctx, c, *certificateConfig, namespace)
			}
		})

		It("should create the kubeconfig", func() {
			secret, err := createKubeconfig()

			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Annotations[util.CAChecksumAnnotation]).ToNot(BeEmpty())
			Expect(secret.Data[secrets.DataKeyKubeconfig]).ToNot(BeEmpty())
		})

		It("should get the kubeconfig", func() {
			certificateSecret.Annotations = make(map[string]string)
			certificateSecret.Annotations[util.CAChecksumAnnotation] = utils.ComputeChecksum(caSecret.Data)

			c.EXPECT().Get(ctx, kutil.Key(namespace, caName), gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *corev1.Secret) error {
					*actual = *caSecret
					return nil
				})

			c.EXPECT().Get(ctx, kutil.Key(namespace, certificateConfig.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *corev1.Secret) error {
					*actual = *certificateSecret
					return nil
				})

			secret, err := util.GetOrCreateShootKubeconfig(ctx, c, *certificateConfig, namespace)

			Expect(err).NotTo(HaveOccurred())
			Expect(secret).To(Equal(certificateSecret))
		})

		It("should update the kubeconfig", func() {
			secret, err := createKubeconfig()
			Expect(err).NotTo(HaveOccurred())

			var (
				certificateSecret = certificateSecret.DeepCopy()
				newCASecret       = createNewCA(caName)
			)

			c.EXPECT().Get(ctx, kutil.Key(namespace, caName), gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *corev1.Secret) error {
					*actual = *newCASecret
					return nil
				})

			c.EXPECT().Get(ctx, kutil.Key(namespace, certificateConfig.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *corev1.Secret) error {
					*actual = *certificateSecret
					return nil
				}).
				MaxTimes(2)

			c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any())

			updatedSecret, err := util.GetOrCreateShootKubeconfig(ctx, c, *certificateConfig, namespace)

			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Data).ToNot(Equal(updatedSecret.Data))
			Expect(secret.Annotations[util.CAChecksumAnnotation]).ToNot(Equal(updatedSecret.Annotations[util.CAChecksumAnnotation]))
		})

		It("should fail because CA is not available", func() {
			c.EXPECT().Get(ctx, kutil.Key(namespace, caName), gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *corev1.Secret) error {
					return apierrors.NewNotFound(schema.GroupResource{}, caName)
				})

			_, err := util.GetOrCreateShootKubeconfig(ctx, c, *certificateConfig, namespace)

			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#VersionMajorMinor", func() {
		It("should return an error due to an invalid version format", func() {
			v, err := util.VersionMajorMinor("invalid-semver")

			Expect(v).To(BeEmpty())
			Expect(err).To(HaveOccurred())
		})

		It("should return the major/minor part of the given version", func() {
			var (
				major = 14
				minor = 123

				expectedVersion = fmt.Sprintf("%d.%d", major, minor)
			)

			v, err := util.VersionMajorMinor(fmt.Sprintf("%s.88", expectedVersion))

			Expect(v).To(Equal(expectedVersion))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#VersionInfo", func() {
		It("should return an error due to an invalid version format", func() {
			v, err := util.VersionInfo("invalid-semver")

			Expect(v).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		It("should convert the given version to a correct version.Info", func() {
			var (
				expectedVersionInfo = &version.Info{
					Major:      "14",
					Minor:      "123",
					GitVersion: "v14.123.42",
				}
			)

			v, err := util.VersionInfo("14.123.42")

			Expect(v).To(Equal(expectedVersionInfo))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

func createNewCA(caName string) *corev1.Secret {
	caCertConfig := &secrets.CertificateSecretConfig{
		Name:       caName,
		CommonName: "ca",
		CertType:   secrets.CACert,
	}
	caCert, err := caCertConfig.GenerateCertificate()
	if err != nil {
		panic(err)
	}
	caSecret := &corev1.Secret{
		Data: caCert.SecretData(),
	}

	return caSecret
}
