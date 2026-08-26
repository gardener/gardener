// SPDX-FileCopyrightText: Contributors to the Gardener project
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

	// LabelKeyCABundleName is a label key used to mark kube-root-ca.crt ConfigMaps that have already been confirmed
	// to contain a specific CA bundle. The label value is the name of the CA bundle secret.
	LabelKeyCABundleName = "credentials.gardener.cloud/ca-bundle-name"

	labelKeyRotationKeyName = "credentials.gardener.cloud/key-name"
	rotationQPS             = 100
)
