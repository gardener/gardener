// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagent

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FetchNodeByHostName tries to fetch the node (metadata-only) object based on the hostname.
func FetchNodeByHostName(ctx context.Context, c client.Client, hostName string) (*corev1.Node, error) {
	// node name not known yet, try to fetch it via label selector based on hostname
	nodeList := &corev1.NodeList{}
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
