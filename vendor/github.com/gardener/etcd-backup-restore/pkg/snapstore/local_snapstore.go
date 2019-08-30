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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"syscall"
)

// LocalSnapStore is snapstore with local disk as backend
type LocalSnapStore struct {
	prefix string
}

// NewLocalSnapStore return the new local disk based snapstore
func NewLocalSnapStore(prefix string) (*LocalSnapStore, error) {
	if len(prefix) != 0 {
		err := os.MkdirAll(prefix, 0700)
		if err != nil && !os.IsExist(err) {
			return nil, err
		}
	}
	return &LocalSnapStore{
		prefix: prefix,
	}, nil
}

// Fetch should open reader for the snapshot file from store
func (s *LocalSnapStore) Fetch(snap Snapshot) (io.ReadCloser, error) {
	return os.Open(path.Join(s.prefix, snap.SnapDir, snap.SnapName))
}

// Save will write the snapshot to store
func (s *LocalSnapStore) Save(snap Snapshot, rc io.ReadCloser) error {
	defer rc.Close()
	err := os.MkdirAll(path.Join(s.prefix, snap.SnapDir), 0700)
	if err != nil && !os.IsExist(err) {
		return err
	}
	f, err := os.Create(path.Join(s.prefix, snap.SnapDir, snap.SnapName))
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, rc)
	if err != nil {
		return err
	}
	return f.Sync()
}

// List will list the snapshots from store
func (s *LocalSnapStore) List() (SnapList, error) {
	snapList := SnapList{}
	directories, err := ioutil.ReadDir(s.prefix)
	if err != nil {
		return nil, err
	}
	for _, dir := range directories {
		if dir.IsDir() {
			files, err := ioutil.ReadDir(path.Join(s.prefix, dir.Name()))
			if err != nil {
				return nil, err
			}

			for _, f := range files {
				snap, err := ParseSnapshot(path.Join(dir.Name(), f.Name()))
				if err != nil {
					// Warning
					fmt.Printf("Invalid snapshot %s found:%v\nIgnoring it.", path.Join(dir.Name(), f.Name()), err)
				} else {
					snapList = append(snapList, snap)
				}
			}
		}
	}
	sort.Sort(snapList)
	return snapList, nil
}

// Delete should delete the snapshot file from store
func (s *LocalSnapStore) Delete(snap Snapshot) error {
	if err := os.Remove(path.Join(s.prefix, snap.SnapDir, snap.SnapName)); err != nil {
		return err
	}
	err := os.Remove(path.Join(s.prefix, snap.SnapDir))
	if pathErr, ok := err.(*os.PathError); ok == true && pathErr.Err != syscall.ENOTEMPTY {
		return err
	}
	return nil
}

// Size should return size of the snapshot file from store
func (s *LocalSnapStore) Size(snap Snapshot) (int64, error) {
	fileInfo, err := os.Stat(path.Join(s.prefix, snap.SnapDir, snap.SnapName))
	if err != nil {
		return -1, err
	}
	return fileInfo.Size(), nil
}
