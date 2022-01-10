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

	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("#PrepareGardenNamespace", func() {
	var testOperation operation

	// mocking
	var (
		ctx               = context.TODO()
		ctrl              *gomock.Controller
		mockRuntimeClient *mockclient.MockClient
		gardenClient      kubernetes.Interface
		expectedNamespace = corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "garden",
				Labels: map[string]string{
					"gardener.cloud/role":         "project",
					"project.gardener.cloud/name": "project",
					"app":                         "gardener",
				},
			},
		}
	)

	AfterEach(func() {
		ctrl.Finish()
	})

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockRuntimeClient = mockclient.NewMockClient(ctrl)

		gardenClient = fake.NewClientSetBuilder().WithClient(mockRuntimeClient).Build()

		testOperation = operation{
			log:           logrus.NewEntry(logger.NewNopLogger()),
			runtimeClient: gardenClient,
			imports:       &imports.Imports{},
		}
	})

	It("should create the garden namespace if it does not exist", func() {
		mockRuntimeClient.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(
			func(_ context.Context, s *corev1.Namespace, _ ...client.CreateOption) error {
				Expect(s).ToNot(BeNil())
				Expect(s).To(Equal(&expectedNamespace))
				return nil
			},
		)

		Expect(testOperation.PrepareGardenNamespace(ctx)).ToNot(HaveOccurred())
	})
})
