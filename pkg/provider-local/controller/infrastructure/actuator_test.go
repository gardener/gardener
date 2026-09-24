// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("workloadIdentityRBACObjects", func() {
	const (
		machineNamespace = "infra-shoot--local--foo"
		shootNamespace   = "shoot--local--foo"
	)

	var subject = rbacv1.Subject{
		Kind:     rbacv1.UserKind,
		APIGroup: rbacv1.SchemeGroupVersion.Group,
		Name:     "gardener.cloud:workloadidentity:garden-local:local:some-uid",
	}

	var (
		role               *rbacv1.Role
		roleBinding        *rbacv1.RoleBinding
		clusterRole        *rbacv1.ClusterRole
		clusterRoleBinding *rbacv1.ClusterRoleBinding
	)

	BeforeEach(func() {
		objects := workloadIdentityRBACObjects(machineNamespace, shootNamespace, subject)
		Expect(objects).To(HaveLen(4))
		for _, obj := range objects {
			switch o := obj.(type) {
			case *rbacv1.Role:
				role = o
			case *rbacv1.RoleBinding:
				roleBinding = o
			case *rbacv1.ClusterRole:
				clusterRole = o
			case *rbacv1.ClusterRoleBinding:
				clusterRoleBinding = o
			default:
				Fail("unexpected object type")
			}
		}
	})

	rulesFor := func(rules []rbacv1.PolicyRule, group, resource string) []string {
		for _, r := range rules {
			for _, res := range r.Resources {
				if res == resource {
					for _, g := range r.APIGroups {
						if g == group {
							return r.Verbs
						}
					}
				}
			}
		}
		return nil
	}

	It("should place the Role and RoleBinding in the machine namespace", func() {
		Expect(role.Namespace).To(Equal(machineNamespace))
		Expect(roleBinding.Namespace).To(Equal(machineNamespace))
	})

	It("should grant the machine-controller-manager the permissions it needs in the machine namespace", func() {
		Expect(rulesFor(role.Rules, "", "pods")).To(ConsistOf("create", "delete", "get", "list", "patch", "watch"))
		Expect(rulesFor(role.Rules, "", "pods/exec")).To(ConsistOf("create", "get"))
		Expect(rulesFor(role.Rules, "", "secrets")).To(ConsistOf("create", "get", "patch"))
		Expect(rulesFor(role.Rules, "", "services")).To(ConsistOf("create", "delete", "get", "patch"))
		Expect(rulesFor(role.Rules, "networking.k8s.io", "networkpolicies")).To(ConsistOf("create", "delete", "get", "patch"))
	})

	It("should grant calico IPPool access via the cluster-scoped role", func() {
		Expect(rulesFor(clusterRole.Rules, "crd.projectcalico.org", "ippools")).To(ConsistOf("create", "delete", "get", "patch"))
	})

	It("should bind every (cluster)role to the workload identity subject", func() {
		Expect(roleBinding.Subjects).To(ConsistOf(subject))
		Expect(roleBinding.RoleRef.Name).To(Equal(role.Name))
		Expect(clusterRoleBinding.Subjects).To(ConsistOf(subject))
		Expect(clusterRoleBinding.RoleRef.Name).To(Equal(clusterRole.Name))
	})

	It("should produce Delete cleanup objects matching the created cluster-scoped objects", func() {
		// Delete removes the machine namespace (cascading the Role/RoleBinding) and only the cluster-scoped objects
		// explicitly. Those must have the same identity as the ones created here.
		Expect(emptyClusterRole(shootNamespace).Name).To(Equal(clusterRole.Name))
		Expect(emptyClusterRoleBinding(shootNamespace).Name).To(Equal(clusterRoleBinding.Name))
	})

	It("should return objects implementing client.Object", func() {
		for _, obj := range workloadIdentityRBACObjects(machineNamespace, shootNamespace, subject) {
			var _ client.Object = obj
		}
	})
})
