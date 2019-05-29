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
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"sort"
	"sync"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/sirupsen/logrus"
)

const (
	absStorageAccount = "STORAGE_ACCOUNT"
	absStorageKey     = "STORAGE_KEY"
)

// ABSSnapStore is an ABS backed snapstore.
type ABSSnapStore struct {
	containerURL *azblob.ContainerURL
	prefix       string
	// maxParallelChunkUploads hold the maximum number of parallel chunk uploads allowed.
	maxParallelChunkUploads int
	tempDir                 string
}

// NewABSSnapStore create new ABSSnapStore from shared configuration with specified bucket
func NewABSSnapStore(container, prefix, tempDir string, maxParallelChunkUploads int) (*ABSSnapStore, error) {
	storageAccount, err := GetEnvVarOrError(absStorageAccount)
	if err != nil {
		return nil, err
	}
	storageKey, err := GetEnvVarOrError(absStorageKey)
	if err != nil {
		return nil, err
	}

	credentials, err := azblob.NewSharedKeyCredential(storageAccount, storageKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create shared key credentials: %v", err)
	}

	p := azblob.NewPipeline(credentials, azblob.PipelineOptions{
		Retry: azblob.RetryOptions{
			TryTimeout: downloadTimeout,
		}})
	u, err := url.Parse(fmt.Sprintf("https://%s.%s", storageAccount, AzureBlobStorageHostName))
	if err != nil {
		return nil, fmt.Errorf("failed to parse service url: %v", err)
	}
	serviceURL := azblob.NewServiceURL(*u, p)
	containerURL := serviceURL.NewContainerURL(container)
	return GetABSSnapstoreFromClient(container, prefix, tempDir, maxParallelChunkUploads, &containerURL)
}

// GetABSSnapstoreFromClient returns a new ABS object for a given container using the supplied storageClient
func GetABSSnapstoreFromClient(container, prefix, tempDir string, maxParallelChunkUploads int, containerURL *azblob.ContainerURL) (*ABSSnapStore, error) {
	// Check if supplied container exists
	ctx, cancel := context.WithTimeout(context.TODO(), providerConnectionTimeout)
	defer cancel()
	_, err := containerURL.GetProperties(ctx, azblob.LeaseAccessConditions{})
	if err != nil {
		aer, ok := err.(azblob.StorageError)
		if !ok {
			return nil, fmt.Errorf("failed to get properties of container %v with err, %v", container, err.Error())
		}
		if aer.ServiceCode() != azblob.ServiceCodeContainerNotFound {
			return nil, fmt.Errorf("failed to get properties of container %v with err, %v", container, aer.Error())
		}
		return nil, fmt.Errorf("container %s does not exist", container)
	}
	return &ABSSnapStore{
		prefix:                  prefix,
		containerURL:            containerURL,
		maxParallelChunkUploads: maxParallelChunkUploads,
		tempDir:                 tempDir,
	}, nil
}

// Fetch should open reader for the snapshot file from store
func (a *ABSSnapStore) Fetch(snap Snapshot) (io.ReadCloser, error) {
	blobName := path.Join(a.prefix, snap.SnapDir, snap.SnapName)
	blob := a.containerURL.NewBlobURL(blobName)
	resp, err := blob.Download(context.Background(), io.SeekStart, azblob.CountToEnd, azblob.BlobAccessConditions{}, false)
	if err != nil {
		return nil, fmt.Errorf("failed to download the blob %s with error:%v", blobName, err)
	}
	return resp.Body(azblob.RetryReaderOptions{}), nil
}

// List will list all snapshot files on store
func (a *ABSSnapStore) List() (SnapList, error) {
	var snapList SnapList
	opts := azblob.ListBlobsSegmentOptions{Prefix: path.Join(a.prefix, "/")}
	for marker := (azblob.Marker{}); marker.NotDone(); {
		// Get a result segment starting with the blob indicated by the current Marker.
		listBlob, err := a.containerURL.ListBlobsFlatSegment(context.TODO(), marker, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list the blobs, error: %v", err)
		}
		marker = listBlob.NextMarker

		// Process the blobs returned in this result segment
		for _, blob := range listBlob.Segment.BlobItems {
			k := blob.Name[len(a.prefix)+1:]
			s, err := ParseSnapshot(k)
			if err != nil {
				logrus.Warnf("Invalid snapshot found. Ignoring it:%s\n", k)
			} else {
				snapList = append(snapList, s)
			}
		}
	}
	sort.Sort(snapList)
	return snapList, nil
}

// Save will write the snapshot to store
func (a *ABSSnapStore) Save(snap Snapshot, rc io.ReadCloser) error {
	// Save it locally
	tmpfile, err := ioutil.TempFile(a.tempDir, tmpBackupFilePrefix)
	if err != nil {
		rc.Close()
		return fmt.Errorf("failed to create snapshot tempfile: %v", err)
	}
	defer func() {
		tmpfile.Close()
		os.Remove(tmpfile.Name())
	}()
	size, err := io.Copy(tmpfile, rc)
	rc.Close()
	if err != nil {
		return fmt.Errorf("failed to save snapshot to tmpfile: %v", err)
	}

	var (
		chunkSize  = minChunkSize
		noOfChunks = size / chunkSize
	)
	if size%chunkSize != 0 {
		noOfChunks++
	}

	var (
		chunkUploadCh = make(chan chunk, noOfChunks)
		resCh         = make(chan chunkUploadResult, noOfChunks)
		wg            sync.WaitGroup
		cancelCh      = make(chan struct{})
	)

	for i := 0; i < a.maxParallelChunkUploads; i++ {
		wg.Add(1)
		go a.blockUploader(&wg, cancelCh, &snap, tmpfile, chunkUploadCh, resCh)
	}
	logrus.Infof("Uploading snapshot of size: %d, chunkSize: %d, noOfChunks: %d", size, chunkSize, noOfChunks)
	for offset, index := int64(0), 1; offset <= size; offset += int64(chunkSize) {
		newChunk := chunk{
			offset: offset,
			size:   chunkSize,
			id:     index,
		}
		chunkUploadCh <- newChunk
		index++
	}
	logrus.Infof("Triggered chunk upload for all chunks, total: %d", noOfChunks)

	snapshotErr := collectChunkUploadError(chunkUploadCh, resCh, cancelCh, noOfChunks)
	wg.Wait()

	if snapshotErr != nil {
		return fmt.Errorf("failed uploading chunk, id: %d, offset: %d, error: %v", snapshotErr.chunk.id, snapshotErr.chunk.offset, snapshotErr.err)
	}
	logrus.Info("All chunk uploaded successfully. Uploading blocklist.")
	blobName := path.Join(a.prefix, snap.SnapDir, snap.SnapName)
	blob := a.containerURL.NewBlockBlobURL(blobName)
	var blockList []string
	for partNumber := int64(1); partNumber <= noOfChunks; partNumber++ {
		blockID := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%010d", partNumber)))
		blockList = append(blockList, blockID)
	}
	ctx, cancel := context.WithTimeout(context.TODO(), chunkUploadTimeout)
	defer cancel()
	if _, err := blob.CommitBlockList(ctx, blockList, azblob.BlobHTTPHeaders{}, azblob.Metadata{}, azblob.BlobAccessConditions{}); err != nil {
		return fmt.Errorf("failed uploading blocklist for snapshot with error: %v", err)
	}
	logrus.Info("Blocklist uploaded successfully.")
	return nil
}

func (a *ABSSnapStore) uploadBlock(snap *Snapshot, file *os.File, offset, chunkSize int64) error {
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}

	size := fileInfo.Size() - offset
	if size > chunkSize {
		size = chunkSize
	}

	sr := io.NewSectionReader(file, offset, size)
	blobName := path.Join(a.prefix, snap.SnapDir, snap.SnapName)
	blob := a.containerURL.NewBlockBlobURL(blobName)
	partNumber := ((offset / chunkSize) + 1)
	blockID := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%010d", partNumber)))
	ctx, cancel := context.WithTimeout(context.TODO(), chunkUploadTimeout)
	defer cancel()
	var transactionMD5 []byte
	if _, err := blob.StageBlock(ctx, blockID, sr, azblob.LeaseAccessConditions{}, transactionMD5); err != nil {
		return fmt.Errorf("failed to upload chunk offset: %d, blob: %s, error: %v", offset, blobName, err)
	}
	return nil
}

func (a *ABSSnapStore) blockUploader(wg *sync.WaitGroup, stopCh <-chan struct{}, snap *Snapshot, file *os.File, chunkUploadCh chan chunk, errCh chan<- chunkUploadResult) {
	defer wg.Done()
	for {
		select {
		case <-stopCh:
			return
		case chunk, ok := <-chunkUploadCh:
			if !ok {
				return
			}
			logrus.Infof("Uploading chunk with offset : %d, attempt: %d", chunk.offset, chunk.attempt)
			err := a.uploadBlock(snap, file, chunk.offset, chunk.size)
			errCh <- chunkUploadResult{
				err:   err,
				chunk: &chunk,
			}
		}
	}
}

// Delete should delete the snapshot file from store
func (a *ABSSnapStore) Delete(snap Snapshot) error {
	blobName := path.Join(a.prefix, snap.SnapDir, snap.SnapName)
	blob := a.containerURL.NewBlobURL(blobName)
	if _, err := blob.Delete(context.TODO(), azblob.DeleteSnapshotsOptionInclude, azblob.BlobAccessConditions{}); err != nil {
		return fmt.Errorf("failed to delete blob %s with error: %v", blobName, err)
	}
	return nil
}
