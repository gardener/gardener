// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package networkpolicies_test

import (
	"context"
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/networkpolicies"
)

var _ = Describe("NetworkPolicies", func() {
	var (
		ctx       = context.TODO()
		fakeErr   = fmt.Errorf("fake error")
		namespace = "shoot--foo--bar"

		ctrl     *gomock.Controller
		c        *mockclient.MockClient
		deployer component.Deployer
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		deployer = New(c, namespace)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		It("should fail if any call fails", func() {
			c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-aggregate-prometheus", Namespace: namespace}}).Return(fakeErr)

			Expect(deployer.Deploy(ctx)).To(MatchError(fakeErr))
		})

		It("w/o any special configuration", func() {
			c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-aggregate-prometheus", Namespace: namespace}})
			c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-seed-prometheus", Namespace: namespace}})
			c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-all-shoot-apiservers", Namespace: namespace}})

			Expect(deployer.Deploy(ctx)).To(Succeed())
		})
	})

	Describe("#Destroy", func() {
		It("should fail if an object fails to delete", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-aggregate-prometheus", Namespace: namespace}}).Return(fakeErr),
			)

			Expect(deployer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should successfully destroy all objects", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-aggregate-prometheus", Namespace: namespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-seed-prometheus", Namespace: namespace}}),
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-all-shoot-apiservers", Namespace: namespace}}),
			)

			Expect(deployer.Destroy(ctx)).To(Succeed())
		})
	})
})
