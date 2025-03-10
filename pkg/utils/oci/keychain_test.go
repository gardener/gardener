// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package oci

import (
	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("keychain", func() {
	var (
		kc *keychain
	)

	BeforeEach(func() {
		kc = &keychain{
			pullSecret: "{\"auths\":{\"example.com\":{\"username\":\"foo\",\"password\":\"bar\"}}}",
		}
	})

	It("should return anonymous authenticator for unmatched repository", func() {
		tag, err := name.NewTag("somewhere-else.com/charts/something:0.1.0")
		Expect(err).NotTo(HaveOccurred())
		authenticator, err := kc.Resolve(tag)
		Expect(err).NotTo(HaveOccurred())
		Expect(authenticator).To(Equal(authn.Anonymous))
	})

	It("should return an authenticator for an matched repository", func() {
		tag, err := name.NewTag("example.com/charts/something:0.1.0")
		Expect(err).NotTo(HaveOccurred())
		authenticator, err := kc.Resolve(tag)
		Expect(err).NotTo(HaveOccurred())
		authConfig, err := authenticator.Authorization()
		Expect(err).NotTo(HaveOccurred())
		Expect(authConfig.Username).To(Equal("foo"))
		Expect(authConfig.Password).To(Equal("bar"))
	})
})
