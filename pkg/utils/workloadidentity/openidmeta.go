// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package workloadidentity

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-jose/go-jose/v4"
)

type openIDMetadata struct {
	Issuer        string   `json:"issuer"`
	JWKSURI       string   `json:"jwks_uri"`
	ResponseTypes []string `json:"response_types_supported"`
	SubjectTypes  []string `json:"subject_types_supported"`
	SigningAlgs   []string `json:"id_token_signing_alg_values_supported"`
}

// OpenIDConfig builds the content for the openid configuration discovery document
// from the provided issuer URL and keys.
func OpenIDConfig(issuerURL string, keys ...any) ([]byte, error) {
	algs := make([]string, 0, len(keys))
	for _, k := range keys {
		alg, err := getAlg(k)
		if err != nil {
			return nil, err
		}
		algs = append(algs, string(alg))
	}

	config := openIDMetadata{
		Issuer:        issuerURL,
		JWKSURI:       issuerURL + "/jwks",
		ResponseTypes: []string{"id_token"},
		SubjectTypes:  []string{"public"},
		SigningAlgs:   algs,
	}

	return json.Marshal(config)
}

// JWKS builds the content for the JWKS discovery document
// from the provided keys.
func JWKS(keys ...any) ([]byte, error) {
	jwks := jose.JSONWebKeySet{}

	for _, key := range keys {
		jwk := jose.JSONWebKey{
			Key: key,
			Use: "sig",
		}
		if !jwk.IsPublic() {
			return nil, errors.New("JWKS supports only public keys")
		}

		kid, err := getKeyID(key)
		if err != nil {
			return nil, fmt.Errorf("failed getting key id: %w", err)
		}
		jwk.KeyID = kid

		alg, err := getAlg(key)
		if err != nil {
			return nil, fmt.Errorf("failed getting key algorithm: %w", err)
		}
		jwk.Algorithm = string(alg)

		jwks.Keys = append(jwks.Keys, jwk)
	}

	return json.Marshal(jwks)
}
