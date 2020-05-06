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

package controller

import (
	"context"

	gardencoreinstall "github.com/gardener/gardener/pkg/apis/core/install"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	"github.com/gardener/gardener/pkg/apis/core"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var gardenscheme *runtime.Scheme

func init() {
	gardenscheme = runtime.NewScheme()
	gardencoreinstall.Install(gardenscheme)
}

// Cluster contains the decoded resources of Gardener's extension Cluster resource.
type Cluster struct {
	ObjectMeta   metav1.ObjectMeta
	CloudProfile *gardencorev1beta1.CloudProfile
	Seed         *gardencorev1beta1.Seed
	Shoot        *gardencorev1beta1.Shoot
}

// GetCluster tries to read Gardener's Cluster extension resource in the given namespace.
func GetCluster(ctx context.Context, c client.Client, namespace string) (*Cluster, error) {
	cluster := &extensionsv1alpha1.Cluster{}
	if err := c.Get(ctx, kutil.Key(namespace), cluster); err != nil {
		return nil, err
	}

	decoder, err := NewGardenDecoder()
	if err != nil {
		return nil, err
	}

	cloudProfile, err := CloudProfileFromCluster(decoder, cluster)
	if err != nil {
		return nil, err
	}
	seed, err := SeedFromCluster(decoder, cluster)
	if err != nil {
		return nil, err
	}
	shoot, err := ShootFromCluster(decoder, cluster)
	if err != nil {
		return nil, err
	}

	return &Cluster{cluster.ObjectMeta, cloudProfile, seed, shoot}, nil
}

// CloudProfileFromCluster returns the CloudProfile resource inside the Cluster resource.
func CloudProfileFromCluster(decoder runtime.Decoder, cluster *extensionsv1alpha1.Cluster) (*gardencorev1beta1.CloudProfile, error) {
	var (
		cloudProfileInternal = &core.CloudProfile{}
		cloudProfile         = &gardencorev1beta1.CloudProfile{}
	)

	if cluster.Spec.CloudProfile.Raw == nil {
		return nil, nil
	}
	if _, _, err := decoder.Decode(cluster.Spec.CloudProfile.Raw, nil, cloudProfileInternal); err != nil {
		return nil, err
	}
	if err := gardenscheme.Convert(cloudProfileInternal, cloudProfile, nil); err != nil {
		return nil, err
	}

	return cloudProfile, nil
}

// SeedFromCluster returns the Seed resource inside the Cluster resource.
func SeedFromCluster(decoder runtime.Decoder, cluster *extensionsv1alpha1.Cluster) (*gardencorev1beta1.Seed, error) {
	var (
		seedInternal = &core.Seed{}
		seed         = &gardencorev1beta1.Seed{}
	)

	if cluster.Spec.Seed.Raw == nil {
		return nil, nil
	}
	if _, _, err := decoder.Decode(cluster.Spec.Seed.Raw, nil, seedInternal); err != nil {
		return nil, err
	}
	if err := gardenscheme.Convert(seedInternal, seed, nil); err != nil {
		return nil, err
	}

	return seed, nil
}

// ShootFromCluster returns the Shoot resource inside the Cluster resource.
func ShootFromCluster(decoder runtime.Decoder, cluster *extensionsv1alpha1.Cluster) (*gardencorev1beta1.Shoot, error) {
	var (
		shootInternal = &core.Shoot{}
		shoot         = &gardencorev1beta1.Shoot{}
	)

	if cluster.Spec.Shoot.Raw == nil {
		return nil, nil
	}
	if _, _, err := decoder.Decode(cluster.Spec.Shoot.Raw, nil, shootInternal); err != nil {
		return nil, err
	}
	if err := gardenscheme.Convert(shootInternal, shoot, nil); err != nil {
		return nil, err
	}

	return shoot, nil
}

// GetShoot tries to read Gardener's Cluster extension resource in the given namespace and return the embedded Shoot resource.
func GetShoot(ctx context.Context, c client.Client, namespace string) (*gardencorev1beta1.Shoot, error) {
	cluster := &extensionsv1alpha1.Cluster{}
	if err := c.Get(ctx, kutil.Key(namespace), cluster); err != nil {
		return nil, err
	}

	decoder, err := NewGardenDecoder()
	if err != nil {
		return nil, err
	}

	return ShootFromCluster(decoder, cluster)
}

// NewGardenDecoder returns a new Garden API decoder.
func NewGardenDecoder() (runtime.Decoder, error) {
	return serializer.NewCodecFactory(gardenscheme).UniversalDecoder(), nil
}
