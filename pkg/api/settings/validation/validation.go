// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"net/url"

	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/settings"
	"github.com/gardener/gardener/pkg/utils"
)

var (
	// See https://tools.ietf.org/html/rfc7518#section-3.1 (without "none")
	validSigningAlgs = sets.New("RS256", "RS384", "RS512", "ES256", "ES384", "ES512", "PS256", "PS384", "PS512")
	// used by oidc-provider
	forbiddenKeys = sets.New("idp-issuer-url", "client-id", "client-secret", "idp-certificate-authority", "idp-certificate-authority-data", "id-token", "refresh-token")
)

func validateOpenIDConnectPresetSpec(spec *settings.OpenIDConnectPresetSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, metav1validation.ValidateLabelSelector(spec.ShootSelector, metav1validation.LabelSelectorValidationOptions{AllowInvalidLabelValueInSelector: true}, fldPath.Child("shootSelector"))...)

	if spec.Weight <= 0 || spec.Weight > 100 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("weight"), spec.Weight, "must be in the range 1-100"))
	}

	allErrs = append(allErrs, validateServer(&spec.Server, fldPath.Child("server"))...)

	if spec.Client != nil {
		allErrs = append(allErrs, validateClient(spec.Client, fldPath.Child("client"))...)
	}

	return allErrs
}

func validateServer(server *settings.KubeAPIServerOpenIDConnect, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(server.IssuerURL) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("issuerURL"), "must not be empty"))
	} else {
		issuer, err := url.Parse(server.IssuerURL)
		if err != nil || (issuer != nil && len(issuer.Host) == 0) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("issuerURL"), server.IssuerURL, "must be a valid URL"))
		}
		if issuer != nil && issuer.Scheme != "https" {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("issuerURL"), server.IssuerURL, "must have https scheme"))
		}
	}
	if len(server.ClientID) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("clientID"), "must not be empty"))
	}
	if server.CABundle != nil {
		if _, err := utils.DecodeCertificate([]byte(*server.CABundle)); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("caBundle"), *server.CABundle, "must be a valid PEM-encoded certificate"))
		}
	}
	if server.GroupsClaim != nil && len(*server.GroupsClaim) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("groupsClaim"), *server.GroupsClaim, "must not be empty"))
	}
	if server.GroupsPrefix != nil && len(*server.GroupsPrefix) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("groupsPrefix"), *server.GroupsPrefix, "must not be empty"))
	}
	if server.SigningAlgs != nil {
		path := fldPath.Child("signingAlgs")
		if len(server.SigningAlgs) == 0 {
			allErrs = append(allErrs, field.Invalid(path, server.SigningAlgs, "must not be empty"))
		}

		existingAlgs := sets.Set[string]{}

		for i, alg := range server.SigningAlgs {
			if !validSigningAlgs.Has(alg) {
				allErrs = append(allErrs, field.Invalid(path.Index(i), alg, fmt.Sprintf("must be one of: %v", sets.List(validSigningAlgs))))
			}
			if existingAlgs.Has(alg) {
				allErrs = append(allErrs, field.Duplicate(path.Index(i), alg))
			}
			existingAlgs.Insert(alg)
		}
	}
	if server.UsernameClaim != nil && len(*server.UsernameClaim) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("usernameClaim"), *server.UsernameClaim, "must not be empty"))
	}
	if server.UsernamePrefix != nil && len(*server.UsernamePrefix) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("usernamePrefix"), *server.UsernamePrefix, "must not be empty"))
	}
	return allErrs
}

func validateClient(client *settings.OpenIDConnectClientAuthentication, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if client.Secret != nil && len(*client.Secret) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("secret"), *client.Secret, "must not be empty"))
	}

	scopeFldPath := fldPath.Child("extraConfig")
	for key, val := range client.ExtraConfig {
		if len(val) == 0 {
			allErrs = append(allErrs, field.Invalid(scopeFldPath.Key(key), val, "must not be empty"))
		}
		if forbiddenKeys.Has(key) {
			allErrs = append(allErrs, field.Forbidden(scopeFldPath.Key(key), fmt.Sprintf("cannot be any of %v", sets.List(forbiddenKeys))))
		}
	}

	return allErrs
}
