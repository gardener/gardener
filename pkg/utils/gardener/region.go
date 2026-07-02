// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// GetRegionConfigMap lists region ConfigMaps using a client.Reader and returns the one matching the given cloud profile name.
func GetRegionConfigMap(ctx context.Context, reader client.Reader, namespace, cloudProfileName string) (*corev1.ConfigMap, error) {
	regionConfigList := &corev1.ConfigMapList{}
	if err := reader.List(ctx, regionConfigList, client.InNamespace(namespace), client.MatchingLabels{v1beta1constants.SchedulingPurpose: v1beta1constants.SchedulingPurposeRegionConfig}); err != nil {
		return nil, err
	}

	regionConfigMaps := make([]*corev1.ConfigMap, 0, len(regionConfigList.Items))
	for i := range regionConfigList.Items {
		regionConfigMaps = append(regionConfigMaps, &regionConfigList.Items[i])
	}

	return findRegionConfigMap(regionConfigMaps, cloudProfileName), nil
}

// GetRegionConfigMapFromLister lists region ConfigMaps from the given lister and returns the one matching the given cloud profile name.
func GetRegionConfigMapFromLister(configMapLister kubecorev1listers.ConfigMapLister, cloudProfileName string) (*corev1.ConfigMap, error) {
	regionConfigMaps, err := configMapLister.ConfigMaps(v1beta1constants.GardenNamespace).List(labels.SelectorFromSet(labels.Set{
		v1beta1constants.SchedulingPurpose: v1beta1constants.SchedulingPurposeRegionConfig,
	}))
	if err != nil {
		return nil, fmt.Errorf("could not list region config ConfigMaps: %w", err)
	}

	return findRegionConfigMap(regionConfigMaps, cloudProfileName), nil
}

func findRegionConfigMap(regionConfigMaps []*corev1.ConfigMap, cloudProfileName string) *corev1.ConfigMap {
	for _, cm := range regionConfigMaps {
		for name := range strings.SplitSeq(cm.Annotations[v1beta1constants.AnnotationSchedulingCloudProfiles], ",") {
			if name == cloudProfileName {
				return cm
			}
		}
	}
	return nil
}
