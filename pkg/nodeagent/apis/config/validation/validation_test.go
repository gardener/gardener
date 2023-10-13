// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package validation_test

import (
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	. "github.com/gardener/gardener/pkg/nodeagent/apis/config"
	. "github.com/gardener/gardener/pkg/nodeagent/apis/config/validation"
)

var _ = Describe("#ValidateNodeAgentConfiguration", func() {
	var config *NodeAgentConfiguration

	BeforeEach(func() {
		config = &NodeAgentConfiguration{
			Controllers: ControllerConfiguration{
				OperatingSystemConfig: OperatingSystemConfigControllerConfig{
					SecretName:        "osc-secret",
					SyncPeriod:        &metav1.Duration{Duration: time.Minute},
					KubernetesVersion: semver.MustParse("v1.27.0"),
				},
				Token: TokenControllerConfig{
					SecretName: "token-secret",
				},
				KubeletUpgrade: KubeletUpgradeControllerConfig{
					Image: "registry.com/hyperkube:v1.27.0",
				},
				SelfUpgrade: SelfUpgradeControllerConfig{
					Image: "registry.com/node-agent:v1.73.0",
				},
			},
		}
	})

	It("should pass because all necessary fields is specified", func() {
		Expect(ValidateNodeAgentConfiguration(config)).To(BeEmpty())
	})

	Context("Operating System Config Controller", func() {
		It("should fail because kubernetes version is empty", func() {
			config.Controllers.OperatingSystemConfig.KubernetesVersion = nil

			Expect(ValidateNodeAgentConfiguration(config)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("controllers.operatingSystemConfig.kubernetesVersion"),
				})),
			))
		})

		It("should fail because operating system config secret name is not specified", func() {
			config.Controllers.OperatingSystemConfig.SecretName = ""

			Expect(ValidateNodeAgentConfiguration(config)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("controllers.operatingSystemConfig.secretName"),
				})),
			))
		})
	})

	Context("Kubelet Upgrade Controller", func() {
		It("should fail because hyperkube image config is not specified", func() {
			config.Controllers.KubeletUpgrade.Image = ""

			Expect(ValidateNodeAgentConfiguration(config)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("controllers.kubeletUpgrade.image"),
				})),
			))
		})
	})

	Context("Self Upgrade Controller", func() {
		It("should fail because image config is not specified", func() {
			config.Controllers.SelfUpgrade.Image = ""

			Expect(ValidateNodeAgentConfiguration(config)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("controllers.selfUpgrade.image"),
				})),
			))
		})
	})

	Context("Token Controller", func() {
		It("should fail because access token secret name is not specified", func() {
			config.Controllers.Token.SecretName = ""

			Expect(ValidateNodeAgentConfiguration(config)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("controllers.token.secretName"),
				})),
			))
		})
	})
})
