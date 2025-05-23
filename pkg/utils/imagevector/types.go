// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package imagevector

// ImageSource contains the repository and the tag of a Docker container image. If the respective
// image is only valid for a specific Kubernetes runtime version, then it must also contain the
// 'runtimeVersion' field describing for which versions it can be used. Similarly, if it is only
// valid for a specific Kubernetes version to operate on, then it must also contain the 'targetVersion'
// field describing for which versions it can be used. Examples of these are CSI controllers that run
// in the seed cluster and act on the shoot cluster. Different versions might be used depending on the
// seed and the shoot version.
type ImageSource struct {
	Name           string   `json:"name" yaml:"name"`
	RuntimeVersion *string  `json:"runtimeVersion,omitempty" yaml:"runtimeVersion,omitempty"`
	TargetVersion  *string  `json:"targetVersion,omitempty" yaml:"targetVersion,omitempty"`
	Architectures  []string `json:"architectures,omitempty" yaml:"architectures,omitempty"`

	// Either Ref or Repository must be provided. If Repository is used, Tag can either be a digest only
	// (e.g., `sha256:073...`), or tag+digest combined (e.g., `v1.2.3@sha256:073...`).
	Ref        *string `json:"ref,omitempty" yaml:"ref,omitempty"`
	Repository *string `json:"repository,omitempty" yaml:"repository,omitempty"`
	Tag        *string `json:"tag,omitempty" yaml:"tag,omitempty"`

	// Version is a human-readable version of the image (helpful in case the ref/tag does not specify it because only a
	// digest is used).
	Version *string `json:"version,omitempty" yaml:"version,omitempty"`
}

// Image is a concrete, pullable image with a nonempty tag.
type Image struct {
	Name       string
	Ref        *string
	Repository *string
	Tag        *string
	Version    *string
}

// ImageVector is a list of image sources.
type ImageVector []*ImageSource

// ComponentImageVector contains an image vector overwrite for a component deployed by Gardener.
type ComponentImageVector struct {
	Name                 string `json:"name" yaml:"name"`
	ImageVectorOverwrite string `json:"imageVectorOverwrite" yaml:"imageVectorOverwrite"`
}

// ComponentImageVectors maps a component with a given name (key) to the image vector overwrite content (value).
type ComponentImageVectors map[string]string

// FindOptions are options that can be supplied during either `FindImage` or `FindImages`.
type FindOptions struct {
	RuntimeVersion *string
	TargetVersion  *string
	Architecture   *string
}

// FindOptionFunc is a function that mutates FindOptions.
type FindOptionFunc func(*FindOptions)
