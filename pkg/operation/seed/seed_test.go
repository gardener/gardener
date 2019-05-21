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

package seed_test

import (
	"context"

	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mock "github.com/gardener/gardener/pkg/mock/gardener/kubernetes"
	. "github.com/gardener/gardener/pkg/operation/seed"
	"github.com/golang/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("seed", func() {
	var (
		ctrl           *gomock.Controller
		restMockClient *mock.MockInterface
		runtimeClient  *mockclient.MockClient
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		restMockClient = mock.NewMockInterface(ctrl)
		runtimeClient = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#GetFluentdReplicaCount", func() {
		It("should return single replica when stateful set does not exist", func() {
			restMockClient.EXPECT().Client().Return(runtimeClient)
			runtimeClient.EXPECT().Get(context.TODO(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, key client.ObjectKey, _ *appsv1.StatefulSet) error {
				return errors.NewNotFound(appsv1.Resource("StatefulSet"), key.Name)
			})

			replicas, err := GetFluentdReplicaCount(restMockClient)

			Expect(err).NotTo(HaveOccurred())
			var expectedReplicas int32 = 1
			Expect(replicas).To(Equal(expectedReplicas))
		})

		It("should get stateful set replicas", func() {
			var expectedReplicas int32 = 3
			restMockClient.EXPECT().Client().Return(runtimeClient)
			runtimeClient.EXPECT().Get(context.TODO(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ client.ObjectKey, statefulSet *appsv1.StatefulSet) error {
				statefulSet.Spec.Replicas = &expectedReplicas
				return nil
			})

			replicas, err := GetFluentdReplicaCount(restMockClient)

			Expect(err).NotTo(HaveOccurred())
			Expect(replicas).To(Equal(expectedReplicas))
		})
	})
})
