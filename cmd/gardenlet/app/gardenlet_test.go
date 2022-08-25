// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package app_test

import (
	"context"

	. "github.com/gardener/gardener/cmd/gardenlet/app"
	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Gardenlet", func() {
	Describe("#CheckSeedConfig", func() {
		var (
			ctx    = context.TODO()
			ctrl   *gomock.Controller
			client *mockclient.MockClient

			podCIDR     = "10.0.0.0/8"
			serviceCIDR = "192.168.0.0/16"
			nodeCIDR    = "172.16.0.0/12"
			otherCIDR   = "1.1.0.0/22"

			shootInfoWithNodes = &corev1.ConfigMap{
				Data: map[string]string{
					"podNetwork":     podCIDR,
					"serviceNetwork": serviceCIDR,
					"nodeNetwork":    nodeCIDR,
				},
			}
			shootInfoWithIncorrectNodes = &corev1.ConfigMap{
				Data: map[string]string{
					"podNetwork":     podCIDR,
					"serviceNetwork": serviceCIDR,
					"nodeNetwork":    otherCIDR,
				},
			}
			shootInfoWithoutNodes = &corev1.ConfigMap{
				Data: map[string]string{
					"podNetwork":     podCIDR,
					"serviceNetwork": serviceCIDR,
				},
			}
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			client = mockclient.NewMockClient(ctrl)
		})

		DescribeTable("validate seed network configuration",
			func(seedConfig *config.SeedConfig, shootInfo *corev1.ConfigMap, secretRetrievalExpected bool, matcher gomegatypes.GomegaMatcher) {
				if secretRetrievalExpected {
					client.EXPECT().Get(ctx, kutil.Key(metav1.NamespaceSystem, v1beta1constants.ConfigMapNameShootInfo), &corev1.ConfigMap{}).DoAndReturn(clientGet(shootInfo))
				}
				Expect(CheckSeedConfig(ctx, client, seedConfig)).To(matcher)
			},
			Entry("no seed configuration", nil, shootInfoWithNodes, false, BeNil()),
			Entry("correct seed configuration with nodes", &config.SeedConfig{SeedTemplate: core.SeedTemplate{Spec: core.SeedSpec{Networks: core.SeedNetworks{
				Nodes:    &nodeCIDR,
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, shootInfoWithNodes, true, BeNil()),
			Entry("correct seed configuration without nodes", &config.SeedConfig{SeedTemplate: core.SeedTemplate{Spec: core.SeedSpec{Networks: core.SeedNetworks{
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, shootInfoWithoutNodes, true, BeNil()),
			Entry("correct seed configuration with nodes but no nodes in shoot-info", &config.SeedConfig{SeedTemplate: core.SeedTemplate{Spec: core.SeedSpec{Networks: core.SeedNetworks{
				Nodes:    &nodeCIDR,
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, shootInfoWithoutNodes, true, BeNil()),
			Entry("correct seed configuration without nodes but nodes in shoot-info", &config.SeedConfig{SeedTemplate: core.SeedTemplate{Spec: core.SeedSpec{Networks: core.SeedNetworks{
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, shootInfoWithNodes, true, BeNil()),
			Entry("correct seed configuration incorrect nodes but no nodes in shoot-info", &config.SeedConfig{SeedTemplate: core.SeedTemplate{Spec: core.SeedSpec{Networks: core.SeedNetworks{
				Nodes:    &otherCIDR,
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, shootInfoWithoutNodes, true, BeNil()),
			Entry("correct seed configuration without nodes but incorrect nodes in shoot-info", &config.SeedConfig{SeedTemplate: core.SeedTemplate{Spec: core.SeedSpec{Networks: core.SeedNetworks{
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, shootInfoWithIncorrectNodes, true, BeNil()),
			Entry("incorrect node cidr", &config.SeedConfig{SeedTemplate: core.SeedTemplate{Spec: core.SeedSpec{Networks: core.SeedNetworks{
				Nodes:    &otherCIDR,
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, shootInfoWithNodes, true, HaveOccurred()),
			Entry("incorrect pod cidr", &config.SeedConfig{SeedTemplate: core.SeedTemplate{Spec: core.SeedSpec{Networks: core.SeedNetworks{
				Nodes:    &nodeCIDR,
				Pods:     otherCIDR,
				Services: serviceCIDR,
			}}}}, shootInfoWithNodes, true, HaveOccurred()),
			Entry("incorrect service cidr", &config.SeedConfig{SeedTemplate: core.SeedTemplate{Spec: core.SeedSpec{Networks: core.SeedNetworks{
				Nodes:    &nodeCIDR,
				Pods:     podCIDR,
				Services: otherCIDR,
			}}}}, shootInfoWithNodes, true, HaveOccurred()),
		)
	})
})

func clientGet(result runtime.Object) interface{} {
	return func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
		switch obj.(type) {
		case *corev1.ConfigMap:
			*obj.(*corev1.ConfigMap) = *result.(*corev1.ConfigMap)
		}
		return nil
	}
}
