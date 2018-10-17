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
	"path/filepath"

	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	yaml "gopkg.in/yaml.v2"
	"strings"
)

// ReadImageVector reads the image.yaml in the chart directory, unmarshals it
// into a []*ImageSource type and returns it.
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
