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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/nodeagent/apis/config"
	. "github.com/gardener/gardener/pkg/nodeagent/apis/config/validation"
)

var _ = Describe("#ValidateNodeAgentConfiguration", func() {
	var conf *config.NodeAgentConfiguration

	BeforeEach(func() {
		conf = &config.NodeAgentConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "1.2.3",
			},
			APIServer: config.APIServer{
				BootstrapToken: "bootstraptoken",
				CABundle:       []byte("base64 encoded ca"),
				URL:            "https://api.shoot.foo.bar",
			},
			KubernetesVersion:               "v1.27.0",
			HyperkubeImage:                  "registry.com/hyperkube:v1.27.0",
			Image:                           "registry.com/node-agent:v1.73.0",
			OperatingSystemConfigSecretName: "osc-secret",
			AccessTokenSecretName:           "token-secret",
		}
	})

	Context("NodeAgentConfiguration", func() {
		It("should pass because apiVersion is specified", func() {
			errorList := ValidateNodeAgentConfiguration(conf)
			Expect(errorList).To(BeEmpty())
		})

		It("should fail because hyperkube image config is not specified", func() {
			conf.HyperkubeImage = ""
			errorList := ValidateNodeAgentConfiguration(conf)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("nodeAgent.config.hyperkubeImage"),
				})),
			))
		})

		It("should fail because image config is not specified", func() {
			conf.Image = ""
			errorList := ValidateNodeAgentConfiguration(conf)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("nodeAgent.config.image"),
				})),
			))
		})

		It("should fail because kubernetes version is empty", func() {
			conf.KubernetesVersion = ""
			errorList := ValidateNodeAgentConfiguration(conf)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("nodeAgent.config.kubernetesVersion"),
				})),
			))
		})

		It("should fail because kubernetes version is unsupported", func() {
			conf.KubernetesVersion = "unsupported"
			errorList := ValidateNodeAgentConfiguration(conf)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("nodeAgent.config.kubernetesVersion"),
				})),
			))
		})

		It("should fail because oscSecretName config is not specified", func() {
			conf.OperatingSystemConfigSecretName = ""
			errorList := ValidateNodeAgentConfiguration(conf)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("nodeAgent.config.oscSecretName"),
				})),
			))
		})

		It("should fail because tokenSecretName config is not specified", func() {
			conf.AccessTokenSecretName = ""
			errorList := ValidateNodeAgentConfiguration(conf)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("nodeAgent.config.tokenSecretName"),
				})),
			))
		})

		It("should fail because apiServer.url config is not specified", func() {
			conf.APIServer.URL = ""
			errorList := ValidateNodeAgentConfiguration(conf)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("nodeAgent.config.apiServer.url"),
				})),
			))
		})

		It("should fail because apiServer.ca config is not specified", func() {
			conf.APIServer.CABundle = nil
			errorList := ValidateNodeAgentConfiguration(conf)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("nodeAgent.config.apiServer.ca"),
				})),
			))
		})

		It("should fail because apiServer.bootstrapToken config is not specified", func() {
			conf.APIServer.BootstrapToken = ""
			errorList := ValidateNodeAgentConfiguration(conf)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("nodeAgent.config.apiServer.bootstrapToken"),
				})),
			))
		})
	})
})
