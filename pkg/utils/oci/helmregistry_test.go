// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package oci

import (
	"context"
	"fmt"

	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory"
	"github.com/google/go-containerregistry/pkg/name"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
)

var _ = Describe("helmregistry", func() {
	var (
		hr  *HelmRegistry
		rc  *recordingCache
		ctx context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		rc = &recordingCache{cache: newCache()}
		hr = &HelmRegistry{cache: rc}
	})

	It("should return error if the repository does not exist", func() {
		_, err := hr.Pull(ctx, &gardencorev1.OCIRepository{
			Repository: registryAddress + "/charts/examplexxx",
			Tag:        "0.1.0",
		})
		Expect(err).To(MatchError(ContainSubstring("failed get manifest from remote")))
	})

	It("should return error if the digest does not exist", func() {
		_, err := hr.Pull(ctx, &gardencorev1.OCIRepository{
			Repository: registryAddress + "/charts/examplexxx",
			Digest:     "sha256:7a855a6d69033dd3240d9648e8bd46a67a528059158e098c7794ac9227735b4a",
		})
		Expect(err).To(MatchError(ContainSubstring("failed to pull artifact")))
	})

	It("should pull the chart by tag", func() {
		out, err := hr.Pull(ctx, &gardencorev1.OCIRepository{
			Repository: registryAddress + "/charts/example",
			Tag:        "0.1.0",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).NotTo(BeEmpty())
	})

	It("should pull the chart by digest", func() {
		out, err := hr.Pull(ctx, &gardencorev1.OCIRepository{
			Repository: registryAddress + "/charts/example",
			Digest:     exampleChartDigest,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).NotTo(BeEmpty())
	})

	It("should pull the chart with ref", func() {
		out, err := hr.Pull(ctx, &gardencorev1.OCIRepository{
			Ref: fmt.Sprintf("%s/charts/example:0.1.0@%s", registryAddress, exampleChartDigest),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).NotTo(BeEmpty())
	})

	It("should use the cache", func() {
		oci := &gardencorev1.OCIRepository{
			Ref: fmt.Sprintf("%s/charts/example:0.1.0@%s", registryAddress, exampleChartDigest),
		}
		_, err := hr.Pull(ctx, oci)
		Expect(err).NotTo(HaveOccurred())
		Expect(rc.cacheHits).To(Equal(0))

		_, err = hr.Pull(ctx, oci)
		Expect(err).NotTo(HaveOccurred())
		Expect(rc.cacheHits).To(Equal(1))
	})

	It("should use the cache no matter if tag or digest is used", func() {
		_, err := hr.Pull(ctx, &gardencorev1.OCIRepository{
			Repository: registryAddress + "/charts/example",
			Digest:     exampleChartDigest,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(rc.cacheHits).To(Equal(0))

		_, err = hr.Pull(ctx, &gardencorev1.OCIRepository{
			Repository: registryAddress + "/charts/example",
			Tag:        "0.1.0",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(rc.cacheHits).To(Equal(1))
	})
})

type recordingCache struct {
	cache     cacher
	cacheHits int
}

func (rc *recordingCache) Get(k string) ([]byte, bool) {
	out, found := rc.cache.Get(k)
	if found {
		rc.cacheHits++
	}
	return out, found
}

func (rc *recordingCache) Set(k string, blob []byte) {
	rc.cache.Set(k, blob)
}

var _ = Describe("buildRef", func() {
	const digest = "sha256:7a855a6d69033dd3240d9648e8bd46a67a528059158e098c7794ac9227735b4a"

	DescribeTable("buildRef",
		func(oci *gardencorev1.OCIRepository, want name.Reference) {
			Expect(buildRef(oci)).To(Equal(want))
		},
		Entry("ref without digest",
			&gardencorev1.OCIRepository{Ref: "example.com/foo:1.0.0"},
			mustNewTag("example.com/foo:1.0.0"),
		),
		Entry("ref with tag and digest",
			&gardencorev1.OCIRepository{Ref: "example.com/foo:1.0.0@" + digest},
			mustNewDigest("example.com/foo:1.0.0@"+digest),
		),
		Entry("repository with tag",
			&gardencorev1.OCIRepository{Repository: "example.com/foo", Tag: "1.0.0"},
			mustNewTag("example.com/foo:1.0.0"),
		),
		Entry("repository with tag and digest",
			&gardencorev1.OCIRepository{Repository: "oci://example.com/foo", Tag: "1.0.0", Digest: digest},
			mustNewDigest("example.com/foo@"+digest),
		),
		Entry("replace registry and configure insecure in local setup",
			&gardencorev1.OCIRepository{Ref: "localhost:5001/foo:1.0.0"},
			name.MustParseReference("garden.local.gardener.cloud:5001/foo:1.0.0", name.Insecure),
		),
	)
})

func mustNewTag(s string) name.Tag {
	t, err := name.NewTag(s)
	if err != nil {
		utilruntime.Must(err)
	}
	return t
}

func mustNewDigest(s string) name.Digest {
	t, err := name.NewDigest(s)
	if err != nil {
		utilruntime.Must(err)
	}
	return t
}
