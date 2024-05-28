// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
			clusterRoleYAML                        string
			clusterRoleBindingYAML                 string
			roleYAML                               string
			roleBindingYAML                        string
			clusterRoleBindingNodeBootstrapperYAML string
			clusterRoleBindingNodeClientYAML       string
			clusterRoleBindingSelfNodeClientYAML   string
		)

		BeforeEach(func() {
			clusterRoleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  name: gardener-node-agent
rules:
- apiGroups:
  - ""
  resources:
  - nodes
  - nodes/status
  verbs:
  - get
  - list
  - watch
  - patch
  - update
- apiGroups:
  - ""
  resources:
  - events
  verbs:
  - get
  - list
  - watch
  - create
  - patch
  - update
`

			clusterRoleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  name: gardener-node-agent
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gardener-node-agent
subjects:
- kind: ServiceAccount
  name: gardener-node-agent
  namespace: kube-system
`

			roleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  creationTimestamp: null
  name: gardener-node-agent
  namespace: kube-system
rules:
- apiGroups:
  - ""
  resourceNames:
  - gardener-node-agent
  - gardener-valitail
  - osc-secret1
  - osc-secret2
  resources:
  - secrets
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - coordination.k8s.io
  resources:
  - leases
  verbs:
  - get
  - list
  - watch
  - create
  - update
`

			roleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  creationTimestamp: null
  name: gardener-node-agent
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: gardener-node-agent
subjects:
- kind: Group
  name: system:bootstrappers
- kind: ServiceAccount
  name: gardener-node-agent
  namespace: kube-system
`

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
			dataMap, err := RBACResourcesData([]string{"osc-secret1", "osc-secret2"})
			Expect(err).NotTo(HaveOccurred())

			Expect(dataMap).To(HaveKey("data.yaml.br"))
			compressedData := dataMap["data.yaml.br"]
			data, err := test.BrotliDecompression(compressedData)
			Expect(err).NotTo(HaveOccurred())

			manifests := strings.Split(string(data), "---\n")
			Expect(manifests).To(ConsistOf(
				clusterRoleYAML,
				clusterRoleBindingYAML,
				roleYAML,
				roleBindingYAML,
				clusterRoleBindingNodeBootstrapperYAML,
				clusterRoleBindingNodeClientYAML,
				clusterRoleBindingSelfNodeClientYAML,
			))
		})
	})
})
