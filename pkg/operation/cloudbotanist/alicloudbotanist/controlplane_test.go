// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
package alicloudbotanist

import (
	"testing"

	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/shoot"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

func TestBotanist(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Alicoud control plane test")
}

var _ = Describe("validation", func() {

	Describe("validationRefreshCloudProviderConfig", func() {
		var (
			curMap = map[string]string{
				common.CloudProviderConfigMapKey: `{
					"Global":
					{
					  "kubernetesClusterTag":"my-k8s-test",
					  "vpcid":"vpc-2zehpnv9w5escf1hfqjsg",
					  "zoneID":"cn-beijing-f",
					  "region":"cn-beijing",
					  "vswitchid":"vsw-2ze3a4pi0j4wbt39g8r8i",
					  "accessKeyID":"ABC",
					  "accessKeySecret":"ABCD"
					}
				}
				`,
			}
			s = shoot.Shoot{
				Secret: &corev1.Secret{
					Data: map[string][]byte{
						AccessKeyID:     []byte("123"),
						AccessKeySecret: []byte("1234"),
					},
				},
			}

			b = &AlicloudBotanist{
				Operation: &operation.Operation{
					Shoot: &s,
				},
				CloudProviderName: "alicloud",
			}
		)

		It("should refresh OK", func() {
			m2 := b.RefreshCloudProviderConfig(curMap)
			expected := `{"Global":{"KubernetesClusterTag":"my-k8s-test","uid":"","vpcid":"vpc-2zehpnv9w5escf1hfqjsg","region":"cn-beijing","zoneid":"cn-beijing-f","vswitchid":"vsw-2ze3a4pi0j4wbt39g8r8i","accessKeyID":"MTIz","accessKeySecret":"MTIzNA=="}}`
			Expect(m2[common.CloudProviderConfigMapKey]).To(Equal(expected))
		})
	})
})
