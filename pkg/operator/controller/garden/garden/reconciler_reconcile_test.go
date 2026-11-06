// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package garden_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/api/operator/v1alpha1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

var _ = Describe("Reconciler Reconcile", func() {
	Describe("#EtcdMainBackupBucketNameAndPrefix", func() {
		var garden *operatorv1alpha1.Garden

		BeforeEach(func() {
			garden = &operatorv1alpha1.Garden{
				ObjectMeta: metav1.ObjectMeta{UID: "garden-uid"},
			}
		})

		DescribeTable("default bucket name",
			func(etcd *operatorv1alpha1.ETCD) {
				garden.Spec.VirtualCluster.ETCD = etcd
				original := garden.DeepCopy()

				name, prefix := helper.EtcdMainBackupBucketNameAndPrefix(garden)

				Expect(name).To(Equal("garden-garden-uid"))
				Expect(prefix).To(Equal(operatorv1alpha1.VirtualGardenETCDMain))
				Expect(garden).To(Equal(original))
			},
			Entry("no ETCD config", nil),
			Entry("no ETCD Main config", &operatorv1alpha1.ETCD{}),
			Entry("no backup config", &operatorv1alpha1.ETCD{Main: &operatorv1alpha1.ETCDMain{}}),
			Entry("no bucket name", &operatorv1alpha1.ETCD{Main: &operatorv1alpha1.ETCDMain{Backup: &operatorv1alpha1.Backup{}}}),
		)

		DescribeTable("explicit bucket name",
			func(bucketName, expectedName, expectedPrefix string) {
				garden.Spec.VirtualCluster.ETCD = &operatorv1alpha1.ETCD{
					Main: &operatorv1alpha1.ETCDMain{
						Backup: &operatorv1alpha1.Backup{BucketName: &bucketName},
					},
				}
				original := garden.DeepCopy()

				name, prefix := helper.EtcdMainBackupBucketNameAndPrefix(garden)

				Expect(name).To(Equal(expectedName))
				Expect(prefix).To(Equal(expectedPrefix))
				Expect(garden).To(Equal(original))
			},
			Entry("without prefix", "my-bucket", "my-bucket", "virtual-garden-etcd-main"),
			Entry("with prefix", "my-bucket/backups", "my-bucket", "backups/virtual-garden-etcd-main"),
			Entry("with nested prefix", "my-bucket/backups/garden", "my-bucket", "backups/garden/virtual-garden-etcd-main"),
			Entry("with trailing slash", "my-bucket/backups/", "my-bucket", "backups/virtual-garden-etcd-main"),
		)
	})
})
