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

	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("#GetVirtualGardenClusterEndpoint", func() {
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
		expectedService   = corev1.Service{Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port: 3000,
				},
			},
		}}
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
			imports:       &imports.Imports{},
		}
	})

	It("should return return an error - virtual garden service in the runtime cluster not found", func() {
		mockRuntimeClient.EXPECT().List(context.Background(), gomock.AssignableToTypeOf(&corev1.ServiceList{}), client.InNamespace(v1beta1constants.GardenNamespace), gomock.Any()).Return(errNotFound)
		Expect(testOperation.GetVirtualGardenClusterEndpoint(ctx)).To(HaveOccurred())
	})

	It("should return return an error - the service does not have a port", func() {
		mockRuntimeClient.EXPECT().List(context.Background(), gomock.AssignableToTypeOf(&corev1.ServiceList{}), client.InNamespace(v1beta1constants.GardenNamespace), gomock.Any()).DoAndReturn(func(ctx context.Context, list *corev1.ServiceList, opts ...client.ListOption) error {
			(&corev1.ServiceList{Items: []corev1.Service{{}}}).DeepCopyInto(list)
			return nil
		})

		err := testOperation.GetVirtualGardenClusterEndpoint(ctx)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("expected the virtual garden service in the runtime cluster to have at least one port"))
	})

	It("should obtain the virtual garden endpoint from the runtime cluster", func() {
		mockRuntimeClient.EXPECT().List(context.Background(), gomock.AssignableToTypeOf(&corev1.ServiceList{}), client.InNamespace(v1beta1constants.GardenNamespace), gomock.Any()).DoAndReturn(func(ctx context.Context, list *corev1.ServiceList, opts ...client.ListOption) error {
			(&corev1.ServiceList{Items: []corev1.Service{expectedService}}).DeepCopyInto(list)
			return nil
		})

		Expect(testOperation.GetVirtualGardenClusterEndpoint(ctx)).ToNot(HaveOccurred())
		Expect(testOperation.VirtualGardenClusterEndpoint).ToNot(BeNil())
		Expect(testOperation.VirtualGardenClusterEndpoint).To(Equal(pointer.String(fmt.Sprintf("%s:%d", expectedService.Name, expectedService.Spec.Ports[0].Port))))
	})
})
