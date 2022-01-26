// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controller

import (
	"context"
	"fmt"

	testutils "github.com/gardener/gardener/landscaper/common/test-utils"
	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("#GenerateVirtualGardenKubeconfig", func() {
	var (
		testOperation                operation
		serviceAccountName           = "gardener-api-server"
		serviceAccountSecretName     = "gardener-api-server-secret"
		serviceAccountSecretToken    = "abb342342"
		virtualGardenCA              = testutils.GenerateCACertificate("test")
		virtualGardenClusterEndpoint = "cluster-endpoint"
	)

	// mocking
	var (
		ctx              = context.TODO()
		ctrl             *gomock.Controller
		mockGardenClient *mockclient.MockClient
		gardenClient     kubernetes.Interface
		errNotFound      = &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
	)

	AfterEach(func() {
		ctrl.Finish()
	})

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockGardenClient = mockclient.NewMockClient(ctrl)

		gardenClient = fake.NewClientSetBuilder().WithClient(mockGardenClient).Build()

		testOperation = operation{
			log:                          logrus.NewEntry(logger.NewNopLogger()),
			runtimeClient:                gardenClient,
			imports:                      &imports.Imports{},
			virtualGardenCA:              virtualGardenCA.CertificatePEM,
			VirtualGardenClusterEndpoint: &virtualGardenClusterEndpoint,
		}
	})

	It("should return an error - service account not found", func() {
		mockGardenClient.EXPECT().Get(ctx, kutil.Key("garden", serviceAccountName), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})).Return(errNotFound)

		_, err := testOperation.GenerateVirtualGardenKubeconfig(ctx, serviceAccountName)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("failed to retrieve service account garden/%s from the runtime cluster", serviceAccountName)))
	})

	It("should return an error - service account secret not found", func() {
		mockGardenClient.EXPECT().Get(ctx, kutil.Key("garden", serviceAccountName), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Secrets: []corev1.ObjectReference{
						{
							Name: serviceAccountSecretName,
						},
					},
				}).DeepCopyInto(obj.(*corev1.ServiceAccount))
				return nil
			},
		)

		mockGardenClient.EXPECT().Get(ctx, kutil.Key("garden", serviceAccountSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(errNotFound)

		_, err := testOperation.GenerateVirtualGardenKubeconfig(ctx, serviceAccountName)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("failed to retrieve service account secret garden/%s from the runtime cluster", serviceAccountSecretName)))
	})

	It("should return an error - service account secret does not contain a JWT token", func() {
		mockGardenClient.EXPECT().Get(ctx, kutil.Key("garden", serviceAccountName), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Secrets: []corev1.ObjectReference{
						{
							Name: serviceAccountSecretName,
						},
					},
				}).DeepCopyInto(obj.(*corev1.ServiceAccount))
				return nil
			},
		)

		mockGardenClient.EXPECT().Get(ctx, kutil.Key("garden", serviceAccountSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"invalid": []byte("something"),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		)

		_, err := testOperation.GenerateVirtualGardenKubeconfig(ctx, serviceAccountName)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("does not contain a JWT token"))
	})

	It("should successfully generate a kubeconfig", func() {
		mockGardenClient.EXPECT().Get(ctx, kutil.Key("garden", serviceAccountName), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Secrets: []corev1.ObjectReference{
						{
							Name: serviceAccountSecretName,
						},
					},
				}).DeepCopyInto(obj.(*corev1.ServiceAccount))
				return nil
			},
		)

		mockGardenClient.EXPECT().Get(ctx, kutil.Key("garden", serviceAccountSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"token": []byte(serviceAccountSecretToken),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		)

		kubeconfig, err := testOperation.GenerateVirtualGardenKubeconfig(ctx, serviceAccountName)
		Expect(err).ToNot(HaveOccurred())
		Expect(kubeconfig).ToNot(BeNil())

		kc := &clientcmdv1.Config{}
		_, _, err = clientcmdlatest.Codec.Decode([]byte(*kubeconfig), nil, kc)
		Expect(err).ToNot(HaveOccurred())
		Expect(kc.AuthInfos).To(HaveLen(1))
		Expect(kc.AuthInfos[0].AuthInfo.Token).To(Equal(serviceAccountSecretToken))

		Expect(kc.Clusters).To(HaveLen(1))
		Expect(kc.Clusters[0].Cluster.CertificateAuthorityData).To(Equal(virtualGardenCA.CertificatePEM))
		Expect(kc.Clusters[0].Cluster.Server).To(Equal(fmt.Sprintf("https://%s", virtualGardenClusterEndpoint)))
	})
})
