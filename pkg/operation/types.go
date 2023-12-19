// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package operation

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/operation/shoot"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// Builder is an object that builds Operation objects.
type Builder struct {
	configFunc                func() (*config.GardenletConfiguration, error)
	gardenFunc                func(context.Context, map[string]*corev1.Secret) (*garden.Garden, error)
	gardenerInfoFunc          func() (*gardencorev1beta1.Gardener, error)
	gardenClusterIdentityFunc func() (string, error)
	loggerFunc                func() (logr.Logger, error)
	secretsFunc               func() (map[string]*corev1.Secret, error)
	seedFunc                  func(context.Context) (*seed.Seed, error)
	shootFunc                 func(context.Context, client.Reader, *garden.Garden, *seed.Seed) (*shoot.Shoot, error)
}

// Operation contains all data required to perform an operation on a Shoot cluster.
type Operation struct {
	secrets        map[string]*corev1.Secret
	secretsMutex   sync.RWMutex
	SecretsManager secretsmanager.Interface

	Config                *config.GardenletConfiguration
	Logger                logr.Logger
	GardenerInfo          *gardencorev1beta1.Gardener
	GardenClusterIdentity string
	Garden                *garden.Garden
	Seed                  *seed.Seed
	Shoot                 *shoot.Shoot
	ManagedSeed           *seedmanagementv1alpha1.ManagedSeed
	ManagedSeedAPIServer  *v1beta1helper.ManagedSeedAPIServer
	GardenClient          client.Client
	SeedClientSet         kubernetes.Interface
	ShootClientMap        clientmap.ClientMap
	ShootClientSet        kubernetes.Interface
	APIServerAddress      string
	APIServerClusterIP    string
	SeedNamespaceObject   *corev1.Namespace

	// ControlPlaneWildcardCert is a wildcard tls certificate which is issued for the seed's ingress domain.
	ControlPlaneWildcardCert *corev1.Secret
}
