// SPDX-FileCopyrightText: Contributors to the Gardener project
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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity"
	seedidentity "github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity/seed"
	authwebhook "github.com/gardener/gardener/pkg/admissioncontroller/webhook/auth"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/operations"
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
func NewAuthorizer(logger logr.Logger, reader client.Reader, graph graph.Interface, authorizeWithSelectors authorizerwebhook.WithSelectorsChecker) *authorizer {
	return &authorizer{
		logger:                 logger,
		reader:                 reader,
		graph:                  graph,
		authorizeWithSelectors: authorizeWithSelectors,
	}
}

type authorizer struct {
	logger                 logr.Logger
	reader                 client.Reader
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

func (a *authorizer) Authorize(ctx context.Context, attrs auth.Attributes) (auth.Decision, string, error) {
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
			Reader:                 a.reader,
			ToType:                 graph.VertexTypeSeed,
			ToName:                 seedName,
		}
	)

	if attrs.IsResourceRequest() {
		requestResource := schema.GroupResource{Group: attrs.GetAPIGroup(), Resource: attrs.GetResource()}
		switch requestResource {
		case backupBucketResource:
			return requestAuthorizer.Check(ctx, graph.VertexTypeBackupBucket, attrs,
				authwebhook.WithAllowedVerbs("update", "patch", "delete"),
				authwebhook.WithAlwaysAllowedVerbs("create", "get", "list", "watch"),
				authwebhook.WithAllowedSubresources("status", "finalizers"),
			)
		case backupEntryResource:
			return requestAuthorizer.Check(ctx, graph.VertexTypeBackupEntry, attrs,
				authwebhook.WithAllowedVerbs("update", "patch", "delete"),
				authwebhook.WithAlwaysAllowedVerbs("create", "get", "list", "watch"),
				authwebhook.WithAllowedSubresources("status"),
			)
		case bastionResource:
			return requestAuthorizer.Check(ctx, graph.VertexTypeBastion, attrs,
				authwebhook.WithAllowedVerbs("get", "list", "watch", "update", "patch"),
				authwebhook.WithAlwaysAllowedVerbs("create"),
				authwebhook.WithAllowedSubresources("status"),
				authwebhook.WithFieldSelectors(map[string]string{operations.BastionSeedName: seedName}),
			)
		case certificateSigningRequestResource:
			if userType == gardenletidentity.UserTypeExtension {
				return requestAuthorizer.CheckRead(ctx, graph.VertexTypeCertificateSigningRequest, attrs)
			}

			return requestAuthorizer.Check(ctx, graph.VertexTypeCertificateSigningRequest, attrs,
				authwebhook.WithAllowedVerbs("get", "list", "watch"),
				authwebhook.WithAlwaysAllowedVerbs("create"),
				authwebhook.WithAllowedSubresources("seedclient"),
			)
		case cloudProfileResource:
			return requestAuthorizer.CheckRead(ctx, graph.VertexTypeCloudProfile, attrs)
		case namespacedCloudProfileResource:
			return requestAuthorizer.CheckRead(ctx, graph.VertexTypeNamespacedCloudProfile, attrs)
		case clusterRoleBindingResource:
			if userType == gardenletidentity.UserTypeExtension {
				// We don't use authorizeRead here, as it would also grant list and watch permissions, which gardenlet doesn't
				// have. We want to grant the read-only subset of gardenlet's permissions.
				return requestAuthorizer.Check(ctx, graph.VertexTypeClusterRoleBinding, attrs,
					authwebhook.WithAllowedVerbs("get"),
				)
			}

			return a.authorizeClusterRoleBinding(ctx, requestAuthorizer, attrs)
		case configMapResource:
			return a.authorizeConfigMap(ctx, requestAuthorizer, attrs)
		case controllerDeploymentResource:
			return requestAuthorizer.CheckRead(ctx, graph.VertexTypeControllerDeployment, attrs)
		case controllerInstallationResource:
			return requestAuthorizer.Check(ctx, graph.VertexTypeControllerInstallation, attrs,
				authwebhook.WithAllowedVerbs("get", "list", "watch", "update", "patch"),
				authwebhook.WithAllowedSubresources("status"),
				authwebhook.WithFieldSelectors(map[string]string{gardencore.SeedRefName: seedName}),
			)
		case controllerRegistrationResource:
			return requestAuthorizer.Check(ctx, graph.VertexTypeControllerRegistration, attrs,
				authwebhook.WithAlwaysAllowedVerbs("get", "list", "watch"),
			)
		case credentialsBindingResource:
			return requestAuthorizer.CheckRead(ctx, graph.VertexTypeCredentialsBinding, attrs)
		case eventCoreResource, eventResource:
			return a.authorizeEvent(log, attrs)
		case exposureClassResource:
			return requestAuthorizer.CheckRead(ctx, graph.VertexTypeExposureClass, attrs)
		case internalSecretResource:
			return requestAuthorizer.Check(ctx, graph.VertexTypeInternalSecret, attrs,
				authwebhook.WithAllowedVerbs("get", "update", "patch", "delete", "list", "watch"),
				authwebhook.WithAlwaysAllowedVerbs("create"),
			)
		case leaseResource:
			return a.authorizeLease(ctx, requestAuthorizer, userType, attrs)
		case gardenletResource:
			return requestAuthorizer.Check(ctx, graph.VertexTypeGardenlet, attrs,
				authwebhook.WithAllowedVerbs("get", "list", "watch", "update", "patch"),
				authwebhook.WithAlwaysAllowedVerbs("create"),
				authwebhook.WithAllowedNamespaces(v1beta1constants.GardenNamespace),
				authwebhook.WithAllowedSubresources("status"),
				authwebhook.WithFieldSelectors(map[string]string{metav1.ObjectNameField: seedName}),
			)
		case managedSeedResource:
			return requestAuthorizer.Check(ctx, graph.VertexTypeManagedSeed, attrs,
				authwebhook.WithAllowedVerbs("get", "list", "watch", "update", "patch"),
				authwebhook.WithAllowedSubresources("status"),
				authwebhook.WithLabelSelectors(map[string]string{v1beta1constants.LabelPrefixSeedName + seedName: "true"}),
			)
		case namespaceResource:
			return requestAuthorizer.CheckRead(ctx, graph.VertexTypeNamespace, attrs)
		case projectResource:
			return requestAuthorizer.CheckRead(ctx, graph.VertexTypeProject, attrs)
		case secretBindingResource:
			return requestAuthorizer.CheckRead(ctx, graph.VertexTypeSecretBinding, attrs)
		case secretResource:
			return a.authorizeSecret(ctx, requestAuthorizer, attrs)
		case workloadIdentityResource:
			return requestAuthorizer.Check(ctx, graph.VertexTypeWorkloadIdentity, attrs,
				authwebhook.WithAllowedVerbs("get", "list", "watch"),
				authwebhook.WithSubresourcesAndVerb([]string{"token"}, []string{"create"}),
			)
		case seedResource:
			return requestAuthorizer.Check(ctx, graph.VertexTypeSeed, attrs,
				authwebhook.WithAllowedVerbs("get", "list", "watch", "update", "patch", "delete"),
				authwebhook.WithAlwaysAllowedVerbs("create"),
				authwebhook.WithAllowedSubresources("status"),
				authwebhook.WithLabelSelectors(map[string]string{v1beta1constants.LabelPrefixSeedName + seedName: "true"}),
			)
		case serviceAccountResource:
			if userType == gardenletidentity.UserTypeExtension {
				// We don't use authorizeRead here, as it would also grant list and watch permissions, which gardenlet doesn't
				// have. We want to grant the read-only subset of gardenlet's permissions.
				return requestAuthorizer.Check(ctx, graph.VertexTypeServiceAccount, attrs,
					authwebhook.WithAllowedVerbs("get"),
				)
			}

			return a.authorizeServiceAccount(ctx, requestAuthorizer, attrs)
		case shootResource:
			return a.authorizeShoot(ctx, requestAuthorizer, attrs)
		case shootStateResource:
			return requestAuthorizer.Check(ctx, graph.VertexTypeShootState, attrs,
				authwebhook.WithAllowedVerbs("get", "update", "patch", "delete", "list", "watch"),
				authwebhook.WithAlwaysAllowedVerbs("create"),
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

func (a *authorizer) ConditionsAwareAuthorize(ctx context.Context, attrs auth.Attributes) auth.ConditionsAwareDecision {
	return auth.ConditionsAwareDecisionFromParts(a.Authorize(ctx, attrs))
}

func (a *authorizer) EvaluateConditions(_ context.Context, _ auth.ConditionsAwareDecision, _ auth.ConditionsData) (auth.Decision, string, error) {
	return auth.DecisionDeny, "", auth.ErrorConditionEvaluationNotSupported
}

func (a *authorizer) authorizeClusterRoleBinding(ctx context.Context, requestAuthorizer *authwebhook.RequestAuthorizer, attrs auth.Attributes) (auth.Decision, string, error) {
	// Allow gardenlet to delete its cluster role binding after bootstrapping (in this case, there is no `Seed` resource
	// in the system yet, so we can't rely on the graph).
	if attrs.GetVerb() == "delete" &&
		strings.HasPrefix(attrs.GetName(), gardenletbootstraputil.ClusterRoleBindingNamePrefix) {
		managedSeedNamespace, managedSeedName := gardenletbootstraputil.ManagedSeedInfoFromClusterRoleBindingName(attrs.GetName())
		if managedSeedNamespace == v1beta1constants.GardenNamespace && managedSeedName == requestAuthorizer.ToName {
			return auth.DecisionAllow, "", nil
		}
	}

	return requestAuthorizer.Check(ctx, graph.VertexTypeClusterRoleBinding, attrs,
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

func (a *authorizer) authorizeLease(ctx context.Context, requestAuthorizer *authwebhook.RequestAuthorizer, userType gardenletidentity.UserType, attrs auth.Attributes) (auth.Decision, string, error) {
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

	return requestAuthorizer.Check(ctx, graph.VertexTypeLease, attrs,
		authwebhook.WithAllowedVerbs("get", "update", "patch", "list", "watch"),
		authwebhook.WithAlwaysAllowedVerbs("create"),
	)
}

func (a *authorizer) authorizeSecret(ctx context.Context, requestAuthorizer *authwebhook.RequestAuthorizer, attrs auth.Attributes) (auth.Decision, string, error) {
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

	return requestAuthorizer.Check(ctx, graph.VertexTypeSecret, attrs,
		authwebhook.WithAllowedVerbs("get", "patch", "update", "delete"),
		authwebhook.WithAlwaysAllowedVerbs("create"),
	)
}

func (a *authorizer) authorizeConfigMap(ctx context.Context, requestAuthorizer *authwebhook.RequestAuthorizer, attrs auth.Attributes) (auth.Decision, string, error) {
	if attrs.GetVerb() == "get" &&
		attrs.GetNamespace() == gardencorev1beta1.GardenerSystemPublicNamespace &&
		attrs.GetName() == v1beta1constants.ConfigMapNameGardenerInfo {
		return auth.DecisionAllow, "", nil
	}

	return requestAuthorizer.Check(ctx, graph.VertexTypeConfigMap, attrs,
		authwebhook.WithAllowedVerbs("get", "patch", "update", "delete", "list", "watch"),
		authwebhook.WithAlwaysAllowedVerbs("create"),
	)
}

func (a *authorizer) authorizeShoot(ctx context.Context, requestAuthorizer *authwebhook.RequestAuthorizer, attrs auth.Attributes) (auth.Decision, string, error) {
	// Allow gardenlet to get its own Shoot when the seed cluster is a self-hosted shoot.
	if attrs.GetVerb() == "get" &&
		attrs.GetNamespace() == v1beta1constants.GardenNamespace &&
		attrs.GetName() == requestAuthorizer.ToName {
		return auth.DecisionAllow, "", nil
	}

	return requestAuthorizer.Check(ctx, graph.VertexTypeShoot, attrs,
		authwebhook.WithAllowedVerbs("get", "list", "watch", "update", "patch"),
		authwebhook.WithAllowedSubresources("status", "finalizers"),
		authwebhook.WithLabelSelectors(map[string]string{v1beta1constants.LabelPrefixSeedName + requestAuthorizer.ToName: "true"}),
	)
}

func (a *authorizer) authorizeServiceAccount(ctx context.Context, requestAuthorizer *authwebhook.RequestAuthorizer, attrs auth.Attributes) (auth.Decision, string, error) {
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

	return requestAuthorizer.Check(ctx, graph.VertexTypeServiceAccount, attrs,
		authwebhook.WithAllowedVerbs("get", "patch", "update"),
		authwebhook.WithAlwaysAllowedVerbs("create"),
	)
}
