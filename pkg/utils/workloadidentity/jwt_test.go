// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package workloadidentity_test

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/util/keyutil"

	"github.com/gardener/gardener/pkg/utils/workloadidentity"
)

var (
	rsaPrivateKey   *rsa.PrivateKey
	ecdsaPrivateKey *ecdsa.PrivateKey
)

const (
	issuer = "https://test.local.gardener.cloud"
)

var _ = BeforeSuite(func() {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 4096)
	Expect(err).ToNot(HaveOccurred())
	rsaPrivateKey = rsaKey

	ecdsaKey, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	Expect(err).ToNot(HaveOccurred())
	ecdsaPrivateKey = ecdsaKey
})

var _ = Describe("#JWT", func() {
	BeforeEach(func() {
		workloadidentity.SetNow(func() time.Time {
			return time.Date(2024, time.July, 9, 2, 0, 0, 0, time.UTC)
		})
	})

	Context("#getKeyID", func() {
		const (
			pubKey = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAySXbkmrd0VD+L24TilvW
wzwAf/M7SpXgn4FTc2pe5XbOAq2CU+rWAVPLEW8oRtGW9F4uenbugiB0usRA+aYW
b8JwsEKRoxpaeKzqPg4P+QL5t/aHsu4Vh9dxk7hEYSNaKZpEOhlJHARk4pPvqx5R
uCKk7csM19Tg2v9ustk6IK5PVieoSmEA55B5iKe6tBAH5IR2Qu2NjvdONcXPGP8Y
ujOpwVgXG82EJLTtbehyZOUjC801g2vxKyo0rdAx483kUBqDLW/GZfYK5Y+ZV5ty
Jc7N1Tp94TBXpmsw0KIMz1gjRtbQcDJpntWAcIRhQ9OyEWpfVW+NZoj8wstqU0pB
wQIDAQAB
-----END PUBLIC KEY-----`
			pubKeyID = "vyA3RTKakSnyzQG6KWl41qjwe-aXFPrjFTdesc3kZLk"
		)

		It("should correctly calculate key id", func() {
			keys, err := keyutil.ParsePublicKeysPEM([]byte(pubKey))
			Expect(err).ToNot(HaveOccurred())
			Expect(keys).To(HaveLen(1))

			keyID, err := workloadidentity.GetKeyID(keys[0])
			Expect(err).ToNot(HaveOccurred())
			Expect(keyID).ToNot(BeEmpty())
			Expect(keyID).To(Equal(pubKeyID))
		})

		It("should provide unique key IDs", func() {
			k1, err := rsa.GenerateKey(rand.Reader, 2048)
			Expect(err).ToNot(HaveOccurred())

			k2, err := rsa.GenerateKey(rand.Reader, 2048)
			Expect(err).ToNot(HaveOccurred())

			Expect(k1.Equal(k2)).To(BeFalse())
			Expect(k2.Equal(k1)).To(BeFalse())

			id1, err := workloadidentity.GetKeyID(k1.Public())
			Expect(err).ToNot(HaveOccurred())
			Expect(id1).ToNot(BeEmpty())
			id2, err := workloadidentity.GetKeyID(k2.Public())
			Expect(err).ToNot(HaveOccurred())
			Expect(id2).ToNot(BeEmpty())

			Expect(id1).ToNot(Equal(pubKeyID))
			Expect(id2).ToNot(Equal(pubKeyID))
			Expect(id1).ToNot((Equal(id2)))
		})

		It("should be idempotent", func() {
			id1, err := workloadidentity.GetKeyID(rsaPrivateKey.Public())
			Expect(err).ToNot(HaveOccurred())
			Expect(id1).ToNot(BeEmpty())

			id2, err := workloadidentity.GetKeyID(rsaPrivateKey.Public())
			Expect(err).ToNot(HaveOccurred())
			Expect(id2).ToNot(BeEmpty())

			Expect(id1).To(Equal(id2))
		})

		It("should fail to get key ID for unsupported key", func() {
			type unsupportedKey struct{}

			var _ crypto.PublicKey = unsupportedKey{}
			u := unsupportedKey{}

			keyID, err := workloadidentity.GetKeyID(u)
			Expect(err).To(HaveOccurred())
			Expect(keyID).To(BeEmpty())
		})
	})

	Context("#getSigner", func() {
		It("should get RSA signer", func() {
			s, err := workloadidentity.GetSigner(rsaPrivateKey)
			Expect(err).ToNot(HaveOccurred())
			Expect(s).ToNot(BeNil())
		})

		It("should get ECDSA signer", func() {
			s, err := workloadidentity.GetSigner(ecdsaPrivateKey)
			Expect(err).ToNot(HaveOccurred())
			Expect(s).ToNot(BeNil())
		})

		It("should fail for unsupported key", func() {
			type unsupportedKey struct{}
			s, err := workloadidentity.GetSigner(unsupportedKey{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("failed to construct signer from key type workloadidentity_test.unsupportedKey"))
			Expect(s).To(BeNil())
		})

		It("should fail for nil key", func() {
			signer, err := workloadidentity.GetRSASigner(nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("rsa: key must not be nil"))
			Expect(signer).To(BeNil())

			signer, err = workloadidentity.GetECDSASigner(nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("ecdsa: key must not be nil"))
			Expect(signer).To(BeNil())
		})
	})

	Context("#newTokenIssuer", func() {
		const (
			minDuration int64 = 600
			maxDuration int64 = 87600
		)

		It("should fail to create token issuer due to erroneous signing key", func() {
			t, err := workloadidentity.NewTokenIssuer("", issuer, minDuration, maxDuration)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("failed to get signer")))
			Expect(t).To(BeNil())
		})

		It("should fail to create token issuer due to empty issuer url", func() {
			t, err := workloadidentity.NewTokenIssuer(rsaPrivateKey, "", minDuration, maxDuration)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("issuer cannot be empty string"))
			Expect(t).To(BeNil())
		})

		It("should fail to create token issuer when the issuer url is not valid url", func() {
			t, err := workloadidentity.NewTokenIssuer(rsaPrivateKey, "://test.local.gardener.cloud", minDuration, maxDuration)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("issuer is not a valid URL")))
			Expect(t).To(BeNil())
		})

		It("should fail to create token issuer due to non https scheme issuer url", func() {
			t, err := workloadidentity.NewTokenIssuer(rsaPrivateKey, "http://test.local.gardener.cloud", minDuration, maxDuration)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("issuer must be using https scheme"))
			Expect(t).To(BeNil())
		})

		It("should  successfully create token issuer", func() {
			t, err := workloadidentity.NewTokenIssuer(rsaPrivateKey, issuer, minDuration, maxDuration)
			Expect(err).ToNot(HaveOccurred())
			Expect(t).ToNot(BeNil())
		})
	})

	Context("#issueToken", func() {
		const (
			sub = "subject"
			aud = "gardener.cloud"
		)

		var (
			minDurationSeconds int64
			maxDurationSeconds int64
			t                  workloadidentity.TokenIssuer
			audiences          []string
		)

		type customClaims struct {
			CLM1 string `json:"clm1,omitempty"`
			CLM2 string `json:"clm2,omitempty"`
			CLM3 string `json:"clm3,omitempty"`
		}

		BeforeEach(func() {
			minDurationSeconds = int64(time.Minute.Seconds()) * 10
			maxDurationSeconds = int64(time.Hour.Seconds()) * 48

			tokenIssuer, err := workloadidentity.NewTokenIssuer(rsaPrivateKey, issuer, minDurationSeconds, maxDurationSeconds)
			Expect(err).ToNot(HaveOccurred())

			t = tokenIssuer
			audiences = []string{aud}
		})

		It("should successfully issue token without claims", func() {
			var (
				n               = workloadidentity.Now()()
				durationSeconds = int64(time.Hour.Seconds()) * 2
			)
			token, exp, err := t.IssueToken(sub, audiences, durationSeconds)

			Expect(err).ToNot(HaveOccurred())
			Expect(exp).ToNot(BeNil())
			Expect(exp.After(n)).To(BeTrue())
			Expect(token).ToNot(BeEmpty())
		})

		It("should successfully issue token with claims", func() {
			var (
				n               = workloadidentity.Now()()
				durationSeconds = int64(time.Hour.Seconds()) * 2
			)

			c := &customClaims{
				CLM1: "claim-1",
				CLM2: "claim-2",
			}

			token, exp, err := t.IssueToken(sub, audiences, durationSeconds, c)
			Expect(err).ToNot(HaveOccurred())

			Expect(exp).ToNot(BeNil())
			Expect(exp.After(n)).To(BeTrue())
			Expect(token).ToNot(BeEmpty())

			parsedToken, err := jwt.ParseSigned(token, []jose.SignatureAlgorithm{jose.RS256})
			Expect(err).ToNot(HaveOccurred())

			dst := &struct {
				jwt.Claims
				customClaims
			}{}

			Expect(parsedToken.UnsafeClaimsWithoutVerification(dst)).ToNot(HaveOccurred())
			Expect(dst.CLM1).To(Equal("claim-1"))
			Expect(dst.CLM2).To(Equal("claim-2"))
			Expect(dst.CLM3).To(BeEmpty())

			Expect(dst.Issuer).To(Equal(issuer))
			Expect(dst.Audience).To(BeEquivalentTo(audiences))
			Expect(dst.Subject).To(Equal(sub))

			Expect(dst.IssuedAt).To(Equal(jwt.NewNumericDate(n)))
			Expect(dst.NotBefore).To(Equal(jwt.NewNumericDate(n)))

			Expect(
				dst.Expiry.Time().UTC(),
			).To(Equal(
				jwt.NewNumericDate(n.Add(time.Second * time.Duration(durationSeconds))).Time().UTC(),
			))
		})

		It("should cap duration to max duration", func() {
			var (
				n               = workloadidentity.Now()()
				durationSeconds = maxDurationSeconds + int64(time.Hour.Seconds())
			)
			token, exp, err := t.IssueToken(sub, audiences, durationSeconds)

			Expect(err).ToNot(HaveOccurred())
			Expect(token).ToNot(BeEmpty())

			Expect(exp).ToNot(BeNil())
			Expect(exp.After(n)).To(BeTrue())
			Expect(exp.Compare(n.Add(time.Second * time.Duration(durationSeconds)))).To(Equal(-1))
		})

		It("should cap duration to min duration", func() {
			var (
				n               = workloadidentity.Now()()
				durationSeconds = minDurationSeconds - int64(time.Minute.Seconds())
			)
			token, exp, err := t.IssueToken(sub, audiences, durationSeconds)

			Expect(err).ToNot(HaveOccurred())
			Expect(token).ToNot(BeEmpty())

			fmt.Fprintf(GinkgoWriter, "%v\n", exp.Sub(n))
			Expect(exp).ToNot(BeNil())
			Expect(exp.After(n)).To(BeTrue())
			Expect(exp.Compare(n.Add(time.Second * time.Duration(durationSeconds)))).To(Equal(1))
		})
	})
})
