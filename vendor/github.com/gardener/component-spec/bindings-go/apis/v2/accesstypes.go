// Copyright 2020 Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
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

package v2

import (
	"encoding/json"

	"github.com/ghodss/yaml"
)

// KnownAccessTypes contains all known access serializer
var KnownAccessTypes = KnownTypes{
	OCIRegistryType:         DefaultJSONTypedObjectCodec,
	OCIBlobType:             DefaultJSONTypedObjectCodec,
	GitHubAccessType:        DefaultJSONTypedObjectCodec,
	WebType:                 DefaultJSONTypedObjectCodec,
	LocalFilesystemBlobType: DefaultJSONTypedObjectCodec,
}

// OCIRegistryType is the access type of a oci registry.
const OCIRegistryType = "ociRegistry"

// OCIRegistryAccess describes the access for a oci registry.
type OCIRegistryAccess struct {
	ObjectType `json:",inline"`

	// ImageReference is the actual reference to the oci image repository and tag.
	// The format is expected to be "repository:tag".
	ImageReference string `json:"imageReference"`
}

// NewOCIRegistryAccess creates a new OCIRegistryAccess accessor
func NewOCIRegistryAccess(ref string) TypedObjectAccessor {
	return &OCIRegistryAccess{
		ObjectType: ObjectType{
			Type: OCIRegistryType,
		},
		ImageReference: ref,
	}
}

func (O OCIRegistryAccess) GetData() ([]byte, error) {
	return json.Marshal(O)
}

func (O *OCIRegistryAccess) SetData(bytes []byte) error {
	var newOCIImage OCIRegistryAccess
	if err := json.Unmarshal(bytes, &newOCIImage); err != nil {
		return err
	}

	O.ImageReference = newOCIImage.ImageReference
	return nil
}

// OCIBlobType is the access type of a oci blob in a manifest.
const OCIBlobType = "ociBlob"

// OCIRegistryAccess describes the access for a oci registry.
type OCIBlobAccess struct {
	ObjectType `json:",inline"`

	// Reference is the oci reference to the manifest
	Reference string `json:"ref"`

	// MediaType is the media type of the object this schema refers to.
	MediaType string `json:"mediaType,omitempty"`

	// Digest is the digest of the targeted content.
	Digest string `json:"digest"`

	// Size specifies the size in bytes of the blob.
	Size int64 `json:"size"`
}

// NewOCIBlobAccess creates a new OCIBlob accessor
func NewOCIBlobAccess(ref, mediaType, digest string, size int64) TypedObjectAccessor {
	return &OCIBlobAccess{
		ObjectType: ObjectType{
			Type: OCIBlobType,
		},
		Reference: ref,
		MediaType: mediaType,
		Digest:    digest,
		Size:      size,
	}
}

func (a OCIBlobAccess) GetData() ([]byte, error) {
	return json.Marshal(a)
}

func (a *OCIBlobAccess) SetData(bytes []byte) error {
	var newOCILayer OCIBlobAccess
	if err := json.Unmarshal(bytes, &newOCILayer); err != nil {
		return err
	}

	a.Reference = newOCILayer.Reference
	a.MediaType = newOCILayer.MediaType
	a.Digest = newOCILayer.Digest
	a.Size = newOCILayer.Size
	return nil
}

// LocalOCIBlobType is the access type of a oci blob in the current component descriptor manifest.
const LocalOCIBlobType = "localOciBlob"

// NewLocalOCIBlobAccess creates a new LocalOCIBlob accessor
func NewLocalOCIBlobAccess(digest string) TypedObjectAccessor {
	return &LocalOCIBlobAccess{
		ObjectType: ObjectType{
			Type: LocalOCIBlobType,
		},
		Digest: digest,
	}
}

// LocalOCIBlobAccess describes the access for a blob that is stored in the component descriptors oci manifest.
type LocalOCIBlobAccess struct {
	ObjectType `json:",inline"`
	// Digest is the digest of the targeted content.
	Digest string `json:"digest"`
}

func (a LocalOCIBlobAccess) GetData() ([]byte, error) {
	return json.Marshal(a)
}

func (a *LocalOCIBlobAccess) SetData(bytes []byte) error {
	var newAccess OCIBlobAccess
	if err := json.Unmarshal(bytes, &newAccess); err != nil {
		return err
	}
	a.Digest = newAccess.Digest
	return nil
}

// LocalBlobType is the access type of a oci blob in a manifest.
const LocalFilesystemBlobType = "localFilesystemBlob"

// NewLocalFilesystemBlobAccess creates a new localFilesystemBlob accessor.
func NewLocalFilesystemBlobAccess(path string, mediaType string) TypedObjectAccessor {
	return &LocalFilesystemBlobAccess{
		ObjectType: ObjectType{
			Type: LocalFilesystemBlobType,
		},
		Filename:  path,
		MediaType: mediaType,
	}
}

// LocalFilesystemBlobAccess describes the access for a blob on the filesystem.
type LocalFilesystemBlobAccess struct {
	ObjectType `json:",inline"`
	// Filename is the name of the blob in the local filesystem.
	// The blob is expected to be at <fs-root>/blobs/<name>
	Filename string `json:"filename"`
	// MediaType is the media type of the object this filename refers to.
	MediaType string `json:"mediaType,omitempty"`
}

func (a LocalFilesystemBlobAccess) GetData() ([]byte, error) {
	return json.Marshal(a)
}

func (a *LocalFilesystemBlobAccess) SetData(bytes []byte) error {
	var newAccess LocalFilesystemBlobAccess
	if err := json.Unmarshal(bytes, &newAccess); err != nil {
		return err
	}
	a.Filename = newAccess.Filename
	return nil
}

// WebType is the type of a web component
const WebType = "web"

// Web describes a web resource access that can be fetched via http GET request.
type Web struct {
	ObjectType `json:",inline"`

	// URL is the http get accessible url resource.
	URL string `json:"url"`
}

// NewWebAccess creates a new Web accessor
func NewWebAccess(url string) TypedObjectAccessor {
	return &Web{
		ObjectType: ObjectType{
			Type: OCIBlobType,
		},
		URL: url,
	}
}

func (w Web) GetData() ([]byte, error) {
	return yaml.Marshal(w)
}

func (w *Web) SetData(bytes []byte) error {
	var newWeb Web
	if err := json.Unmarshal(bytes, &newWeb); err != nil {
		return err
	}

	w.URL = newWeb.URL
	return nil
}

// WebType is the type of a web component
const GitHubAccessType = "github"

// GitHubAccess describes a github respository resource access.
type GitHubAccess struct {
	ObjectType `json:",inline"`

	// RepoURL is the url pointing to the remote repository.
	RepoURL string `json:"repoUrl"`
	// Ref describes the git reference.
	Ref string `json:"ref"`
	// Commit describes the git commit of the referenced repository.
	// +optional
	Commit string `json:"commit,omitempty"`
}

// NewGitHubAccess creates a new Web accessor
func NewGitHubAccess(url, ref, commit string) TypedObjectAccessor {
	return &GitHubAccess{
		ObjectType: ObjectType{
			Type: GitHubAccessType,
		},
		RepoURL: url,
		Ref:     ref,
		Commit:  commit,
	}
}

func (w GitHubAccess) GetData() ([]byte, error) {
	return yaml.Marshal(w)
}

func (w *GitHubAccess) SetData(bytes []byte) error {
	var newGitHubAccess GitHubAccess
	if err := json.Unmarshal(bytes, &newGitHubAccess); err != nil {
		return err
	}

	w.RepoURL = newGitHubAccess.RepoURL
	w.Ref = newGitHubAccess.Ref
	return nil
}
