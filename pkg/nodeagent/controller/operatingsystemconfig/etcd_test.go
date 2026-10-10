// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

var _ = Describe("etcd", func() {
	var (
		ctx        context.Context
		reconciler *Reconciler
	)

	BeforeEach(func() {
		ctx = context.Background()
		reconciler = &Reconciler{
			APIReader: fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build(),
		}
	})

	Describe("#fileContentForPath", func() {
		var osc *extensionsv1alpha1.OperatingSystemConfig

		BeforeEach(func() {
			osc = &extensionsv1alpha1.OperatingSystemConfig{}
		})

		It("should return not-found when path is absent", func() {
			data, found, err := reconciler.fileContentForPath(ctx, osc, "/some/path/ca.crt")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
			Expect(data).To(BeNil())
		})

		It("should return data for an inline plain-encoded file", func() {
			osc.Spec.Files = []extensionsv1alpha1.File{
				{
					Path: "/etc/ca.crt",
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: string(extensionsv1alpha1.PlainFileCodecID),
							Data:     "cert-data",
						},
					},
				},
			}

			data, found, err := reconciler.fileContentForPath(ctx, osc, "/etc/ca.crt")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(data).To(Equal([]byte("cert-data")))
		})

		It("should return an error for a file without inline or secretRef content", func() {
			osc.Spec.Files = []extensionsv1alpha1.File{
				{
					Path:    "/etc/ca.crt",
					Content: extensionsv1alpha1.FileContent{},
				},
			}

			data, found, err := reconciler.fileContentForPath(ctx, osc, "/etc/ca.crt")
			Expect(err).To(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(data).To(BeNil())
		})
	})

	Describe("#rawCADataFromOperatingSystemConfig", func() {
		inlineFile := func(path, data string) extensionsv1alpha1.File {
			return extensionsv1alpha1.File{
				Path: path,
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: string(extensionsv1alpha1.PlainFileCodecID),
						Data:     data,
					},
				},
			}
		}

		currentCertPath := func() string {
			return "/var/lib/etcd/ca/current/ca.crt"
		}
		currentKeyPath := func() string {
			return "/var/lib/etcd/ca/current/ca.key"
		}
		oldCertPath := func() string {
			return "/var/lib/etcd/ca/old/ca.crt"
		}
		oldKeyPath := func() string {
			return "/var/lib/etcd/ca/old/ca.key"
		}

		It("should return an error when the current cert is missing", func() {
			osc := &extensionsv1alpha1.OperatingSystemConfig{}

			_, _, _, _, err := reconciler.rawCADataFromOperatingSystemConfig(ctx, osc, "/var/lib/etcd/ca")
			Expect(err).To(MatchError(ContainSubstring("current ETCD-related CA not found")))
		})

		It("should return an error when the current key is missing", func() {
			osc := &extensionsv1alpha1.OperatingSystemConfig{
				Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
					Files: []extensionsv1alpha1.File{
						inlineFile(currentCertPath(), "current-cert"),
					},
				},
			}

			_, _, _, _, err := reconciler.rawCADataFromOperatingSystemConfig(ctx, osc, "/var/lib/etcd/ca")
			Expect(err).To(MatchError(ContainSubstring("current ETCD-related CA not found")))
		})

		It("should return current cert and key when old CA is absent", func() {
			osc := &extensionsv1alpha1.OperatingSystemConfig{
				Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
					Files: []extensionsv1alpha1.File{
						inlineFile(currentCertPath(), "current-cert"),
						inlineFile(currentKeyPath(), "current-key"),
					},
				},
			}

			currentCert, currentKey, oldCert, oldKey, err := reconciler.rawCADataFromOperatingSystemConfig(ctx, osc, "/var/lib/etcd/ca")
			Expect(err).NotTo(HaveOccurred())
			Expect(currentCert).To(Equal([]byte("current-cert")))
			Expect(currentKey).To(Equal([]byte("current-key")))
			Expect(oldCert).To(BeNil())
			Expect(oldKey).To(BeNil())
		})

		It("should return current and old cert+key when old CA is present", func() {
			osc := &extensionsv1alpha1.OperatingSystemConfig{
				Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
					Files: []extensionsv1alpha1.File{
						inlineFile(currentCertPath(), "current-cert"),
						inlineFile(currentKeyPath(), "current-key"),
						inlineFile(oldCertPath(), "old-cert"),
						inlineFile(oldKeyPath(), "old-key"),
					},
				},
			}

			currentCert, currentKey, oldCert, oldKey, err := reconciler.rawCADataFromOperatingSystemConfig(ctx, osc, "/var/lib/etcd/ca")
			Expect(err).NotTo(HaveOccurred())
			Expect(currentCert).To(Equal([]byte("current-cert")))
			Expect(currentKey).To(Equal([]byte("current-key")))
			Expect(oldCert).To(Equal([]byte("old-cert")))
			Expect(oldKey).To(Equal([]byte("old-key")))
		})
	})
})
