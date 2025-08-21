// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	certificatesv1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	auth "k8s.io/apiserver/pkg/authorization/authorizer"

	"github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity"
	shootidentity "github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity/shoot"
	authwebhook "github.com/gardener/gardener/pkg/admissioncontroller/webhook/auth"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/gardener/gardenlet"
	"github.com/gardener/gardener/pkg/utils/graph"
	authorizerwebhook "github.com/gardener/gardener/pkg/webhook/authorizer"
)

// NewAuthorizer returns a new authorizer for requests from gardenlets running in autonomous shoots.
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

var (
	_ = auth.Authorizer(&authorizer{})

	// Only take v1beta1 for the core.gardener.cloud API group because the Authorize function only checks the resource
	// group and the resource (but it ignores the version).
	certificateSigningRequestResource = certificatesv1.Resource("certificatesigningrequests")
	gardenletResource                 = seedmanagementv1alpha1.Resource("gardenlets")
)

func (a *authorizer) Authorize(_ context.Context, attrs auth.Attributes) (auth.Decision, string, error) {
	shootNamespace, shootName, isAutonomousShoot, userType := shootidentity.FromUserInfoInterface(attrs.GetUser())
	if !isAutonomousShoot {
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

		case gardenletResource:
			return requestAuthorizer.Check(graph.VertexTypeGardenlet, attrs,
				authwebhook.WithAllowedVerbs("get", "list", "watch", "update", "patch"),
				authwebhook.WithAlwaysAllowedVerbs("create"),
				authwebhook.WithAllowedSubresources("status"),
				authwebhook.WithFieldSelectors(metav1.ObjectNameField, gardenlet.ResourcePrefixAutonomousShoot+requestAuthorizer.ToName),
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
