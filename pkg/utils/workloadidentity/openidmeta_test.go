// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package workloadidentity_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"

	"github.com/go-jose/go-jose/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/utils/workloadidentity"
)

var _ = Describe("#OpenIDMeta", func() {
	Describe("#OpenIDConfig", func() {
		It("should correctly build the openid configuration document", func() {
			const issuerURL = "https://test.gardener.cloud/workload-identity"
			key, err := rsa.GenerateKey(rand.Reader, 2048)
			Expect(err).ToNot(HaveOccurred())

			openedIDConfig, err := workloadidentity.OpenIDConfig(issuerURL, key.Public())
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
			Expect(openedIDConfig).To(Equal(expectedOpenIDConfigMarshaled))
		})
	})

	Describe("#JWKS", func() {
		It("should fail to build JWKS document with private key", func() {
			key, err := rsa.GenerateKey(rand.Reader, 2048)
			Expect(err).ToNot(HaveOccurred())

			jwks, err := workloadidentity.JWKS(key)
			Expect(err).To(MatchError("JWKS supports only public keys"))
			Expect(jwks).To(BeNil())
		})

		It("should correctly build jwks document", func() {
			privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
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
})
