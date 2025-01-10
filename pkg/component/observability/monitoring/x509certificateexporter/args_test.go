// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package x509certificateexporter_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	x "github.com/gardener/gardener/pkg/component/observability/monitoring/x509certificateexporter"
)

var _ = Describe("x509 certificate exporter - arg calculation", func() {
	Describe("CertificatePath", func() {
		It("Should calculate string correctly", func() {
			Expect(x.CertificatePath("/path/to/cert").AsArg()).To(Equal("--watch-file=/path/to/cert"))
		})
	})
	Describe("CertificateDirPath", func() {
		It("Should calculate string correctly", func() {
			Expect(x.CertificateDirPath("/path/to/cert/dir").AsArg()).To(Equal("--watch-dir=/path/to/cert/dir"))
		})
	})
	Describe("HostCertificates", func() {
		Describe("New HostCertificates", func() {
			Context("Relative mount path", func() {
				It("should return error", func() {
					_, err := x.NewHostCertificates("relative/path", []string{}, []string{})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("Path relative/path is not absolute file path"))
				})
			})
			Context("Windows style path", func() {
				It("should return error", func() {
					_, err := x.NewHostCertificates("C:\\windows\\path", []string{}, []string{})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("Path C:\\windows\\path is not absolute file path"))
				})
			})
			Context("Absolute paths for certificates", func() {
				It("should return host certificates", func() {
					hostCertificates, err := x.NewHostCertificates(
						"/absolute/mount/path",
						[]string{"/absolute/cert/path"},
						[]string{"/absolute/cert/dir"},
					)
					Expect(err).NotTo(HaveOccurred())
					Expect(hostCertificates.MountPath).To(Equal("/absolute/mount/path"))
					Expect(hostCertificates.CertificatePaths).To(Equal([]x.CertificatePath{"/absolute/cert/path"}))
					Expect(hostCertificates.CertificateDirPaths).To(Equal([]x.CertificateDirPath{"/absolute/cert/dir"}))
				})
			})
			Context("Mixed absolute and relative paths for certificates, paths are sorted", func() {
				It("should return host certificates", func() {
					hostCertificates, err := x.NewHostCertificates(
						"/absolute/mount/path",
						[]string{"/full/cert/path", "relative/cert/path"},
						[]string{"/full/cert/dir", "relative/cert/dir"},
					)
					Expect(err).NotTo(HaveOccurred())
					Expect(hostCertificates.MountPath).To(Equal("/absolute/mount/path"))
					Expect(hostCertificates.CertificatePaths).To(Equal([]x.CertificatePath{
						"/absolute/mount/path/relative/cert/path",
						"/full/cert/path",
					}))
					Expect(hostCertificates.CertificateDirPaths).To(Equal([]x.CertificateDirPath{
						"/absolute/mount/path/relative/cert/dir",
						"/full/cert/dir",
					}))
				})
			})
			Context("Watch only mount path", func() {
				It("should return host certificates", func() {
					hostCertificates, err := x.NewHostCertificates(
						"/absolute/mount/path",
						[]string{},
						[]string{},
					)
					Expect(err).NotTo(HaveOccurred())
					Expect(hostCertificates.MountPath).To(Equal("/absolute/mount/path"))
					Expect(hostCertificates.CertificatePaths).To(BeEmpty())
					Expect(hostCertificates.CertificateDirPaths).To(Equal([]x.CertificateDirPath{"/absolute/mount/path"}))
				})
			})
		})
		Describe("Converted to args", func() {
			Context("HostCertificates with multiple certificates and directories", func() {
				It("should return args", func() {
					hostCertificates, err := x.NewHostCertificates(
						"/absolute/mount/path",
						[]string{"/full/cert/path", "relative/cert/path"},
						[]string{"/full/cert/dir", "relative/cert/dir"},
					)
					Expect(err).NotTo(HaveOccurred())
					Expect(hostCertificates.AsArgs()).To(Equal([]string{
						"--watch-file=/absolute/mount/path/relative/cert/path",
						"--watch-file=/full/cert/path",
						"--watch-dir=/absolute/mount/path/relative/cert/dir",
						"--watch-dir=/full/cert/dir",
					}))
				})
			})
		})
	})
	Describe("SecretType", func() {
		It("should return string", func() {
			Expect(x.SecretType{Type: "my-type", Key: "my-key"}.String()).To(Equal("my-type:my-key"))
		})
		It("should return arg string", func() {
			Expect(x.SecretType{Type: "my-type", Key: "my-key"}.AsArg()).To(Equal("--secret-type=my-type:my-key"))
		})
	})
	Describe("SecretTypeList", func() {
		It("should return arg string", func() {
			Expect(x.SecretTypeList{
				{Type: "type1", Key: "key1"},
				{Type: "type2", Key: "key2"},
			}.AsArgs()).To(Equal([]string{"--secret-type=type1:key1", "--secret-type=type2:key2"}))
		})
	})
	Describe("IncludeLabels", func() {
		It("should return arg string", func() {
			Expect(x.IncludeLabels{"label1": "", "label2": "val2"}.AsArgs()).To(Equal([]string{
				"--include-label=label1",
				"--include-label=label2=val2",
			}))
		})
	})
	Describe("ExcludeLabels", func() {
		It("should return arg string", func() {
			Expect(x.ExcludeLabels{"label1": "", "label2": "val2"}.AsArgs()).To(Equal([]string{
				"--exclude-label=label1",
				"--exclude-label=label2=val2",
			}))
		})
	})
	Describe("ConfigMapKeys", func() {
		It("should return arg string", func() {
			Expect(x.ConfigMapKeys{"key1", "key2", "key2"}.AsArgs()).To(Equal([]string{
				"--configmap-keys=key1",
				"--configmap-keys=key2",
			}))
		})
	})
})
