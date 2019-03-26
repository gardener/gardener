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

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gardener/gardener/pkg/utils"
	"gopkg.in/yaml.v2"
)

// OverrideEnv is the name of the image vector override environment variable.
const OverrideEnv = "IMAGEVECTOR_OVERWRITE"

// Read reads an ImageVector from the given io.Reader.
func Read(r io.Reader) (ImageVector, error) {
	vector := struct {
		Images ImageVector `json:"images" yaml:"images"`
	}{}

	if err := yaml.NewDecoder(r).Decode(&vector); err != nil {
		return nil, err
	}
	return vector.Images, nil
}

// ReadFile reads an ImageVector from the file with the given name.
func ReadFile(name string) (ImageVector, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return Read(file)
}

// mergeImageSources merges the two given ImageSources.
//
// If the tag of the override is non-empty, it immediately returns the override.
// Otherwise, the override is copied, gets the tag of the old source and is returned.
func mergeImageSources(old, override *ImageSource) *ImageSource {
	if len(override.Tag) != 0 {
		return override
	}
	merged := *override
	merged.Tag = old.Tag
	return &merged
}

// Merge merges the given ImageVectors into one.
//
// Images of ImageVectors that are later in the given sequence with the same name override
// previous images.
func Merge(vectors ...ImageVector) ImageVector {
	var (
		out              ImageVector
		imageNameToIndex = make(map[string]int)
	)

	for _, vector := range vectors {
		for _, image := range vector {
			if idx, ok := imageNameToIndex[image.Name]; ok {
				out[idx] = mergeImageSources(out[idx], image)
				continue
			}

			imageNameToIndex[image.Name] = len(out)
			out = append(out, image)
		}
	}
	return out
}

// WithEnvOverride checks if an environment variable with the key IMAGEVECTOR_OVERWRITE is set.
// If yes, it reads the ImageVector at the value of the variable and merges it with the given one.
// Otherwise, it returns the unmodified ImageVector.
func WithEnvOverride(vector ImageVector) (ImageVector, error) {
	overwritePath := os.Getenv(OverrideEnv)
	if len(overwritePath) == 0 {
		return vector, nil
	}

	override, err := ReadFile(overwritePath)
	if err != nil {
		return nil, err
	}

	return Merge(vector, override), nil
}

func checkConstraintMatchesK8sVersion(constraint, k8sVersion string) (bool, error) {
	if constraint == "" {
		return true, nil
	}
	return utils.CheckVersionMeetsConstraint(k8sVersion, constraint)
}

// FindImage returns an image with the given <name> from the sources in the image vector.
// The <k8sVersion> specifies the kubernetes version the image will be running on.
// The <targetK8sVersion> specifies the kubernetes version the image shall target.
// If multiple entries were found, the provided <k8sVersion> is compared with the constraints
// stated in the image definition.
// In case multiple images match the search, the first which was found is returned.
// In case no image was found, an error is returned.
func (v ImageVector) FindImage(name, k8sVersionRuntime, k8sVersionTarget string) (*Image, error) {
	for _, source := range v {
		if source.Name == name {
			matches, err := checkConstraintMatchesK8sVersion(source.Versions, k8sVersionRuntime)
			if err != nil {
				return nil, err
			}

			if matches {
				return source.ToImage(k8sVersionTarget), nil
			}
		}
	}

	return nil, fmt.Errorf("could not find image %q for Kubernetes runtime version %q in the image vector", name, k8sVersionRuntime)
}

// FindImages returns an image map with the given <names> from the sources in the image vector.
// The <k8sVersion> specifies the kubernetes version the image will be running on.
// The <targetK8sVersion> specifies the kubernetes version the image shall target.
// If multiple entries were found, the provided <k8sVersion> is compared with the constraints
// stated in the image definition.
// In case multiple images match the search, the first which was found is returned.
// In case no image was found, an error is returned.
func (v ImageVector) FindImages(names []string, k8sVersionRuntime, k8sVersionTarget string) (map[string]interface{}, error) {
	images := map[string]interface{}{}
	for _, imageName := range names {
		image, err := v.FindImage(imageName, k8sVersionRuntime, k8sVersionTarget)
		if err != nil {
			return nil, err
		}
		images[imageName] = image.String()
	}
	return images, nil
}

// InjectImages injects images from a given image vector into the provided <values> map.
func (v ImageVector) InjectImages(values map[string]interface{}, k8sVersionRuntime, k8sVersionTarget string, images ...string) (map[string]interface{}, error) {
	var (
		copy = make(map[string]interface{})
		i    = make(map[string]interface{})
	)

	for k, v := range values {
		copy[k] = v
	}

	for _, imageName := range images {
		image, err := v.FindImage(imageName, k8sVersionRuntime, k8sVersionTarget)
		if err != nil {
			return nil, err
		}
		i[imageName] = image.String()
	}

	copy["images"] = i
	return copy, nil
}

// ToImage applies the given <targetK8sVersion> to the source to produce an output image.
// If the tag of an image source is empty, it will use the given <k8sVersion> as tag.
func (i *ImageSource) ToImage(targetK8sVersion string) *Image {
	tag := i.Tag
	if tag == "" {
		tag = fmt.Sprintf("v%s", strings.TrimLeft(targetK8sVersion, "v"))
	}

	return &Image{
		Name:       i.Name,
		Repository: i.Repository,
		Tag:        tag,
	}
}

// String will returns the string representation of the image.
func (i *Image) String() string {
	if len(i.Tag) == 0 {
		return i.Repository
	}
	return fmt.Sprintf("%s:%s", i.Repository, i.Tag)
}
