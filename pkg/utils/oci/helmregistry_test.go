// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package oci

import (
	"context"
	"encoding/base64"
	"fmt"

	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory"
	"github.com/google/go-containerregistry/pkg/name"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

var _ = Describe("helmregistry", func() {
	var (
		hr  *HelmRegistry
		rc  *recordingCache
		ctx context.Context

		caBundleSecretName = "test-ca-bundle"
		caBundleSecret     *corev1.Secret
	)

	BeforeEach(func() {
		ctx = context.Background()
		rc = &recordingCache{cache: newCache()}

		caBundleSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      caBundleSecretName,
				Namespace: v1beta1constants.GardenNamespace,
			},
			Data: map[string][]byte{
				"bundle.crt": registryCABundle,
			},
		}
		pullSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: v1beta1constants.GardenNamespace,
				Name:      "pull-secret",
			},
			Data: map[string][]byte{
				corev1.DockerConfigJsonKey: fmt.Appendf(nil, "{\"auths\":{\"%s\":{\"username\":\"foo\",\"password\":\"bar\"}}}", registryAddress),
			},
		}

		fakeClient := fake.NewClientBuilder().WithObjects(caBundleSecret, pullSecret).Build()
		hr = &HelmRegistry{cache: rc, client: fakeClient}
		authProvider.receivedAuthorization = "" // Reset before test
	})

	It("should return error if the repository does not exist", func() {
		_, err := hr.Pull(ctx, &gardencorev1.OCIRepository{
			Repository: ptr.To(registryAddress + "/charts/examplexxx"),
			Tag:        ptr.To("0.1.0"),
		})
		Expect(err).To(MatchError(ContainSubstring("failed get manifest from remote")))
	})

	It("should return error if the digest does not exist", func() {
		_, err := hr.Pull(ctx, &gardencorev1.OCIRepository{
			Repository: ptr.To(registryAddress + "/charts/examplexxx"),
			Digest:     ptr.To("sha256:7a855a6d69033dd3240d9648e8bd46a67a528059158e098c7794ac9227735b4a"),
		})
		Expect(err).To(MatchError(ContainSubstring("failed to pull artifact")))
	})

	It("should pull the chart by tag", func() {
		out, err := hr.Pull(ctx, &gardencorev1.OCIRepository{
			Repository:        ptr.To(registryAddress + "/charts/example"),
			Tag:               ptr.To("0.1.0"),
			CABundleSecretRef: &corev1.LocalObjectReference{Name: caBundleSecretName},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).NotTo(BeEmpty())
	})

	It("should pull the chart by digest", func() {
		out, err := hr.Pull(ctx, &gardencorev1.OCIRepository{
			Repository:        ptr.To(registryAddress + "/charts/example"),
			Digest:            ptr.To(exampleChartDigest),
			CABundleSecretRef: &corev1.LocalObjectReference{Name: caBundleSecretName},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).NotTo(BeEmpty())
	})

	It("should pull the chart with ref", func() {
		out, err := hr.Pull(ctx, &gardencorev1.OCIRepository{
			Ref:               ptr.To(fmt.Sprintf("%s/charts/example:0.1.0@%s", registryAddress, exampleChartDigest)),
			CABundleSecretRef: &corev1.LocalObjectReference{Name: caBundleSecretName},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).NotTo(BeEmpty())
	})

	It("should use the cache", func() {
		oci := &gardencorev1.OCIRepository{
			Ref:               ptr.To(fmt.Sprintf("%s/charts/example:0.1.0@%s", registryAddress, exampleChartDigest)),
			CABundleSecretRef: &corev1.LocalObjectReference{Name: caBundleSecretName},
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
			Repository:        ptr.To(registryAddress + "/charts/example"),
			Digest:            ptr.To(exampleChartDigest),
			CABundleSecretRef: &corev1.LocalObjectReference{Name: caBundleSecretName},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(rc.cacheHits).To(Equal(0))

		_, err = hr.Pull(ctx, &gardencorev1.OCIRepository{
			Repository:        ptr.To(registryAddress + "/charts/example"),
			Tag:               ptr.To("0.1.0"),
			CABundleSecretRef: &corev1.LocalObjectReference{Name: caBundleSecretName},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(rc.cacheHits).To(Equal(1))
	})

	It("should pull the chart with pull secret", func() {
		out, err := hr.Pull(ctx,
			&gardencorev1.OCIRepository{
				Repository:        ptr.To(registryAddress + "/charts/example"),
				Tag:               ptr.To("0.1.0"),
				CABundleSecretRef: &corev1.LocalObjectReference{Name: caBundleSecretName},
				PullSecretRef:     &corev1.LocalObjectReference{Name: "pull-secret"},
			})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).NotTo(BeEmpty())
		Expect(authProvider.receivedAuthorization).To(Equal(fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte("foo:bar")))))
	})

	It("should pull the chart by tag without pull secret if repository is not matching", func() {
		hr := &HelmRegistry{
			cache: rc,
			client: fake.NewFakeClient(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: v1beta1constants.GardenNamespace,
					Name:      "pull-secret",
				},
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: fmt.Appendf(nil, "{\"auths\":{\"%s\":{\"username\":\"foo\",\"password\":\"bar\"}}}", "other-"+registryAddress),
				},
			}, caBundleSecret)}

		out, err := hr.Pull(ctx,
			&gardencorev1.OCIRepository{
				Repository:        ptr.To(registryAddress + "/charts/example"),
				Tag:               ptr.To("0.1.0"),
				PullSecretRef:     &corev1.LocalObjectReference{Name: "pull-secret"},
				CABundleSecretRef: &corev1.LocalObjectReference{Name: caBundleSecretName},
			})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).NotTo(BeEmpty())
		Expect(authProvider.receivedAuthorization).To(BeEmpty())
	})

	It("should return error when CA bundle secret does not exist", func() {
		_, err := hr.Pull(ctx, &gardencorev1.OCIRepository{
			Repository:        ptr.To(registryAddress + "/charts/example"),
			Tag:               ptr.To("0.1.0"),
			CABundleSecretRef: &corev1.LocalObjectReference{Name: "non-existing-ca-bundle"},
		})
		Expect(err).To(MatchError(ContainSubstring("failed to get CA bundle secret")))
	})

	It("should return error when CA bundle secret contains invalid PEM", func() {
		invalidCABundleSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "invalid-ca-bundle",
				Namespace: v1beta1constants.GardenNamespace,
			},
			Data: map[string][]byte{
				"bundle.crt": []byte("invalid-pem-data"),
			},
		}
		fakeClient := fake.NewClientBuilder().WithObjects(invalidCABundleSecret).Build()
		invalidHr := &HelmRegistry{cache: rc, client: fakeClient}

		_, err := invalidHr.Pull(ctx, &gardencorev1.OCIRepository{
			Repository:        ptr.To(registryAddress + "/charts/example"),
			Tag:               ptr.To("0.1.0"),
			CABundleSecretRef: &corev1.LocalObjectReference{Name: "invalid-ca-bundle"},
		})
		Expect(err).To(MatchError("failed to append CA certificates from bundle"))
	})

	It("should fail without CA bundle for TLS registry", func() {
		_, err := hr.Pull(ctx, &gardencorev1.OCIRepository{
			Repository: ptr.To(registryAddress + "/charts/example"),
			Tag:        ptr.To("0.1.0"),
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("tls: failed to verify certificate: x509"))
	})

	It("should work with CA bundle and pull secret together", func() {
		out, err := hr.Pull(ctx, &gardencorev1.OCIRepository{
			Repository:        ptr.To(registryAddress + "/charts/example"),
			Tag:               ptr.To("0.1.0"),
			PullSecretRef:     &corev1.LocalObjectReference{Name: "pull-secret"},
			CABundleSecretRef: &corev1.LocalObjectReference{Name: caBundleSecretName},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).NotTo(BeEmpty())
		Expect(authProvider.receivedAuthorization).To(Equal(fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte("foo:bar")))))
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
			&gardencorev1.OCIRepository{Ref: ptr.To("example.com/foo:1.0.0")},
			mustNewTag("example.com/foo:1.0.0"),
		),
		Entry("ref with tag and digest",
			&gardencorev1.OCIRepository{Ref: ptr.To("example.com/foo:1.0.0@" + digest)},
			mustNewDigest("example.com/foo:1.0.0@"+digest),
		),
		Entry("repository with tag",
			&gardencorev1.OCIRepository{Repository: ptr.To("example.com/foo"), Tag: ptr.To("1.0.0")},
			mustNewTag("example.com/foo:1.0.0"),
		),
		Entry("repository with tag and digest",
			&gardencorev1.OCIRepository{Repository: ptr.To("oci://example.com/foo"), Tag: ptr.To("1.0.0"), Digest: ptr.To(digest)},
			mustNewDigest("example.com/foo@"+digest),
		),
		Entry("configure insecure in local setup when using registry.local.gardener.cloud",
			&gardencorev1.OCIRepository{Ref: ptr.To("registry.local.gardener.cloud:5001/foo:1.0.0")},
			name.MustParseReference("registry.local.gardener.cloud:5001/foo:1.0.0", name.Insecure),
		),
	)
})

func mustNewTag(s string) name.Tag {
	t, err := name.NewTag(s)
	utilruntime.Must(err)
	return t
}

func mustNewDigest(s string) name.Digest {
	t, err := name.NewDigest(s)
	utilruntime.Must(err)
	return t
}
