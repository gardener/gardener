// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package backuprestore_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	. "github.com/gardener/gardener/pkg/component/etcd/bootstrap/backuprestore"
)

var _ = Describe("BackupRestore", func() {
	const (
		etcdbrctlImage = "europe-docker.pkg.dev/gardener-project/public/gardener/etcdbrctl:v0.43.0"
		namespace      = "shoot--foo--bar"
	)

	Describe("#ConfigFromBackupDataPath", func() {
		It("should decompose a well-formed backup data path", func() {
			cfg, err := ConfigFromBackupDataPath("/etc/gardener/local-backupbuckets/my-bucket/shoot--foo--bar--abc123/etcd-main/v2", etcdbrctlImage)
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg).To(Equal(&Config{
				EtcdbrctlImage:        etcdbrctlImage,
				StoreContainer:        "my-bucket",
				StorePrefix:           "shoot--foo--bar--abc123/etcd-main",
				BackupBucketsHostPath: "/etc/gardener/local-backupbuckets",
			}))
		})

		It("should pass the etcdbrctl image through unchanged", func() {
			cfg, err := ConfigFromBackupDataPath("/root/bucket/ns--uid/etcd-main/v2", "some-other-image:latest")
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.EtcdbrctlImage).To(Equal("some-other-image:latest"))
		})

		It("should return an error for an empty path", func() {
			cfg, err := ConfigFromBackupDataPath("", etcdbrctlImage)
			Expect(err).To(MatchError(ContainSubstring("must not be empty")))
			Expect(cfg).To(BeNil())
		})

		It("should return an error for a too-short/malformed path", func() {
			cfg, err := ConfigFromBackupDataPath("etcd-main/v2", etcdbrctlImage)
			Expect(err).To(MatchError(ContainSubstring("does not have the expected structure")))
			Expect(cfg).To(BeNil())
		})
	})

	Describe("#ShouldRun", func() {
		It("should return false for a nil config", func() {
			var cfg *Config
			Expect(cfg.ShouldRun()).To(BeFalse())
		})

		It("should return false when required fields are empty", func() {
			Expect((&Config{}).ShouldRun()).To(BeFalse())
			Expect((&Config{BackupBucketsHostPath: "/root"}).ShouldRun()).To(BeFalse())
			Expect((&Config{StoreContainer: "bucket"}).ShouldRun()).To(BeFalse())
		})

		It("should return true when required fields are populated", func() {
			cfg := &Config{BackupBucketsHostPath: "/root", StoreContainer: "bucket"}
			Expect(cfg.ShouldRun()).To(BeTrue())
		})
	})

	Describe("#InitContainer", func() {
		var cfg *Config

		BeforeEach(func() {
			cfg = &Config{
				EtcdbrctlImage:        etcdbrctlImage,
				StoreContainer:        "my-bucket",
				StorePrefix:           "shoot--foo--bar--abc123/etcd-main",
				BackupBucketsHostPath: "/etc/gardener/local-backupbuckets",
			}
		})

		It("should render the etcdbrctl-initialize init container spec", func() {
			container := cfg.InitContainer("data")

			Expect(container.Name).To(Equal("etcdbrctl-initialize"))
			Expect(container.Image).To(Equal(etcdbrctlImage))
			Expect(container.Args).To(Equal([]string{
				"initialize",
				"--storage-provider=Local",
				"--store-container=my-bucket",
				"--store-prefix=shoot--foo--bar--abc123/etcd-main",
				"--data-dir=/var/lib/etcd-main/data/new.etcd",
				"--restoration-temp-snapshots-dir=/tmp/restorationtmp",
			}))
			Expect(container.Env).To(ContainElement(
				MatchFields(IgnoreExtras, Fields{"Name": Equal("POD_NAME"), "Value": Equal("etcd-bootstrap-main")}),
			))
			Expect(container.VolumeMounts).To(ConsistOf(
				MatchFields(IgnoreExtras, Fields{"Name": Equal("backup-buckets"), "MountPath": Equal("/root")}),
				MatchFields(IgnoreExtras, Fields{"Name": Equal("data"), "MountPath": Equal("/var/lib/etcd-main/data")}),
				MatchFields(IgnoreExtras, Fields{"Name": Equal("restoration-tmp"), "MountPath": Equal("/tmp/restorationtmp")}),
				MatchFields(IgnoreExtras, Fields{"Name": Equal("etcd-conf"), "MountPath": Equal("/var/etcd/config")}),
			))
		})
	})

	Describe("#Volumes", func() {
		It("should render the volumes needed by the init container", func() {
			cfg := &Config{BackupBucketsHostPath: "/etc/gardener/local-backupbuckets"}
			volumes := cfg.Volumes()

			Expect(volumes).To(ConsistOf(
				MatchFields(IgnoreExtras, Fields{"Name": Equal("backup-buckets")}),
				MatchFields(IgnoreExtras, Fields{"Name": Equal("restoration-tmp")}),
				MatchFields(IgnoreExtras, Fields{"Name": Equal("etcd-conf")}),
			))

			for _, v := range volumes {
				if v.Name == "backup-buckets" {
					Expect(v.VolumeSource.HostPath).NotTo(BeNil())
					Expect(v.VolumeSource.HostPath.Path).To(Equal("/etc/gardener/local-backupbuckets"))
				}
			}
		})
	})

	Describe("#ConfigMap", func() {
		It("should render the etcd-config ConfigMap", func() {
			cfg := &Config{}
			cm := cfg.ConfigMap(namespace)

			Expect(cm.Name).To(Equal("etcd-bootstrap-main-config"))
			Expect(cm.Namespace).To(Equal(namespace))
			Expect(cm.Data).To(HaveKey(EtcdConfigFileName))
		})
	})

	Describe("#EtcdInitializeConfig", func() {
		It("should render the etcd config YAML with the expected cluster fields", func() {
			config := (&Config{}).EtcdInitializeConfig()

			Expect(config).To(ContainSubstring("advertise-client-urls:"))
			Expect(config).To(ContainSubstring("initial-advertise-peer-urls:"))
			Expect(config).To(ContainSubstring("initial-cluster: etcd-bootstrap-main=http://localhost:2380"))
			Expect(config).To(ContainSubstring("etcd-bootstrap-main:"))
			// data-dir is intentionally not set here; the restore target comes from the init container's --data-dir flag.
			Expect(config).NotTo(ContainSubstring("data-dir:"))
		})
	})
})
