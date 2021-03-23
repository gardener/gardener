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

package operation

import (
	"context"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"

	prometheusclient "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Builder is an object that builds Operation objects.
type Builder struct {
	configFunc                func() (*config.GardenletConfiguration, error)
	gardenFunc                func(context.Context, map[string]*corev1.Secret) (*garden.Garden, error)
	gardenerInfoFunc          func() (*gardencorev1beta1.Gardener, error)
	gardenClusterIdentityFunc func() (string, error)
	imageVectorFunc           func() (imagevector.ImageVector, error)
	loggerFunc                func() (*logrus.Entry, error)
	secretsFunc               func() (map[string]*corev1.Secret, error)
	seedFunc                  func(context.Context) (*seed.Seed, error)
	shootFunc                 func(context.Context, client.Client, *garden.Garden, *seed.Seed) (*shoot.Shoot, error)
	chartsRootPathFunc        func() string
}

// Operation contains all data required to perform an operation on a Shoot cluster.
type Operation struct {
	Config                    *config.GardenletConfiguration
	Logger                    *logrus.Entry
	GardenerInfo              *gardencorev1beta1.Gardener
	GardenClusterIdentity     string
	Secrets                   map[string]*corev1.Secret
	CheckSums                 map[string]string
	ImageVector               imagevector.ImageVector
	Garden                    *garden.Garden
	Seed                      *seed.Seed
	Shoot                     *shoot.Shoot
	ShootState                *gardencorev1alpha1.ShootState
	ManagedSeed               *seedmanagementv1alpha1.ManagedSeed
	ManagedSeedAPIServer      *gardencorev1beta1helper.ShootedSeedAPIServer
	ClientMap                 clientmap.ClientMap
	K8sGardenClient           kubernetes.Interface
	K8sSeedClient             kubernetes.Interface
	K8sShootClient            kubernetes.Interface
	ChartsRootPath            string
	APIServerAddress          string
	APIServerClusterIP        string
	APIServerHealthCheckToken string
	SeedNamespaceObject       *corev1.Namespace
	MonitoringClient          prometheusclient.API

	// ControlPlaneWildcardCert is a wildcard tls certificate which is issued for the seed's ingress domain.
	ControlPlaneWildcardCert *corev1.Secret
}
