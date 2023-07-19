// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/nodeagent/apis/config"
	. "github.com/gardener/gardener/pkg/nodeagent/apis/config/validation"
)

var _ = Describe("#ValidateNodeAgentConfiguration", func() {
	var conf *config.NodeAgentConfiguration

	BeforeEach(func() {
		conf = &config.NodeAgentConfiguration{
			APIServer: config.APIServer{},
		}
	})

	Context("NodeAgentConfiguration", func() {
		It("should pass because apiversion is specified", func() {
			conf.APIVersion = "1.2.3"
			conf.HyperkubeImage = "registry.com/hyperkube:v1.27.0"
			conf.Image = "registry.com/node-agent:v1.73.0"
			conf.KubernetesVersion = "v1.27.0"
			conf.OSCSecretName = "osc-secret"
			conf.TokenSecretName = "token-secret"
			conf.APIServer.BootstrapToken = "bootstraptoken"
			conf.APIServer.CA = "base64 encoded ca"
			conf.APIServer.URL = "https://api.shoot.foo.bar"
			errorList := ValidateNodeAgentConfiguration(conf)
			Expect(errorList).To(BeEmpty())
		})
		It("should fail because apiversion config is not specified", func() {
			conf.HyperkubeImage = "registry.com/hyperkube:v1.27.0"
			conf.Image = "registry.com/node-agent:v1.73.0"
			conf.KubernetesVersion = "v1.27.0"
			conf.OSCSecretName = "osc-secret"
			conf.TokenSecretName = "token-secret"
			conf.APIServer.BootstrapToken = "bootstraptoken"
			conf.APIServer.CA = "base64 encoded ca"
			conf.APIServer.URL = "https://api.shoot.foo.bar"
			errorList := ValidateNodeAgentConfiguration(conf)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("nodeagent.config.apiversion"),
				})),
			))
		})
		It("should fail because apiserver.URL config is not specified", func() {
			conf.APIVersion = "1.2.3"
			conf.HyperkubeImage = "registry.com/hyperkube:v1.27.0"
			conf.Image = "registry.com/node-agent:v1.73.0"
			conf.KubernetesVersion = "v1.27.0"
			conf.OSCSecretName = "osc-secret"
			conf.TokenSecretName = "token-secret"
			conf.APIServer.BootstrapToken = "bootstraptoken"
			conf.APIServer.CA = "base64 encoded ca"
			errorList := ValidateNodeAgentConfiguration(conf)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("nodeagent.config.apiserver.url"),
				})),
			))
		})
	})
})
