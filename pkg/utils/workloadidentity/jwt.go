// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package workloadidentity

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

const (
	keyUsageSig string = "sig"
)

var (
	now = time.Now
)

// TokenIssuer is JSON Web Token issuer.
type TokenIssuer struct {
	signer             jose.Signer
	issuer             string
	minDurationSeconds int64
	maxDurationSeconds int64
}

// NewTokenIssuer creates new TokenIssuer.
func NewTokenIssuer(signingKey any, issuer string, minDuration, maxDuration int64) (*TokenIssuer, error) {
	signer, err := getSigner(signingKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get signer: %w", err)
	}

	return &TokenIssuer{
		signer:             signer,
		issuer:             issuer,
		minDurationSeconds: minDuration,
		maxDurationSeconds: maxDuration,
	}, nil
}

// GetIssuer returns the issuer value used for the `iss` JWT claim.
func (t *TokenIssuer) GetIssuer() string {
	return t.issuer
}

// IssueToken issue JSON Web tokens with the configured claims signed by the signer.
func (t *TokenIssuer) IssueToken(sub string, aud []string, duration int64, claims ...any) (string, *time.Time, error) {
	builder := jwt.Signed(t.signer)

	for _, c := range claims {
		builder = builder.Claims(c)
	}

	if duration < t.minDurationSeconds {
		duration = t.minDurationSeconds
	} else if duration > t.maxDurationSeconds {
		duration = t.maxDurationSeconds
	}

	iat := now()
	exp := iat.Add(time.Second * time.Duration(duration))

	c := jwt.Claims{
		Issuer:    t.issuer,
		IssuedAt:  jwt.NewNumericDate(iat),
		Expiry:    jwt.NewNumericDate(exp),
		NotBefore: jwt.NewNumericDate(iat),
		Subject:   sub,
		Audience:  aud,
	}

	token, err := builder.Claims(c).Serialize()
	if err != nil {
		return "", nil, fmt.Errorf("failed to issue JSON Web token: %w", err)
	}

	return token, &exp, nil
}

func getSigner(key any) (jose.Signer, error) {
	switch k := key.(type) {
	case *rsa.PrivateKey:
		return getRSASigner(k)
	case *ecdsa.PrivateKey:
		return getECDSASigner(k)
	case jose.OpaqueSigner:
		return getOpaqueSigner(k)
	}

	return nil, fmt.Errorf("failed to construct signer from key type %T", key)
}

func getRSASigner(key *rsa.PrivateKey) (jose.Signer, error) {
	if key == nil {
		return nil, fmt.Errorf("rsa: key must not be nil")
	}

	keyID, err := getKeyID(key.Public())
	if err != nil {
		return nil, fmt.Errorf("rsa: failed to get key id: %w", err)
	}

	alg := jose.RS256
	sk := jose.SigningKey{
		Algorithm: alg,
		Key: jose.JSONWebKey{
			Algorithm: string(alg),
			Use:       keyUsageSig,
			Key:       key,
			KeyID:     keyID,
		},
	}

	return jose.NewSigner(sk, nil)
}

func getECDSASigner(key *ecdsa.PrivateKey) (jose.Signer, error) {
	if key == nil {
		return nil, fmt.Errorf("ecdsa: key must not be nil")
	}

	var alg jose.SignatureAlgorithm

	switch key.Curve {
	case elliptic.P256():
		alg = jose.ES256
	case elliptic.P384():
		alg = jose.ES384
	case elliptic.P521():
		alg = jose.ES512
	default:
		return nil, fmt.Errorf("ecdsa: failed to determine signature algorithm")
	}

	keyID, err := getKeyID(key.Public())
	if err != nil {
		return nil, fmt.Errorf("ecdsa: failed to get key id: %w", err)
	}

	sk := jose.SigningKey{
		Algorithm: alg,
		Key: jose.JSONWebKey{
			Algorithm: string(alg),
			Use:       keyUsageSig,
			Key:       key,
			KeyID:     keyID,
		},
	}

	return jose.NewSigner(sk, nil)
}

func getOpaqueSigner(key jose.OpaqueSigner) (jose.Signer, error) {
	alg := jose.SignatureAlgorithm(key.Public().Algorithm)

	sk := jose.SigningKey{
		Algorithm: alg,
		Key: jose.JSONWebKey{
			Algorithm: string(alg),
			Use:       keyUsageSig,
			Key:       key,
			KeyID:     key.Public().KeyID,
		},
	}

	return jose.NewSigner(sk, nil)
}

func getKeyID(publicKey crypto.PublicKey) (string, error) {
	marshaled, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", err
	}

	shaSum := sha256.Sum256(marshaled)
	id := base64.RawURLEncoding.EncodeToString(shaSum[:])

	return id, nil
}
