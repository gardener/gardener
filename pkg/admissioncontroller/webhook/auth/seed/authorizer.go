// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	certificatesv1 "k8s.io/api/certificates/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	auth "k8s.io/apiserver/pkg/authorization/authorizer"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"

	seedidentity "github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity/seed"
	authhelper "github.com/gardener/gardener/pkg/admissioncontroller/webhook/auth"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	gardenletbootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/graph"
	"github.com/gardener/gardener/pkg/utils/kubernetes/bootstraptoken"
	authorizerwebhook "github.com/gardener/gardener/pkg/webhook/authorizer"
)

// NewAuthorizer returns a new authorizer for requests from gardenlets. It never has an opinion on the request.
func NewAuthorizer(logger logr.Logger, graph graph.Interface, authorizeWithSelectors authorizerwebhook.WithSelectorsChecker) *authorizer {
	return &authorizer{
		logger:                 logger,
		graph:                  graph,
		authorizeWithSelectors: authorizeWithSelectors,
	}
}

type authorizer struct {
	logger                 logr.Logger
	graph                  graph.Interface
	authorizeWithSelectors authorizerwebhook.WithSelectorsChecker
}

var _ = auth.Authorizer(&authorizer{})

var (
	// Only take v1beta1 for the core.gardener.cloud API group because the Authorize function only checks the resource
	// group and the resource (but it ignores the version).
	backupBucketResource              = gardencorev1beta1.Resource("backupbuckets")
	backupEntryResource               = gardencorev1beta1.Resource("backupentries")
	bastionResource                   = operationsv1alpha1.Resource("bastions")
	certificateSigningRequestResource = certificatesv1.Resource("certificatesigningrequests")
	cloudProfileResource              = gardencorev1beta1.Resource("cloudprofiles")
	namespacedCloudProfileResource    = gardencorev1beta1.Resource("namespacedcloudprofiles")
	clusterRoleBindingResource        = rbacv1.Resource("clusterrolebindings")
	configMapResource                 = corev1.Resource("configmaps")
	controllerDeploymentResource      = gardencorev1beta1.Resource("controllerdeployments")
	controllerInstallationResource    = gardencorev1beta1.Resource("controllerinstallations")
	controllerRegistrationResource    = gardencorev1beta1.Resource("controllerregistrations")
	eventCoreResource                 = corev1.Resource("events")
	eventResource                     = eventsv1.Resource("events")
	exposureClassResource             = gardencorev1beta1.Resource("exposureclasses")
	internalSecretResource            = gardencorev1beta1.Resource("internalsecrets")
	leaseResource                     = coordinationv1.Resource("leases")
	gardenletResource                 = seedmanagementv1alpha1.Resource("gardenlets")
	managedSeedResource               = seedmanagementv1alpha1.Resource("managedseeds")
	namespaceResource                 = corev1.Resource("namespaces")
	projectResource                   = gardencorev1beta1.Resource("projects")
	secretBindingResource             = gardencorev1beta1.Resource("secretbindings")
	secretResource                    = corev1.Resource("secrets")
	seedResource                      = gardencorev1beta1.Resource("seeds")
	serviceAccountResource            = corev1.Resource("serviceaccounts")
	shootResource                     = gardencorev1beta1.Resource("shoots")
	shootStateResource                = gardencorev1beta1.Resource("shootstates")
	credentialsBindingResource        = securityv1alpha1.Resource("credentialsbindings")
	workloadIdentityResource          = securityv1alpha1.Resource("workloadidentities")
)

// TODO: Revisit all `DecisionNoOpinion` later. Today we cannot deny the request for backwards compatibility
// because older Gardenlet versions might not be compatible at the time this authorization plugin is enabled.
// With `DecisionNoOpinion`, RBAC will be respected in the authorization chain afterwards.

func (a *authorizer) Authorize(_ context.Context, attrs auth.Attributes) (auth.Decision, string, error) {
	seedName, isSeed, userType := seedidentity.FromUserInfoInterface(attrs.GetUser())
	if !isSeed {
		return auth.DecisionNoOpinion, "", nil
	}

	requestLog := a.logger.WithValues("seedName", seedName, "attributes", fmt.Sprintf("%#v", attrs), "userType", userType)

	if attrs.IsResourceRequest() {
		requestResource := schema.GroupResource{Group: attrs.GetAPIGroup(), Resource: attrs.GetResource()}
		switch requestResource {
		case backupBucketResource:
			return authhelper.Authorize(requestLog, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeBackupBucket, attrs,
				authhelper.WithAllowedVerbs("update", "patch", "delete"),
				authhelper.WithAlwaysAllowedVerbs("create", "get", "list", "watch"),
				authhelper.WithAllowedSubresources("status"),
			)
		case backupEntryResource:
			return authhelper.Authorize(requestLog, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeBackupEntry, attrs,
				authhelper.WithAllowedVerbs("update", "patch", "delete"),
				authhelper.WithAlwaysAllowedVerbs("create", "get", "list", "watch"),
				authhelper.WithAllowedSubresources("status"),
			)
		case bastionResource:
			return authhelper.Authorize(requestLog, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeBastion, attrs,
				authhelper.WithAllowedVerbs("update", "patch"),
				authhelper.WithAlwaysAllowedVerbs("create", "get", "list", "watch"),
				authhelper.WithAllowedSubresources("status"),
			)
		case certificateSigningRequestResource:
			if userType == seedidentity.UserTypeExtension {
				return authhelper.AuthorizeRead(requestLog, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeCertificateSigningRequest, attrs)
			}

			return authhelper.Authorize(requestLog, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeCertificateSigningRequest, attrs,
				authhelper.WithAllowedVerbs("get", "list", "watch"),
				authhelper.WithAlwaysAllowedVerbs("create"),
				authhelper.WithAllowedSubresources("seedclient"),
			)
		case cloudProfileResource:
			return authhelper.AuthorizeRead(requestLog, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeCloudProfile, attrs)
		case namespacedCloudProfileResource:
			return authhelper.AuthorizeRead(requestLog, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeNamespacedCloudProfile, attrs)
		case clusterRoleBindingResource:
			if userType == seedidentity.UserTypeExtension {
				// We don't use authorizeRead here, as it would also grant list and watch permissions, which gardenlet doesn't
				// have. We want to grant the read-only subset of gardenlet's permissions.
				return authhelper.Authorize(requestLog, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeClusterRoleBinding, attrs,
					authhelper.WithAllowedVerbs("get"),
				)
			}

			return a.authorizeClusterRoleBinding(requestLog, seedName, attrs)
		case configMapResource:
			return a.authorizeConfigMap(requestLog, seedName, attrs)
		case controllerDeploymentResource:
			return authhelper.AuthorizeRead(requestLog, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeControllerDeployment, attrs)
		case controllerInstallationResource:
			return authhelper.Authorize(requestLog, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeControllerInstallation, attrs,
				authhelper.WithAllowedVerbs("update", "patch"),
				authhelper.WithAlwaysAllowedVerbs("get", "list", "watch"),
				authhelper.WithAllowedSubresources("status"),
			)
		case controllerRegistrationResource:
			return authhelper.Authorize(requestLog, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeControllerRegistration, attrs,
				authhelper.WithAlwaysAllowedVerbs("get", "list", "watch"),
			)
		case credentialsBindingResource:
			return authhelper.AuthorizeRead(requestLog, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeCredentialsBinding, attrs)
		case eventCoreResource, eventResource:
			return a.authorizeEvent(requestLog, attrs)
		case exposureClassResource:
			return authhelper.AuthorizeRead(requestLog, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeExposureClass, attrs)
		case internalSecretResource:
			return authhelper.Authorize(requestLog, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeInternalSecret, attrs,
				authhelper.WithAllowedVerbs("get", "update", "patch", "delete", "list", "watch"),
				authhelper.WithAlwaysAllowedVerbs("create"),
			)
		case leaseResource:
			return a.authorizeLease(requestLog, seedName, userType, attrs)
		case gardenletResource:
			return authhelper.Authorize(requestLog, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeGardenlet, attrs,
				authhelper.WithAllowedVerbs("update", "patch"),
				authhelper.WithAlwaysAllowedVerbs("get", "list", "watch", "create"),
				authhelper.WithAllowedSubresources("status"),
			)
		case managedSeedResource:
			return authhelper.Authorize(requestLog, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeManagedSeed, attrs,
				authhelper.WithAllowedVerbs("update", "patch"),
				authhelper.WithAlwaysAllowedVerbs("get", "list", "watch"),
				authhelper.WithAllowedSubresources("status"),
			)
		case namespaceResource:
			return authhelper.AuthorizeRead(requestLog, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeNamespace, attrs)
		case projectResource:
			return authhelper.AuthorizeRead(requestLog, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeProject, attrs)
		case secretBindingResource:
			return authhelper.AuthorizeRead(requestLog, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeSecretBinding, attrs)
		case secretResource:
			return a.authorizeSecret(requestLog, seedName, attrs)
		case workloadIdentityResource:
			return authhelper.Authorize(requestLog, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeWorkloadIdentity, attrs,
				authhelper.WithAllowedVerbs("get", "list", "watch", "create", "patch"),
				authhelper.WithAllowedSubresources("token"),
			)
		case seedResource:
			return authhelper.Authorize(requestLog, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeSeed, attrs,
				authhelper.WithAllowedVerbs("update", "patch", "delete"),
				authhelper.WithAlwaysAllowedVerbs("create", "get", "list", "watch"),
				authhelper.WithAllowedSubresources("status"),
			)
		case serviceAccountResource:
			if userType == seedidentity.UserTypeExtension {
				// We don't use authorizeRead here, as it would also grant list and watch permissions, which gardenlet doesn't
				// have. We want to grant the read-only subset of gardenlet's permissions.
				return authhelper.Authorize(requestLog, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeServiceAccount, attrs,
					authhelper.WithAllowedVerbs("get"),
				)
			}

			return a.authorizeServiceAccount(requestLog, seedName, attrs)
		case shootResource:
			return authhelper.Authorize(requestLog, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeShoot, attrs,
				authhelper.WithAllowedVerbs("update", "patch"),
				authhelper.WithAlwaysAllowedVerbs("get", "list", "watch"),
				authhelper.WithAllowedSubresources("status"),
			)
		case shootStateResource:
			return authhelper.Authorize(requestLog, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeShootState, attrs,
				authhelper.WithAllowedVerbs("get", "update", "patch", "delete", "list", "watch"),
				authhelper.WithAlwaysAllowedVerbs("create"),
			)
		case workloadIdentityResource:
			return authhelper.Authorize(requestLog, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeWorkloadIdentity, attrs,
				authhelper.WithAllowedVerbs("get", "list", "watch", "create"),
				authhelper.WithAllowedSubresources("token"),
			)
		default:
			a.logger.Info(
				"Unhandled resource request",
				"seed", seedName,
				"group", attrs.GetAPIGroup(),
				"version", attrs.GetAPIVersion(),
				"resource", attrs.GetResource(),
				"verb", attrs.GetVerb(),
			)
		}
	}

	return auth.DecisionNoOpinion, "", nil
}

func (a *authorizer) authorizeClusterRoleBinding(log logr.Logger, seedName string, attrs auth.Attributes) (auth.Decision, string, error) {
	// Allow gardenlet to delete its cluster role binding after bootstrapping (in this case, there is no `Seed` resource
	// in the system yet, so we can't rely on the graph).
	if attrs.GetVerb() == "delete" &&
		strings.HasPrefix(attrs.GetName(), gardenletbootstraputil.ClusterRoleBindingNamePrefix) {
		managedSeedNamespace, managedSeedName := gardenletbootstraputil.ManagedSeedInfoFromClusterRoleBindingName(attrs.GetName())
		if managedSeedNamespace == v1beta1constants.GardenNamespace && managedSeedName == seedName {
			return auth.DecisionAllow, "", nil
		}
	}

	return authhelper.Authorize(log, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeClusterRoleBinding, attrs,
		authhelper.WithAllowedVerbs("get", "patch", "update"),
		authhelper.WithAlwaysAllowedVerbs("create"),
	)
}

func (a *authorizer) authorizeEvent(log logr.Logger, attrs auth.Attributes) (auth.Decision, string, error) {
	if ok, reason := authhelper.CheckVerb(log, attrs, "create", "patch"); !ok {
		return auth.DecisionNoOpinion, reason, nil
	}

	if ok, reason := authhelper.CheckSubresource(log, attrs); !ok {
		return auth.DecisionNoOpinion, reason, nil
	}

	return auth.DecisionAllow, "", nil
}

func (a *authorizer) authorizeLease(log logr.Logger, seedName string, userType seedidentity.UserType, attrs auth.Attributes) (auth.Decision, string, error) {
	// extension clients may only work with leases in the seed namespace
	if userType == seedidentity.UserTypeExtension {
		if attrs.GetNamespace() == gardenerutils.ComputeGardenNamespace(seedName) {
			if ok, reason := authhelper.CheckVerb(log, attrs, "create", "get", "list", "watch", "update", "patch", "delete", "deletecollection"); !ok {
				return auth.DecisionNoOpinion, reason, nil
			}

			return auth.DecisionAllow, "", nil
		}

		return auth.DecisionNoOpinion, "lease object is not in seed namespace", nil
	}

	// This is needed if the seed cluster is a garden cluster at the same time.
	if attrs.GetName() == "gardenlet-leader-election" &&
		slices.Contains([]string{"create", "get", "list", "watch", "update"}, attrs.GetVerb()) {
		return auth.DecisionAllow, "", nil
	}

	return authhelper.Authorize(log, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeLease, attrs,
		authhelper.WithAllowedVerbs("get", "update", "patch", "list", "watch"),
		authhelper.WithAlwaysAllowedVerbs("create"),
	)
}

func (a *authorizer) authorizeSecret(log logr.Logger, seedName string, attrs auth.Attributes) (auth.Decision, string, error) {
	// Allow gardenlets to get/list/watch secrets in their seed-<name> namespaces.
	if slices.Contains([]string{"get", "list", "watch"}, attrs.GetVerb()) && attrs.GetNamespace() == gardenerutils.ComputeGardenNamespace(seedName) {
		return auth.DecisionAllow, "", nil
	}

	// Allow gardenlet to delete its bootstrap token (in this case, there is no `Seed` resource in the system yet, so
	// we can't rely on the graph).
	if (attrs.GetVerb() == "delete" &&
		attrs.GetNamespace() == metav1.NamespaceSystem &&
		strings.HasPrefix(attrs.GetName(), bootstraptokenapi.BootstrapTokenSecretPrefix)) &&
		(attrs.GetName() == bootstraptokenapi.BootstrapTokenSecretPrefix+bootstraptoken.TokenID(metav1.ObjectMeta{Name: seedName, Namespace: v1beta1constants.GardenNamespace})) {
		return auth.DecisionAllow, "", nil
	}

	return authhelper.Authorize(log, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeSecret, attrs,
		authhelper.WithAllowedVerbs("get", "patch", "update", "delete"),
		authhelper.WithAlwaysAllowedVerbs("create"),
	)
}

func (a *authorizer) authorizeConfigMap(log logr.Logger, seedName string, attrs auth.Attributes) (auth.Decision, string, error) {
	if attrs.GetVerb() == "get" &&
		attrs.GetNamespace() == gardencorev1beta1.GardenerSystemPublicNamespace &&
		attrs.GetName() == v1beta1constants.ConfigMapNameGardenerInfo {
		return auth.DecisionAllow, "", nil
	}

	return authhelper.Authorize(log, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeConfigMap, attrs,
		authhelper.WithAllowedVerbs("get", "patch", "update", "delete", "list", "watch"),
		authhelper.WithAlwaysAllowedVerbs("create"),
	)
}

func (a *authorizer) authorizeServiceAccount(log logr.Logger, seedName string, attrs auth.Attributes) (auth.Decision, string, error) {
	// Allow gardenlet to delete its service account after bootstrapping (in this case, there is no `Seed` resource in
	// the system yet, so we can't rely on the graph).
	if attrs.GetVerb() == "delete" &&
		attrs.GetNamespace() == v1beta1constants.GardenNamespace &&
		strings.HasPrefix(attrs.GetName(), gardenletbootstraputil.ServiceAccountNamePrefix) &&
		strings.TrimPrefix(attrs.GetName(), gardenletbootstraputil.ServiceAccountNamePrefix) == seedName {
		return auth.DecisionAllow, "", nil
	}

	// Allow all verbs for service accounts in gardenlets' seed-<name> namespaces.
	if attrs.GetNamespace() == gardenerutils.ComputeGardenNamespace(seedName) {
		return auth.DecisionAllow, "", nil
	}

	return authhelper.Authorize(log, a.graph, a.authorizeWithSelectors, seedName, graph.VertexTypeServiceAccount, attrs,
		authhelper.WithAllowedVerbs("get", "patch", "update"),
		authhelper.WithAlwaysAllowedVerbs("create"),
	)
}
