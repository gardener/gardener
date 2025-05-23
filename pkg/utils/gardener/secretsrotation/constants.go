// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secretsrotation

const (
	// AnnotationKeyNewEncryptionKeyPopulated is an annotation indicating that the new ETCD encryption key was populated
	AnnotationKeyNewEncryptionKeyPopulated = "credentials.gardener.cloud/new-encryption-key-populated"

	// AnnotationKeyResourcesLabeled is an annotation indicating the completion of labeling the resources with the credentials.gardener.cloud/key-name label
	AnnotationKeyResourcesLabeled = "credentials.gardener.cloud/resources-labeled"
	// AnnotationKeyEtcdSnapshotted is an annotation indicating that ETCD snapshot was completed
	AnnotationKeyEtcdSnapshotted = "credentials.gardener.cloud/etcd-snapshotted"

	labelKeyRotationKeyName = "credentials.gardener.cloud/key-name"
	rotationQPS             = 100
)
