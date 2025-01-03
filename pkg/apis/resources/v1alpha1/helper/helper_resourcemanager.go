// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

// ClusterIDSeparator separates clusterID and ManagedResource key in an origin value.
const ClusterIDSeparator = ":"

// OriginForManagedResource encodes clusterID and ManagedResource key into an origin value.
func OriginForManagedResource(clusterID string, mr *resourcesv1alpha1.ManagedResource) string {
	if clusterID != "" {
		return clusterID + ClusterIDSeparator + mr.Namespace + string(types.Separator) + mr.Name
	}
	return mr.Namespace + string(types.Separator) + mr.Name
}

// SplitOrigin returns the clusterID and ManagedResource key encoded in an origin value.
func SplitOrigin(origin string) (string, types.NamespacedName, error) {
	var (
		parts     = strings.Split(origin, ClusterIDSeparator)
		clusterID string
		key       string
	)

	switch len(parts) {
	case 1:
		// no clusterID
		key = parts[0]
	case 2:
		// clusterID and key
		clusterID = parts[0]
		key = parts[1]
	default:
		return "", types.NamespacedName{}, fmt.Errorf("unexpected origin format: %q", origin)
	}

	parts = strings.Split(key, string(types.Separator))
	if len(parts) != 2 {
		return "", types.NamespacedName{}, fmt.Errorf("unexpected origin format: %q", origin)
	}

	return clusterID, types.NamespacedName{Namespace: parts[0], Name: parts[1]}, nil
}
