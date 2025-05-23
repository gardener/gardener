// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package workloadidentity_test

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"

	"github.com/go-jose/go-jose/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/workloadidentity"
)

var _ = Describe("#OpenIDMeta", func() {
	Describe("#OpenIDConfig", func() {
		It("should correctly build the openid configuration document", func() {
			const issuerURL = "https://test.gardener.cloud/workload-identity"
			rsaKey, err := secretsutils.FakeGenerateKey(rand.Reader, 2048)
			Expect(err).ToNot(HaveOccurred())
			ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			Expect(err).ToNot(HaveOccurred())

			openIDConfig, err := workloadidentity.OpenIDConfig(issuerURL, rsaKey.Public(), ecdsaKey.Public())
			Expect(err).ToNot(HaveOccurred())

			expectedOpenIDConfig := workloadidentity.OpenIDMetadata{
				Issuer:        issuerURL,
				JWKSURI:       issuerURL + "/jwks",
				ResponseTypes: []string{"id_token"},
				SubjectTypes:  []string{"public"},
				SigningAlgs:   []string{"ES256", "RS256"},
			}
			expectedOpenIDConfigMarshaled, err := json.Marshal(expectedOpenIDConfig)
			Expect(err).ToNot(HaveOccurred())
			Expect(openIDConfig).To(MatchJSON(expectedOpenIDConfigMarshaled))
		})

		It("should not duplicate algs in the openid configuration document", func() {
			const issuerURL = "https://test.gardener.cloud/workload-identity"
			rsaKey, err := secretsutils.FakeGenerateKey(rand.Reader, 2048)
			Expect(err).ToNot(HaveOccurred())

			openIDConfig, err := workloadidentity.OpenIDConfig(issuerURL, rsaKey.Public(), rsaKey.Public())
			Expect(err).ToNot(HaveOccurred())

			expectedOpenIDConfig := workloadidentity.OpenIDMetadata{
				Issuer:        issuerURL,
				JWKSURI:       issuerURL + "/jwks",
				ResponseTypes: []string{"id_token"},
				SubjectTypes:  []string{"public"},
				SigningAlgs:   []string{"RS256"},
			}
			expectedOpenIDConfigMarshaled, err := json.Marshal(expectedOpenIDConfig)
			Expect(err).ToNot(HaveOccurred())
			Expect(openIDConfig).To(MatchJSON(expectedOpenIDConfigMarshaled))
		})
	})

	Describe("#JWKS", func() {
		It("should fail to build JWKS document with private key", func() {
			key, err := secretsutils.FakeGenerateKey(rand.Reader, 2048)
			Expect(err).ToNot(HaveOccurred())

			jwks, err := workloadidentity.JWKS(key)
			Expect(err).To(MatchError("all keys must be public"))
			Expect(jwks).To(BeNil())
		})

		It("should correctly build jwks document", func() {
			privateKey, err := secretsutils.FakeGenerateKey(rand.Reader, 2048)
			Expect(err).ToNot(HaveOccurred())
			publicKey := privateKey.Public()
			rsaPublicKey, ok := publicKey.(*rsa.PublicKey)
			Expect(ok).To(BeTrue())

			jwks, err := workloadidentity.JWKS(rsaPublicKey)
			Expect(err).ToNot(HaveOccurred())

			kid, err := workloadidentity.GetKeyID(rsaPublicKey)
			Expect(err).ToNot(HaveOccurred())
			expectedJWKS := jose.JSONWebKeySet{
				Keys: []jose.JSONWebKey{
					{
						Key:       rsaPublicKey,
						KeyID:     kid,
						Algorithm: string(jose.RS256),
						Use:       "sig",
					},
				},
			}

			expectedJWKSMarshaled, err := json.Marshal(expectedJWKS)
			Expect(err).ToNot(HaveOccurred())
			Expect(jwks).To(Equal(expectedJWKSMarshaled))
		})
	})

	Describe("#getAlg", func() {
		It("should get correct algorithm", func() {
			privateKey, err := secretsutils.FakeGenerateKey(rand.Reader, 2048)
			Expect(err).ToNot(HaveOccurred())
			alg, err := workloadidentity.GetAlg(privateKey.Public())
			Expect(err).ToNot(HaveOccurred())
			Expect(alg).To(Equal(jose.RS256))

			ecdsaPrivateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			Expect(err).ToNot(HaveOccurred())
			alg, err = workloadidentity.GetAlg(ecdsaPrivateKey.Public())
			Expect(err).ToNot(HaveOccurred())
			Expect(alg).To(Equal(jose.ES256))

			ecdsaPrivateKey, err = ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
			Expect(err).ToNot(HaveOccurred())
			alg, err = workloadidentity.GetAlg(ecdsaPrivateKey.Public())
			Expect(err).ToNot(HaveOccurred())
			Expect(alg).To(Equal(jose.ES384))

			ecdsaPrivateKey, err = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
			Expect(err).ToNot(HaveOccurred())
			alg, err = workloadidentity.GetAlg(ecdsaPrivateKey.Public())
			Expect(err).ToNot(HaveOccurred())
			Expect(alg).To(Equal(jose.ES512))
		})

		It("should return error for unknown ECDSA curve", func() {
			key, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
			Expect(err).ToNot(HaveOccurred())
			_, err = workloadidentity.GetAlg(key.Public())
			Expect(err).To(MatchError("unknown ecdsa key curve, must be 256, 384, or 521"))
		})

		It("should return error for unknown key type", func() {
			key, err := ecdh.P256().GenerateKey(rand.Reader)
			Expect(err).ToNot(HaveOccurred())
			_, err = workloadidentity.GetAlg(key.Public())
			Expect(err).To(MatchError("unknown public key type, must be *rsa.PublicKey, *ecdsa.PublicKey, or jose.OpaqueSigner"))
		})
	})
})
