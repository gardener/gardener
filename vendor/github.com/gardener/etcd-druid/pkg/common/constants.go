// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package common

const (
	// Etcd is the key for the etcd image in the image vector.
	Etcd = "etcd"
	// BackupRestore is the key for the etcd-backup-restore image in the image vector.
	BackupRestore = "etcd-backup-restore"
	// EtcdWrapper is the key for the etcd image in the image vector.
	EtcdWrapper = "etcd-wrapper"
	// BackupRestoreDistroless is the key for the etcd-backup-restore image in the image vector.
	BackupRestoreDistroless = "etcd-backup-restore-distroless"
	// ChartPath is the directory containing the default image vector file.
	ChartPath = "charts"
	// GardenerOwnedBy is a constant for an annotation on a resource that describes the owner resource.
	GardenerOwnedBy = "gardener.cloud/owned-by"
	// GardenerOwnerType is a constant for an annotation on a resource that describes the type of owner resource.
	GardenerOwnerType = "gardener.cloud/owner-type"
	// FinalizerName is the name of the etcd finalizer.
	FinalizerName = "druid.gardener.cloud/etcd-druid"
	// STORAGE_CONTAINER is the environment variable key for the storage container.
	STORAGE_CONTAINER = "STORAGE_CONTAINER"
	// AWS_APPLICATION_CREDENTIALS is the environment variable key for AWS application credentials.
	AWS_APPLICATION_CREDENTIALS = "AWS_APPLICATION_CREDENTIALS"
	// AZURE_APPLICATION_CREDENTIALS is the environment variable key for Azure application credentials.
	AZURE_APPLICATION_CREDENTIALS = "AZURE_APPLICATION_CREDENTIALS"
	// GOOGLE_APPLICATION_CREDENTIALS is the environment variable key for Google application credentials.
	GOOGLE_APPLICATION_CREDENTIALS = "GOOGLE_APPLICATION_CREDENTIALS"
	// OPENSTACK_APPLICATION_CREDENTIALS is the environment variable key for OpenStack application credentials.
	OPENSTACK_APPLICATION_CREDENTIALS = "OPENSTACK_APPLICATION_CREDENTIALS"
	// OPENSHIFT_APPLICATION_CREDENTIALS is the environment variable key for OpenShift application credentials.
	OPENSHIFT_APPLICATION_CREDENTIALS = "OPENSHIFT_APPLICATION_CREDENTIALS"
	// ALICLOUD_APPLICATION_CREDENTIALS is the environment variable key for Alicloud application credentials.
	ALICLOUD_APPLICATION_CREDENTIALS = "ALICLOUD_APPLICATION_CREDENTIALS"
)
