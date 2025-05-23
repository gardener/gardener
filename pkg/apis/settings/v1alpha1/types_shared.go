// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DefaultUsernameClaim is the default username claim.
	DefaultUsernameClaim = "sub"
	// DefaultSignAlg is the default signing algorithm.
	DefaultSignAlg = "RS256"
)

// OpenIDConnectPresetSpec contains the Shoot selector for which
// a specific OpenID Connect configuration is applied.
type OpenIDConnectPresetSpec struct {

	// Server contains the kube-apiserver's OpenID Connect configuration.
	// This configuration is not overwriting any existing OpenID Connect
	// configuration already set on the Shoot object.
	Server KubeAPIServerOpenIDConnect `json:"server" protobuf:"bytes,1,opt,name=server"`

	// Client contains the configuration used for client OIDC authentication
	// of Shoot clusters.
	// This configuration is not overwriting any existing OpenID Connect
	// client authentication already set on the Shoot object.
	//
	// Deprecated: The OpenID Connect configuration this field specifies is not used and will be forbidden starting from Kubernetes 1.31.
	// It's use was planned for genereting OIDC kubeconfig https://github.com/gardener/gardener/issues/1433
	// TODO(AleksandarSavchev): Drop this field after support for Kubernetes 1.30 is dropped.
	// +optional
	Client *OpenIDConnectClientAuthentication `json:"client,omitempty" protobuf:"bytes,2,opt,name=client"`

	// ShootSelector decides whether to apply the configuration if the
	// Shoot has matching labels.
	// Use the selector only if the OIDC Preset is opt-in, because end
	// users may skip the admission by setting the labels.
	// Default to the empty LabelSelector, which matches everything.
	// +optional
	ShootSelector *metav1.LabelSelector `json:"shootSelector,omitempty" protobuf:"bytes,3,opt,name=shootSelector"`

	// Weight associated with matching the corresponding preset,
	// in the range 1-100.
	// Required.
	Weight int32 `json:"weight" protobuf:"varint,4,opt,name=weight"`
}

// This is copied to not depend on the gardener package and keep it version agnostic.

// KubeAPIServerOpenIDConnect contains configuration settings for the OIDC provider.
// Note: Descriptions were taken from the Kubernetes documentation.
type KubeAPIServerOpenIDConnect struct {
	// If set, the OpenID server's certificate will be verified by one of the authorities in the oidc-ca-file, otherwise the host's root CA set will be used.
	// +optional
	CABundle *string `json:"caBundle,omitempty" protobuf:"bytes,1,opt,name=caBundle"`
	// The client ID for the OpenID Connect client.
	// Required.
	ClientID string `json:"clientID" protobuf:"bytes,2,opt,name=clientID"`
	// If provided, the name of a custom OpenID Connect claim for specifying user groups. The claim value is expected to be a string or array of strings. This field is experimental, please see the authentication documentation for further details.
	// +optional
	GroupsClaim *string `json:"groupsClaim,omitempty" protobuf:"bytes,3,opt,name=groupsClaim"`
	// If provided, all groups will be prefixed with this value to prevent conflicts with other authentication strategies.
	// +optional
	GroupsPrefix *string `json:"groupsPrefix,omitempty" protobuf:"bytes,4,opt,name=groupsPrefix"`
	// The URL of the OpenID issuer, only HTTPS scheme will be accepted. If set, it will be used to verify the OIDC JSON Web Token (JWT).
	// Required.
	IssuerURL string `json:"issuerURL" protobuf:"bytes,5,opt,name=issuerURL"`
	// key=value pairs that describes a required claim in the ID Token. If set, the claim is verified to be present in the ID Token with a matching value.
	// +optional
	RequiredClaims map[string]string `json:"requiredClaims,omitempty" protobuf:"bytes,6,rep,name=requiredClaims"`
	// List of allowed JOSE asymmetric signing algorithms. JWTs with a 'alg' header value not in this list will be rejected. Values are defined by RFC 7518 https://tools.ietf.org/html/rfc7518#section-3.1
	// Defaults to [RS256]
	// +optional
	SigningAlgs []string `json:"signingAlgs,omitempty" protobuf:"bytes,7,rep,name=signingAlgs"`
	// The OpenID claim to use as the user name. Note that claims other than the default ('sub') is not guaranteed to be unique and immutable. This field is experimental, please see the authentication documentation for further details.
	// Defaults to "sub".
	// +optional
	UsernameClaim *string `json:"usernameClaim,omitempty" protobuf:"bytes,8,opt,name=usernameClaim"`
	// If provided, all usernames will be prefixed with this value. If not provided, username claims other than 'email' are prefixed by the issuer URL to avoid clashes. To skip any prefixing, provide the value '-'.
	// +optional
	UsernamePrefix *string `json:"usernamePrefix,omitempty" protobuf:"bytes,9,opt,name=usernamePrefix"`
}

// OpenIDConnectClientAuthentication contains configuration for OIDC clients.
type OpenIDConnectClientAuthentication struct {
	// The client Secret for the OpenID Connect client.
	// +optional
	Secret *string `json:"secret,omitempty" protobuf:"bytes,1,opt,name=secret"`

	// Extra configuration added to kubeconfig's auth-provider.
	// Must not be any of idp-issuer-url, client-id, client-secret, idp-certificate-authority, idp-certificate-authority-data, id-token or refresh-token
	// +optional
	ExtraConfig map[string]string `json:"extraConfig,omitempty" protobuf:"bytes,2,rep,name=extraConfig"`
}
