// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"slices"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime/schema"
	auth "k8s.io/apiserver/pkg/authorization/authorizer"
)

func (a *authorizer) authorizeGardenadmRequests(requestLog logr.Logger, shootNamespace, _ string, attrs auth.Attributes) (auth.Decision, string, error) {
	if attrs.IsResourceRequest() {
		requestResource := schema.GroupResource{Group: attrs.GetAPIGroup(), Resource: attrs.GetResource()}
		switch requestResource {
		case cloudProfileResource, controllerDeploymentResource, controllerRegistrationResource:
			if isGardenadmRequestAllowed(attrs, nil, "get") {
				return auth.DecisionAllow, "", nil
			}

		case projectResource:
			if isGardenadmRequestAllowed(attrs, nil, "create") {
				return auth.DecisionAllow, "", nil
			}

		case configMapResource, secretResource, secretBindingResource, credentialsBindingResource:
			if isGardenadmRequestAllowed(attrs, &shootNamespace, "create") {
				return auth.DecisionAllow, "", nil
			}

		case shootResource:
			if isGardenadmRequestAllowed(attrs, &shootNamespace, "create", "mark-self-hosted") {
				return auth.DecisionAllow, "", nil
			}

		default:
			requestLog.Info(
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

func isGardenadmRequestAllowed(attrs auth.Attributes, shootNamespace *string, allowedVerbs ...string) bool {
	if shootNamespace != nil && attrs.GetNamespace() != *shootNamespace {
		return false
	}
	return slices.Contains(allowedVerbs, attrs.GetVerb())
}
