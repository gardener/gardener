// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gardener/gardener/pkg/client/kubernetes"
)

var _ = Describe("Client", func() {

	var (
		authInfo clientcmdapi.AuthInfo
		config   clientcmdapi.Config
	)

	BeforeEach(func() {
		authInfo = clientcmdapi.AuthInfo{}
		config = clientcmdapi.Config{
			AuthInfos: map[string]*clientcmdapi.AuthInfo{
				"test": &authInfo,
			},
		}
	})

	Describe("#ValidateConfig", func() {
		It("should not return an error for an empty kubeconfig", func() {
			err := kubernetes.ValidateConfig(config)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should not return an error for a kubeconfig with valid field", func() {
			config.AuthInfos["test"].Token = "test"

			err := kubernetes.ValidateConfig(config)
			Expect(err).ToNot(HaveOccurred())
		})

		DescribeTable("kubeconfig should be validated properly",
			func(config clientcmdapi.Config, matcher gomegatypes.GomegaMatcher) {
				err := kubernetes.ValidateConfig(config)

				Expect(err).To(matcher)
			},
			Entry("auth-provider",
				clientcmdapi.Config{
					AuthInfos: map[string]*clientcmdapi.AuthInfo{
						"test": {
							AuthProvider: &clientcmdapi.AuthProviderConfig{Config: map[string]string{"provider": "test"}},
						},
					},
				},
				HaveOccurred(),
			),
			Entry("exec",
				clientcmdapi.Config{
					AuthInfos: map[string]*clientcmdapi.AuthInfo{
						"test": {
							Exec: &clientcmdapi.ExecConfig{Command: "/bin/test"},
						},
					},
				},
				HaveOccurred(),
			),
			Entry("tokenFile",
				clientcmdapi.Config{
					AuthInfos: map[string]*clientcmdapi.AuthInfo{
						"test": {
							TokenFile: "test",
						},
					},
				},
				HaveOccurred(),
			),
			Entry("act-as",
				clientcmdapi.Config{
					AuthInfos: map[string]*clientcmdapi.AuthInfo{
						"test": {
							Impersonate: "test",
						},
					},
				},
				HaveOccurred(),
			),
			Entry("act-as-group",
				clientcmdapi.Config{
					AuthInfos: map[string]*clientcmdapi.AuthInfo{
						"test": {
							ImpersonateGroups: []string{"test"},
						},
					},
				},
				HaveOccurred(),
			),
			Entry("client-key",
				clientcmdapi.Config{
					AuthInfos: map[string]*clientcmdapi.AuthInfo{
						"test": {
							ClientKey: "test",
						},
					},
				},
				HaveOccurred(),
			),
			Entry("client-certificate",
				clientcmdapi.Config{
					AuthInfos: map[string]*clientcmdapi.AuthInfo{
						"test": {
							ClientCertificate: "test",
						},
					},
				},
				HaveOccurred(),
			),
		)
	})

	Describe("#ValidateConfigWithAllowList", func() {
		It("should not return an error for an empty kubeconfig", func() {
			err := kubernetes.ValidateConfigWithAllowList(config, nil)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should not return an error for a kubeconfig with valid field", func() {
			config.AuthInfos["test"].Token = "test"

			err := kubernetes.ValidateConfigWithAllowList(config, nil)
			Expect(err).ToNot(HaveOccurred())
		})

		DescribeTable("kubeconfig should be validated properly",
			func(config clientcmdapi.Config, allowedFields []string, matcher gomegatypes.GomegaMatcher) {
				err := kubernetes.ValidateConfigWithAllowList(config, allowedFields)

				Expect(err).To(matcher)
			},
			Entry("reject auth-provider",
				clientcmdapi.Config{
					AuthInfos: map[string]*clientcmdapi.AuthInfo{
						"test": {
							AuthProvider: &clientcmdapi.AuthProviderConfig{Config: map[string]string{"provider": "test"}},
						},
					},
				},
				nil,
				HaveOccurred(),
			),
			Entry("accept auth-provider",
				clientcmdapi.Config{
					AuthInfos: map[string]*clientcmdapi.AuthInfo{
						"test": {
							AuthProvider: &clientcmdapi.AuthProviderConfig{Config: map[string]string{"provider": "test"}},
						},
					},
				},
				[]string{kubernetes.AuthProvider},
				Not(HaveOccurred()),
			),
			Entry("reject exec",
				clientcmdapi.Config{
					AuthInfos: map[string]*clientcmdapi.AuthInfo{
						"test": {
							Exec: &clientcmdapi.ExecConfig{Command: "/bin/test"},
						},
					},
				},
				nil,
				HaveOccurred(),
			),
			Entry("accept exec",
				clientcmdapi.Config{
					AuthInfos: map[string]*clientcmdapi.AuthInfo{
						"test": {
							Exec: &clientcmdapi.ExecConfig{Command: "/bin/test"},
						},
					},
				},
				[]string{kubernetes.AuthExec},
				Not(HaveOccurred()),
			),
			Entry("reject tokenFile",
				clientcmdapi.Config{
					AuthInfos: map[string]*clientcmdapi.AuthInfo{
						"test": {
							TokenFile: "test",
						},
					},
				},
				nil,
				HaveOccurred(),
			),
			Entry("accept tokenFile",
				clientcmdapi.Config{
					AuthInfos: map[string]*clientcmdapi.AuthInfo{
						"test": {
							TokenFile: "test",
						},
					},
				},
				[]string{kubernetes.AuthTokenFile},
				Not(HaveOccurred()),
			),
			Entry("reject act-as",
				clientcmdapi.Config{
					AuthInfos: map[string]*clientcmdapi.AuthInfo{
						"test": {
							Impersonate: "test",
						},
					},
				},
				nil,
				HaveOccurred(),
			),
			Entry("accept act-as",
				clientcmdapi.Config{
					AuthInfos: map[string]*clientcmdapi.AuthInfo{
						"test": {
							Impersonate: "test",
						},
					},
				},
				[]string{kubernetes.AuthImpersonate},
				Not(HaveOccurred()),
			),
			Entry("reject act-as-group",
				clientcmdapi.Config{
					AuthInfos: map[string]*clientcmdapi.AuthInfo{
						"test": {
							ImpersonateGroups: []string{"test"},
						},
					},
				},
				nil,
				HaveOccurred(),
			),
			Entry("accept act-as-group",
				clientcmdapi.Config{
					AuthInfos: map[string]*clientcmdapi.AuthInfo{
						"test": {
							ImpersonateGroups: []string{"test"},
						},
					},
				},
				[]string{kubernetes.AuthImpersonate},
				Not(HaveOccurred()),
			),
			Entry("reject client-key",
				clientcmdapi.Config{
					AuthInfos: map[string]*clientcmdapi.AuthInfo{
						"test": {
							ClientKey: "test",
						},
					},
				},
				nil,
				HaveOccurred(),
			),
			Entry("accept client-key",
				clientcmdapi.Config{
					AuthInfos: map[string]*clientcmdapi.AuthInfo{
						"test": {
							ClientKey: "test",
						},
					},
				},
				[]string{kubernetes.AuthClientKey},
				Not(HaveOccurred()),
			),
			Entry("reject client-certificate",
				clientcmdapi.Config{
					AuthInfos: map[string]*clientcmdapi.AuthInfo{
						"test": {
							ClientCertificate: "test",
						},
					},
				},
				[]string{""},
				HaveOccurred(),
			),
			Entry("accept client-certificate",
				clientcmdapi.Config{
					AuthInfos: map[string]*clientcmdapi.AuthInfo{
						"test": {
							ClientCertificate: "test",
						},
					},
				},
				[]string{kubernetes.AuthClientCertificate},
				Not(HaveOccurred()),
			),
		)
	})
})
