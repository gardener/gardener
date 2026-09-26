// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	cloudproviderapi "k8s.io/cloud-provider/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/nodeagent"
)

// TaintControlPlaneNodeForCloudProviderInitialization adds the taint of external cloud providers
// (node.cloudprovider.kubernetes.io/uninitialized) to the control plane node this gardenadm command runs on, unless the
// node has already been initialized by the cloud-controller-manager (i.e., its provider ID is set).
//
// Typically, kubelet adds this taint itself when registering a node with --cloud-provider=external. However, the first
// control plane node is bootstrapped with an OperatingSystemConfig that is not mutated by the provider extension's
// webhook, because the extension is not running yet at that point in time (there is no API server yet). Hence, kubelet
// registers the node without the taint, and the node controller of the cloud-controller-manager (which only
// initializes tainted nodes) would never set the node's provider ID, addresses, and topology labels. Adding the taint
// after the cloud-controller-manager has been deployed makes its node controller initialize the node as it does for
// all other nodes.
func (b *GardenadmBotanist) TaintControlPlaneNodeForCloudProviderInitialization(ctx context.Context) error {
	node, err := b.fetchControlPlaneNode(ctx)
	if err != nil {
		return err
	}

	log := b.Logger.WithValues("node", client.ObjectKeyFromObject(node))

	if node.Spec.ProviderID != "" {
		log.Info("Control plane node has already been initialized by the cloud-controller-manager, skipping taint", "providerID", node.Spec.ProviderID)
		return nil
	}

	if hasCloudProviderTaint(node) {
		return nil
	}

	log.Info("Adding cloud provider taint to control plane node so that the cloud-controller-manager initializes it", "taint", cloudproviderapi.TaintExternalCloudProvider)

	patch := client.MergeFrom(node.DeepCopy())
	node.Spec.Taints = append(node.Spec.Taints, corev1.Taint{
		Key:    cloudproviderapi.TaintExternalCloudProvider,
		Value:  "true",
		Effect: corev1.TaintEffectNoSchedule,
	})

	if err := b.SeedClientSet.Client().Patch(ctx, node, patch); err != nil {
		return fmt.Errorf("failed adding cloud provider taint to node %s: %w", node.Name, err)
	}

	return nil
}

// WaitUntilControlPlaneNodeIsInitializedByCloudProvider returns an error if the control plane node has not yet been
// initialized by the cloud-controller-manager, i.e., its provider ID is not set or it still has the cloud provider
// taint. It is expected to be retried until it succeeds.
func (b *GardenadmBotanist) WaitUntilControlPlaneNodeIsInitializedByCloudProvider(ctx context.Context) error {
	node, err := b.fetchControlPlaneNode(ctx)
	if err != nil {
		return err
	}

	if node.Spec.ProviderID == "" || hasCloudProviderTaint(node) {
		return fmt.Errorf("control plane node %s has not yet been initialized by the cloud-controller-manager (provider ID: %q, cloud provider taint present: %t)", node.Name, node.Spec.ProviderID, hasCloudProviderTaint(node))
	}

	return nil
}

func (b *GardenadmBotanist) fetchControlPlaneNode(ctx context.Context) (*corev1.Node, error) {
	node, err := nodeagent.FetchNodeByHostName(ctx, b.SeedClientSet.Client(), b.HostName)
	if err != nil {
		return nil, fmt.Errorf("failed fetching node object by hostname %q: %w", b.HostName, err)
	}
	if node == nil {
		return nil, fmt.Errorf("node object for hostname %q not found", b.HostName)
	}
	return node, nil
}

func hasCloudProviderTaint(node *corev1.Node) bool {
	return slices.ContainsFunc(node.Spec.Taints, func(taint corev1.Taint) bool {
		return taint.Key == cloudproviderapi.TaintExternalCloudProvider
	})
}
