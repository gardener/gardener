// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"

	"github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

// GetETCDMainBackup returns the backup configuration for etcd main of the given garden object or nil if not configured.
func GetETCDMainBackup(garden *v1alpha1.Garden) *v1alpha1.Backup {
	if garden != nil && garden.Spec.VirtualCluster.ETCD != nil && garden.Spec.VirtualCluster.ETCD.Main != nil {
		return garden.Spec.VirtualCluster.ETCD.Main.Backup
	}
	return nil
}

// EtcdMainBackupBucketNameAndPrefixFromGarden splits the bucketName into its name and prefix component
func EtcdMainBackupBucketNameAndPrefixFromGarden(garden *v1alpha1.Garden) (string, string) {
	var bucketName *string
	if backup := GetETCDMainBackup(garden); backup != nil {
		bucketName = backup.BucketName
	}
	return EtcdMainBackupBucketNameAndPrefix(bucketName, garden.UID)
}

// EtcdMainBackupBucketNameAndPrefix splits the bucketName into its name and prefix component
func EtcdMainBackupBucketNameAndPrefix(bucketName *string, uid types.UID) (string, string) {
	prefix := v1alpha1.VirtualGardenETCDMain
	if bucketName != nil {
		name := *bucketName
		if idx := strings.Index(name, "/"); idx != -1 {
			prefix = fmt.Sprintf("%s/%s", strings.TrimSuffix(name[idx+1:], "/"), prefix)
			name = name[:idx]
		}
		return name, prefix
	}
	return "garden-" + string(uid), prefix
}
