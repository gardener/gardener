// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes_test

import (
	"fmt"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const impersonateGroup = "act-as-group"

var _ = Describe("Client", func() {

	var (
		authInfo clientcmdapi.AuthInfo
		config   clientcmdapi.Config
	)

	fieldsToTest := []string{kubernetes.AuthTokenFile, kubernetes.AuthImpersonate, impersonateGroup, kubernetes.AuthExec, kubernetes.AuthProvider, kubernetes.AuthClientCertificate, kubernetes.AuthClientKey}

	BeforeEach(func() {
		authInfo = clientcmdapi.AuthInfo{}
		config = clientcmdapi.Config{
			AuthInfos: map[string]*clientcmdapi.AuthInfo{
				"test": &authInfo,
			},
		}
	})

	Context("ValidateConfig", func() {
		It("An empty Kubeconfig should with be validated successfully", func() {
			err := kubernetes.ValidateConfig(config)
			Expect(err).ToNot(HaveOccurred())
		})
		It("A Kubeconfig with a valid field should with be validated successfully", func() {
			config.AuthInfos["test"].Token = "test"

			err := kubernetes.ValidateConfig(config)
			Expect(err).ToNot(HaveOccurred())
		})
		for _, field := range fieldsToTest {
			It(fmt.Sprintf("A kubeconfig with %s should be validated properly", field), func() {
				switch field {
				case kubernetes.AuthProvider:
					config.AuthInfos["test"].AuthProvider = &clientcmdapi.AuthProviderConfig{Config: map[string]string{"provider": "test"}}
				case kubernetes.AuthExec:
					config.AuthInfos["test"].Exec = &clientcmdapi.ExecConfig{Command: "/bin/test"}
				case kubernetes.AuthTokenFile:
					config.AuthInfos["test"].TokenFile = "test"
				case kubernetes.AuthImpersonate:
					config.AuthInfos["test"].Impersonate = "test"
				case impersonateGroup:
					config.AuthInfos["test"].ImpersonateGroups = []string{"test"}
				case kubernetes.AuthClientKey:
					config.AuthInfos["test"].ClientKey = "test"
				case kubernetes.AuthClientCertificate:
					config.AuthInfos["test"].ClientCertificate = "test"
				}
				err := kubernetes.ValidateConfig(config)
				Expect(err).To(HaveOccurred())
			})
		}
	})

	Context("ValidateConfigWithAllowList", func() {
		It("An empty Kubeconfig should with be validated successfully", func() {
			err := kubernetes.ValidateConfigWithAllowList(config, nil)
			Expect(err).ToNot(HaveOccurred())
		})
		It("A Kubeconfig with valid fields should with be validated successfully", func() {
			config.AuthInfos["test"].Token = "test"

			err := kubernetes.ValidateConfigWithAllowList(config, nil)
			Expect(err).ToNot(HaveOccurred())
		})
		for _, field := range fieldsToTest {
			It(fmt.Sprintf("A kubeconfig with %s should be validated properly", field), func() {
				var allowedFields []string
				switch field {
				case kubernetes.AuthProvider:
					allowedFields = append(allowedFields, kubernetes.AuthProvider)
					config.AuthInfos["test"].AuthProvider = &clientcmdapi.AuthProviderConfig{Config: map[string]string{"provider": "test"}}
				case kubernetes.AuthExec:
					allowedFields = append(allowedFields, kubernetes.AuthExec)
					config.AuthInfos["test"].Exec = &clientcmdapi.ExecConfig{Command: "/bin/test"}
				case kubernetes.AuthTokenFile:
					allowedFields = append(allowedFields, kubernetes.AuthTokenFile)
					config.AuthInfos["test"].TokenFile = "test"
				case kubernetes.AuthImpersonate:
					allowedFields = append(allowedFields, kubernetes.AuthImpersonate)
					config.AuthInfos["test"].Impersonate = "test"
				case impersonateGroup:
					allowedFields = append(allowedFields, kubernetes.AuthImpersonate)
					config.AuthInfos["test"].ImpersonateGroups = []string{"test"}
				case kubernetes.AuthClientKey:
					allowedFields = append(allowedFields, kubernetes.AuthClientKey)
					config.AuthInfos["test"].ClientKey = "test"
				case kubernetes.AuthClientCertificate:
					allowedFields = append(allowedFields, kubernetes.AuthClientCertificate)
					config.AuthInfos["test"].ClientCertificate = "test"
				}
				err := kubernetes.ValidateConfigWithAllowList(config, allowedFields)
				Expect(err).ToNot(HaveOccurred())
				err = kubernetes.ValidateConfigWithAllowList(config, nil)
				Expect(err).To(HaveOccurred())
			})
		}
	})
})
