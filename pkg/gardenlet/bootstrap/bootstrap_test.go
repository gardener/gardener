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

package bootstrap_test

import (
	"context"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	. "github.com/gardener/gardener/pkg/gardenlet/bootstrap"
	bootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Bootstrap", func() {
	var (
		ctrl       *gomock.Controller
		c          *mockclient.MockClient
		ctx        = context.TODO()
		testLogger = logger.NewNopLogger()
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#RequestBootstrapKubeconfig", func() {
		var (
			seedName = "test"

			seedClient            client.Client
			bootstrapClientConfig *rest.Config

			gardenClientConnection *config.GardenClientConnection

			approvedCSR = certificatesv1beta1.CertificateSigningRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: "watched-csr",
				},
				Status: certificatesv1beta1.CertificateSigningRequestStatus{
					Conditions: []certificatesv1beta1.CertificateSigningRequestCondition{
						{
							Type: certificatesv1beta1.CertificateApproved,
						},
					},
					Certificate: []byte("my-cert"),
				},
			}

			deniedCSR = certificatesv1beta1.CertificateSigningRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: "watched-csr",
				},
				Status: certificatesv1beta1.CertificateSigningRequestStatus{
					Conditions: []certificatesv1beta1.CertificateSigningRequestCondition{
						{
							Type: certificatesv1beta1.CertificateDenied,
						},
					},
				},
			}
		)

		BeforeEach(func() {
			secretReference := corev1.SecretReference{
				Name:      "gardenlet-kubeconfig",
				Namespace: "garden",
			}

			bootstrapSecretReference := corev1.SecretReference{
				Name:      "bootstrap-kubeconfig",
				Namespace: "garden",
			}

			// gardenClientConnection with required bootstrap secret kubeconfig secret
			// in a non-test environment we would use two different secrets
			gardenClientConnection = &config.GardenClientConnection{
				BootstrapKubeconfig: &bootstrapSecretReference,
				KubeconfigSecret:    &secretReference,
			}

			// create mock seed client
			s := runtime.NewScheme()
			Expect(corev1.AddToScheme(s)).ToNot(HaveOccurred())
			seedClient = fakeclient.NewFakeClientWithScheme(s)

			// rest config for the bootstrap client
			bootstrapClientConfig = &rest.Config{Host: "testhost", TLSClientConfig: rest.TLSClientConfig{
				Insecure: false,
				CAFile:   "filepath",
			}}

		})

		It("should not return an error", func() {
			bootstrapClient := fake.NewSimpleClientset(&approvedCSR).CertificatesV1beta1()

			c.EXPECT().Get(ctx, kutil.Key(gardenClientConnection.KubeconfigSecret.Namespace, gardenClientConnection.KubeconfigSecret.Name), gomock.AssignableToTypeOf(&corev1.Secret{}))

			c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
				secret, ok := obj.(*corev1.Secret)
				Expect(ok).To(BeTrue())
				Expect(secret.Name).To(Equal(gardenClientConnection.KubeconfigSecret.Name))
				Expect(secret.Namespace).To(Equal(gardenClientConnection.KubeconfigSecret.Namespace))
				Expect(secret.Data).ToNot(BeNil())
				Expect(secret.Data[kubernetes.KubeConfig]).ToNot(BeEmpty())
				return nil
			})
			c.EXPECT().Delete(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gardenClientConnection.BootstrapKubeconfig.Name,
					Namespace: gardenClientConnection.BootstrapKubeconfig.Namespace,
				},
			})

			kubeconfig, csrName, seedName, err := RequestBootstrapKubeconfig(ctx, testLogger, c, bootstrapClient, bootstrapClientConfig, gardenClientConnection, seedName, "my-cluster")

			Expect(err).NotTo(HaveOccurred())
			Expect(kubeconfig).ToNot(BeEmpty())
			Expect(len(csrName)).ToNot(Equal(0))
			Expect(len(seedName)).ToNot(Equal(0))
		})

		It("should return an error - the CSR got denied", func() {
			bootstrapClient := fake.NewSimpleClientset(&deniedCSR).CertificatesV1beta1()

			_, _, _, err := RequestBootstrapKubeconfig(ctx, testLogger, seedClient, bootstrapClient, bootstrapClientConfig, gardenClientConnection, seedName, "my-cluster")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#DeleteBootstrapAuth", func() {
		var (
			csrName = "csr-name"
			csrKey  = kutil.Key(csrName)
		)

		It("should return an error because the CSR was not found", func() {
			c.EXPECT().
				Get(ctx, csrKey, gomock.AssignableToTypeOf(&certificatesv1beta1.CertificateSigningRequest{})).
				Return(apierrors.NewNotFound(schema.GroupResource{Resource: "CertificateSigningRequests"}, csrName))

			Expect(DeleteBootstrapAuth(ctx, c, csrName, "")).NotTo(Succeed())
		})

		It("should delete nothing because the username in the CSR does not match a known pattern", func() {
			c.EXPECT().
				Get(ctx, csrKey, gomock.AssignableToTypeOf(&certificatesv1beta1.CertificateSigningRequest{})).
				Return(nil)

			Expect(DeleteBootstrapAuth(ctx, c, csrName, "")).To(Succeed())
		})

		It("should delete the bootstrap token secret", func() {
			var (
				bootstrapTokenID         = "12345"
				bootstrapTokenSecretName = "bootstrap-token-" + bootstrapTokenID
				bootstrapTokenUserName   = bootstraptokenapi.BootstrapUserPrefix + bootstrapTokenID
				bootstrapTokenSecret     = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceSystem, Name: bootstrapTokenSecretName}}
			)

			gomock.InOrder(
				c.EXPECT().
					Get(ctx, csrKey, gomock.AssignableToTypeOf(&certificatesv1beta1.CertificateSigningRequest{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, csr *certificatesv1beta1.CertificateSigningRequest) error {
						csr.Spec.Username = bootstrapTokenUserName
						return nil
					}),

				c.EXPECT().
					Delete(ctx, bootstrapTokenSecret),
			)

			Expect(DeleteBootstrapAuth(ctx, c, csrName, "")).To(Succeed())
		})

		It("should delete the service account and cluster role binding", func() {
			var (
				seedName                = "foo"
				serviceAccountName      = "foo"
				serviceAccountNamespace = v1beta1constants.GardenNamespace
				serviceAccountUserName  = serviceaccount.MakeUsername(serviceAccountNamespace, serviceAccountName)
				serviceAccount          = &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: serviceAccountNamespace, Name: serviceAccountName}}

				clusterRoleBinding = &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: bootstraputil.BuildBootstrapperName(seedName)}}
			)

			gomock.InOrder(
				c.EXPECT().
					Get(ctx, csrKey, gomock.AssignableToTypeOf(&certificatesv1beta1.CertificateSigningRequest{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, csr *certificatesv1beta1.CertificateSigningRequest) error {
						csr.Spec.Username = serviceAccountUserName
						return nil
					}),

				c.EXPECT().
					Delete(ctx, serviceAccount),

				c.EXPECT().
					Delete(ctx, clusterRoleBinding),
			)

			Expect(DeleteBootstrapAuth(ctx, c, csrName, seedName)).To(Succeed())
		})
	})
})
