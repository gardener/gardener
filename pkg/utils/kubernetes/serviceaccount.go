// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// Types extracted and adapted from kubernetes/kubernetes:
// https://github.com/kubernetes/kubernetes/blob/b4980b8d6e65602818ad27eab6c9c12398569420/pkg/serviceaccount/claims.go#L52-L68

type privateClaims struct {
	Kubernetes kubernetes `json:"kubernetes.io,omitempty"`
}

type kubernetes struct {
	Namespace string `json:"namespace,omitempty"`
	Svcacct   ref    `json:"serviceaccount,omitempty"`
}

type ref struct {
	Name string `json:"name,omitempty"`
	UID  string `json:"uid,omitempty"`
}

// ExtractServiceAccountUID extracts the UID of the service account from the given Kubernetes JWT token string.
// It does not verify the token signature and should only be used in trusted environments.
func ExtractServiceAccountUID(tokenStr string) (string, error) {
	parsed, err := jwt.ParseSigned(tokenStr, []jose.SignatureAlgorithm{jose.HS256, jose.ES256, jose.RS256, jose.ES384, jose.ES512})
	if err != nil {
		return "", err
	}
	var claims privateClaims
	if err = parsed.UnsafeClaimsWithoutVerification(&claims); err != nil {
		return "", err
	}
	return claims.Kubernetes.Svcacct.UID, nil
}
