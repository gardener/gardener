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

	. "github.com/gardener/gardener/extensions/pkg/util"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
)

var _ = Describe("Deployments", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#ScaleDeployment", func() {
		It("should scale the deployment", func() {
			var replicas int32 = 5

			c.EXPECT().
				Update(gomock.Any(), gomock.AssignableToTypeOf(&appsv1.Deployment{})).
				DoAndReturn(func(_ context.Context, deployment *appsv1.Deployment) error {
					deployment.Spec.Replicas = &replicas
					return nil
				})

			Expect(ScaleDeployment(context.TODO(), c, &appsv1.Deployment{}, replicas)).NotTo(HaveOccurred())
		})
	})
})
