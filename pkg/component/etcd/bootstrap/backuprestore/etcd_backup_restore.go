// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backuprestore

import (
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
// The init container is only added when this config is not nil.
type Config struct {
	EtcdbrctlImage        string
	StoreContainer        string
	StorePrefix           string
	BackupBucketsHostPath string
}

// ShouldRun reports whether the backup-restore init container should be injected.
func (cfg *Config) ShouldRun() bool {
	return cfg != nil &&
		cfg.BackupBucketsHostPath != "" &&
		cfg.StoreContainer != ""
}

// ConfigMap returns the fully populated etcd-config ConfigMap for the etcdbrctl init container.
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
func (cfg *Config) InitContainer(role, dataVolumeName string) corev1.Container {
	dataDir := staticpodtranslator.StatefulSetVolumeClaimTemplateHostPath(etcd.Name(role))

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
			{Name: dataVolumeName, MountPath: staticpodtranslator.StatefulSetVolumeClaimTemplateHostPath(etcd.Name(role))},
			{Name: volumeNameRestoreTmp, MountPath: volumeMountPathRestoreTmp},
			{Name: volumeNameEtcdConf, MountPath: volumeMountPathEtcdConf},
		},
	}
}

// Volumes returns the volumes needed by the backup-restore init container.
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
data-dir: /var/etcd/data/new.etcd
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
