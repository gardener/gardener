// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package core

// ImageVector is a list of images.
type ImageVector []ImageSource

// ImageSource specified the name, the repository, the tag, and version constraints of a container image.
type ImageSource struct {
	// Name is the image name, e.g. "gardenlet"
	Name string
	// SourceRepository is the image source repository, e.g. "github.com/gardener/gardener".
	SourceRepository *string
	// Repository is the image repository, e.g. "eu.gcr.io/gardener-project/gardener/gardenlet".
	Repository string
	// Tag is the image tag, e.g. "v1.0". Defaults to "latest".
	Tag *string
	// RuntimeVersion is the Kubernetes version on which the image can be deployed.
	// It should be specified if the image can only be deployed on specific Kubernetes version(s).
	// For supported syntax, see https://github.com/Masterminds/semver#hyphen-range-comparisons
	RuntimeVersion *string
	// TargetVersion is the Kubernetes version that the image can target (operate on).
	// It should be specified if the image can target only specific Kubernetes version(s).
	// For supported syntax, see https://github.com/Masterminds/semver#hyphen-range-comparisons
	TargetVersion *string
}

// ComponentImageVectors is a list of components and their images.
type ComponentImageVectors []ComponentImageVector

// ComponentImageVector specifies the name and the list of images for a component.
type ComponentImageVector struct {
	// Name is the component name, e.g. "etcd-druid"
	Name string
	// ImageVector is the list of images for the component.
	ImageVector ImageVector
}
