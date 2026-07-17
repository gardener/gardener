// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// GetRegionConfigMaps lists the region ConfigMaps in the given namespace and returns those whose
// scheduling.gardener.cloud/cloudprofiles annotation references cloudProfileName. Callers decide how to handle the case
// of multiple matching ConfigMaps.
func GetRegionConfigMaps(ctx context.Context, reader client.Reader, namespace, cloudProfileName string) ([]*corev1.ConfigMap, error) {
	regionConfigList := &corev1.ConfigMapList{}
	if err := reader.List(ctx, regionConfigList, client.InNamespace(namespace), client.MatchingLabels{v1beta1constants.SchedulingPurpose: v1beta1constants.SchedulingPurposeRegionConfig}); err != nil {
		return nil, fmt.Errorf("failed to list region ConfigMaps: %w", err)
	}

	regionConfigMaps := make([]*corev1.ConfigMap, 0, len(regionConfigList.Items))
	for i := range regionConfigList.Items {
		regionConfigMaps = append(regionConfigMaps, &regionConfigList.Items[i])
	}

	return FindRegionConfigMaps(regionConfigMaps, cloudProfileName), nil
}

// FindRegionConfigMaps returns the ConfigMaps from the given list whose scheduling.gardener.cloud/cloudprofiles
// annotation references cloudProfileName.
func FindRegionConfigMaps(regionConfigMaps []*corev1.ConfigMap, cloudProfileName string) []*corev1.ConfigMap {
	var matches []*corev1.ConfigMap
	for _, cm := range regionConfigMaps {
		for name := range strings.SplitSeq(cm.Annotations[v1beta1constants.AnnotationSchedulingCloudProfiles], ",") {
			if strings.TrimSpace(name) == cloudProfileName {
				matches = append(matches, cm)
				break
			}
		}
	}
	return matches
}
