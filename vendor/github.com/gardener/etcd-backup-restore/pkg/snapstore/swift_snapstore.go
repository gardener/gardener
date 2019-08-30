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
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/objectstorage/v1/objects"
	"github.com/gophercloud/gophercloud/pagination"
	"github.com/gophercloud/utils/openstack/clientconfig"
)

// SwiftSnapStore is snapstore with Openstack Swift as backend
type SwiftSnapStore struct {
	prefix string
	client *gophercloud.ServiceClient
	bucket string
	// maxParallelChunkUploads hold the maximum number of parallel chunk uploads allowed.
	maxParallelChunkUploads int
	tempDir                 string
}

const (
	swiftNoOfChunk int64 = 1000 //Default configuration in swift installation
)

// NewSwiftSnapStore create new SwiftSnapStore from shared configuration with specified bucket
func NewSwiftSnapStore(bucket, prefix, tempDir string, maxParallelChunkUploads int) (*SwiftSnapStore, error) {
	authOpts, err := clientconfig.AuthOptions(nil)
	if err != nil {
		return nil, err
	}
	// AllowReauth should be set to true if you grant permission for Gophercloud to
	// cache your credentials in memory, and to allow Gophercloud to attempt to
	// re-authenticate automatically if/when your token expires.
	authOpts.AllowReauth = true
	provider, err := openstack.AuthenticatedClient(*authOpts)
	if err != nil {
		return nil, err

	}
	client, err := openstack.NewObjectStorageV1(provider, gophercloud.EndpointOpts{})
	if err != nil {
		return nil, err

	}

	return NewSwiftSnapstoreFromClient(bucket, prefix, tempDir, maxParallelChunkUploads, client), nil

}

// NewSwiftSnapstoreFromClient will create the new Swift snapstore object from Swift client
func NewSwiftSnapstoreFromClient(bucket, prefix, tempDir string, maxParallelChunkUploads int, cli *gophercloud.ServiceClient) *SwiftSnapStore {
	return &SwiftSnapStore{
		bucket:                  bucket,
		prefix:                  prefix,
		client:                  cli,
		maxParallelChunkUploads: maxParallelChunkUploads,
		tempDir:                 tempDir,
	}
}

// Fetch should open reader for the snapshot file from store
func (s *SwiftSnapStore) Fetch(snap Snapshot) (io.ReadCloser, error) {
	resp := objects.Download(s.client, s.bucket, path.Join(s.prefix, snap.SnapDir, snap.SnapName), nil)
	return resp.Body, resp.Err
}

// Save will write the snapshot to store
func (s *SwiftSnapStore) Save(snap Snapshot, rc io.ReadCloser) error {
	// Save it locally
	tmpfile, err := ioutil.TempFile(s.tempDir, tmpBackupFilePrefix)
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
		chunkSize  = int64(math.Max(float64(minChunkSize), float64(size/swiftNoOfChunk)))
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

	for i := 0; i < s.maxParallelChunkUploads; i++ {
		wg.Add(1)
		go s.chunkUploader(&wg, cancelCh, &snap, tmpfile, chunkUploadCh, resCh)
	}

	logrus.Infof("Uploading snapshot of size: %d, chunkSize: %d, noOfChunks: %d", size, chunkSize, noOfChunks)
	for offset, index := int64(0), 1; offset <= size; offset += int64(chunkSize) {
		newChunk := chunk{
			id:     index,
			offset: offset,
			size:   chunkSize,
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
	logrus.Info("All chunk uploaded successfully. Uploading manifest.")
	b := make([]byte, 0)
	opts := objects.CreateOpts{
		Content:        bytes.NewReader(b),
		ContentLength:  chunkSize,
		ObjectManifest: path.Join(s.bucket, s.prefix, snap.SnapDir, snap.SnapName),
	}
	if res := objects.Create(s.client, s.bucket, path.Join(s.prefix, snap.SnapDir, snap.SnapName), opts); res.Err != nil {
		return fmt.Errorf("failed uploading manifest for snapshot with error: %v", res.Err)
	}
	logrus.Info("Manifest object uploaded successfully.")
	return nil
}

func (s *SwiftSnapStore) uploadChunk(snap *Snapshot, file *os.File, offset, chunkSize int64) error {
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}
	size := fileInfo.Size() - offset
	if size > chunkSize {
		size = chunkSize
	}

	sr := io.NewSectionReader(file, offset, size)

	opts := objects.CreateOpts{
		Content:       sr,
		ContentLength: size,
	}
	partNumber := ((offset / chunkSize) + 1)
	res := objects.Create(s.client, s.bucket, path.Join(s.prefix, snap.SnapDir, snap.SnapName, fmt.Sprintf("%010d", partNumber)), opts)
	return res.Err
}

func (s *SwiftSnapStore) chunkUploader(wg *sync.WaitGroup, stopCh <-chan struct{}, snap *Snapshot, file *os.File, chunkUploadCh chan chunk, errCh chan<- chunkUploadResult) {
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
			err := s.uploadChunk(snap, file, chunk.offset, chunk.size)
			errCh <- chunkUploadResult{
				err:   err,
				chunk: &chunk,
			}
		}
	}
}

// List will list the snapshots from store
func (s *SwiftSnapStore) List() (SnapList, error) {
	opts := &objects.ListOpts{
		Full:   false,
		Prefix: s.prefix,
	}
	// Retrieve a pager (i.e. a paginated collection)
	pager := objects.List(s.client, s.bucket, opts)
	var snapList SnapList
	// Define an anonymous function to be executed on each page's iteration
	err := pager.EachPage(func(page pagination.Page) (bool, error) {

		objectList, err := objects.ExtractNames(page)
		if err != nil {
			return false, err
		}
		for _, object := range objectList {
			name := strings.Replace(object, s.prefix+"/", "", 1)
			snap, err := ParseSnapshot(name)
			if err != nil {
				// Warning: the file can be a non snapshot file. Do not return error.
				logrus.Warnf("Invalid snapshot found. Ignoring it:%s, %v", name, err)
			} else {
				snapList = append(snapList, snap)
			}
		}
		return true, nil

	})
	if err != nil {
		return nil, err
	}

	sort.Sort(snapList)
	return snapList, nil

}

// Delete should delete the snapshot file from store
func (s *SwiftSnapStore) Delete(snap Snapshot) error {
	result := objects.Delete(s.client, s.bucket, path.Join(s.prefix, snap.SnapDir, snap.SnapName), nil)
	return result.Err
}
