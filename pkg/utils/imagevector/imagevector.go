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

// ReadImageVector reads the image vector yaml file in the charts directory, unmarshals the content
import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/gardener/gardener/pkg/utils"
	"gopkg.in/yaml.v2"
)

// ReadImageVector reads the image.yaml in the chart directory, unmarshals it
// into a []*ImageSource type and returns it.
func ReadImageVector(path string) (ImageVector, error) {
	vector, err := readImageVector(path)
	if err != nil {
		return nil, err
	}

	overwritePath := os.Getenv("IMAGEVECTOR_OVERWRITE")
	if len(overwritePath) == 0 {
		return vector, nil
	}

	overwrite, err := readImageVector(overwritePath)
	if err != nil {
		return nil, err
	}

	overwrittenImages := make(map[string]*ImageSource, len(overwrite))
	for _, image := range overwrite {
		overwrittenImages[image.Name] = image
	}

	var out ImageVector
	for _, image := range vector {
		if overwritten, ok := overwrittenImages[image.Name]; ok {
			out = append(out, overwritten)
			continue
		}
		out = append(out, image)
	}
	return out, nil
}

func readImageVector(filePath string) (ImageVector, error) {
	vector := struct {
		Images ImageVector `json:"images" yaml:"images"`
	}{}

	bytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(bytes, &vector); err != nil {
		return nil, err
	}

	return vector.Images, nil
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
