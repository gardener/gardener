// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
