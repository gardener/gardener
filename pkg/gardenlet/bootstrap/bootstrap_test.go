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
	"fmt"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/gardenlet/bootstrap"
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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Bootstrap", func() {
	Describe("BuildBootstrapperName", func() {
		It("should return the correct name", func() {
			name := "foo"
			result := BuildBootstrapperName(name)
			Expect(result).To(Equal(fmt.Sprintf("%s:%s", GardenerSeedBootstrapper, name)))
		})
	})

	Describe("#DeleteBootstrapAuth", func() {
		var (
			ctrl *gomock.Controller
			c    *mockclient.MockClient

			ctx     = context.TODO()
			csrName = "csr-name"
			csrKey  = kutil.Key(csrName)
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			c = mockclient.NewMockClient(ctrl)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

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

				clusterRoleBinding = &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: BuildBootstrapperName(seedName)}}
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
