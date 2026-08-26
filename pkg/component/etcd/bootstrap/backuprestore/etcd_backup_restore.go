// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package backuprestore

import (
	"fmt"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	staticpodtranslator "github.com/gardener/gardener/pkg/gardenadm/staticpod"
)

const (
	// EtcdConfigFileName is the key/filename of the etcd config inside the ConfigMap.
	EtcdConfigFileName = "etcd.conf.yaml"

	configMapName = "etcd-bootstrap-main-config"

	volumeNameBackupBuckets = "backup-buckets"
	volumeNameRestoreTmp    = "restoration-tmp"
	volumeNameEtcdConf      = "etcd-conf"

	volumeMountPathBackupBuckets = "/root"
	volumeMountPathRestoreTmp    = "/tmp/restorationtmp"
	volumeMountPathEtcdConf      = "/var/etcd/config"
)

// Config contains configuration for running etcdbrctl initialize before starting the bootstrap etcd.
//
// The etcdbrctl-initialize init container is only added when this config is not nil.
type Config struct {
	EtcdbrctlImage        string
	StoreContainer        string
	StorePrefix           string
	BackupBucketsHostPath string
}

// ConfigFromBackupDataPath builds a Config from the local backup data path on the node and the etcdbrctl image.
// The path is expected to have the structure:
//
//	<backupBucketsRoot>/<bucketName>/<namespace>--<uid>/etcd-main/v2
func ConfigFromBackupDataPath(path, etcdbrctlImage string) (*Config, error) {
	if path == "" {
		return nil, fmt.Errorf("backup data path must not be empty")
	}

	// Strip the trailing version dir (e.g. "v2") to get the etcd-main dir, then walk up the structure.
	etcdMainDir := filepath.Dir(path)                                  // .../etcd-main
	entryDir := filepath.Dir(etcdMainDir)                              // .../<namespace>--<uid>
	bucketDir := filepath.Dir(entryDir)                                // .../<bucketName>
	backupBucketsRoot := filepath.Dir(bucketDir)                       // <backupBucketsRoot>
	storeContainer := filepath.Base(bucketDir)                         // <bucketName>
	storePrefix := filepath.Join(filepath.Base(entryDir), "etcd-main") // <namespace>--<uid>/etcd-main

	if storeContainer == "" || storeContainer == "." || storeContainer == string(filepath.Separator) ||
		backupBucketsRoot == "" || filepath.Base(entryDir) == "." {
		return nil, fmt.Errorf("backup data path %q does not have the expected structure <backupBucketsRoot>/<bucketName>/<namespace>--<uid>/etcd-main/v2", path)
	}

	return &Config{
		EtcdbrctlImage:        etcdbrctlImage,
		StoreContainer:        storeContainer,
		StorePrefix:           storePrefix,
		BackupBucketsHostPath: backupBucketsRoot,
	}, nil
}

// ShouldRun reports whether the etcdbrctl-initialize init container should be injected.
func (cfg *Config) ShouldRun() bool {
	return cfg != nil &&
		cfg.BackupBucketsHostPath != "" &&
		cfg.StoreContainer != ""
}

// ConfigMap returns the fully populated etcd-config ConfigMap for the etcdbrctl-initialize init container.
func (cfg *Config) ConfigMap(namespace string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
		},
		Data: map[string]string{
			EtcdConfigFileName: cfg.EtcdInitializeConfig(),
		},
	}
}

// InitContainer returns the etcdbrctl-initialize init container spec.
//
// The backup-restore initialization only ever runs for the main etcd (see ShouldRun and the caller in
// staticpods.go), hence the main etcd's data directory is used directly.
func (cfg *Config) InitContainer(dataVolumeName string) corev1.Container {
	dataDir := staticpodtranslator.StatefulSetVolumeClaimTemplateHostPath(etcd.Name(v1beta1constants.ETCDRoleMain))

	return corev1.Container{
		Name:            "etcdbrctl-initialize",
		Image:           cfg.EtcdbrctlImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:                new(int64(0)),
			RunAsGroup:               new(int64(0)),
			AllowPrivilegeEscalation: new(false),
		},
		Args: []string{
			// using "initialize" (instead of the "restore" command) as it is the safer, idempotent
			// choice for a re-runnable init container: it validates the existing data directory and
			// only restores from the backup store when the data is missing or corrupt (in contrast to
			// "restore" which unconditionally overwrites the data directory from the latest snapshot
			// without validation, and would roll back healthy state on every pod restart)
			"initialize",
			"--storage-provider=Local",
			"--store-container=" + cfg.StoreContainer,
			"--store-prefix=" + cfg.StorePrefix,
			"--data-dir=" + filepath.Join(dataDir, "new.etcd"),
			"--restoration-temp-snapshots-dir=" + volumeMountPathRestoreTmp,
		},
		Env: []corev1.EnvVar{
			{Name: "POD_NAME", Value: "etcd-bootstrap-main"},
			{Name: "POD_NAMESPACE", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}}},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: volumeNameBackupBuckets, MountPath: volumeMountPathBackupBuckets},
			{Name: dataVolumeName, MountPath: dataDir},
			{Name: volumeNameRestoreTmp, MountPath: volumeMountPathRestoreTmp},
			{Name: volumeNameEtcdConf, MountPath: volumeMountPathEtcdConf},
		},
	}
}

// Volumes returns the volumes needed by the etcdbrctl-initialize init container.
func (cfg *Config) Volumes() []corev1.Volume {
	return []corev1.Volume{
		{Name: volumeNameBackupBuckets, VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: cfg.BackupBucketsHostPath, Type: new(corev1.HostPathDirectoryOrCreate)}}},
		{Name: volumeNameRestoreTmp, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
		{Name: volumeNameEtcdConf, VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: configMapName}, Items: []corev1.KeyToPath{{Key: EtcdConfigFileName, Path: EtcdConfigFileName}}}}},
	}
}

// EtcdInitializeConfig returns the etcd config YAML used during initialization.
func (cfg *Config) EtcdInitializeConfig() string {
	return `advertise-client-urls:
  etcd-bootstrap-main:
  - https://localhost:2379
auto-compaction-mode: periodic
auto-compaction-retention: 30m
client-transport-security:
  auto-tls: false
  cert-file: /var/etcd/ssl/server/tls.crt
  client-cert-auth: true
  key-file: /var/etcd/ssl/server/tls.key
  trusted-ca-file: /var/etcd/ssl/ca/bundle.crt
enable-v2: false
initial-advertise-peer-urls:
  etcd-bootstrap-main:
  - http://localhost:2380
initial-cluster: etcd-bootstrap-main=http://localhost:2380
initial-cluster-state: new
initial-cluster-token: etcd-cluster
listen-client-urls: https://0.0.0.0:2379
listen-peer-urls: http://0.0.0.0:2380
metrics: extensive
name: etcd-config
quota-backend-bytes: 8589934592
snapshot-count: 10000
`
}
