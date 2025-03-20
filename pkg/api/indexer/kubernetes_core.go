// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package indexer

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PodNodeName is a constant for the spec.nodeName field selector in pods.
const PodNodeName = "spec.nodeName"

// PodNodeNameIndexerFunc extracts the .spec.nodeName field of a Node.
var PodNodeNameIndexerFunc = func(obj client.Object) []string {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return []string{""}
	}
	return []string{pod.Spec.NodeName}
}

// AddPodNodeName adds an index for PodNodeName to the given indexer.
func AddPodNodeName(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &corev1.Pod{}, PodNodeName, PodNodeNameIndexerFunc); err != nil {
		return fmt.Errorf("failed to add indexer for %s to Pod Informer: %w", PodNodeName, err)
	}
	return nil
}
