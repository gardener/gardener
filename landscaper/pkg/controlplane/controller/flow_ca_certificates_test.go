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
	"time"

	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/gomega"
)

var _ = Describe("#GenerateCACertificates", func() {
	var (
		testOperation operation
	)

	// mocking
	var (
		ctx               = context.TODO()
		ctrl              *gomock.Controller
		mockRuntimeClient *mockclient.MockClient
		runtimeClient     kubernetes.Interface
		errNotFound       = &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
	)

	AfterEach(func() {
		ctrl.Finish()
	})

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockRuntimeClient = mockclient.NewMockClient(ctrl)

		runtimeClient = fake.NewClientSetBuilder().WithClient(mockRuntimeClient).Build()

		testOperation = operation{
			log:           logrus.NewEntry(logger.NewNopLogger()),
			runtimeClient: runtimeClient,
			imports: &imports.Imports{
				GardenerAPIServer: &imports.GardenerAPIServer{
					ComponentConfiguration: &imports.APIServerComponentConfiguration{
						CA: &imports.CA{
							Validity: &metav1.Duration{Duration: 10 * time.Second},
						},
					},
				},
				GardenerAdmissionController: &imports.GardenerAdmissionController{
					Enabled: true,
					ComponentConfiguration: &imports.AdmissionControllerComponentConfiguration{
						CA: &imports.CA{
							Validity: &metav1.Duration{Duration: 10 * time.Second},
						},
					},
				},
			},
		}
	})

	It("should get the Gardener API Server CA from the APIService", func() {
		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key("garden", "landscaper-controlplane-apiserver-ca-key"), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(errNotFound)
		mockRuntimeClient.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(_ context.Context, s *corev1.Secret, _ ...client.CreateOption) error {
				Expect(s).ToNot(BeNil())
				Expect(s.Data).To(HaveLen(1))
				Expect(s.Data["ca.key"]).ToNot(BeNil())
				return nil
			},
		)

		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key("garden", "landscaper-controlplane-admission-controller-ca-key"), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(errNotFound)
		mockRuntimeClient.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(_ context.Context, s *corev1.Secret, _ ...client.CreateOption) error {
				Expect(s).ToNot(BeNil())
				Expect(s.Data).To(HaveLen(1))
				Expect(s.Data["ca.key"]).ToNot(BeNil())
				return nil
			},
		)

		Expect(testOperation.GenerateCACertificates(ctx)).ToNot(HaveOccurred())
		Expect(testOperation.imports.GardenerAPIServer.ComponentConfiguration.CA.Key).ToNot(BeNil())
		Expect(testOperation.imports.GardenerAPIServer.ComponentConfiguration.CA.Crt).ToNot(BeNil())

		Expect(testOperation.imports.GardenerAdmissionController.ComponentConfiguration.CA.Crt).ToNot(BeNil())
		Expect(testOperation.imports.GardenerAdmissionController.ComponentConfiguration.CA.Key).ToNot(BeNil())
	})
})
