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
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/chartrenderer"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Operation contains all data required to perform an operation on a Shoot cluster.
type Operation struct {
	Logger               *logrus.Entry
	GardenerInfo         *gardenv1beta1.Gardener
	Secrets              map[string]*corev1.Secret
	CheckSums            map[string]string
	ImageVector          imagevector.ImageVector
	Garden               *garden.Garden
	Seed                 *seed.Seed
	Shoot                *shoot.Shoot
	ShootedSeed          *helper.ShootedSeed
	K8sGardenClient      kubernetes.Client
	K8sGardenInformers   gardeninformers.Interface
	K8sSeedClient        kubernetes.Client
	K8sShootClient       kubernetes.Client
	ChartGardenRenderer  chartrenderer.ChartRenderer
	ChartSeedRenderer    chartrenderer.ChartRenderer
	ChartShootRenderer   chartrenderer.ChartRenderer
	APIServerIngresses   []corev1.LoadBalancerIngress
	APIServerAddress     string
	SeedNamespaceObject  *corev1.Namespace
	BackupInfrastructure *gardenv1beta1.BackupInfrastructure
	MachineDeployments   MachineDeployments
}

// MachineDeployment holds insformation about the name, class, replicas of a MachineDeployment
// managed by the machine-controller-manager.
type MachineDeployment struct {
	Name           string
	ClassName      string
	Minimum        int
	Maximum        int
	MaxSurge       intstr.IntOrString
	MaxUnavailable intstr.IntOrString
}

// MachineDeployments is a list of machine deployments.
type MachineDeployments []MachineDeployment
