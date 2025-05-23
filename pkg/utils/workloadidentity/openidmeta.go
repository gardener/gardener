// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package workloadidentity

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/go-jose/go-jose/v4"
	"k8s.io/apimachinery/pkg/util/sets"
)

type openIDMetadata struct {
	Issuer        string   `json:"issuer"`
	JWKSURI       string   `json:"jwks_uri"`
	ResponseTypes []string `json:"response_types_supported"`
	SubjectTypes  []string `json:"subject_types_supported"`
	SigningAlgs   []string `json:"id_token_signing_alg_values_supported"`
}

// OpenIDConfig builds the content for the openid configuration discovery document
// from the provided issuer URL and public keys.
func OpenIDConfig(issuerURL string, publicKeys ...any) ([]byte, error) {
	algs := sets.New[string]()
	for _, k := range publicKeys {
		alg, err := getAlg(k)
		if err != nil {
			return nil, err
		}
		algs.Insert(string(alg))
	}

	algsList := algs.UnsortedList()
	slices.Sort(algsList)

	config := openIDMetadata{
		Issuer:        issuerURL,
		JWKSURI:       issuerURL + "/jwks",
		ResponseTypes: []string{"id_token"},
		SubjectTypes:  []string{"public"},
		SigningAlgs:   algsList,
	}

	return json.Marshal(config)
}

// JWKS builds the content for the JWKS discovery document
// from the provided public keys.
func JWKS(publicKeys ...any) ([]byte, error) {
	jwks := jose.JSONWebKeySet{}

	for _, key := range publicKeys {
		jwk := jose.JSONWebKey{
			Key: key,
			Use: "sig",
		}
		if !jwk.IsPublic() {
			return nil, errors.New("all keys must be public")
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

func getAlg(publicKey crypto.PublicKey) (jose.SignatureAlgorithm, error) {
	switch pk := publicKey.(type) {
	case *rsa.PublicKey:
		return jose.RS256, nil
	case *ecdsa.PublicKey:
		switch pk.Curve {
		case elliptic.P256():
			return jose.ES256, nil
		case elliptic.P384():
			return jose.ES384, nil
		case elliptic.P521():
			return jose.ES512, nil
		default:
			return "", fmt.Errorf("unknown ecdsa key curve, must be 256, 384, or 521")
		}
	case jose.OpaqueSigner:
		return jose.SignatureAlgorithm(pk.Public().Algorithm), nil
	default:
		return "", fmt.Errorf("unknown public key type, must be *rsa.PublicKey, *ecdsa.PublicKey, or jose.OpaqueSigner")
	}
}
