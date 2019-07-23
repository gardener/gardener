// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package imagevector

// ImageSource contains the repository and the tag of a Docker container image. If the respective
// image is only valid for a specific Kubernetes runtime version, then it must also contain the
// 'runtimeVersion' field describing for which versions it can be used. Similarly, if it is only
// valid for a specific Kubernetes version to operate on, then it must also contain the 'targetVersion'
// field describing for which versions it can be used. Examples of these are CSI controllers that run
// in the seed cluster and act on the shoot cluster. Different versions might be used depending on the
// seed and the shoot version.
type ImageSource struct {
	Name           string  `json:"name" yaml:"name"`
	RuntimeVersion *string `json:"runtimeVersion,omitempty" yaml:"runtimeVersion,omitempty"`
	TargetVersion  *string `json:"targetVersion,omitempty" yaml:"targetVersion,omitempty"`

	Repository string  `json:"repository" yaml:"repository"`
	Tag        *string `json:"tag,omitempty" yaml:"tag,omitempty"`
}

// Image is a concrete, pullable image with a nonempty tag.
type Image struct {
	Name       string
	Repository string
	Tag        *string
}

// ImageVector is a list of image sources.
type ImageVector []*ImageSource

// FindOptions are options that can be supplied during either `FindImage` or `FindImages`.
type FindOptions struct {
	RuntimeVersion *string
	TargetVersion  *string
}

// FindOptionFunc is a function that mutates FindOptions.
type FindOptionFunc func(*FindOptions)
