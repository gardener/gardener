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

package nodeagent_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/nodeagent"
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
  resources:
  - secrets
  verbs:
  - get
  - list
  - watch
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
			data, err := RBACResourcesData(nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(data).To(HaveLen(7))
			Expect(string(data["clusterrole____gardener-node-agent.yaml"])).To(Equal(clusterRoleYAML))
			Expect(string(data["clusterrolebinding____gardener-node-agent.yaml"])).To(Equal(clusterRoleBindingYAML))
			Expect(string(data["role__kube-system__gardener-node-agent.yaml"])).To(Equal(roleYAML))
			Expect(string(data["rolebinding__kube-system__gardener-node-agent.yaml"])).To(Equal(roleBindingYAML))
			Expect(string(data["clusterrolebinding____system_node-bootstrapper.yaml"])).To(Equal(clusterRoleBindingNodeBootstrapperYAML))
			Expect(string(data["clusterrolebinding____system_certificates.k8s.io_certificatesigningrequests_nodeclient.yaml"])).To(Equal(clusterRoleBindingNodeClientYAML))
			Expect(string(data["clusterrolebinding____system_certificates.k8s.io_certificatesigningrequests_selfnodeclient.yaml"])).To(Equal(clusterRoleBindingSelfNodeClientYAML))
		})
	})
})
