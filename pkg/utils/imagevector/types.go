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
// image is only valid for a specific Kubernetes version, then it must also contain the 'versions'
// field describing for which versions it can be used.
type ImageSource struct {
	Name       string `json:"name" yaml:"name"`
	Repository string `json:"repository" yaml:"repository"`
	Tag        string `json:"tag" yaml:"tag"`
	Versions   string `json:"versions" yaml:"versions"`
}

// Image is a concrete, pullable image with a nonempty tag.
type Image struct {
	Name       string
	Repository string
	Tag        string
}

// ImageVector is a list of image sources.
type ImageVector []*ImageSource
