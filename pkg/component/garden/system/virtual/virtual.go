// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package virtual

import (
	"context"
	"time"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	settingsv1alpha1 "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

// ManagedResourceName is the name of the ManagedResource containing the resource specifications.
const ManagedResourceName = "garden-system-virtual"

// New creates a new instance of DeployWaiter for virtual garden system resources.
func New(client client.Client, namespace string, values Values) component.DeployWaiter {
	return &gardenSystem{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type gardenSystem struct {
	client    client.Client
	namespace string
	values    Values
}

// Values contains values for the system resources.
type Values struct {
	// SeedAuthorizerEnabled determines whether the seed authorizer is enabled.
	SeedAuthorizerEnabled bool
}

func (g *gardenSystem) Deploy(ctx context.Context) error {
	data, err := g.computeResourcesData()
	if err != nil {
		return err
	}

	return managedresources.CreateForShootWithLabels(ctx, g.client, g.namespace, ManagedResourceName, managedresources.LabelValueGardener, false, map[string]string{v1beta1constants.LabelCareConditionType: string(operatorv1alpha1.VirtualComponentsHealthy)}, data)
}

func (g *gardenSystem) Destroy(ctx context.Context) error {
	return managedresources.DeleteForShoot(ctx, g.client, g.namespace, ManagedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (g *gardenSystem) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, g.client, g.namespace, ManagedResourceName)
}

func (g *gardenSystem) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, g.client, g.namespace, ManagedResourceName)
}

func (g *gardenSystem) computeResourcesData() (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

		namespaceGarden = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:        v1beta1constants.GardenNamespace,
				Labels:      map[string]string{v1beta1constants.LabelApp: v1beta1constants.LabelGardener},
				Annotations: map[string]string{resourcesv1alpha1.KeepObject: "true"},
			},
		}
		namespaceGardenerSystemPublic = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: gardencorev1beta1.GardenerSystemPublicNamespace,
			},
		}
		clusterRoleSeedBootstrapper = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:seed-bootstrapper",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{certificatesv1.GroupName},
					Resources: []string{"certificatesigningrequests"},
					Verbs:     []string{"create", "get"},
				},
				{
					APIGroups: []string{certificatesv1.GroupName},
					Resources: []string{"certificatesigningrequests/seedclient"},
					Verbs:     []string{"create"},
				},
			},
		}
		clusterRoleBindingSeedBootstrapper = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterRoleSeedBootstrapper.Name,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRoleSeedBootstrapper.Name,
			},
			Subjects: []rbacv1.Subject{{
				APIGroup: rbacv1.GroupName,
				Kind:     "Group",
				Name:     bootstraptokenapi.BootstrapDefaultGroup,
			}},
		}
		clusterRoleGardenerAdmin = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "gardener.cloud:admin",
				Labels: map[string]string{v1beta1constants.GardenRole: "admin"},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{
						gardencorev1beta1.GroupName,
						seedmanagementv1alpha1.GroupName,
						"dashboard.gardener.cloud",
						settingsv1alpha1.GroupName,
						operationsv1alpha1.GroupName,
					},
					Resources: []string{"*"},
					Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{securityv1alpha1.GroupName},
					Resources: []string{
						"credentialsbindings",
						"workloadidentities", // Do not use wildcard here to avoid granting users with permissions to send `create workloadidentity/token` requests.
					},
					Verbs: []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{corev1.GroupName},
					Resources: []string{"events", "namespaces", "resourcequotas"},
					Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{eventsv1.GroupName},
					Resources: []string{"events"},
					Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{rbacv1.GroupName},
					Resources: []string{"clusterroles", "clusterrolebindings", "roles", "rolebindings"},
					Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{admissionregistrationv1.GroupName},
					Resources: []string{"mutatingwebhookconfigurations", "validatingwebhookconfigurations"},
					Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{apiregistrationv1.GroupName},
					Resources: []string{"apiservices"},
					Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{apiextensionsv1.GroupName},
					Resources: []string{"customresourcedefinitions"},
					Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{coordinationv1.GroupName},
					Resources: []string{"leases"},
					Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{certificatesv1.GroupName},
					Resources: []string{"certificatesigningrequests"},
					Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update", "approve", "deny"},
				},
			},
		}
		clusterRoleBindingGardenerAdmin = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterRoleGardenerAdmin.Name,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRoleGardenerAdmin.Name,
			},
			Subjects: []rbacv1.Subject{{
				APIGroup: rbacv1.GroupName,
				Kind:     "User",
				Name:     "system:kube-aggregator",
			}},
		}
		clusterRoleGardenerAdminAggregated = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   v1beta1constants.ClusterRoleNameGardenerAdministrators,
				Labels: map[string]string{v1beta1constants.GardenRole: "administrators"},
			},
			AggregationRule: &rbacv1.AggregationRule{
				ClusterRoleSelectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{v1beta1constants.GardenRole: "admin"}},
					{MatchLabels: map[string]string{v1beta1constants.GardenRole: "project-member"}},
					{MatchLabels: map[string]string{v1beta1constants.GardenRole: "project-serviceaccountmanager"}},
				},
			},
		}
		clusterRoleGardenerViewer = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "gardener.cloud:viewer",
				Labels: map[string]string{v1beta1constants.GardenRole: "viewer"},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{gardencorev1beta1.GroupName},
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
					APIGroups: []string{securityv1alpha1.GroupName},
					Resources: []string{"credentialsbindings"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{
						seedmanagementv1alpha1.GroupName,
						"dashboard.gardener.cloud",
						settingsv1alpha1.GroupName,
						operationsv1alpha1.GroupName,
					},
					Resources: []string{
						"backupbuckets",
						"backupentrie",
						"cloudprofiles",
						"controllerdeployments",
						"controllerinstallations",
						"controllerregistrations",
						"exposureclasse",
						"internalsecrets",
						"namespacedcloudprofiles",
						"projects",
						"quotas",
						"secretbindings",
						"seeds",
						"shoots",
						"shootstates",
						"gardenlets",
						"managedseeds",
						"managedseedsets",
						"terminals",
						"clusteropenidconnectpresets",
						"openidconnectpresets",
						"bastions",
					},
					Verbs: []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{corev1.GroupName},
					Resources: []string{"events", "namespaces", "resourcequotas"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{eventsv1.GroupName},
					Resources: []string{"events"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{rbacv1.GroupName},
					Resources: []string{"clusterroles", "clusterrolebindings", "roles", "rolebindings"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{admissionregistrationv1.GroupName},
					Resources: []string{"mutatingwebhookconfigurations", "validatingwebhookconfigurations"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{apiregistrationv1.GroupName},
					Resources: []string{"apiservices"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{apiextensionsv1.GroupName},
					Resources: []string{"customresourcedefinitions"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{coordinationv1.GroupName},
					Resources: []string{"leases"},
					Verbs:     []string{"get", "list", "watch"},
				},
			},
		}
		clusterRoleGardenerViewerAggregated = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "gardener.cloud:system:viewers",
				Labels: map[string]string{v1beta1constants.GardenRole: "viewers"},
			},
			AggregationRule: &rbacv1.AggregationRule{
				ClusterRoleSelectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{v1beta1constants.GardenRole: "viewer"}},
					{MatchLabels: map[string]string{v1beta1constants.GardenRole: "project-viewer"}},
				},
			},
		}
		clusterRoleReadGlobalResources = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:read-global-resources",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{gardencorev1beta1.GroupName},
					Resources: []string{
						"cloudprofiles",
						"exposureclasses",
						"seeds",
					},
					Verbs: []string{"get", "list", "watch"},
				},
				{
					// allow shoot owners to use kube-state-metrics with a custom resource state configuration to expose metrics about e.g. shoots
					APIGroups: []string{apiextensionsv1.GroupName},
					Resources: []string{"customresourcedefinitions"},
					Verbs:     []string{"get", "list", "watch"},
				},
			},
		}
		clusterRoleBindingReadGlobalResources = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterRoleReadGlobalResources.Name,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRoleReadGlobalResources.Name,
			},
			Subjects: []rbacv1.Subject{{
				APIGroup: rbacv1.GroupName,
				Kind:     "Group",
				Name:     user.AllAuthenticated,
			}},
		}
		clusterRoleUserAuth = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:user-auth",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{authenticationv1.GroupName},
					Resources: []string{"tokenreviews"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups: []string{authorizationv1.GroupName},
					Resources: []string{"selfsubjectaccessreviews"},
					Verbs:     []string{"create"},
				},
			},
		}
		clusterRoleBindingUserAuth = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterRoleUserAuth.Name,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRoleUserAuth.Name,
			},
			Subjects: []rbacv1.Subject{{
				APIGroup: rbacv1.GroupName,
				Kind:     "Group",
				Name:     user.AllAuthenticated,
			}},
		}
		clusterRoleProjectCreation = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:project-creation",
			},
			Rules: []rbacv1.PolicyRule{{
				APIGroups: []string{gardencorev1beta1.GroupName},
				Resources: []string{"projects"},
				Verbs:     []string{"create"},
			}},
		}
		clusterRoleProjectMember = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "gardener.cloud:system:project-member-aggregation",
				Labels: map[string]string{v1beta1constants.LabelKeyAggregateToProjectMember: "true"},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{corev1.GroupName},
					Resources: []string{
						"secrets",
						"configmaps",
					},
					Verbs: []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{corev1.GroupName},
					Resources: []string{
						"events",
						"resourcequotas",
						"serviceaccounts",
					},
					Verbs: []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{eventsv1.GroupName},
					Resources: []string{"events"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{gardencorev1beta1.GroupName},
					Resources: []string{
						"shoots",
						"secretbindings",
						"quotas",
					},
					Verbs: []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{securityv1alpha1.GroupName},
					Resources: []string{
						"credentialsbindings",
						"workloadidentities", // Do not use wildcard here to avoid granting users with permissions to send `create workloadidentity/token` requests.
					},
					Verbs: []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{settingsv1alpha1.GroupName},
					Resources: []string{"openidconnectpresets"},
					Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{operationsv1alpha1.GroupName},
					Resources: []string{"bastions"},
					Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{rbacv1.GroupName},
					Resources: []string{
						"roles",
						"rolebindings",
					},
					Verbs: []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{gardencorev1beta1.GroupName},
					Resources: []string{
						"shoots/adminkubeconfig",
						"shoots/viewerkubeconfig",
					},
					Verbs: []string{"create"},
				},
				{
					APIGroups: []string{gardencorev1beta1.GroupName},
					Resources: []string{"namespacedcloudprofiles"},
					Verbs:     []string{"get", "list", "watch", "create", "patch", "update", "delete"},
				},
			},
		}
		clusterRoleProjectMemberAggregated = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "gardener.cloud:system:project-member",
				Labels: map[string]string{v1beta1constants.GardenRole: "project-member"},
			},
			AggregationRule: &rbacv1.AggregationRule{
				ClusterRoleSelectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{v1beta1constants.LabelKeyAggregateToProjectMember: "true"}},
				},
			},
		}
		labelKeyAggregateToProjectServiceAccountManager = "rbac.gardener.cloud/aggregate-to-project-serviceaccountmanager"
		clusterRoleProjectServiceAccountManager         = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "gardener.cloud:system:project-serviceaccountmanager-aggregation",
				Labels: map[string]string{labelKeyAggregateToProjectServiceAccountManager: "true"},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{corev1.GroupName},
					Resources: []string{"serviceaccounts"},
					Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{corev1.GroupName},
					Resources: []string{"serviceaccounts/token"},
					Verbs:     []string{"create"},
				},
			},
		}
		clusterRoleProjectServiceAccountManagerAggregated = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "gardener.cloud:system:project-serviceaccountmanager",
				Labels: map[string]string{v1beta1constants.GardenRole: "project-serviceaccountmanager"},
			},
			AggregationRule: &rbacv1.AggregationRule{
				ClusterRoleSelectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{labelKeyAggregateToProjectServiceAccountManager: "true"}},
				},
			},
		}
		labelKeyAggregateToProjectViewer = "rbac.gardener.cloud/aggregate-to-project-viewer"
		clusterRoleProjectViewer         = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "gardener.cloud:system:project-viewer-aggregation",
				Labels: map[string]string{labelKeyAggregateToProjectViewer: "true"},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{corev1.GroupName},
					Resources: []string{
						"events",
						"configmaps",
						"resourcequotas",
						"serviceaccounts",
					},
					Verbs: []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{eventsv1.GroupName},
					Resources: []string{"events"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{gardencorev1beta1.GroupName},
					Resources: []string{
						"shoots",
						"secretbindings",
						"quotas",
						"namespacedcloudprofiles",
					},
					Verbs: []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{securityv1alpha1.GroupName},
					Resources: []string{"credentialsbindings"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{settingsv1alpha1.GroupName},
					Resources: []string{"openidconnectpresets"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{operationsv1alpha1.GroupName},
					Resources: []string{"bastions"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{rbacv1.GroupName},
					Resources: []string{
						"roles",
						"rolebindings",
					},
					Verbs: []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{gardencorev1beta1.GroupName},
					Resources: []string{"shoots/viewerkubeconfig"},
					Verbs:     []string{"create"},
				},
			},
		}
		clusterRoleProjectViewerAggregated = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "gardener.cloud:system:project-viewer",
				Labels: map[string]string{v1beta1constants.GardenRole: "project-viewer"},
			},
			AggregationRule: &rbacv1.AggregationRule{
				ClusterRoleSelectors: []metav1.LabelSelector{
					{MatchLabels: map[string]string{labelKeyAggregateToProjectViewer: "true"}},
				},
			},
		}
		roleReadClusterIdentityConfigMap = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:system:read-cluster-identity-configmap",
				Namespace: metav1.NamespaceSystem,
			},
			Rules: []rbacv1.PolicyRule{{
				APIGroups:     []string{corev1.GroupName},
				Resources:     []string{"configmaps"},
				ResourceNames: []string{v1beta1constants.ClusterIdentity},
				Verbs:         []string{"get", "list", "watch"},
			}},
		}
		roleBindingReadClusterIdentityConfigMap = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      roleReadClusterIdentityConfigMap.Name,
				Namespace: metav1.NamespaceSystem,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     roleReadClusterIdentityConfigMap.Name,
			},
			Subjects: []rbacv1.Subject{{
				APIGroup: rbacv1.GroupName,
				Kind:     "Group",
				Name:     user.AllAuthenticated,
			}},
		}
		roleReadGardenerInfoConfigMap = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:system:read-gardener-info-configmap",
				Namespace: gardencorev1beta1.GardenerSystemPublicNamespace,
			},
			Rules: []rbacv1.PolicyRule{{
				APIGroups:     []string{corev1.GroupName},
				Resources:     []string{"configmaps"},
				ResourceNames: []string{"gardener-info"},
				Verbs:         []string{"get", "list", "watch"},
			}},
		}
		roleBindingReadGardenerInfoConfigMap = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      roleReadGardenerInfoConfigMap.Name,
				Namespace: gardencorev1beta1.GardenerSystemPublicNamespace,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     roleReadGardenerInfoConfigMap.Name,
			},
			Subjects: []rbacv1.Subject{{
				APIGroup: rbacv1.GroupName,
				Kind:     "Group",
				Name:     user.AllAuthenticated,
			}},
		}
	)

	if err := registry.Add(
		namespaceGarden,
		namespaceGardenerSystemPublic,
		clusterRoleSeedBootstrapper,
		clusterRoleBindingSeedBootstrapper,
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
		clusterRoleProjectMember,
		clusterRoleProjectMemberAggregated,
		clusterRoleProjectServiceAccountManager,
		clusterRoleProjectServiceAccountManagerAggregated,
		clusterRoleProjectViewer,
		clusterRoleProjectViewerAggregated,
		roleReadClusterIdentityConfigMap,
		roleBindingReadClusterIdentityConfigMap,
		roleReadGardenerInfoConfigMap,
		roleBindingReadGardenerInfoConfigMap,
	); err != nil {
		return nil, err
	}

	if !g.values.SeedAuthorizerEnabled {
		var (
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
					Name: clusterRoleSeeds.Name,
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.GroupName,
					Kind:     "ClusterRole",
					Name:     clusterRoleSeeds.Name,
				},
				Subjects: []rbacv1.Subject{{
					APIGroup: rbacv1.GroupName,
					Kind:     "Group",
					Name:     v1beta1constants.SeedsGroup,
				}},
			}
		)

		if err := registry.Add(
			clusterRoleSeeds,
			clusterRoleBindingSeeds,
		); err != nil {
			return nil, err
		}
	}

	return registry.SerializedObjects()
}
