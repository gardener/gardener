// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package virtual_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/garden/system/virtual"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Virtual", func() {
	var (
		ctx = context.Background()

		managedResourceName = "garden-system-virtual"
		namespace           = "some-namespace"

		c         client.Client
		component component.DeployWaiter
		values    Values
		consistOf func(...client.Object) types.GomegaMatcher

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret

		namespaceGarden                                   *corev1.Namespace
		clusterRoleSeedBootstrapper                       *rbacv1.ClusterRole
		clusterRoleBindingSeedBootstrapper                *rbacv1.ClusterRoleBinding
		clusterRoleSeeds                                  *rbacv1.ClusterRole
		clusterRoleBindingSeeds                           *rbacv1.ClusterRoleBinding
		clusterRoleGardenerAdmin                          *rbacv1.ClusterRole
		clusterRoleBindingGardenerAdmin                   *rbacv1.ClusterRoleBinding
		clusterRoleGardenerAdminAggregated                *rbacv1.ClusterRole
		clusterRoleGardenerViewer                         *rbacv1.ClusterRole
		clusterRoleGardenerViewerAggregated               *rbacv1.ClusterRole
		clusterRoleReadGlobalResources                    *rbacv1.ClusterRole
		clusterRoleBindingReadGlobalResources             *rbacv1.ClusterRoleBinding
		clusterRoleUserAuth                               *rbacv1.ClusterRole
		clusterRoleBindingUserAuth                        *rbacv1.ClusterRoleBinding
		clusterRoleProjectCreation                        *rbacv1.ClusterRole
		clusterRoleProjectMember                          *rbacv1.ClusterRole
		clusterRoleProjectMemberAggregated                *rbacv1.ClusterRole
		clusterRoleProjectServiceAccountManager           *rbacv1.ClusterRole
		clusterRoleProjectServiceAccountManagerAggregated *rbacv1.ClusterRole
		clusterRoleProjectViewer                          *rbacv1.ClusterRole
		clusterRoleProjectViewerAggregated                *rbacv1.ClusterRole
		roleReadClusterIdentityConfigMap                  *rbacv1.Role
		roleBindingReadClusterIdentityConfigMap           *rbacv1.RoleBinding
		roleReadGardenerInfoConfigMap                     *rbacv1.Role
		roleBindingReadGardenerInfoConfigMap              *rbacv1.RoleBinding
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()
		values = Values{}
		component = New(c, namespace, values)
		consistOf = NewManagedResourceConsistOfObjectsMatcher(c)

		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: namespace,
			},
		}
		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResource.Name,
				Namespace: namespace,
			},
		}

		namespaceGarden = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "garden",
				Labels:      map[string]string{"app": "gardener"},
				Annotations: map[string]string{"resources.gardener.cloud/keep-object": "true"},
			},
		}
		clusterRoleSeedBootstrapper = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:seed-bootstrapper",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"certificates.k8s.io"},
					Resources: []string{"certificatesigningrequests"},
					Verbs:     []string{"create", "get"},
				},
				{
					APIGroups: []string{"certificates.k8s.io"},
					Resources: []string{"certificatesigningrequests/seedclient"},
					Verbs:     []string{"create"},
				},
			},
		}
		clusterRoleBindingSeedBootstrapper = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:seed-bootstrapper",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:system:seed-bootstrapper",
			},
			Subjects: []rbacv1.Subject{{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Group",
				Name:     "system:bootstrappers",
			}},
		}
		clusterRoleSeeds = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:seeds",
			},
			Rules: []rbacv1.PolicyRule{{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			}},
		}
		clusterRoleBindingSeeds = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:seeds",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:system:seeds",
			},
			Subjects: []rbacv1.Subject{{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Group",
				Name:     "gardener.cloud:system:seeds",
			}},
		}
		clusterRoleGardenerAdmin = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "gardener.cloud:admin",
				Labels: map[string]string{"gardener.cloud/role": "admin"},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{
						"core.gardener.cloud",
						"seedmanagement.gardener.cloud",
						"dashboard.gardener.cloud",
						"settings.gardener.cloud",
						"operations.gardener.cloud",
					},
					Resources: []string{"*"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{"security.gardener.cloud"},
					Resources: []string{
						"credentialsbindings",
						"workloadidentities"},
					Verbs: []string{"*"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"events", "namespaces", "resourcequotas"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{"events.k8s.io"},
					Resources: []string{"events"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{"rbac.authorization.k8s.io"},
					Resources: []string{"clusterroles", "clusterrolebindings", "roles", "rolebindings"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{"admissionregistration.k8s.io"},
					Resources: []string{"mutatingwebhookconfigurations", "validatingwebhookconfigurations"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{"apiregistration.k8s.io"},
					Resources: []string{"apiservices"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{"apiextensions.k8s.io"},
					Resources: []string{"customresourcedefinitions"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{"coordination.k8s.io"},
					Resources: []string{"leases"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{"certificates.k8s.io"},
					Resources: []string{"certificatesigningrequests"},
					Verbs:     []string{"*"},
				},
			},
		}
		clusterRoleBindingGardenerAdmin = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:admin",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:admin",
			},
			Subjects: []rbacv1.Subject{{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "User",
				Name:     "system:kube-aggregator",
			}},
		}
		clusterRoleGardenerAdminAggregated = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:administrators",
			},
			AggregationRule: &rbacv1.AggregationRule{
				ClusterRoleSelectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{"gardener.cloud/role": "admin"}},
					{MatchLabels: map[string]string{"gardener.cloud/role": "project-member"}},
					{MatchLabels: map[string]string{"gardener.cloud/role": "project-serviceaccountmanager"}},
				},
			},
		}
		clusterRoleGardenerViewer = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "gardener.cloud:viewer",
				Labels: map[string]string{"gardener.cloud/role": "viewer"},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"core.gardener.cloud"},
					Resources: []string{
						"backupbuckets",
						"backupentries",
						"cloudprofiles",
						"namespacedcloudprofiles",
						"controllerinstallations",
						"quotas",
						"projects",
						"seeds",
						"shoots",
					},
					Verbs: []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"security.gardener.cloud"},
					Resources: []string{"credentialsbindings"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{
						"seedmanagement.gardener.cloud",
						"dashboard.gardener.cloud",
						"settings.gardener.cloud",
						"operations.gardener.cloud",
					},
					Resources: []string{"*"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"events", "namespaces", "resourcequotas"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"events.k8s.io"},
					Resources: []string{"events"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"rbac.authorization.k8s.io"},
					Resources: []string{"clusterroles", "clusterrolebindings", "roles", "rolebindings"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"admissionregistration.k8s.io"},
					Resources: []string{"mutatingwebhookconfigurations", "validatingwebhookconfigurations"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"apiregistration.k8s.io"},
					Resources: []string{"apiservices"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"apiextensions.k8s.io"},
					Resources: []string{"customresourcedefinitions"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"coordination.k8s.io"},
					Resources: []string{"leases"},
					Verbs:     []string{"get", "list", "watch"},
				},
			},
		}
		clusterRoleGardenerViewerAggregated = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:viewers",
			},
			AggregationRule: &rbacv1.AggregationRule{
				ClusterRoleSelectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{"gardener.cloud/role": "viewer"}},
					{MatchLabels: map[string]string{"gardener.cloud/role": "project-viewer"}},
				},
			},
		}
		clusterRoleReadGlobalResources = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:read-global-resources",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"core.gardener.cloud"},
					Resources: []string{
						"cloudprofiles",
						"exposureclasses",
						"seeds",
					},
					Verbs: []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"apiextensions.k8s.io"},
					Resources: []string{"customresourcedefinitions"},
					Verbs:     []string{"get", "list", "watch"},
				},
			},
		}
		clusterRoleBindingReadGlobalResources = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:read-global-resources",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:system:read-global-resources",
			},
			Subjects: []rbacv1.Subject{{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Group",
				Name:     "system:authenticated",
			}},
		}
		clusterRoleUserAuth = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:user-auth",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"authentication.k8s.io"},
					Resources: []string{"tokenreviews"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups: []string{"authorization.k8s.io"},
					Resources: []string{"selfsubjectaccessreviews"},
					Verbs:     []string{"create"},
				},
			},
		}
		clusterRoleBindingUserAuth = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:user-auth",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:system:user-auth",
			},
			Subjects: []rbacv1.Subject{{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Group",
				Name:     "system:authenticated",
			}},
		}
		clusterRoleProjectCreation = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:project-creation",
			},
			Rules: []rbacv1.PolicyRule{{
				APIGroups: []string{"core.gardener.cloud"},
				Resources: []string{"projects"},
				Verbs:     []string{"create"},
			}},
		}
		clusterRoleProjectMember = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "gardener.cloud:system:project-member-aggregation",
				Labels: map[string]string{"rbac.gardener.cloud/aggregate-to-project-member": "true"},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{
						"secrets",
						"configmaps",
					},
					Verbs: []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{
						"events",
						"resourcequotas",
						"serviceaccounts",
					},
					Verbs: []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"events.k8s.io"},
					Resources: []string{"events"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"core.gardener.cloud"},
					Resources: []string{
						"shoots",
						"secretbindings",
						"quotas",
					},
					Verbs: []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{"security.gardener.cloud"},
					Resources: []string{
						"credentialsbindings",
						"workloadidentities",
					},
					Verbs: []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{"settings.gardener.cloud"},
					Resources: []string{"openidconnectpresets"},
					Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{"operations.gardener.cloud"},
					Resources: []string{"bastions"},
					Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{"rbac.authorization.k8s.io"},
					Resources: []string{
						"roles",
						"rolebindings",
					},
					Verbs: []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{"core.gardener.cloud"},
					Resources: []string{
						"shoots/adminkubeconfig",
						"shoots/viewerkubeconfig",
					},
					Verbs: []string{"create"},
				},
				{
					APIGroups: []string{"core.gardener.cloud"},
					Resources: []string{"namespacedcloudprofiles"},
					Verbs:     []string{"get", "list", "watch", "create", "patch", "update", "delete"},
				},
			},
		}
		clusterRoleProjectMemberAggregated = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "gardener.cloud:system:project-member",
				Labels: map[string]string{"gardener.cloud/role": "project-member"},
			},
			AggregationRule: &rbacv1.AggregationRule{
				ClusterRoleSelectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{"rbac.gardener.cloud/aggregate-to-project-member": "true"}},
				},
			},
		}
		clusterRoleProjectServiceAccountManager = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "gardener.cloud:system:project-serviceaccountmanager-aggregation",
				Labels: map[string]string{"rbac.gardener.cloud/aggregate-to-project-serviceaccountmanager": "true"},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"serviceaccounts"},
					Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"serviceaccounts/token"},
					Verbs:     []string{"create"},
				},
			},
		}
		clusterRoleProjectServiceAccountManagerAggregated = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "gardener.cloud:system:project-serviceaccountmanager",
				Labels: map[string]string{"gardener.cloud/role": "project-serviceaccountmanager"},
			},
			AggregationRule: &rbacv1.AggregationRule{
				ClusterRoleSelectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{"rbac.gardener.cloud/aggregate-to-project-serviceaccountmanager": "true"}},
				},
			},
		}
		clusterRoleProjectViewer = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "gardener.cloud:system:project-viewer-aggregation",
				Labels: map[string]string{"rbac.gardener.cloud/aggregate-to-project-viewer": "true"},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{
						"events",
						"configmaps",
						"resourcequotas",
						"serviceaccounts",
					},
					Verbs: []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"events.k8s.io"},
					Resources: []string{"events"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"core.gardener.cloud"},
					Resources: []string{
						"shoots",
						"secretbindings",
						"quotas",
						"namespacedcloudprofiles",
					},
					Verbs: []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"security.gardener.cloud"},
					Resources: []string{"credentialsbindings"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"settings.gardener.cloud"},
					Resources: []string{"openidconnectpresets"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"operations.gardener.cloud"},
					Resources: []string{"bastions"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"rbac.authorization.k8s.io"},
					Resources: []string{
						"roles",
						"rolebindings",
					},
					Verbs: []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"core.gardener.cloud"},
					Resources: []string{"shoots/viewerkubeconfig"},
					Verbs:     []string{"create"},
				},
			},
		}
		clusterRoleProjectViewerAggregated = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "gardener.cloud:system:project-viewer",
				Labels: map[string]string{"gardener.cloud/role": "project-viewer"},
			},
			AggregationRule: &rbacv1.AggregationRule{
				ClusterRoleSelectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{"rbac.gardener.cloud/aggregate-to-project-viewer": "true"}},
				},
			},
		}
		roleReadClusterIdentityConfigMap = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:system:read-cluster-identity-configmap",
				Namespace: "kube-system",
			},
			Rules: []rbacv1.PolicyRule{{
				APIGroups:     []string{""},
				Resources:     []string{"configmaps"},
				ResourceNames: []string{"cluster-identity"},
				Verbs:         []string{"get", "list", "watch"},
			}},
		}
		roleBindingReadClusterIdentityConfigMap = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:system:read-cluster-identity-configmap",
				Namespace: "kube-system",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "gardener.cloud:system:read-cluster-identity-configmap",
			},
			Subjects: []rbacv1.Subject{{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Group",
				Name:     "system:authenticated",
			}},
		}
		roleReadGardenerInfoConfigMap = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:system:read-gardener-info-configmap",
				Namespace: "gardener-system-info",
			},
			Rules: []rbacv1.PolicyRule{{
				APIGroups:     []string{""},
				Resources:     []string{"configmaps"},
				ResourceNames: []string{"gardener-info"},
				Verbs:         []string{"get", "watch"},
			}},
		}
		roleBindingReadGardenerInfoConfigMap = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:system:read-gardener-info-configmap",
				Namespace: "gardener-system-info",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "gardener.cloud:system:read-gardener-info-configmap",
			},
			Subjects: []rbacv1.Subject{{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Group",
				Name:     "system:authenticated",
			}},
		}
	})

	Describe("#Deploy", func() {
		JustBeforeEach(func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())

			Expect(component.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResource.Name,
					Namespace:       managedResource.Namespace,
					ResourceVersion: "1",
					Labels: map[string]string{
						"origin":                             "gardener",
						"care.gardener.cloud/condition-type": "VirtualComponentsHealthy",
					},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
					SecretRefs:   []corev1.LocalObjectReference{{Name: managedResource.Spec.SecretRefs[0].Name}},
					KeepObjects:  ptr.To(false),
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedMr))
			Expect(managedResource).To(DeepEqual(expectedMr))

			managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
		})

		It("should successfully deploy the resources when seed authorizer is disabled", func() {
			Expect(managedResource).To(consistOf(
				namespaceGarden,
				clusterRoleSeedBootstrapper,
				clusterRoleBindingSeedBootstrapper,
				clusterRoleSeeds,
				clusterRoleBindingSeeds,
				clusterRoleGardenerAdmin,
				clusterRoleBindingGardenerAdmin,
				clusterRoleGardenerAdminAggregated,
				clusterRoleGardenerViewer,
				clusterRoleGardenerViewerAggregated,
				clusterRoleReadGlobalResources,
				clusterRoleBindingReadGlobalResources,
				clusterRoleUserAuth,
				clusterRoleBindingUserAuth,
				clusterRoleProjectCreation,
				clusterRoleProjectMemberAggregated,
				clusterRoleProjectMember,
				clusterRoleProjectServiceAccountManagerAggregated,
				clusterRoleProjectServiceAccountManager,
				clusterRoleProjectViewerAggregated,
				clusterRoleProjectViewer,
				roleReadClusterIdentityConfigMap,
				roleBindingReadClusterIdentityConfigMap,
				roleReadGardenerInfoConfigMap,
				roleBindingReadGardenerInfoConfigMap,
			))
		})

		Context("when seed authorizer is enabled", func() {
			BeforeEach(func() {
				values.SeedAuthorizerEnabled = true
				component = New(c, namespace, values)
			})

			It("should successfully deploy the resources when seed authorizer is enabled", func() {
				Expect(managedResourceSecret.Data).NotTo(HaveKey("clusterrole____gardener.cloud_system_seeds.yaml"))
				Expect(managedResourceSecret.Data).NotTo(HaveKey("clusterrolebinding____gardener.cloud_system_seeds.yaml"))
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

			Expect(component.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
		})
	})

	Context("waiting functions", func() {
		var fakeOps *retryfake.Ops

		BeforeEach(func() {
			fakeOps = &retryfake.Ops{MaxAttempts: 1}
			DeferCleanup(test.WithVars(
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			))
		})

		Describe("#Wait", func() {
			It("should fail because reading the ManagedResource fails", func() {
				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the ManagedResource doesn't become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionFalse,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should successfully wait for the managed resource to become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})).To(Succeed())

				Expect(component.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, managedResource)).To(Succeed())

				Expect(component.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it's already removed", func() {
				Expect(component.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})
