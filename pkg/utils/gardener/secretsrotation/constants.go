// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
