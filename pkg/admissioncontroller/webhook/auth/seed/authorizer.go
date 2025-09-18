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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	auth "k8s.io/apiserver/pkg/authorization/authorizer"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"

	"github.com/gardener/gardener/pkg/admissioncontroller/seedidentity"
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
			return a.authorize(requestLog, seedName, graph.VertexTypeBackupBucket, attrs,
				withAllowedVerbs("update", "patch", "delete"),
				withAlwaysAllowedVerbs("create", "get", "list", "watch"),
				withAllowedSubresources("status"),
			)
		case backupEntryResource:
			return a.authorize(requestLog, seedName, graph.VertexTypeBackupEntry, attrs,
				withAllowedVerbs("update", "patch", "delete"),
				withAlwaysAllowedVerbs("create", "get", "list", "watch"),
				withAllowedSubresources("status"),
			)
		case bastionResource:
			return a.authorize(requestLog, seedName, graph.VertexTypeBastion, attrs,
				withAllowedVerbs("update", "patch"),
				withAlwaysAllowedVerbs("create", "get", "list", "watch"),
				withAllowedSubresources("status"),
			)
		case certificateSigningRequestResource:
			if userType == seedidentity.UserTypeExtension {
				return a.authorizeRead(requestLog, seedName, graph.VertexTypeCertificateSigningRequest, attrs)
			}

			return a.authorize(requestLog, seedName, graph.VertexTypeCertificateSigningRequest, attrs,
				withAllowedVerbs("get", "list", "watch"),
				withAlwaysAllowedVerbs("create"),
				withAllowedSubresources("seedclient"),
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
					withAllowedVerbs("get"),
				)
			}

			return a.authorizeClusterRoleBinding(requestLog, seedName, attrs)
		case configMapResource:
			return a.authorizeConfigMap(requestLog, seedName, attrs)
		case controllerDeploymentResource:
			return a.authorizeRead(requestLog, seedName, graph.VertexTypeControllerDeployment, attrs)
		case controllerInstallationResource:
			return a.authorize(requestLog, seedName, graph.VertexTypeControllerInstallation, attrs,
				withAllowedVerbs("update", "patch"),
				withAlwaysAllowedVerbs("get", "list", "watch"),
				withAllowedSubresources("status"),
			)
		case controllerRegistrationResource:
			return a.authorize(requestLog, seedName, graph.VertexTypeControllerRegistration, attrs,
				withAlwaysAllowedVerbs("get", "list", "watch"),
			)
		case credentialsBindingResource:
			return a.authorizeRead(requestLog, seedName, graph.VertexTypeCredentialsBinding, attrs)
		case eventCoreResource, eventResource:
			return a.authorizeEvent(requestLog, attrs)
		case exposureClassResource:
			return a.authorizeRead(requestLog, seedName, graph.VertexTypeExposureClass, attrs)
		case internalSecretResource:
			return a.authorize(requestLog, seedName, graph.VertexTypeInternalSecret, attrs,
				withAllowedVerbs("get", "update", "patch", "delete", "list", "watch"),
				withAlwaysAllowedVerbs("create"),
			)
		case leaseResource:
			return a.authorizeLease(requestLog, seedName, userType, attrs)
		case gardenletResource:
			return a.authorize(requestLog, seedName, graph.VertexTypeGardenlet, attrs,
				withAllowedVerbs("update", "patch"),
				withAlwaysAllowedVerbs("get", "list", "watch", "create"),
				withAllowedSubresources("status"),
			)
		case managedSeedResource:
			return a.authorize(requestLog, seedName, graph.VertexTypeManagedSeed, attrs,
				withAllowedVerbs("update", "patch"),
				withAlwaysAllowedVerbs("get", "list", "watch"),
				withAllowedSubresources("status"),
			)
		case namespaceResource:
			return a.authorizeRead(requestLog, seedName, graph.VertexTypeNamespace, attrs)
		case projectResource:
			return a.authorizeRead(requestLog, seedName, graph.VertexTypeProject, attrs)
		case secretBindingResource:
			return a.authorizeRead(requestLog, seedName, graph.VertexTypeSecretBinding, attrs)
		case secretResource:
			return a.authorizeSecret(requestLog, seedName, attrs)
		case workloadIdentityResource:
			return a.authorize(requestLog, seedName, graph.VertexTypeWorkloadIdentity, attrs,
				withAllowedVerbs("get", "list", "watch", "create", "patch"),
				withAllowedSubresources("token"),
			)
		case seedResource:
			return a.authorize(requestLog, seedName, graph.VertexTypeSeed, attrs,
				withAllowedVerbs("update", "patch", "delete"),
				withAlwaysAllowedVerbs("create", "get", "list", "watch"),
				withAllowedSubresources("status"),
			)
		case serviceAccountResource:
			if userType == seedidentity.UserTypeExtension {
				// We don't use authorizeRead here, as it would also grant list and watch permissions, which gardenlet doesn't
				// have. We want to grant the read-only subset of gardenlet's permissions.
				return a.authorize(requestLog, seedName, graph.VertexTypeServiceAccount, attrs,
					withAllowedVerbs("get"),
				)
			}

			return a.authorizeServiceAccount(requestLog, seedName, attrs)
		case shootResource:
			return a.authorize(requestLog, seedName, graph.VertexTypeShoot, attrs,
				withAllowedVerbs("update", "patch"),
				withAlwaysAllowedVerbs("get", "list", "watch"),
				withAllowedSubresources("status"),
			)
		case shootStateResource:
			return a.authorize(requestLog, seedName, graph.VertexTypeShootState, attrs,
				withAllowedVerbs("get", "update", "patch", "delete", "list", "watch"),
				withAlwaysAllowedVerbs("create"),
			)
		case workloadIdentityResource:
			return a.authorize(requestLog, seedName, graph.VertexTypeWorkloadIdentity, attrs,
				withAllowedVerbs("get", "list", "watch", "create"),
				withAllowedSubresources("token"),
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
		withAllowedVerbs("get", "patch", "update"),
		withAlwaysAllowedVerbs("create"),
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
		withAllowedVerbs("get", "update", "patch", "list", "watch"),
		withAlwaysAllowedVerbs("create"),
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
		withAllowedVerbs("get", "patch", "update", "delete"),
		withAlwaysAllowedVerbs("create"),
	)
}

func (a *authorizer) authorizeConfigMap(log logr.Logger, seedName string, attrs auth.Attributes) (auth.Decision, string, error) {
	if attrs.GetVerb() == "get" &&
		attrs.GetNamespace() == gardencorev1beta1.GardenerSystemPublicNamespace &&
		attrs.GetName() == v1beta1constants.ConfigMapNameGardenerInfo {
		return auth.DecisionAllow, "", nil
	}

	return a.authorize(log, seedName, graph.VertexTypeConfigMap, attrs,
		withAllowedVerbs("get", "patch", "update", "delete", "list", "watch"),
		withAlwaysAllowedVerbs("create"),
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
		withAllowedVerbs("get", "patch", "update"),
		withAlwaysAllowedVerbs("create"),
	)
}

func (a *authorizer) authorizeRead(log logr.Logger, seedName string, fromType graph.VertexType, attrs auth.Attributes) (auth.Decision, string, error) {
	return a.authorize(log, seedName, fromType, attrs,
		withAllowedVerbs("get", "list", "watch"),
	)
}

type authzRequest struct {
	allowedVerbs          sets.Set[string]
	alwaysAllowedVerbs    sets.Set[string]
	allowedSubresources   sets.Set[string]
	listWatchSeedSelector seedSelector
}

func newAuthzRequest() *authzRequest {
	return &authzRequest{
		allowedVerbs:          sets.Set[string]{},
		alwaysAllowedVerbs:    sets.Set[string]{},
		allowedSubresources:   sets.Set[string]{},
		listWatchSeedSelector: seedSelector{fieldNames: sets.Set[string]{}, labelKeys: sets.Set[string]{}},
	}
}

type seedSelector struct {
	fieldNames sets.Set[string]
	labelKeys  sets.Set[string]
}

type configFunc func(req *authzRequest)

func (a *authorizer) authorize(
	log logr.Logger,
	seedName string,
	fromType graph.VertexType,
	attrs auth.Attributes,
	fns ...configFunc,
) (
	auth.Decision,
	string,
	error,
) {
	req := newAuthzRequest()
	for _, f := range fns {
		f(req)
	}

	if ok, reason := a.checkSubresource(log, attrs, sets.List(req.allowedSubresources)...); !ok {
		return auth.DecisionNoOpinion, reason, nil
	}

	// When a new object is created then it doesn't yet exist in the graph, so usually such requests are always allowed
	// as the 'create case' is typically handled in the SeedRestriction admission handler. Similarly, resources for
	// which the gardenlet has a controller need to be listed/watched, so those verbs would also be allowed here.
	if req.alwaysAllowedVerbs.Has(attrs.GetVerb()) {
		return auth.DecisionAllow, "", nil
	}

	if ok, reason := a.checkVerb(log, attrs, sets.List(req.alwaysAllowedVerbs.Union(req.allowedVerbs))...); !ok {
		return auth.DecisionNoOpinion, reason, nil
	}

	if (attrs.GetVerb() == "list" || attrs.GetVerb() == "watch") &&
		// A resource name is also set when a specific object is read with `.metadata.name` field selector (e.g., in the
		// single object cache), even for the list verb.
		// If we have a resource name then we want to consult the graph. There is an exception, though, which is when
		// the request specifies `.metadata.name` as field name for a seed selector. This means that the client wants
		// to list/watch the resource with the seed name as field selector. This is a valid scenario and needs to be
		// handled by the below check function.
		(attrs.GetName() == "" || req.listWatchSeedSelector.fieldNames.Has(metav1.ObjectNameField)) {
		if canAuthorizeWithSelectors, err := a.authorizeWithSelectors.IsPossible(); err != nil {
			return auth.DecisionNoOpinion, "", fmt.Errorf("failed checking if authorization with selectors is possible: %w", err)
		} else if canAuthorizeWithSelectors {
			if ok, reason := a.checkListWatchRequests(attrs, seedName, req.listWatchSeedSelector); !ok {
				log.Info("Denying list/watch request because field/label selectors don't select seed name", "fieldNames", req.listWatchSeedSelector.fieldNames, "labelKeys", req.listWatchSeedSelector.labelKeys)
				return auth.DecisionNoOpinion, reason, nil
			} else {
				return auth.DecisionAllow, "", nil
			}
		} else if len(req.listWatchSeedSelector.labelKeys) > 0 || len(req.listWatchSeedSelector.fieldNames) > 0 {
			// TODO(rfranzke): Remove this else-if branch once the lowest supported Kubernetes version is 1.34.
			return auth.DecisionAllow, "", nil
		}
	}

	return a.hasPathFrom(log, seedName, fromType, attrs)
}

func withAllowedVerbs(verbs ...string) configFunc {
	return func(req *authzRequest) {
		req.allowedVerbs.Insert(verbs...)
	}
}

func withAlwaysAllowedVerbs(verbs ...string) configFunc {
	return func(req *authzRequest) {
		req.alwaysAllowedVerbs.Insert(verbs...)
	}
}

func withAllowedSubresources(resources ...string) configFunc {
	return func(req *authzRequest) {
		req.allowedSubresources.Insert(resources...)
	}
}

// TODO: Remove this 'nolint' annotation once the function is used.
//
//nolint:unused
func withSeedFieldSelectorFields(fieldNames ...string) configFunc {
	return func(req *authzRequest) {
		req.listWatchSeedSelector.fieldNames.Insert(fieldNames...)
	}
}

// TODO: Remove this 'nolint' annotation once the function is used.
//
//nolint:unused
func withSeedLabelSelectorKeys(labelKeys ...string) configFunc {
	return func(req *authzRequest) {
		req.listWatchSeedSelector.labelKeys.Insert(labelKeys...)
	}
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

func (a *authorizer) checkListWatchRequests(attrs auth.Attributes, seedName string, seedSelector seedSelector) (bool, string) {
	// The authorization request originates from the kube-apiserver. It has already parsed the field/label selector
	// and converted it to {fields,labels}.Requirements. Hence, it is safe to ignore the error here. Furthermore, we
	// require at least one selector. When the parsing failed, the list of selectors would be empty, resulting in
	// below code to deny the request anyway.
	fieldSelectorRequirements, _ := attrs.GetFieldSelector()
	labelSelectorRequirements, _ := attrs.GetLabelSelector()

	for _, req := range fieldSelectorRequirements {
		if (req.Operator == selection.Equals || req.Operator == selection.DoubleEquals || req.Operator == selection.In) &&
			req.Value == seedName &&
			seedSelector.fieldNames.Has(req.Field) {
			return true, "field selector provided and matches seed name"
		}
	}

	for _, req := range labelSelectorRequirements {
		for key := range seedSelector.labelKeys {
			if req.Matches(labels.Set{key: "true"}) {
				return true, "label selector provided and matches seed name"
			}
		}
	}

	return false, "must specify field or label selector for seed name " + seedName
}
