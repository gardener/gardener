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

	"github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity"
	seedidentity "github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity/seed"
	authwebhook "github.com/gardener/gardener/pkg/admissioncontroller/webhook/auth"
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

	var (
		log               = a.logger.WithValues("seedName", seedName, "attributes", fmt.Sprintf("%#v", attrs), "userType", userType)
		requestAuthorizer = &authwebhook.RequestAuthorizer{
			Log:                    log,
			Graph:                  a.graph,
			AuthorizeWithSelectors: a.authorizeWithSelectors,
			ToType:                 graph.VertexTypeSeed,
			ToName:                 seedName,
		}
	)

	if attrs.IsResourceRequest() {
		requestResource := schema.GroupResource{Group: attrs.GetAPIGroup(), Resource: attrs.GetResource()}
		switch requestResource {
		case backupBucketResource:
			return requestAuthorizer.Check(graph.VertexTypeBackupBucket, attrs,
				authwebhook.WithAllowedVerbs("update", "patch", "delete"),
				authwebhook.WithAlwaysAllowedVerbs("create", "get", "list", "watch"),
				authwebhook.WithAllowedSubresources("status", "finalizers"),
			)
		case backupEntryResource:
			return requestAuthorizer.Check(graph.VertexTypeBackupEntry, attrs,
				authwebhook.WithAllowedVerbs("update", "patch", "delete"),
				authwebhook.WithAlwaysAllowedVerbs("create", "get", "list", "watch"),
				authwebhook.WithAllowedSubresources("status"),
			)
		case bastionResource:
			return requestAuthorizer.Check(graph.VertexTypeBastion, attrs,
				authwebhook.WithAllowedVerbs("update", "patch"),
				authwebhook.WithAlwaysAllowedVerbs("create", "get", "list", "watch"),
				authwebhook.WithAllowedSubresources("status"),
			)
		case certificateSigningRequestResource:
			if userType == gardenletidentity.UserTypeExtension {
				return requestAuthorizer.CheckRead(graph.VertexTypeCertificateSigningRequest, attrs)
			}

			return requestAuthorizer.Check(graph.VertexTypeCertificateSigningRequest, attrs,
				authwebhook.WithAllowedVerbs("get", "list", "watch"),
				authwebhook.WithAlwaysAllowedVerbs("create"),
				authwebhook.WithAllowedSubresources("seedclient"),
			)
		case cloudProfileResource:
			return requestAuthorizer.CheckRead(graph.VertexTypeCloudProfile, attrs)
		case namespacedCloudProfileResource:
			return requestAuthorizer.CheckRead(graph.VertexTypeNamespacedCloudProfile, attrs)
		case clusterRoleBindingResource:
			if userType == gardenletidentity.UserTypeExtension {
				// We don't use authorizeRead here, as it would also grant list and watch permissions, which gardenlet doesn't
				// have. We want to grant the read-only subset of gardenlet's permissions.
				return requestAuthorizer.Check(graph.VertexTypeClusterRoleBinding, attrs,
					authwebhook.WithAllowedVerbs("get"),
				)
			}

			return a.authorizeClusterRoleBinding(requestAuthorizer, attrs)
		case configMapResource:
			return a.authorizeConfigMap(requestAuthorizer, attrs)
		case controllerDeploymentResource:
			return requestAuthorizer.CheckRead(graph.VertexTypeControllerDeployment, attrs)
		case controllerInstallationResource:
			return requestAuthorizer.Check(graph.VertexTypeControllerInstallation, attrs,
				authwebhook.WithAllowedVerbs("update", "patch"),
				authwebhook.WithAlwaysAllowedVerbs("get", "list", "watch"),
				authwebhook.WithAllowedSubresources("status"),
			)
		case controllerRegistrationResource:
			return requestAuthorizer.Check(graph.VertexTypeControllerRegistration, attrs,
				authwebhook.WithAlwaysAllowedVerbs("get", "list", "watch"),
			)
		case credentialsBindingResource:
			return requestAuthorizer.CheckRead(graph.VertexTypeCredentialsBinding, attrs)
		case eventCoreResource, eventResource:
			return a.authorizeEvent(log, attrs)
		case exposureClassResource:
			return requestAuthorizer.CheckRead(graph.VertexTypeExposureClass, attrs)
		case internalSecretResource:
			return requestAuthorizer.Check(graph.VertexTypeInternalSecret, attrs,
				authwebhook.WithAllowedVerbs("get", "update", "patch", "delete", "list", "watch"),
				authwebhook.WithAlwaysAllowedVerbs("create"),
			)
		case leaseResource:
			return a.authorizeLease(requestAuthorizer, userType, attrs)
		case gardenletResource:
			return requestAuthorizer.Check(graph.VertexTypeGardenlet, attrs,
				authwebhook.WithAllowedVerbs("update", "patch"),
				authwebhook.WithAlwaysAllowedVerbs("get", "list", "watch", "create"),
				authwebhook.WithAllowedSubresources("status"),
			)
		case managedSeedResource:
			return requestAuthorizer.Check(graph.VertexTypeManagedSeed, attrs,
				authwebhook.WithAllowedVerbs("update", "patch"),
				authwebhook.WithAlwaysAllowedVerbs("get", "list", "watch"),
				authwebhook.WithAllowedSubresources("status"),
			)
		case namespaceResource:
			return requestAuthorizer.CheckRead(graph.VertexTypeNamespace, attrs)
		case projectResource:
			return requestAuthorizer.CheckRead(graph.VertexTypeProject, attrs)
		case secretBindingResource:
			return requestAuthorizer.CheckRead(graph.VertexTypeSecretBinding, attrs)
		case secretResource:
			return a.authorizeSecret(requestAuthorizer, attrs)
		case workloadIdentityResource:
			return requestAuthorizer.Check(graph.VertexTypeWorkloadIdentity, attrs,
				authwebhook.WithAllowedVerbs("get", "list", "watch", "create", "patch"),
				authwebhook.WithAllowedSubresources("token"),
			)
		case seedResource:
			return requestAuthorizer.Check(graph.VertexTypeSeed, attrs,
				authwebhook.WithAllowedVerbs("update", "patch", "delete"),
				authwebhook.WithAlwaysAllowedVerbs("create", "get", "list", "watch"),
				authwebhook.WithAllowedSubresources("status"),
			)
		case serviceAccountResource:
			if userType == gardenletidentity.UserTypeExtension {
				// We don't use authorizeRead here, as it would also grant list and watch permissions, which gardenlet doesn't
				// have. We want to grant the read-only subset of gardenlet's permissions.
				return requestAuthorizer.Check(graph.VertexTypeServiceAccount, attrs,
					authwebhook.WithAllowedVerbs("get"),
				)
			}

			return a.authorizeServiceAccount(requestAuthorizer, attrs)
		case shootResource:
			return requestAuthorizer.Check(graph.VertexTypeShoot, attrs,
				authwebhook.WithAllowedVerbs("update", "patch"),
				authwebhook.WithAlwaysAllowedVerbs("get", "list", "watch"),
				authwebhook.WithAllowedSubresources("status", "finalizers"),
			)
		case shootStateResource:
			return requestAuthorizer.Check(graph.VertexTypeShootState, attrs,
				authwebhook.WithAllowedVerbs("get", "update", "patch", "delete", "list", "watch"),
				authwebhook.WithAlwaysAllowedVerbs("create"),
			)
		case workloadIdentityResource:
			return requestAuthorizer.Check(graph.VertexTypeWorkloadIdentity, attrs,
				authwebhook.WithAllowedVerbs("get", "list", "watch", "create"),
				authwebhook.WithAllowedSubresources("token"),
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

func (a *authorizer) authorizeClusterRoleBinding(requestAuthorizer *authwebhook.RequestAuthorizer, attrs auth.Attributes) (auth.Decision, string, error) {
	// Allow gardenlet to delete its cluster role binding after bootstrapping (in this case, there is no `Seed` resource
	// in the system yet, so we can't rely on the graph).
	if attrs.GetVerb() == "delete" &&
		strings.HasPrefix(attrs.GetName(), gardenletbootstraputil.ClusterRoleBindingNamePrefix) {
		managedSeedNamespace, managedSeedName := gardenletbootstraputil.ManagedSeedInfoFromClusterRoleBindingName(attrs.GetName())
		if managedSeedNamespace == v1beta1constants.GardenNamespace && managedSeedName == requestAuthorizer.ToName {
			return auth.DecisionAllow, "", nil
		}
	}

	return requestAuthorizer.Check(graph.VertexTypeClusterRoleBinding, attrs,
		authwebhook.WithAllowedVerbs("get", "patch", "update"),
		authwebhook.WithAlwaysAllowedVerbs("create"),
	)
}

func (a *authorizer) authorizeEvent(log logr.Logger, attrs auth.Attributes) (auth.Decision, string, error) {
	if ok, reason := authwebhook.CheckVerb(log, attrs, "create", "patch"); !ok {
		return auth.DecisionNoOpinion, reason, nil
	}

	if ok, reason := authwebhook.CheckSubresource(log, attrs); !ok {
		return auth.DecisionNoOpinion, reason, nil
	}

	return auth.DecisionAllow, "", nil
}

func (a *authorizer) authorizeLease(requestAuthorizer *authwebhook.RequestAuthorizer, userType gardenletidentity.UserType, attrs auth.Attributes) (auth.Decision, string, error) {
	// extension clients may only work with leases in the seed namespace
	if userType == gardenletidentity.UserTypeExtension {
		if attrs.GetNamespace() == gardenerutils.ComputeGardenNamespace(requestAuthorizer.ToName) {
			if ok, reason := authwebhook.CheckVerb(requestAuthorizer.Log, attrs, "create", "get", "list", "watch", "update", "patch", "delete", "deletecollection"); !ok {
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

	return requestAuthorizer.Check(graph.VertexTypeLease, attrs,
		authwebhook.WithAllowedVerbs("get", "update", "patch", "list", "watch"),
		authwebhook.WithAlwaysAllowedVerbs("create"),
	)
}

func (a *authorizer) authorizeSecret(requestAuthorizer *authwebhook.RequestAuthorizer, attrs auth.Attributes) (auth.Decision, string, error) {
	// Allow gardenlets to get/list/watch secrets in their seed-<name> namespaces.
	if slices.Contains([]string{"get", "list", "watch"}, attrs.GetVerb()) && attrs.GetNamespace() == gardenerutils.ComputeGardenNamespace(requestAuthorizer.ToName) {
		return auth.DecisionAllow, "", nil
	}

	// Allow gardenlet to delete its bootstrap token (in this case, there is no `Seed` resource in the system yet, so
	// we can't rely on the graph).
	if (attrs.GetVerb() == "delete" &&
		attrs.GetNamespace() == metav1.NamespaceSystem &&
		strings.HasPrefix(attrs.GetName(), bootstraptokenapi.BootstrapTokenSecretPrefix)) &&
		(attrs.GetName() == bootstraptokenapi.BootstrapTokenSecretPrefix+bootstraptoken.TokenID(metav1.ObjectMeta{Name: requestAuthorizer.ToName, Namespace: v1beta1constants.GardenNamespace})) {
		return auth.DecisionAllow, "", nil
	}

	return requestAuthorizer.Check(graph.VertexTypeSecret, attrs,
		authwebhook.WithAllowedVerbs("get", "patch", "update", "delete"),
		authwebhook.WithAlwaysAllowedVerbs("create"),
	)
}

func (a *authorizer) authorizeConfigMap(requestAuthorizer *authwebhook.RequestAuthorizer, attrs auth.Attributes) (auth.Decision, string, error) {
	if attrs.GetVerb() == "get" &&
		attrs.GetNamespace() == gardencorev1beta1.GardenerSystemPublicNamespace &&
		attrs.GetName() == v1beta1constants.ConfigMapNameGardenerInfo {
		return auth.DecisionAllow, "", nil
	}

	return requestAuthorizer.Check(graph.VertexTypeConfigMap, attrs,
		authwebhook.WithAllowedVerbs("get", "patch", "update", "delete", "list", "watch"),
		authwebhook.WithAlwaysAllowedVerbs("create"),
	)
}

func (a *authorizer) authorizeServiceAccount(requestAuthorizer *authwebhook.RequestAuthorizer, attrs auth.Attributes) (auth.Decision, string, error) {
	// Allow gardenlet to delete its service account after bootstrapping (in this case, there is no `Seed` resource in
	// the system yet, so we can't rely on the graph).
	if attrs.GetVerb() == "delete" &&
		attrs.GetNamespace() == v1beta1constants.GardenNamespace &&
		strings.HasPrefix(attrs.GetName(), gardenletbootstraputil.ServiceAccountNamePrefix) &&
		strings.TrimPrefix(attrs.GetName(), gardenletbootstraputil.ServiceAccountNamePrefix) == requestAuthorizer.ToName {
		return auth.DecisionAllow, "", nil
	}

	// Allow all verbs for service accounts in gardenlets' seed-<name> namespaces.
	if attrs.GetNamespace() == gardenerutils.ComputeGardenNamespace(requestAuthorizer.ToName) {
		return auth.DecisionAllow, "", nil
	}

	return requestAuthorizer.Check(graph.VertexTypeServiceAccount, attrs,
		authwebhook.WithAllowedVerbs("get", "patch", "update"),
		authwebhook.WithAlwaysAllowedVerbs("create"),
	)
}
