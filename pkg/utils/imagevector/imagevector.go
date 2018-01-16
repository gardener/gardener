// Copyright 2018 The Gardener Authors.
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
	"path/filepath"

	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	yaml "gopkg.in/yaml.v2"
)

// ReadImageVector reads the image.yaml in the chart directory, unmarshals it
// into a []*Image type and returns it.
func ReadImageVector() (ImageVector, error) {
	var (
		path   = filepath.Join(common.ChartPath, "images.yaml")
		vector = struct {
			Images ImageVector `json:"images" yaml:"images"`
		}{}
	)

	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(bytes, &vector); err != nil {
		return nil, err
	}

	return vector.Images, nil
}

// FindImage returns the image with the given <name> in the image vector. If multiple entries were
// found, the provided <k8sVersion> is compared with the constraints stated in the image definition.
// In case multiple images match the search, the first which was found is returned.
// In case no image was found, an error is returned.
func (v ImageVector) FindImage(name, k8sVersion string) (*Image, error) {
	foundImages := []*Image{}

	for _, image := range v {
		if image.Name == name {
			foundImages = append(foundImages, image)
		}
	}

	if len(foundImages) == 0 {
		return nil, fmt.Errorf("could not find image '%s' in the image vector", name)
	}

	if len(foundImages) == 1 {
		return foundImages[0], nil
	}

	for _, image := range foundImages {
		if len(image.Versions) == 0 {
			return image, nil
		}

		k8sVersionMeetsConstraint, err := utils.CheckVersionMeetsConstraint(k8sVersion, image.Versions)
		if err != nil {
			return nil, err
		}
		if k8sVersionMeetsConstraint {
			return image, nil
		}
	}

	return nil, fmt.Errorf("could not find image '%s' matching the version constraint", name)
}

// String will returns the string representation of the image.
func (i *Image) String() string {
	if len(i.Tag) == 0 {
		return i.Repository
	}
	return fmt.Sprintf("%s:%s", i.Repository, i.Tag)
}
