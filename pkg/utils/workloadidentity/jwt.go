// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	"errors"
	"fmt"
	"net/url"
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

// TokenIssuer is an interface for JSON Web Token issuers.
type TokenIssuer interface {
	// IssueToken generates JSON Web Token based on the provided subject, audiences, duration and claims.
	// It returns the token and its expiration time if successfully generated
	IssueToken(sub string, aud []string, duration int64, claims ...any) (string, *time.Time, error)
}

// tokenIssuer is JSON Web Token issuer implementing the TokenIssuer interface.
type tokenIssuer struct {
	signer             jose.Signer
	issuer             string
	minDurationSeconds int64
	maxDurationSeconds int64
}

var _ TokenIssuer = &tokenIssuer{}

// NewTokenIssuer creates new JSON Web Token issuer.
func NewTokenIssuer(signingKey any, issuer string, minDuration, maxDuration int64) (*tokenIssuer, error) {
	signer, err := getSigner(signingKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get signer: %w", err)
	}

	if issuer == "" {
		return nil, errors.New("issuer cannot be empty string")
	}

	issuerURL, err := url.Parse(issuer)
	if err != nil {
		return nil, fmt.Errorf("issuer is not a valid URL, err: %w", err)
	}

	if issuerURL.Scheme != "https" {
		return nil, fmt.Errorf("issuer must be using https scheme")
	}

	return &tokenIssuer{
		signer:             signer,
		issuer:             issuer,
		minDurationSeconds: minDuration,
		maxDurationSeconds: maxDuration,
	}, nil
}

// IssueToken issue JSON Web tokens with the configured claims signed by the signer.
func (t *tokenIssuer) IssueToken(sub string, aud []string, duration int64, claims ...any) (string, *time.Time, error) {
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
		return "", nil, fmt.Errorf("failed to issue JSON Web Token: %w", err)
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
