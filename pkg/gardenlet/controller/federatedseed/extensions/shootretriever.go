// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
