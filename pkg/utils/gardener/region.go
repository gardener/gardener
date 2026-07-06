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

// GetRegionConfigMap lists the region ConfigMaps in the given namespace and returns the one whose
// scheduling.gardener.cloud/cloudprofiles annotation references cloudProfileName. It returns nil if no matching
// ConfigMap is found, and an error if more than one ConfigMap matches.
func GetRegionConfigMap(ctx context.Context, reader client.Reader, namespace, cloudProfileName string) (*corev1.ConfigMap, error) {
	regionConfigList := &corev1.ConfigMapList{}
	if err := reader.List(ctx, regionConfigList, client.InNamespace(namespace), client.MatchingLabels{v1beta1constants.SchedulingPurpose: v1beta1constants.SchedulingPurposeRegionConfig}); err != nil {
		return nil, err
	}

	regionConfigMaps := make([]*corev1.ConfigMap, 0, len(regionConfigList.Items))
	for i := range regionConfigList.Items {
		regionConfigMaps = append(regionConfigMaps, &regionConfigList.Items[i])
	}

	return FindRegionConfigMap(regionConfigMaps, cloudProfileName)
}

// FindRegionConfigMap returns the ConfigMap from the given list whose scheduling.gardener.cloud/cloudprofiles
// annotation references cloudProfileName. It returns an error if more than one ConfigMap matches.
func FindRegionConfigMap(regionConfigMaps []*corev1.ConfigMap, cloudProfileName string) (*corev1.ConfigMap, error) {
	var match *corev1.ConfigMap
	for _, cm := range regionConfigMaps {
		for name := range strings.SplitSeq(cm.Annotations[v1beta1constants.AnnotationSchedulingCloudProfiles], ",") {
			if strings.TrimSpace(name) != cloudProfileName {
				continue
			}
			if match != nil {
				return nil, fmt.Errorf("multiple scheduler region ConfigMaps reference cloud profile %q: %s/%s and %s/%s",
					cloudProfileName, match.Namespace, match.Name, cm.Namespace, cm.Name)
			}
			match = cm
			break
		}
	}
	return match, nil
}
