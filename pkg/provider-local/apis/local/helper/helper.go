// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package helper

import (
	"fmt"

	api "github.com/gardener/gardener/pkg/provider-local/apis/local"
)

// FindMachineImage takes a list of machine images and tries to find the first entry
// whose name, version, and zone matches with the given name, version, and region. If no such entry is
// found then an error will be returned.
func FindMachineImage(machineImages []api.MachineImage, name, version string) (*api.MachineImage, error) {
	for _, machineImage := range machineImages {
		if machineImage.Name == name && machineImage.Version == version {
			return &machineImage, nil
		}
	}
	return nil, fmt.Errorf("no machine image with name %q, version %q found", name, version)
}

// FindImageFromCloudProfile takes a list of machine images, and the desired image name, version.. It tries to find the
// image with the given name and version in the desired region. If it cannot be found then an error is returned.
func FindImageFromCloudProfile(cloudProfileConfig *api.CloudProfileConfig, imageName, imageVersion string) (string, error) {
	if cloudProfileConfig != nil {
		for _, machineImage := range cloudProfileConfig.MachineImages {
			if machineImage.Name != imageName {
				continue
			}

			for _, version := range machineImage.Versions {
				if imageVersion != version.Version {
					continue
				}

				return version.Image, nil
			}
		}
	}

	return "", fmt.Errorf("could not find an image for name %q in version %q", imageName, imageVersion)
}
