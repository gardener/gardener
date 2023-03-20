// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package secrets_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/secrets"
)

var _ = Describe("Kubeconfig Secrets", func() {
	var (
		name        = "bundle"
		contextName = "shoot--foo--bar"
		cluster     = clientcmdv1.Cluster{Server: "server", CertificateAuthority: "/some/path"}
		authInfo    = clientcmdv1.AuthInfo{Token: "token"}
	)

	Describe("Configuration", func() {
		var config *KubeconfigSecretConfig

		BeforeEach(func() {
			config = &KubeconfigSecretConfig{
				Name:        name,
				ContextName: contextName,
				Cluster:     cluster,
				AuthInfo:    authInfo,
			}
		})

		Describe("#GetName", func() {
			It("should return the name", func() {
				Expect(config.GetName()).To(Equal(name))
			})
		})

		Describe("#Generate", func() {
			It("should generate the kubeconfig", func() {
				obj, err := config.Generate()
				Expect(err).NotTo(HaveOccurred())

				kubeconfig, ok := obj.(*Kubeconfig)
				Expect(ok).To(BeTrue())

				Expect(kubeconfig.Name).To(Equal(name))
				Expect(kubeconfig.Kubeconfig).To(Equal(kubernetesutils.NewKubeconfig(contextName, cluster, authInfo)))
			})
		})

		Describe("#SecretData", func() {
			It("should return the correct data map", func() {
				obj, err := config.Generate()
				Expect(err).NotTo(HaveOccurred())

				kubeconfig, ok := obj.(*Kubeconfig)
				Expect(ok).To(BeTrue())

				raw, err := runtime.Encode(clientcmdlatest.Codec, kubeconfig.Kubeconfig)
				Expect(err).NotTo(HaveOccurred())

				Expect(kubeconfig.SecretData()).To(Equal(map[string][]byte{"kubeconfig": raw}))
			})
		})
	})
})
