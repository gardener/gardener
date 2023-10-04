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
	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"
	componentbaseconfig "k8s.io/component-base/config"

	. "github.com/gardener/gardener/pkg/nodeagent/apis/config"
	. "github.com/gardener/gardener/pkg/nodeagent/apis/config/validation"
)

var _ = Describe("#ValidateNodeAgentConfiguration", func() {
	var config *NodeAgentConfiguration

	BeforeEach(func() {
		config = &NodeAgentConfiguration{
			ClientConnection:                componentbaseconfig.ClientConnectionConfiguration{Kubeconfig: "path/to/kubeconfig"},
			KubernetesVersion:               semver.MustParse("v1.27.0"),
			HyperkubeImage:                  "registry.com/hyperkube:v1.27.0",
			Image:                           "registry.com/node-agent:v1.73.0",
			OperatingSystemConfigSecretName: "osc-secret",
			AccessTokenSecretName:           "token-secret",
		}
	})

	It("should fail because clientConnection.kubeconfig config is not specified", func() {
		config.ClientConnection.Kubeconfig = ""
		Expect(ValidateNodeAgentConfiguration(config)).To(ConsistOf(
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("clientConnection.kubeconfig"),
			})),
		))
	})

	It("should pass because all necessary fields is specified", func() {
		Expect(ValidateNodeAgentConfiguration(config)).To(BeEmpty())
	})

	It("should fail because hyperkube image config is not specified", func() {
		config.HyperkubeImage = ""

		Expect(ValidateNodeAgentConfiguration(config)).To(ConsistOf(
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("hyperkubeImage"),
			})),
		))
	})

	It("should fail because image config is not specified", func() {
		config.Image = ""

		Expect(ValidateNodeAgentConfiguration(config)).To(ConsistOf(
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("image"),
			})),
		))
	})

	It("should fail because kubernetes version is empty", func() {
		config.KubernetesVersion = nil

		Expect(ValidateNodeAgentConfiguration(config)).To(ConsistOf(
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("kubernetesVersion"),
			})),
		))
	})

	It("should fail because kubernetes version is unsupported", func() {
		config.KubernetesVersion = semver.MustParse("0.0.1+unsupported")

		Expect(ValidateNodeAgentConfiguration(config)).To(ConsistOf(
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("kubernetesVersion"),
			})),
		))
	})

	It("should fail because operating system config secret name is not specified", func() {
		config.OperatingSystemConfigSecretName = ""

		Expect(ValidateNodeAgentConfiguration(config)).To(ConsistOf(
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("operatingSystemConfigSecretName"),
			})),
		))
	})

	It("should fail because access token secret name is not specified", func() {
		config.AccessTokenSecretName = ""

		Expect(ValidateNodeAgentConfiguration(config)).To(ConsistOf(
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("accessTokenSecretName"),
			})),
		))
	})
})
