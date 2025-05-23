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

	"github.com/gardener/gardener/pkg/admissioncontroller/seedidentity"
	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/auth/seed/graph"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	gardenletbootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/kubernetes/bootstraptoken"
)

// NewAuthorizer returns a new authorizer for requests from gardenlets. It never has an opinion on the request.
func NewAuthorizer(logger logr.Logger, graph graph.Interface) *authorizer {
	return &authorizer{
		logger: logger,
		graph:  graph,
	}
}

type authorizer struct {
	logger logr.Logger
	graph  graph.Interface
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
			return a.authorize(requestLog, seedName, graph.VertexTypeBackupBucket, attrs,
				[]string{"update", "patch", "delete"},
				[]string{"create", "get", "list", "watch"},
				[]string{"status"},
			)
		case backupEntryResource:
			return a.authorize(requestLog, seedName, graph.VertexTypeBackupEntry, attrs,
				[]string{"update", "patch", "delete"},
				[]string{"create", "get", "list", "watch"},
				[]string{"status"},
			)
		case bastionResource:
			return a.authorize(requestLog, seedName, graph.VertexTypeBastion, attrs,
				[]string{"update", "patch"},
				[]string{"create", "get", "list", "watch"},
				[]string{"status"},
			)
		case certificateSigningRequestResource:
			if userType == seedidentity.UserTypeExtension {
				return a.authorizeRead(requestLog, seedName, graph.VertexTypeCertificateSigningRequest, attrs)
			}

			return a.authorize(requestLog, seedName, graph.VertexTypeCertificateSigningRequest, attrs,
				[]string{"get", "list", "watch"},
				[]string{"create"},
				[]string{"seedclient"},
			)
		case cloudProfileResource:
			return a.authorizeRead(requestLog, seedName, graph.VertexTypeCloudProfile, attrs)
		case namespacedCloudProfileResource:
			return a.authorizeRead(requestLog, seedName, graph.VertexTypeNamespacedCloudProfile, attrs)
		case clusterRoleBindingResource:
			if userType == seedidentity.UserTypeExtension {
				// We don't use authorizeRead here, as it would also grant list and watch permissions, which gardenlet doesn't
				// have. We want to grant the read-only subset of gardenlet's permissions.
				return a.authorize(requestLog, seedName, graph.VertexTypeClusterRoleBinding, attrs,
					[]string{"get"},
					nil,
					nil,
				)
			}

			return a.authorizeClusterRoleBinding(requestLog, seedName, attrs)
		case configMapResource:
			return a.authorizeConfigMap(requestLog, seedName, attrs)
		case controllerDeploymentResource:
			return a.authorizeRead(requestLog, seedName, graph.VertexTypeControllerDeployment, attrs)
		case controllerInstallationResource:
			return a.authorize(requestLog, seedName, graph.VertexTypeControllerInstallation, attrs,
				[]string{"update", "patch"},
				[]string{"get", "list", "watch"},
				[]string{"status"},
			)
		case controllerRegistrationResource:
			return a.authorize(requestLog, seedName, graph.VertexTypeControllerRegistration, attrs,
				nil,
				[]string{"get", "list", "watch"},
				nil,
			)
		case eventCoreResource, eventResource:
			return a.authorizeEvent(requestLog, attrs)
		case exposureClassResource:
			return a.authorizeRead(requestLog, seedName, graph.VertexTypeExposureClass, attrs)
		case internalSecretResource:
			return a.authorize(requestLog, seedName, graph.VertexTypeInternalSecret, attrs,
				[]string{"get", "update", "patch", "delete", "list", "watch"},
				[]string{"create"},
				nil,
			)
		case leaseResource:
			return a.authorizeLease(requestLog, seedName, userType, attrs)
		case gardenletResource:
			return a.authorize(requestLog, seedName, graph.VertexTypeGardenlet, attrs,
				[]string{"update", "patch"},
				[]string{"get", "list", "watch", "create"},
				[]string{"status"},
			)
		case managedSeedResource:
			return a.authorize(requestLog, seedName, graph.VertexTypeManagedSeed, attrs,
				[]string{"update", "patch"},
				[]string{"get", "list", "watch"},
				[]string{"status"},
			)
		case namespaceResource:
			return a.authorizeRead(requestLog, seedName, graph.VertexTypeNamespace, attrs)
		case projectResource:
			return a.authorizeRead(requestLog, seedName, graph.VertexTypeProject, attrs)
		case secretBindingResource:
			return a.authorizeRead(requestLog, seedName, graph.VertexTypeSecretBinding, attrs)
		case credentialsBindingResource:
			return a.authorizeRead(requestLog, seedName, graph.VertexTypeCredentialsBinding, attrs)
		case secretResource:
			return a.authorizeSecret(requestLog, seedName, attrs)
		case workloadIdentityResource:
			return a.authorize(requestLog, seedName, graph.VertexTypeWorkloadIdentity, attrs,
				[]string{"get", "list", "watch", "create"},
				nil,
				[]string{"token"},
			)
		case seedResource:
			return a.authorize(requestLog, seedName, graph.VertexTypeSeed, attrs,
				[]string{"update", "patch", "delete"},
				[]string{"create", "get", "list", "watch"},
				[]string{"status"},
			)
		case serviceAccountResource:
			if userType == seedidentity.UserTypeExtension {
				// We don't use authorizeRead here, as it would also grant list and watch permissions, which gardenlet doesn't
				// have. We want to grant the read-only subset of gardenlet's permissions.
				return a.authorize(requestLog, seedName, graph.VertexTypeServiceAccount, attrs,
					[]string{"get"},
					nil,
					nil,
				)
			}

			return a.authorizeServiceAccount(requestLog, seedName, attrs)
		case shootResource:
			return a.authorize(requestLog, seedName, graph.VertexTypeShoot, attrs,
				[]string{"update", "patch"},
				[]string{"get", "list", "watch"},
				[]string{"status"},
			)
		case shootStateResource:
			return a.authorize(requestLog, seedName, graph.VertexTypeShootState, attrs,
				[]string{"get", "update", "patch", "delete", "list", "watch"},
				[]string{"create"},
				nil,
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

	return a.authorize(log, seedName, graph.VertexTypeClusterRoleBinding, attrs,
		[]string{"get", "patch", "update"},
		[]string{"create"},
		nil,
	)
}

func (a *authorizer) authorizeEvent(log logr.Logger, attrs auth.Attributes) (auth.Decision, string, error) {
	if ok, reason := a.checkVerb(log, attrs, "create", "patch"); !ok {
		return auth.DecisionNoOpinion, reason, nil
	}

	if ok, reason := a.checkSubresource(log, attrs); !ok {
		return auth.DecisionNoOpinion, reason, nil
	}

	return auth.DecisionAllow, "", nil
}

func (a *authorizer) authorizeLease(log logr.Logger, seedName string, userType seedidentity.UserType, attrs auth.Attributes) (auth.Decision, string, error) {
	// extension clients may only work with leases in the seed namespace
	if userType == seedidentity.UserTypeExtension {
		if attrs.GetNamespace() == gardenerutils.ComputeGardenNamespace(seedName) {
			if ok, reason := a.checkVerb(log, attrs, "create", "get", "list", "watch", "update", "patch", "delete", "deletecollection"); !ok {
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

	return a.authorize(log, seedName, graph.VertexTypeLease, attrs,
		[]string{"get", "update", "patch", "list", "watch"},
		[]string{"create"},
		nil,
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

	return a.authorize(log, seedName, graph.VertexTypeSecret, attrs,
		[]string{"get", "patch", "update", "delete"},
		[]string{"create"},
		nil,
	)
}

func (a *authorizer) authorizeConfigMap(log logr.Logger, seedName string, attrs auth.Attributes) (auth.Decision, string, error) {
	return a.authorize(log, seedName, graph.VertexTypeConfigMap, attrs,
		[]string{"get", "patch", "update", "delete", "list", "watch"},
		[]string{"create"},
		nil,
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

	return a.authorize(log, seedName, graph.VertexTypeServiceAccount, attrs,
		[]string{"get", "patch", "update"},
		[]string{"create"},
		nil,
	)
}

func (a *authorizer) authorizeRead(log logr.Logger, seedName string, fromType graph.VertexType, attrs auth.Attributes) (auth.Decision, string, error) {
	return a.authorize(log, seedName, fromType, attrs,
		[]string{"get", "list", "watch"},
		nil,
		nil,
	)
}

func (a *authorizer) authorize(
	log logr.Logger,
	seedName string,
	fromType graph.VertexType,
	attrs auth.Attributes,
	allowedVerbs []string,
	alwaysAllowedVerbs []string,
	allowedSubresources []string,
) (
	auth.Decision,
	string,
	error,
) {
	if ok, reason := a.checkSubresource(log, attrs, allowedSubresources...); !ok {
		return auth.DecisionNoOpinion, reason, nil
	}

	// When a new object is created then it doesn't yet exist in the graph, so usually such requests are always allowed
	// as the 'create case' is typically handled in the SeedRestriction admission handler. Similarly, resources for
	// which the gardenlet has a controller need to be listed/watched, so those verbs would also be allowed here.
	if slices.Contains(alwaysAllowedVerbs, attrs.GetVerb()) {
		return auth.DecisionAllow, "", nil
	}

	if ok, reason := a.checkVerb(log, attrs, append(alwaysAllowedVerbs, allowedVerbs...)...); !ok {
		return auth.DecisionNoOpinion, reason, nil
	}

	return a.hasPathFrom(log, seedName, fromType, attrs)
}

func (a *authorizer) hasPathFrom(log logr.Logger, seedName string, fromType graph.VertexType, attrs auth.Attributes) (auth.Decision, string, error) {
	if len(attrs.GetName()) == 0 {
		log.Info("Denying authorization because attributes are missing object name")
		return auth.DecisionNoOpinion, "No Object name found", nil
	}

	// If the request is made for a namespace then the attributes.Namespace field is not empty. It contains the name of
	// the namespace.
	namespace := attrs.GetNamespace()
	if fromType == graph.VertexTypeNamespace {
		namespace = ""
	}

	// If the vertex does not exist in the graph (i.e., the resource does not exist in the system) then we allow the
	// request.
	if attrs.GetVerb() == "delete" && !a.graph.HasVertex(fromType, namespace, attrs.GetName()) {
		return auth.DecisionAllow, "", nil
	}

	if !a.graph.HasPathFrom(fromType, namespace, attrs.GetName(), graph.VertexTypeSeed, "", seedName) {
		log.Info("Denying authorization because no relationship is found between seed and object")
		return auth.DecisionNoOpinion, fmt.Sprintf("no relationship found between seed '%s' and this object", seedName), nil
	}

	return auth.DecisionAllow, "", nil
}

func (a *authorizer) checkVerb(log logr.Logger, attrs auth.Attributes, allowedVerbs ...string) (bool, string) {
	if !slices.Contains(allowedVerbs, attrs.GetVerb()) {
		log.Info("Denying authorization because verb is not allowed for this resource type", "allowedVerbs", allowedVerbs)
		return false, fmt.Sprintf("only the following verbs are allowed for this resource type: %+v", allowedVerbs)
	}

	return true, ""
}

func (a *authorizer) checkSubresource(log logr.Logger, attrs auth.Attributes, allowedSubresources ...string) (bool, string) {
	if subresource := attrs.GetSubresource(); len(subresource) > 0 && !slices.Contains(allowedSubresources, attrs.GetSubresource()) {
		log.Info("Denying authorization because subresource is not allowed for this resource type", "allowedSubresources", allowedSubresources)
		return false, fmt.Sprintf("only the following subresources are allowed for this resource type: %+v", allowedSubresources)
	}

	return true, ""
}
