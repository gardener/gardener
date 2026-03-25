// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package indexer

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
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

// ServiceNamespaceSelectors is a constant for the namespace-selectors annotation field index on Services.
const ServiceNamespaceSelectors = "metadata.annotations.networking.resources.gardener.cloud/namespace-selectors"

// ServiceNamespaceSelectorsIndexerFunc returns "true" if the Service has the namespace-selectors annotation.
var ServiceNamespaceSelectorsIndexerFunc = func(obj client.Object) []string {
	service, ok := obj.(*corev1.Service)
	if !ok {
		return nil
	}
	if _, hasAnnotation := service.Annotations[resourcesv1alpha1.NetworkingNamespaceSelectors]; hasAnnotation {
		return []string{"true"}
	}
	return nil
}

// AddServiceNamespaceSelectors adds an index for ServiceNamespaceSelectors to the given indexer.
func AddServiceNamespaceSelectors(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &corev1.Service{}, ServiceNamespaceSelectors, ServiceNamespaceSelectorsIndexerFunc); err != nil {
		return fmt.Errorf("failed to add indexer for %s to Service Informer: %w", ServiceNamespaceSelectors, err)
	}
	return nil
}
