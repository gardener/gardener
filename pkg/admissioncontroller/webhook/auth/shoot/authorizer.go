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
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	auth "k8s.io/apiserver/pkg/authorization/authorizer"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity"
	shootidentity "github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity/shoot"
	authwebhook "github.com/gardener/gardener/pkg/admissioncontroller/webhook/auth"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	gardenletutils "github.com/gardener/gardener/pkg/utils/gardener/gardenlet"
	"github.com/gardener/gardener/pkg/utils/graph"
	authorizerwebhook "github.com/gardener/gardener/pkg/webhook/authorizer"
)

// NewAuthorizer returns a new authorizer for requests from gardenlets running in self-hosted shoots.
func NewAuthorizer(logger logr.Logger, c client.Client, graph graph.Interface, authorizeWithSelectors authorizerwebhook.WithSelectorsChecker) *authorizer {
	return &authorizer{
		logger:                 logger,
		client:                 c,
		graph:                  graph,
		authorizeWithSelectors: authorizeWithSelectors,
	}
}

type authorizer struct {
	logger                 logr.Logger
	client                 client.Client
	graph                  graph.Interface
	authorizeWithSelectors authorizerwebhook.WithSelectorsChecker
}

var (
	_ = auth.Authorizer(&authorizer{})

	// Only take v1beta1 for the core.gardener.cloud API group because the Authorize function only checks the resource
	// group and the resource (but it ignores the version).
	certificateSigningRequestResource = certificatesv1.Resource("certificatesigningrequests")
	cloudProfileResource              = gardencorev1beta1.Resource("cloudprofiles")
	configMapResource                 = corev1.Resource("configmaps")
	controllerDeploymentResource      = gardencorev1beta1.Resource("controllerdeployments")
	controllerRegistrationResource    = gardencorev1beta1.Resource("controllerregistrations")
	credentialsBindingResource        = securityv1alpha1.Resource("credentialsbindings")
	eventCoreResource                 = corev1.Resource("events")
	eventResource                     = eventsv1.Resource("events")
	gardenletResource                 = seedmanagementv1alpha1.Resource("gardenlets")
	projectResource                   = gardencorev1beta1.Resource("projects")
	secretResource                    = corev1.Resource("secrets")
	secretBindingResource             = gardencorev1beta1.Resource("secretbindings")
	shootResource                     = gardencorev1beta1.Resource("shoots")
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

	if attrs.IsResourceRequest() {
		requestResource := schema.GroupResource{Group: attrs.GetAPIGroup(), Resource: attrs.GetResource()}
		switch requestResource {
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

		case secretResource:
			return a.authorizeSecret(ctx, requestAuthorizer, attrs)

		case shootResource:
			// This allows the gardenlet to read its own Shoot resource even if it does not yet exist in the system.
			// For other verbs, the graph-based authorization takes over.
			if slices.Contains([]string{"get", "list", "watch"}, attrs.GetVerb()) &&
				attrs.GetName() == shootName && attrs.GetNamespace() == shootNamespace {
				return auth.DecisionAllow, "", nil
			}

			return requestAuthorizer.Check(graph.VertexTypeShoot, attrs,
				authwebhook.WithAllowedVerbs("get", "list", "watch"),
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

func (a *authorizer) authorizeSecret(ctx context.Context, requestAuthorizer *authwebhook.RequestAuthorizer, attrs auth.Attributes) (auth.Decision, string, error) {
	// Allow gardenlet to delete its bootstrap token.
	if attrs.GetVerb() == "delete" && attrs.GetNamespace() == metav1.NamespaceSystem && strings.HasPrefix(attrs.GetName(), bootstraptokenapi.BootstrapTokenSecretPrefix) {
		shootMeta, err := gardenletutils.ShootMetaFromBootstrapToken(ctx, a.client, attrs.GetName())
		if err != nil {
			return auth.DecisionNoOpinion, "", fmt.Errorf("failed fetching shoot meta from bootstrap token description: %w", err)
		}

		if shootMeta.Namespace == requestAuthorizer.ToNamespace && shootMeta.Name == requestAuthorizer.ToName {
			return auth.DecisionAllow, "", nil
		} else {
			return auth.DecisionNoOpinion, fmt.Sprintf("shoot meta in bootstrap token secret %s does not match with identity of requestor %s/%s", shootMeta, requestAuthorizer.ToNamespace, requestAuthorizer.ToName), nil
		}
	}

	return auth.DecisionNoOpinion, "", nil
}
