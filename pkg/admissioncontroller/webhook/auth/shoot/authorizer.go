// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/apiserver/pkg/authentication/user"
	auth "k8s.io/apiserver/pkg/authorization/authorizer"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity"
	shootidentity "github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity/shoot"
	authwebhook "github.com/gardener/gardener/pkg/admissioncontroller/webhook/auth"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/operations"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	gardenletutils "github.com/gardener/gardener/pkg/utils/gardener/gardenlet"
	"github.com/gardener/gardener/pkg/utils/graph"
	authorizerwebhook "github.com/gardener/gardener/pkg/webhook/authorizer"
)

// NewAuthorizer returns a new authorizer for requests from gardenlets running in self-hosted shoots.
func NewAuthorizer(logger logr.Logger, c client.Client, graph graph.Interface, authorizeWithSelectors authorizerwebhook.WithSelectorsChecker, seedAuthorizer auth.Authorizer) *authorizer {
	return &authorizer{
		logger:                 logger,
		client:                 c,
		graph:                  graph,
		authorizeWithSelectors: authorizeWithSelectors,
		seedAuthorizer:         seedAuthorizer,
	}
}

type authorizer struct {
	logger                 logr.Logger
	client                 client.Client
	graph                  graph.Interface
	authorizeWithSelectors authorizerwebhook.WithSelectorsChecker

	// seedAuthorizer is used when the self-hosted shoot has been promoted to a seed cluster. The extensions still run
	// with their self-hosted shoot identity, but we want them to have the same privileges that are granted by the seed
	// authorizer.
	seedAuthorizer auth.Authorizer
}

var (
	_ = auth.Authorizer(&authorizer{})

	// Only take v1beta1 for the core.gardener.cloud API group because the Authorize function only checks the resource
	// group and the resource (but it ignores the version).
	backupBucketResource              = gardencorev1beta1.Resource("backupbuckets")
	backupEntryResource               = gardencorev1beta1.Resource("backupentries")
	bastionResource                   = operationsv1alpha1.Resource("bastions")
	certificateSigningRequestResource = certificatesv1.Resource("certificatesigningrequests")
	cloudProfileResource              = gardencorev1beta1.Resource("cloudprofiles")
	configMapResource                 = corev1.Resource("configmaps")
	controllerDeploymentResource      = gardencorev1beta1.Resource("controllerdeployments")
	controllerInstallationResource    = gardencorev1beta1.Resource("controllerinstallations")
	controllerRegistrationResource    = gardencorev1beta1.Resource("controllerregistrations")
	credentialsBindingResource        = securityv1alpha1.Resource("credentialsbindings")
	eventCoreResource                 = corev1.Resource("events")
	eventResource                     = eventsv1.Resource("events")
	gardenletResource                 = seedmanagementv1alpha1.Resource("gardenlets")
	leaseResource                     = coordinationv1.Resource("leases")
	managedSeedResource               = seedmanagementv1alpha1.Resource("managedseeds")
	namespaceResource                 = corev1.Resource("namespaces")
	projectResource                   = gardencorev1beta1.Resource("projects")
	secretResource                    = corev1.Resource("secrets")
	secretBindingResource             = gardencorev1beta1.Resource("secretbindings")
	seedResource                      = gardencorev1beta1.Resource("seeds")
	serviceAccountResource            = corev1.Resource("serviceaccounts")
	shootResource                     = gardencorev1beta1.Resource("shoots")
	shootStateResource                = gardencorev1beta1.Resource("shootstates")
	workloadIdentityResource          = securityv1alpha1.Resource("workloadidentities")
)

func (a *authorizer) Authorize(ctx context.Context, attrs auth.Attributes) (auth.Decision, string, error) {
	shootNamespace, shootName, isSelfHostedShoot, userType := shootidentity.FromUserInfoInterface(attrs.GetUser())
	if !isSelfHostedShoot {
		return auth.DecisionNoOpinion, "", nil
	}

	var (
		log               = a.logger.WithValues("shootNamespace", shootNamespace, "shootName", shootName, "attributes", fmt.Sprintf("%#v", attrs), "userType", userType)
		requestAuthorizer = &authwebhook.RequestAuthorizer{
			Log:                    log,
			Graph:                  a.graph,
			AuthorizeWithSelectors: a.authorizeWithSelectors,
			ToType:                 graph.VertexTypeShoot,
			ToNamespace:            shootNamespace,
			ToName:                 shootName,
		}
	)

	if userType == gardenletidentity.UserTypeGardenadm {
		return a.authorizeGardenadmRequests(log, shootNamespace, shootName, attrs)
	}

	// When the self-hosted shoot has been promoted to a seed, delegate to the seed authorizer for extension requests.
	// The extensions still authenticate with their shoot identity, but they need seed-level privileges for managing
	// seed resources. We construct a synthetic ServiceAccount identity matching the seed namespace so that the seed
	// authorizer recognizes it as an extension. This only applies to shoots in the garden namespace since only those
	// can be promoted to seeds.
	if userType == gardenletidentity.UserTypeExtension && shootNamespace == v1beta1constants.GardenNamespace && a.seedAuthorizer != nil && a.graph.HasVertex(graph.VertexTypeSeed, "", shootName) {
		seedNamespace := gardenerutils.ComputeGardenNamespace(shootName)
		decision, reason, err := a.seedAuthorizer.Authorize(ctx, auth.AttributesRecord{
			User: &user.DefaultInfo{
				Name:   serviceaccount.MakeUsername(seedNamespace, v1beta1constants.ExtensionGardenServiceAccountPrefix+"delegated"),
				Groups: []string{serviceaccount.AllServiceAccountsGroup, serviceaccount.MakeNamespaceGroupName(seedNamespace)},
			},
			Verb:            attrs.GetVerb(),
			Namespace:       attrs.GetNamespace(),
			APIGroup:        attrs.GetAPIGroup(),
			APIVersion:      attrs.GetAPIVersion(),
			Resource:        attrs.GetResource(),
			Subresource:     attrs.GetSubresource(),
			Name:            attrs.GetName(),
			ResourceRequest: attrs.IsResourceRequest(),
			Path:            attrs.GetPath(),
		})
		if err != nil {
			return auth.DecisionNoOpinion, "", fmt.Errorf("failed delegating request %+v to seed authorizer: %w", attrs, err)
		}
		if decision == auth.DecisionAllow {
			return decision, reason, nil
		}
	}

	if attrs.IsResourceRequest() {
		requestResource := schema.GroupResource{Group: attrs.GetAPIGroup(), Resource: attrs.GetResource()}
		switch requestResource {
		case backupBucketResource:
			return requestAuthorizer.Check(graph.VertexTypeBackupBucket, attrs,
				authwebhook.WithAllowedVerbs("get", "list", "watch", "update", "patch", "delete"),
				authwebhook.WithAlwaysAllowedVerbs("create"),
				authwebhook.WithAllowedSubresources("status", "finalizers"),
				authwebhook.WithFieldSelectors(map[string]string{
					core.BackupBucketShootRefName:      shootName,
					core.BackupBucketShootRefNamespace: shootNamespace,
				}),
			)

		case backupEntryResource:
			return requestAuthorizer.Check(graph.VertexTypeBackupEntry, attrs,
				authwebhook.WithAllowedVerbs("get", "list", "watch", "update", "patch", "delete"),
				authwebhook.WithAlwaysAllowedVerbs("create"),
				authwebhook.WithAllowedSubresources("status"),
				authwebhook.WithFieldSelectors(map[string]string{
					core.BackupEntryShootRefName:      shootName,
					core.BackupEntryShootRefNamespace: shootNamespace,
				}),
			)

		case bastionResource:
			return requestAuthorizer.Check(graph.VertexTypeBastion, attrs,
				authwebhook.WithAllowedVerbs("get", "list", "watch", "update", "patch"),
				authwebhook.WithAllowedSubresources("status"),
				authwebhook.WithAllowedNamespaces(requestAuthorizer.ToNamespace),
				authwebhook.WithFieldSelectors(map[string]string{
					operations.BastionShootName: shootName,
				}),
			)

		case certificateSigningRequestResource:
			if userType == gardenletidentity.UserTypeExtension {
				return requestAuthorizer.CheckRead(graph.VertexTypeCertificateSigningRequest, attrs)
			}

			return requestAuthorizer.Check(graph.VertexTypeCertificateSigningRequest, attrs,
				authwebhook.WithAllowedVerbs("get", "list", "watch"),
				authwebhook.WithAlwaysAllowedVerbs("create"),
				authwebhook.WithAllowedSubresources("shootclient"),
			)

		case configMapResource:
			return requestAuthorizer.CheckRead(graph.VertexTypeConfigMap, attrs)

		case controllerDeploymentResource:
			return requestAuthorizer.CheckRead(graph.VertexTypeControllerDeployment, attrs)

		case controllerInstallationResource:
			return requestAuthorizer.Check(graph.VertexTypeControllerInstallation, attrs,
				authwebhook.WithAllowedVerbs("get", "list", "watch", "update", "patch"),
				authwebhook.WithAllowedSubresources("status"),
				authwebhook.WithFieldSelectors(map[string]string{
					core.ShootRefName:      shootName,
					core.ShootRefNamespace: shootNamespace,
				}),
			)

		case controllerRegistrationResource:
			return requestAuthorizer.CheckRead(graph.VertexTypeControllerRegistration, attrs)

		case eventCoreResource, eventResource:
			return a.authorizeEvent(log, attrs)

		case gardenletResource:
			return requestAuthorizer.Check(graph.VertexTypeGardenlet, attrs,
				authwebhook.WithAllowedVerbs("get", "list", "watch", "update", "patch"),
				authwebhook.WithAlwaysAllowedVerbs("create"),
				authwebhook.WithAllowedSubresources("status"),
				authwebhook.WithAllowedNamespaces(requestAuthorizer.ToNamespace),
				authwebhook.WithFieldSelectors(map[string]string{metav1.ObjectNameField: gardenletutils.ResourcePrefixSelfHostedShoot + requestAuthorizer.ToName}),
			)

		case leaseResource:
			return a.authorizeLease(requestAuthorizer, userType, shootNamespace, shootName, attrs)

		case managedSeedResource:
			return requestAuthorizer.Check(graph.VertexTypeManagedSeed, attrs,
				authwebhook.WithAllowedVerbs("get", "list", "watch", "update", "patch"),
				authwebhook.WithAllowedSubresources("status"),
				authwebhook.WithAllowedNamespaces(requestAuthorizer.ToNamespace),
				authwebhook.WithFieldSelectors(map[string]string{
					seedmanagement.ManagedSeedShootName: shootName,
				}),
			)

		case secretResource:
			return a.authorizeSecret(ctx, requestAuthorizer, attrs)

		case seedResource:
			// The self-hosted shoot gardenlet needs read access for the Seed whose name matches its shoot name.
			if slices.Contains([]string{"get", "list", "watch"}, attrs.GetVerb()) &&
				(attrs.GetName() == "" || attrs.GetName() == shootName) {
				return auth.DecisionAllow, "", nil
			}

			return requestAuthorizer.Check(graph.VertexTypeSeed, attrs,
				authwebhook.WithAllowedVerbs("get", "delete"),
				authwebhook.WithAlwaysAllowedVerbs("list", "watch"),
			)

		case namespaceResource:
			return requestAuthorizer.CheckRead(graph.VertexTypeNamespace, attrs)

		case projectResource:
			return requestAuthorizer.CheckRead(graph.VertexTypeProject, attrs)

		case serviceAccountResource:
			if userType == gardenletidentity.UserTypeExtension {
				return requestAuthorizer.Check(graph.VertexTypeServiceAccount, attrs,
					authwebhook.WithAllowedVerbs("get"),
				)
			}

			return a.authorizeServiceAccount(requestAuthorizer, shootNamespace, shootName, attrs)

		case shootResource:
			// This allows the gardenlet to read its own Shoot resource even if it does not yet exist in the system.
			// For other verbs, the graph-based authorization takes over.
			if slices.Contains([]string{"get", "list", "watch"}, attrs.GetVerb()) &&
				attrs.GetName() == shootName && attrs.GetNamespace() == shootNamespace {
				return auth.DecisionAllow, "", nil
			}

			return requestAuthorizer.Check(graph.VertexTypeShoot, attrs,
				authwebhook.WithAllowedVerbs("update", "patch"),
				authwebhook.WithAllowedSubresources("status"),
			)

		case shootStateResource:
			return requestAuthorizer.Check(graph.VertexTypeShootState, attrs,
				authwebhook.WithAllowedVerbs("get", "list", "watch", "patch"),
				authwebhook.WithAlwaysAllowedVerbs("create"),
			)

		case workloadIdentityResource:
			return requestAuthorizer.Check(graph.VertexTypeWorkloadIdentity, attrs,
				authwebhook.WithAllowedVerbs("get", "list", "watch", "create", "patch"),
				authwebhook.WithAllowedSubresources("token"),
			)

		default:
			log.Info(
				"Unhandled resource request",
				"group", attrs.GetAPIGroup(),
				"version", attrs.GetAPIVersion(),
				"resource", attrs.GetResource(),
				"verb", attrs.GetVerb(),
			)
		}
	}

	return auth.DecisionNoOpinion, "", nil
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

func (a *authorizer) authorizeLease(requestAuthorizer *authwebhook.RequestAuthorizer, userType gardenletidentity.UserType, shootNamespace, shootName string, attrs auth.Attributes) (auth.Decision, string, error) {
	// Extension clients may only work with leases in the shoot namespace and whose name is prefixed with
	// the shoot name to avoid tampering with leases belonging to other shoots in the same project namespace.
	if userType == gardenletidentity.UserTypeExtension {
		if attrs.GetNamespace() != shootNamespace {
			return auth.DecisionNoOpinion, "lease object is not in shoot namespace", nil
		}
		// List/watch have no name; for all other verbs check the name prefix.
		if attrs.GetName() != "" && !strings.HasPrefix(attrs.GetName(), shootName+"--") {
			return auth.DecisionNoOpinion, "lease object name does not have the shoot name as prefix", nil
		}
		if ok, reason := authwebhook.CheckVerb(requestAuthorizer.Log, attrs, "create", "get", "list", "watch", "update", "patch", "delete", "deletecollection"); !ok {
			return auth.DecisionNoOpinion, reason, nil
		}
		return auth.DecisionAllow, "", nil
	}

	return requestAuthorizer.Check(graph.VertexTypeLease, attrs,
		authwebhook.WithAllowedVerbs("get", "update", "patch", "list", "watch"),
		authwebhook.WithAlwaysAllowedVerbs("create"),
	)
}

func (a *authorizer) authorizeServiceAccount(requestAuthorizer *authwebhook.RequestAuthorizer, shootNamespace, shootName string, attrs auth.Attributes) (auth.Decision, string, error) {
	// Allow all verbs for extension ServiceAccounts belonging to this shoot in the shoot's project namespace.
	// The name must be prefixed with extension-shoot--<shootName>-- to scope to this shoot and prevent a
	// gardenlet from accessing unrelated ServiceAccounts of other shoots sharing the same project namespace.
	if attrs.GetNamespace() == shootNamespace &&
		strings.HasPrefix(attrs.GetName(), v1beta1constants.ExtensionShootServiceAccountPrefix+shootName+"--") {
		return auth.DecisionAllow, "", nil
	}

	return requestAuthorizer.Check(graph.VertexTypeServiceAccount, attrs,
		authwebhook.WithAllowedVerbs("get", "patch", "update"),
		authwebhook.WithAlwaysAllowedVerbs("create"),
	)
}

func (a *authorizer) authorizeSecret(ctx context.Context, requestAuthorizer *authwebhook.RequestAuthorizer, attrs auth.Attributes) (auth.Decision, string, error) {
	// Allow gardenlet to delete bootstrap tokens for its own shoot or for ManagedSeeds referencing its shoot.
	if attrs.GetVerb() == "delete" && attrs.GetNamespace() == metav1.NamespaceSystem && strings.HasPrefix(attrs.GetName(), bootstraptokenapi.BootstrapTokenSecretPrefix) {
		shootMeta, found, err := gardenletutils.ShootMetaFromBootstrapToken(ctx, a.client, attrs.GetName())
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return auth.DecisionNoOpinion, "", err
			}
		} else if found {
			if shootMeta.Namespace == requestAuthorizer.ToNamespace && shootMeta.Name == requestAuthorizer.ToName {
				return auth.DecisionAllow, "", nil
			}
			return auth.DecisionNoOpinion, fmt.Sprintf("shoot meta in bootstrap token secret %s does not match with identity of requestor %s/%s", shootMeta, requestAuthorizer.ToNamespace, requestAuthorizer.ToName), nil
		}
		// No shoot meta found — fall through to graph-based authorization which handles ManagedSeed bootstrap
		// tokens via the Secret → ManagedSeed → Shoot edges.
	}

	return requestAuthorizer.Check(graph.VertexTypeSecret, attrs,
		authwebhook.WithAllowedVerbs("get", "list", "watch", "patch", "update", "delete"),
		authwebhook.WithAlwaysAllowedVerbs("create"),
	)
}
