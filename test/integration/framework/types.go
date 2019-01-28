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
	"fmt"
	"path/filepath"

	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/sirupsen/logrus"
)

// Helm is the home for the HELM repo
type Helm string

// Path returns Helm path with elements appended.
func (h Helm) Path(elem ...string) string {
	p := []string{h.String()}
	p = append(p, elem...)
	return filepath.Join(p...)
}

// Path returns the home for the helm repo with.
func (h Helm) String(elem ...string) string {
	return string(h)
}

// Repository returns the path to the local repository.
func (h Helm) Repository() string {
	return h.Path("repository")
}

// RepositoryFile returns the path to the repositories.yaml file.
func (h Helm) RepositoryFile() string {
	return h.Path("repository", "repositories.yaml")
}

// CacheIndex returns the path to an index for the given named repository.
func (h Helm) CacheIndex(name string) string {
	target := fmt.Sprintf("%s-index.yaml", name)
	return h.Path("repository", "cache", target)
}

// ShootGardenerTest represents an instance of shoot tests which entails all necessary data
type ShootGardenerTest struct {
	GardenClient kubernetes.Interface

	Shoot  *v1beta1.Shoot
	Logger *logrus.Logger
}

// GardenerTestOperation holds all requried instances for doing a test operation
type GardenerTestOperation struct {
	Logger       logrus.FieldLogger
	GardenClient kubernetes.Interface
	SeedClient   kubernetes.Interface
	ShootClient  kubernetes.Interface

	Seed    *v1beta1.Seed
	Shoot   *v1beta1.Shoot
	Project *v1beta1.Project
}

// HelmAccess is a struct that holds the helm home
type HelmAccess struct {
	HelmPath Helm
}
