// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package snapstore

import (
	"io"
	"time"
)

// SnapStore is the interface to be implemented for different
// storage backend like local file system, S3, ABS, GCS, Swift, OSS etc.
// Only purpose of these implementation to provide CPI layer to
// access files.
type SnapStore interface {
	// Fetch should open reader for the snapshot file from store.
	Fetch(Snapshot) (io.ReadCloser, error)
	// List will list all snapshot files on store.
	List() (SnapList, error)
	// Save will write the snapshot to store.
	Save(Snapshot, io.ReadCloser) error
	// Delete should delete the snapshot file from store.
	Delete(Snapshot) error
}

const (
	// minChunkSize is set to 5Mib since it is lower chunk size limit for AWS.
	minChunkSize int64 = 5 * (1 << 20) //5 MiB
	// SnapstoreProviderLocal is constant for local disk storage provider.
	SnapstoreProviderLocal = "Local"
	// SnapstoreProviderS3 is constant for aws S3 storage provider.
	SnapstoreProviderS3 = "S3"
	// SnapstoreProviderABS is constant for azure blob storage provider.
	SnapstoreProviderABS = "ABS"
	// SnapstoreProviderGCS is constant for GCS object storage provider.
	SnapstoreProviderGCS = "GCS"
	// SnapstoreProviderSwift is constant for Swift object storage.
	SnapstoreProviderSwift = "Swift"
	// SnapstoreProviderOSS is constant for Alicloud OSS storage provider.
	SnapstoreProviderOSS = "OSS"
	// SnapshotKindFull is constant for full snapshot kind.
	SnapshotKindFull = "Full"
	// SnapshotKindDelta is constant for delta snapshot kind.
	SnapshotKindDelta = "Incr"
	// SnapshotKindChunk is constant for chunk snapshot kind.
	SnapshotKindChunk = "Chunk"

	// chunkUploadTimeout is timeout for uploading chunk.
	chunkUploadTimeout = 180 * time.Second
	// providerConnectionTimeout is timeout for connection/short queries to cloud provider.
	providerConnectionTimeout = 30 * time.Second
	// downloadTimeout is timeout for downloading chunk.
	downloadTimeout = 5 * time.Minute

	tmpBackupFilePrefix = "etcd-backup-"

	// maxRetryAttempts indicates the number of attempts to be retried in case of failure to upload chunk.
	maxRetryAttempts = 5

	// AzureBlobStorageHostName is the host name for azure blob storage service.
	AzureBlobStorageHostName = "blob.core.windows.net"
)

// Snapshot structure represents the metadata of snapshot.s
type Snapshot struct {
	Kind          string //incr:incremental,full:full
	StartRevision int64
	LastRevision  int64 //latest revision on snapshot
	CreatedOn     time.Time
	SnapDir       string
	SnapName      string
	IsChunk       bool
}

// SnapList is list of snapshots.
type SnapList []*Snapshot

// Config defines the configuration to create snapshot store.
type Config struct {
	// Provider indicated the cloud provider.
	Provider string
	// Container holds the name of bucket or container to which snapshot will be stored.
	Container string
	// Prefix holds the prefix or directory under StorageContainer under which snapshot will be stored.
	Prefix string
	// MaxParallelChunkUploads hold the maximum number of parallel chunk uploads allowed.
	MaxParallelChunkUploads int
	// Temporary Directory
	TempDir string
}

type chunk struct {
	offset  int64
	size    int64
	attempt uint
	id      int
}
type chunkUploadResult struct {
	err   error
	chunk *chunk
}
