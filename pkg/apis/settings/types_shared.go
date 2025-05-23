// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package settings

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpenIDConnectPresetSpec contains the Shoot selector for which
// a specific OpenID Connect configuration is applied.
type OpenIDConnectPresetSpec struct {

	// Server contains the kube-apiserver's OpenID Connect configuration.
	// This configuration is not overwriting any existing OpenID Connect
	// configuration already set on the Shoot object.
	Server KubeAPIServerOpenIDConnect

	// Client contains the configuration used for client OIDC authentication
	// of Shoot clusters.
	// This configuration is not overwriting any existing OpenID Connect
	// client authentication already set on the Shoot object.
	//
	// Deprecated: The OpenID Connect configuration this field specifies is not used and will be forbidden starting from Kubernetes 1.31.
	// It's use was planned for genereting OIDC kubeconfig https://github.com/gardener/gardener/issues/1433
	// TODO(AleksandarSavchev): Drop this field after support for Kubernetes 1.30 is dropped.
	Client *OpenIDConnectClientAuthentication

	// ShootSelector decides whether to apply the configuration if the
	// Shoot has matching labels.
	// Use the selector only if the OIDC Preset is opt-in, because end
	// users may skip the admission by setting the labels.
	// Default to the empty LabelSelector, which matches everything.
	ShootSelector *metav1.LabelSelector

	// Weight associated with matching the corresponding preset,
	// in the range 1-100.
	// Required.
	Weight int32
}

// This is copied to not depend on the gardener package and keep it version agnostic.

// KubeAPIServerOpenIDConnect contains configuration settings for the OIDC provider.
// Note: Descriptions were taken from the Kubernetes documentation.
type KubeAPIServerOpenIDConnect struct {
	// If set, the OpenID server's certificate will be verified by one of the authorities in the oidc-ca-file, otherwise the host's root CA set will be used.
	CABundle *string
	// The client ID for the OpenID Connect client.
	// Required.
	ClientID string
	// If provided, the name of a custom OpenID Connect claim for specifying user groups. The claim value is expected to be a string or array of strings. This flag is experimental, please see the authentication documentation for further details.
	GroupsClaim *string
	// If provided, all groups will be prefixed with this value to prevent conflicts with other authentication strategies.
	GroupsPrefix *string
	// The URL of the OpenID issuer, only HTTPS scheme will be accepted. If set, it will be used to verify the OIDC JSON Web Token (JWT).
	// Required.
	IssuerURL string
	// key=value pairs that describes a required claim in the ID Token. If set, the claim is verified to be present in the ID Token with a matching value.
	RequiredClaims map[string]string
	// List of allowed JOSE asymmetric signing algorithms. JWTs with a 'alg' header value not in this list will be rejected. Values are defined by RFC 7518 https://tools.ietf.org/html/rfc7518#section-3.1
	SigningAlgs []string
	// The OpenID claim to use as the user name. Note that claims other than the default ('sub') is not guaranteed to be unique and immutable. This flag is experimental, please see the authentication documentation for further details. (default "sub")
	UsernameClaim *string
	// If provided, all usernames will be prefixed with this value. If not provided, username claims other than 'email' are prefixed by the issuer URL to avoid clashes. To skip any prefixing, provide the value '-'.
	UsernamePrefix *string
}

// OpenIDConnectClientAuthentication contains configuration for OIDC clients.
type OpenIDConnectClientAuthentication struct {
	// The client Secret for the OpenID Connect client.
	Secret *string

	// Extra configuration added to kubeconfig's auth-provider.
	// Must not be any of idp-issuer-url, client-id, client-secret, idp-certificate-authority, idp-certificate-authority-data, id-token or refresh-token
	ExtraConfig map[string]string
}

// Preset offers access to the specification of a OpenID preset object.
// Mainly used for tests.
type Preset interface {
	metav1.ObjectMetaAccessor
	GetPresetSpec() *OpenIDConnectPresetSpec
	SetPresetSpec(s *OpenIDConnectPresetSpec)
}
