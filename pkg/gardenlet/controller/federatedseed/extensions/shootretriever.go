// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package extensions

import (
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

// ShootRetriever decodes raw Shoots into objects
type ShootRetriever struct {
	runtime.Decoder
}

// NewShootRetriever returns a new shoot retriever.
func NewShootRetriever() *ShootRetriever {
	return &ShootRetriever{serializer.NewCodecFactory(kubernetes.GardenScheme).UniversalDecoder()}
}

// FromCluster retrieves the shoot resource from the Cluster resource
func (s *ShootRetriever) FromCluster(cluster *extensionsv1alpha1.Cluster) (*gardencorev1beta1.Shoot, error) {
	shoot := &gardencorev1beta1.Shoot{}
	if cluster.Spec.Shoot.Raw == nil {
		return nil, fmt.Errorf("cluster resource %s doesn't contain shoot resource in raw format", cluster.Name)
	}
	if _, _, err := s.Decode(cluster.Spec.Shoot.Raw, nil, shoot); err != nil {
		return nil, err
	}
	return shoot, nil
}
