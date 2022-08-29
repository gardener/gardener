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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Gardenlet", func() {
	Describe("#CheckSeedConfig", func() {
		var (
			ctx    = context.TODO()
			client client.Client

			podCIDR     = "10.0.0.0/8"
			serviceCIDR = "192.168.0.0/16"
			nodeCIDR    = "172.16.0.0/12"
			otherCIDR   = "1.1.0.0/22"

			shootInfoMeta = metav1.ObjectMeta{
				Name:      v1beta1constants.ConfigMapNameShootInfo,
				Namespace: metav1.NamespaceSystem,
			}
			shootInfoWithNodes = &corev1.ConfigMap{
				ObjectMeta: shootInfoMeta,
				Data: map[string]string{
					"podNetwork":     podCIDR,
					"serviceNetwork": serviceCIDR,
					"nodeNetwork":    nodeCIDR,
				},
			}
			shootInfoWithIncorrectNodes = &corev1.ConfigMap{
				ObjectMeta: shootInfoMeta,
				Data: map[string]string{
					"podNetwork":     podCIDR,
					"serviceNetwork": serviceCIDR,
					"nodeNetwork":    otherCIDR,
				},
			}
			shootInfoWithoutNodes = &corev1.ConfigMap{
				ObjectMeta: shootInfoMeta,
				Data: map[string]string{
					"podNetwork":     podCIDR,
					"serviceNetwork": serviceCIDR,
				},
			}
		)

		DescribeTable("validate seed network configuration",
			func(seedConfig *config.SeedConfig, shootInfo *corev1.ConfigMap, secretRetrievalExpected bool, matcher gomegatypes.GomegaMatcher) {
				client = fakeclient.NewClientBuilder().WithObjects(shootInfo).Build()
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
			Entry("correct seed configuration with incorrect nodes but no nodes in shoot-info", &config.SeedConfig{SeedTemplate: core.SeedTemplate{Spec: core.SeedSpec{Networks: core.SeedNetworks{
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
