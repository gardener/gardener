// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package nodeagent

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FetchNodeByHostName tries to fetch the node (metadata-only) object based on the hostname.
func FetchNodeByHostName(ctx context.Context, c client.Client, hostName string) (*metav1.PartialObjectMetadata, error) {
	// node name not known yet, try to fetch it via label selector based on hostname
	nodeList := &metav1.PartialObjectMetadataList{}
	nodeList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("NodeList"))

	if err := c.List(ctx, nodeList, client.MatchingLabels{corev1.LabelHostname: hostName}); err != nil {
		return nil, fmt.Errorf("unable to list nodes with label selector %s=%s: %w", corev1.LabelHostname, hostName, err)
	}

	switch len(nodeList.Items) {
	case 0:
		return nil, nil
	case 1:
		return &nodeList.Items[0], nil
	default:
		return nil, fmt.Errorf("found more than one node with label %s=%s", corev1.LabelHostname, hostName)
	}
}
