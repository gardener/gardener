// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package framework

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/gardener/gardener/pkg/utils"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	decoder = serializer.NewCodecFactory(kubernetes.GardenScheme).UniversalDecoder()

	// TemplateDir relative path for helm templates dir
	TemplateDir = filepath.Join("..", "..", "resources", "templates")
)

const (

	// IntegrationTestPrefix is the default prefix that will be used for test shoots if none other is specified
	IntegrationTestPrefix = "itest-"
)

// CreateShootTestArtifacts creates the necessary artifacts for a shoot tests including a random integration test name and
// a shoot object which is read from the resources directory
func CreateShootTestArtifacts(shootTestYamlPath, prefix string) (string, *v1beta1.Shoot, error) {
	integrationTestName, err := generateRandomShootName(prefix)
	if err != nil {
		return "", nil, err
	}

	shoot := &v1beta1.Shoot{}
	if err := ReadObject(shootTestYamlPath, shoot); err != nil {
		return "", nil, err
	}

	shoot.Name = integrationTestName
	shoot.Spec.DNS = v1beta1.DNS{}
	return integrationTestName, shoot, nil
}

func generateRandomShootName(prefix string) (string, error) {
	randomString, err := utils.GenerateRandomString(10)
	if err != nil {
		return "", err
	}

	if len(prefix) > 0 {
		return prefix + strings.ToLower(randomString), nil
	}

	return IntegrationTestPrefix + strings.ToLower(randomString), nil
}

// CreatePlantTestArtifacts creates a plant object which is read from the resources directory
func CreatePlantTestArtifacts(plantTestYamlPath string) (*gardencorev1alpha1.Plant, error) {

	plant := &gardencorev1alpha1.Plant{}
	if err := ReadObject(plantTestYamlPath, plant); err != nil {
		return nil, err
	}

	return plant, nil
}

// ReadObject loads the contents of file and decodes it as an object.
func ReadObject(file string, into runtime.Object) error {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	_, _, err = decoder.Decode(data, nil, into)
	return err
}
