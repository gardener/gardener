// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagent_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/nodeagent"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("RBAC", func() {
	Describe("#RBACResourcesData", func() {
		var (
			clusterRoleBindingNodeBootstrapperYAML string
			clusterRoleBindingNodeClientYAML       string
			clusterRoleBindingSelfNodeClientYAML   string
		)

		BeforeEach(func() {
			clusterRoleBindingNodeBootstrapperYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  name: system:node-bootstrapper
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:node-bootstrapper
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:bootstrappers
`

			clusterRoleBindingNodeClientYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  name: system:certificates.k8s.io:certificatesigningrequests:nodeclient
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:certificates.k8s.io:certificatesigningrequests:nodeclient
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:bootstrappers
`
			clusterRoleBindingSelfNodeClientYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  name: system:certificates.k8s.io:certificatesigningrequests:selfnodeclient
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:certificates.k8s.io:certificatesigningrequests:selfnodeclient
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:nodes
`
		})

		It("should generate the expected RBAC resources", func() {
			dataMap, err := RBACResourcesData()
			Expect(err).NotTo(HaveOccurred())

			Expect(dataMap).To(HaveKey("data.yaml.br"))
			compressedData := dataMap["data.yaml.br"]
			data, err := test.BrotliDecompression(compressedData)
			Expect(err).NotTo(HaveOccurred())

			manifests := strings.Split(string(data), "---\n")
			Expect(manifests).To(ConsistOf(
				clusterRoleBindingNodeBootstrapperYAML,
				clusterRoleBindingNodeClientYAML,
				clusterRoleBindingSelfNodeClientYAML,
			))
		})
	})
})
