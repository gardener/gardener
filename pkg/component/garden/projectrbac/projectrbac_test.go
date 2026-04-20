// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package projectrbac_test

import (
	"context"
	"errors"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component/garden/projectrbac"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ProjectRBAC", func() {
	var (
		c client.Client

		project     *gardencorev1beta1.Project
		projectRBAC Interface
		err         error

		ctx         = context.TODO()
		fakeErr     = errors.New("fake error")
		projectName = "foobar"
		namespace   = "garden-" + projectName

		extensionRolePrefix = "gardener.cloud:extension:project:" + projectName + ":"
		extensionRole1      = "foo"
		extensionRole2      = "bar"

		member1 = rbacv1.Subject{Kind: rbacv1.UserKind, Name: "member1"}
		member2 = rbacv1.Subject{Kind: rbacv1.UserKind, Name: "member2"}
		member3 = rbacv1.Subject{Kind: rbacv1.UserKind, Name: "member3"}
		member4 = rbacv1.Subject{Kind: rbacv1.UserKind, Name: "member4"}

		clusterRoleProjectAdmin        *rbacv1.ClusterRole
		clusterRoleBindingProjectAdmin *rbacv1.ClusterRoleBinding

		clusterRoleProjectUAM        *rbacv1.ClusterRole
		clusterRoleBindingProjectUAM *rbacv1.ClusterRoleBinding

		roleBindingProjectServiceAccountManager *rbacv1.RoleBinding

		clusterRoleProjectMember        *rbacv1.ClusterRole
		clusterRoleBindingProjectMember *rbacv1.ClusterRoleBinding
		roleBindingProjectMember        *rbacv1.RoleBinding

		clusterRoleProjectViewer        *rbacv1.ClusterRole
		clusterRoleBindingProjectViewer *rbacv1.ClusterRoleBinding
		roleBindingProjectViewer        *rbacv1.RoleBinding

		clusterRoleProjectExtensionRole1 *rbacv1.ClusterRole
		roleBindingProjectExtensionRole1 *rbacv1.RoleBinding

		roleLabels = map[string]string{
			v1beta1constants.GardenRole:  v1beta1constants.LabelExtensionProjectRole,
			v1beta1constants.ProjectName: projectName,
		}

		emptyExtensionRoleBinding1 = rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      extensionRolePrefix + extensionRole1,
				Namespace: namespace,
				Labels:    roleLabels,
			},
		}
		emptyExtensionRoleBinding2 = rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      extensionRolePrefix + extensionRole2,
				Namespace: namespace,
				Labels:    roleLabels,
			},
		}
		emptyExtensionClusterRole1 = rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   extensionRolePrefix + extensionRole1,
				Labels: roleLabels,
			},
		}
		emptyExtensionClusterRole2 = rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   extensionRolePrefix + extensionRole2,
				Labels: roleLabels,
			},
		}
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

		project = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: projectName,
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: &namespace,
			},
		}
		projectRBAC, err = New(c, project)
		Expect(err).NotTo(HaveOccurred())

		clusterRoleProjectAdmin = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:project:" + projectName,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Project",
					Name:               projectName,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(false),
				}},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups:     []string{""},
					Resources:     []string{"namespaces"},
					ResourceNames: []string{namespace},
					Verbs:         []string{"get"},
				},
				{
					APIGroups:     []string{gardencorev1beta1.SchemeGroupVersion.Group},
					Resources:     []string{"projects"},
					ResourceNames: []string{projectName},
					Verbs:         []string{"get", "patch", "manage-members", "update", "delete"},
				},
			},
		}
		clusterRoleBindingProjectAdmin = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:project:" + projectName,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Project",
					Name:               projectName,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(false),
				}},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:system:project:" + projectName,
			},
		}

		clusterRoleProjectUAM = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:project-uam:" + projectName,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Project",
					Name:               projectName,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(false),
				}},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups:     []string{gardencorev1beta1.SchemeGroupVersion.Group},
					Resources:     []string{"projects"},
					ResourceNames: []string{projectName},
					Verbs:         []string{"get", "manage-members", "patch", "update"},
				},
			},
		}
		clusterRoleBindingProjectUAM = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:project-uam:" + projectName,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Project",
					Name:               projectName,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(false),
				}},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:system:project-uam:" + projectName,
			},
		}

		roleBindingProjectServiceAccountManager = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:system:project-serviceaccountmanager",
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Project",
					Name:               projectName,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(false),
				}},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:system:project-serviceaccountmanager",
			},
		}

		clusterRoleProjectMember = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:project-member:" + projectName,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Project",
					Name:               projectName,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(false),
				}},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups:     []string{""},
					Resources:     []string{"namespaces"},
					ResourceNames: []string{namespace},
					Verbs:         []string{"get"},
				},
				{
					APIGroups:     []string{gardencorev1beta1.SchemeGroupVersion.Group},
					Resources:     []string{"projects"},
					ResourceNames: []string{projectName},
					Verbs:         []string{"get", "patch", "update", "delete"},
				},
			},
		}
		clusterRoleBindingProjectMember = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:project-member:" + projectName,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Project",
					Name:               projectName,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(false),
				}},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:system:project-member:" + projectName,
			},
		}
		roleBindingProjectMember = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:system:project-member",
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Project",
					Name:               projectName,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(false),
				}},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:system:project-member",
			},
		}

		clusterRoleProjectViewer = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:project-viewer:" + projectName,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Project",
					Name:               projectName,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(false),
				}},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups:     []string{""},
					Resources:     []string{"namespaces"},
					ResourceNames: []string{namespace},
					Verbs:         []string{"get"},
				},
				{
					APIGroups:     []string{gardencorev1beta1.SchemeGroupVersion.Group},
					Resources:     []string{"projects"},
					ResourceNames: []string{projectName},
					Verbs:         []string{"get"},
				},
			},
		}
		clusterRoleBindingProjectViewer = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:project-viewer:" + projectName,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Project",
					Name:               projectName,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(false),
				}},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:system:project-viewer:" + projectName,
			},
		}
		roleBindingProjectViewer = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:system:project-viewer",
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Project",
					Name:               projectName,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(false),
				}},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:system:project-viewer",
			},
		}

		clusterRoleProjectExtensionRole1 = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: extensionRolePrefix + extensionRole1,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Project",
					Name:               projectName,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(false),
				}},
				Labels: map[string]string{
					"gardener.cloud/role":         "extension-project-role",
					"project.gardener.cloud/name": projectName,
				},
			},
			AggregationRule: &rbacv1.AggregationRule{
				ClusterRoleSelectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{"rbac.gardener.cloud/aggregate-to-extension-role": extensionRole1}},
				},
			},
		}
		roleBindingProjectExtensionRole1 = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      extensionRolePrefix + extensionRole1,
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "core.gardener.cloud/v1beta1",
					Kind:               "Project",
					Name:               projectName,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(false),
				}},
				Labels: map[string]string{
					"gardener.cloud/role":         "extension-project-role",
					"project.gardener.cloud/name": projectName,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     extensionRolePrefix + extensionRole1,
			},
		}
	})

	Describe("#Deploy", func() {
		It("should successfully reconcile all project RBAC resources", func() {
			project.Spec.Owner = &member3
			project.Spec.Members = []gardencorev1beta1.ProjectMember{
				{
					Subject: member1,
					Role:    "extension:" + extensionRole1,
					Roles:   []string{"viewer"},
				},
				{
					Subject: member2,
					Roles:   []string{"admin", "uam"},
				},
				{
					Subject: member3,
					Roles:   []string{"owner", "viewer", "admin"},
				},
				{
					Subject: member4,
					Roles:   []string{"serviceaccountmanager"},
				},
			}

			clusterRoleBindingProjectAdmin.Subjects = []rbacv1.Subject{member3}
			clusterRoleBindingProjectUAM.Subjects = []rbacv1.Subject{member2}
			roleBindingProjectServiceAccountManager.Subjects = []rbacv1.Subject{member3, member4}
			clusterRoleBindingProjectMember.Subjects = []rbacv1.Subject{member2, member3}
			roleBindingProjectMember.Subjects = []rbacv1.Subject{member2, member3}
			clusterRoleBindingProjectViewer.Subjects = []rbacv1.Subject{member1, member3}
			roleBindingProjectViewer.Subjects = []rbacv1.Subject{member1, member3}
			roleBindingProjectExtensionRole1.Subjects = []rbacv1.Subject{member1}

			Expect(projectRBAC.Deploy(ctx)).To(Succeed())

			// project admin
			actualCR := &rbacv1.ClusterRole{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(clusterRoleProjectAdmin), actualCR)).To(Succeed())
			Expect(actualCR.Rules).To(Equal(clusterRoleProjectAdmin.Rules))
			actualCRB := &rbacv1.ClusterRoleBinding{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(clusterRoleBindingProjectAdmin), actualCRB)).To(Succeed())
			Expect(actualCRB.Subjects).To(Equal(clusterRoleBindingProjectAdmin.Subjects))

			// project uam
			Expect(c.Get(ctx, client.ObjectKeyFromObject(clusterRoleProjectUAM), actualCR)).To(Succeed())
			Expect(actualCR.Rules).To(Equal(clusterRoleProjectUAM.Rules))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(clusterRoleBindingProjectUAM), actualCRB)).To(Succeed())
			Expect(actualCRB.Subjects).To(Equal(clusterRoleBindingProjectUAM.Subjects))

			// project service account manager
			actualRB := &rbacv1.RoleBinding{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(roleBindingProjectServiceAccountManager), actualRB)).To(Succeed())
			Expect(actualRB.Subjects).To(Equal(roleBindingProjectServiceAccountManager.Subjects))

			// project member
			Expect(c.Get(ctx, client.ObjectKeyFromObject(clusterRoleProjectMember), actualCR)).To(Succeed())
			Expect(actualCR.Rules).To(Equal(clusterRoleProjectMember.Rules))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(clusterRoleBindingProjectMember), actualCRB)).To(Succeed())
			Expect(actualCRB.Subjects).To(Equal(clusterRoleBindingProjectMember.Subjects))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(roleBindingProjectMember), actualRB)).To(Succeed())
			Expect(actualRB.Subjects).To(Equal(roleBindingProjectMember.Subjects))

			// project viewer
			Expect(c.Get(ctx, client.ObjectKeyFromObject(clusterRoleProjectViewer), actualCR)).To(Succeed())
			Expect(actualCR.Rules).To(Equal(clusterRoleProjectViewer.Rules))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(clusterRoleBindingProjectViewer), actualCRB)).To(Succeed())
			Expect(actualCRB.Subjects).To(Equal(clusterRoleBindingProjectViewer.Subjects))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(roleBindingProjectViewer), actualRB)).To(Succeed())
			Expect(actualRB.Subjects).To(Equal(roleBindingProjectViewer.Subjects))

			// project extension roles
			Expect(c.Get(ctx, client.ObjectKeyFromObject(clusterRoleProjectExtensionRole1), actualCR)).To(Succeed())
			Expect(actualCR.AggregationRule).To(Equal(clusterRoleProjectExtensionRole1.AggregationRule))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(roleBindingProjectExtensionRole1), actualRB)).To(Succeed())
			Expect(actualRB.Subjects).To(Equal(roleBindingProjectExtensionRole1.Subjects))
		})
	})

	Describe("#Destroy", func() {
		It("should successfully delete all project RBAC resources", func() {
			project.Spec.Members = []gardencorev1beta1.ProjectMember{{Role: "extension:" + extensionRole1}}

			Expect(projectRBAC.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(clusterRoleProjectAdmin), &rbacv1.ClusterRole{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(clusterRoleBindingProjectAdmin), &rbacv1.ClusterRoleBinding{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(clusterRoleProjectUAM), &rbacv1.ClusterRole{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(clusterRoleBindingProjectUAM), &rbacv1.ClusterRoleBinding{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(roleBindingProjectServiceAccountManager), &rbacv1.RoleBinding{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(clusterRoleProjectMember), &rbacv1.ClusterRole{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(clusterRoleBindingProjectMember), &rbacv1.ClusterRoleBinding{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(roleBindingProjectMember), &rbacv1.RoleBinding{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(clusterRoleProjectViewer), &rbacv1.ClusterRole{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(clusterRoleBindingProjectViewer), &rbacv1.ClusterRoleBinding{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(roleBindingProjectViewer), &rbacv1.RoleBinding{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(roleBindingProjectExtensionRole1), &rbacv1.RoleBinding{})).To(Succeed())

			Expect(projectRBAC.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(clusterRoleProjectAdmin), &rbacv1.ClusterRole{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(clusterRoleBindingProjectAdmin), &rbacv1.ClusterRoleBinding{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(clusterRoleProjectUAM), &rbacv1.ClusterRole{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(clusterRoleBindingProjectUAM), &rbacv1.ClusterRoleBinding{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(roleBindingProjectServiceAccountManager), &rbacv1.RoleBinding{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(clusterRoleProjectMember), &rbacv1.ClusterRole{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(clusterRoleBindingProjectMember), &rbacv1.ClusterRoleBinding{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(roleBindingProjectMember), &rbacv1.RoleBinding{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(clusterRoleProjectViewer), &rbacv1.ClusterRole{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(clusterRoleBindingProjectViewer), &rbacv1.ClusterRoleBinding{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(roleBindingProjectViewer), &rbacv1.RoleBinding{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(roleBindingProjectExtensionRole1), &rbacv1.RoleBinding{})).To(BeNotFoundError())
		})
	})

	Describe("#DeleteStaleExtensionRolesResources", func() {
		It("should do nothing because neither rolebindings nor clusterroles exist", func() {
			Expect(projectRBAC.DeleteStaleExtensionRolesResources(ctx)).To(Succeed())
		})

		It("should return an error because listing the rolebindings failed", func() {
			fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
				List: func(ctx context.Context, cl client.WithWatch, obj client.ObjectList, opts ...client.ListOption) error {
					if _, ok := obj.(*rbacv1.RoleBindingList); ok {
						return fakeErr
					}
					return cl.List(ctx, obj, opts...)
				},
			}).Build()
			projectRBAC, err = New(fakeClient, project)
			Expect(err).NotTo(HaveOccurred())

			Expect(projectRBAC.DeleteStaleExtensionRolesResources(ctx)).To(MatchError(fakeErr))
		})

		It("should return an error because listing the clusterroles failed", func() {
			fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
				List: func(ctx context.Context, cl client.WithWatch, obj client.ObjectList, opts ...client.ListOption) error {
					if _, ok := obj.(*rbacv1.ClusterRoleList); ok {
						return fakeErr
					}
					return cl.List(ctx, obj, opts...)
				},
			}).Build()
			projectRBAC, err = New(fakeClient, project)
			Expect(err).NotTo(HaveOccurred())

			Expect(projectRBAC.DeleteStaleExtensionRolesResources(ctx)).To(MatchError(fakeErr))
		})

		It("should return an error because deleting a stale rolebinding failed", func() {
			fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
				Delete: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.DeleteOption) error {
					return fakeErr
				},
			}).Build()

			// Create stale rolebinding with the right labels
			rb1 := emptyExtensionRoleBinding1.DeepCopy()
			Expect(fakeClient.Create(ctx, rb1)).To(Succeed())

			projectRBAC, err = New(fakeClient, project)
			Expect(err).NotTo(HaveOccurred())

			Expect(projectRBAC.DeleteStaleExtensionRolesResources(ctx)).To(MatchError(fakeErr))
		})

		It("should return an error because deleting a stale clusterrole failed", func() {
			fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
				List: func(ctx context.Context, cl client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
					// The fake client filters cluster-scoped resources by namespace (unlike the real
					// API server which ignores InNamespace for cluster-scoped resources). Strip the
					// InNamespace option so that ClusterRoles are returned by the list call.
					if _, ok := list.(*rbacv1.ClusterRoleList); ok {
						filteredOpts := slices.DeleteFunc(opts, func(opt client.ListOption) bool {
							_, isInNamespace := opt.(client.InNamespace)
							return isInNamespace
						})

						return cl.List(ctx, list, filteredOpts...)
					}
					return cl.List(ctx, list, opts...)
				},
				Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
					// The rolebinding deletes succeed, but the first clusterrole delete fails
					if _, ok := obj.(*rbacv1.ClusterRole); ok {
						return fakeErr
					}
					return c.Delete(ctx, obj, opts...)
				},
			}).Build()

			// Create stale rolebindings
			rb1 := emptyExtensionRoleBinding1.DeepCopy()
			Expect(fakeClient.Create(ctx, rb1)).To(Succeed())
			rb2 := emptyExtensionRoleBinding2.DeepCopy()
			Expect(fakeClient.Create(ctx, rb2)).To(Succeed())

			// Create stale clusterroles
			cr1 := emptyExtensionClusterRole1.DeepCopy()
			Expect(fakeClient.Create(ctx, cr1)).To(Succeed())
			cr2 := emptyExtensionClusterRole2.DeepCopy()
			Expect(fakeClient.Create(ctx, cr2)).To(Succeed())

			projectRBAC, err = New(fakeClient, project)
			Expect(err).NotTo(HaveOccurred())

			Expect(projectRBAC.DeleteStaleExtensionRolesResources(ctx)).To(MatchError(fakeErr))
		})

		It("should succeed deleting the stale rolebindings and clusterroles", func() {
			project.Spec.Members = []gardencorev1beta1.ProjectMember{{Role: "extension:" + extensionRole1}}

			rb1 := emptyExtensionRoleBinding1.DeepCopy()
			Expect(c.Create(ctx, rb1)).To(Succeed())
			rb2 := emptyExtensionRoleBinding2.DeepCopy()
			Expect(c.Create(ctx, rb2)).To(Succeed())

			cr1 := emptyExtensionClusterRole1.DeepCopy()
			Expect(c.Create(ctx, cr1)).To(Succeed())
			cr2 := emptyExtensionClusterRole2.DeepCopy()
			Expect(c.Create(ctx, cr2)).To(Succeed())

			Expect(projectRBAC.DeleteStaleExtensionRolesResources(ctx)).To(Succeed())

			// Verify role1 still exists (not stale), role2 was deleted (stale)
			Expect(c.Get(ctx, client.ObjectKeyFromObject(rb1), &rbacv1.RoleBinding{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(rb2), &rbacv1.RoleBinding{})).To(BeNotFoundError())
		})
	})
})
