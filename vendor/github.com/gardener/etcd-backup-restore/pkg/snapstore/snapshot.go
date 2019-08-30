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
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// NewSnapshot returns the snapshot object.
func NewSnapshot(kind string, startRevision, lastRevision int64) *Snapshot {
	snap := &Snapshot{
		Kind:          kind,
		StartRevision: startRevision,
		LastRevision:  lastRevision,
		CreatedOn:     time.Now().UTC(),
	}
	snap.GenerateSnapshotDirectory()
	snap.GenerateSnapshotName()
	return snap
}

// GenerateSnapshotName prepares the snapshot name from metadata
func (s *Snapshot) GenerateSnapshotName() {
	s.SnapName = fmt.Sprintf("%s-%08d-%08d-%d", s.Kind, s.StartRevision, s.LastRevision, s.CreatedOn.Unix())
}

// GenerateSnapshotDirectory prepares the snapshot directory name from metadata
func (s *Snapshot) GenerateSnapshotDirectory() {
	s.SnapDir = fmt.Sprintf("Backup-%d", s.CreatedOn.Unix())
}

// GetSnapshotDirectoryCreationTimeInUnix returns the creation time for snapshot directory.
func (s *Snapshot) GetSnapshotDirectoryCreationTimeInUnix() (int64, error) {
	tok := strings.TrimPrefix(s.SnapDir, "Backup-")
	return strconv.ParseInt(tok, 10, 64)
}

// ParseSnapshot parse <snapPath> to create snapshot structure
func ParseSnapshot(snapPath string) (*Snapshot, error) {
	var err error
	s := &Snapshot{}
	tok := strings.Split(snapPath, "/")
	logrus.Debugf("no of tokens:=%d", len(tok))
	if len(tok) <= 1 || len(tok) > 3 {
		return nil, fmt.Errorf("invalid snapshot name: %s", snapPath)
	}
	snapName := tok[1]
	snapDir := tok[0]
	tokens := strings.Split(snapName, "-")
	if len(tokens) != 4 {
		return nil, fmt.Errorf("invalid snapshot name: %s", snapName)
	}
	//parse kind
	switch tokens[0] {
	case SnapshotKindFull:
		s.Kind = SnapshotKindFull
	case SnapshotKindDelta:
		s.Kind = SnapshotKindDelta
	default:
		return nil, fmt.Errorf("unknown snapshot kind: %s", tokens[0])
	}

	if len(tok) == 3 {
		s.IsChunk = true
		snapName = path.Join(snapName, tok[2])
	}

	//parse start revision
	s.StartRevision, err = strconv.ParseInt(tokens[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid start revision: %s", tokens[1])
	}
	//parse last revision
	s.LastRevision, err = strconv.ParseInt(tokens[2], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid last revision: %s", tokens[2])
	}

	if s.StartRevision > s.LastRevision {
		return nil, fmt.Errorf("last revision (%s) should be at least start revision(%s) ", tokens[2], tokens[1])
	}
	//parse creation time
	unixTime, err := strconv.ParseInt(tokens[3], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid creation time: %s", tokens[3])
	}
	s.CreatedOn = time.Unix(unixTime, 0).UTC()
	s.SnapName = snapName
	s.SnapDir = snapDir
	return s, nil
}

// SnapList override sorting related function
func (s SnapList) Len() int      { return len(s) }
func (s SnapList) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s SnapList) Less(i, j int) bool {
	// Ignoring errors here, as we assume at this stage the error won't happen.
	iCreationTime, err := s[i].GetSnapshotDirectoryCreationTimeInUnix()
	if err != nil {
		logrus.Errorf("Failed to get snapshot directory creation time for snapshot: %s, with error: %v", path.Join(s[i].SnapDir, s[i].SnapName), err)
	}
	jCreationTime, err := s[j].GetSnapshotDirectoryCreationTimeInUnix()
	if err != nil {
		logrus.Errorf("Failed to get snapshot directory creation time for snapshot: %s, with error: %v", path.Join(s[j].SnapDir, s[j].SnapName), err)
	}
	if iCreationTime < jCreationTime {
		return true
	}
	if iCreationTime > jCreationTime {
		return false
	}
	if s[i].CreatedOn.Unix() == s[j].CreatedOn.Unix() {
		if !s[i].IsChunk && s[j].IsChunk {
			return true
		}
		if s[i].IsChunk && !s[j].IsChunk {
			return false
		}
		if !s[i].IsChunk && !s[j].IsChunk {
			return (s[i].StartRevision < s[j].StartRevision)
		}
		// If both are chunks, ordering doesn't matter.
		return true
	}
	return (s[i].CreatedOn.Unix() < s[j].CreatedOn.Unix())
}
