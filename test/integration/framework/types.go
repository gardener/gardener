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
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	"k8s.io/client-go/informers"

	v1beta1informers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions/garden/v1beta1"
	v1beta1listers "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/sirupsen/logrus"
	v1k8sinformers "k8s.io/client-go/informers/core/v1"
	v1k8slisters "k8s.io/client-go/listers/core/v1"
)

// ClusterType is the ClusterType cluster that the tests will run on
type ClusterType int

const (
	// Shoot is the shoot cluster type
	Shoot ClusterType = 1 << iota

	// Seed is the seed cluster type
	Seed
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

// GardenerTest controller manager.
type GardenerTest struct {
	GardenerTestNamespace string
	K8sGardenClient       kubernetes.Interface
	K8sGardenInformers    gardeninformers.SharedInformerFactory
	KubeInformerFactory   informers.SharedInformerFactory
}

// ShootGardenerTest represents an instance of shoot tests which entails all necessary data
type ShootGardenerTest struct {
	GardenerTest

	SecretsInformer v1k8sinformers.SecretInformer
	SecretsLister   v1k8slisters.SecretLister

	SecretBindingInformer v1beta1informers.SecretBindingInformer
	SecretBindingLister   v1beta1listers.SecretBindingLister

	CloudProfileInformer v1beta1informers.CloudProfileInformer
	CloudProfileLister   v1beta1listers.CloudProfileLister

	ProjectInfomer v1beta1informers.ProjectInformer
	ProjectLister  v1beta1listers.ProjectLister

	ShootInformer v1beta1informers.ShootInformer
	ShootLister   v1beta1listers.ShootLister

	SeedInformer v1beta1informers.SeedInformer
	SeedLister   v1beta1listers.SeedLister

	Shoot  *v1beta1.Shoot
	Logger *logrus.Logger
}

// TillerOptions defines Tiller deployment Configuration Parameters
type TillerOptions struct {
	UseCanary         bool
	DeploymentName    string
	Namespace         string
	ServiceAccount    string
	EnableHostNetwork bool
	Replicas          int
	NodeSelectors     string
	Values            []string
}

// GardenerTestOperation holds all requried instances for doing a test operation
type GardenerTestOperation struct {
	ShootGardenerTest *ShootGardenerTest
	Seed              *seed.Seed
	Shoot             *shoot.Shoot
	K8sSeedClient     kubernetes.Interface
	K8sShootClient    kubernetes.Interface
}

// HelmAccess is a struct that holds the helm home
type HelmAccess struct {
	HelmPath Helm
}
