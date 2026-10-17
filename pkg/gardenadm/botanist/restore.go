// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"time"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/api/indexer"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	"github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// DeletePriorNode deletes the Node object of the prior control plane Node that was lost during the disaster.
func (b *GardenadmBotanist) DeletePriorNode(ctx context.Context, realClient client.Client, priorNodeName string) error {
	if priorNodeName == "" {
		return fmt.Errorf("priorNodeName must not be empty")
	}

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: priorNodeName}}
	b.Logger.Info("Deleting prior Node", "node", client.ObjectKeyFromObject(node))

	return kubernetesutils.DeleteObject(ctx, realClient, node)
}

// ForceDeletePriorNodePods force-deletes all Pods that were running on the prior control plane Node
// that was lost during the disaster.
func (b *GardenadmBotanist) ForceDeletePriorNodePods(ctx context.Context, realClient client.Client, priorNodeName string) error {
	if priorNodeName == "" {
		return fmt.Errorf("priorNodeName must not be empty")
	}

	podList := &corev1.PodList{}
	if err := realClient.List(ctx, podList, client.MatchingFields{indexer.PodNodeName: priorNodeName}); err != nil {
		return fmt.Errorf("failed listing Pods: %w", err)
	}

	var (
		taskFns []flow.TaskFn

		forceDeleteOptions = &client.DeleteOptions{
			GracePeriodSeconds: new(int64(0)),
			PropagationPolicy:  new(metav1.DeletePropagationBackground),
		}
	)
	for _, pod := range podList.Items {
		taskFns = append(taskFns, func(ctx context.Context) error {
			b.Logger.Info("Force deleting Pod", "pod", client.ObjectKeyFromObject(&pod), "nodeName", pod.Spec.NodeName)
			if err := realClient.Delete(ctx, &pod, forceDeleteOptions); client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("failed force deleting Pod %s: %w", client.ObjectKeyFromObject(&pod), err)
			}

			return nil
		})
	}

	return flow.ParallelN(5, taskFns...)(ctx)
}

// DeleteStaleOperatingSystemConfigSecret deletes the gardener-node-agent OperatingSystemConfig Secret restored from the
// ETCD snapshot. It carries the *managed* etcd static-pod manifests under the content-independent Secret name, so
// deleting it lets the subsequent MigrateSecrets task reinstall the *bootstrap*-content Secret under the same name -
// mirroring the healthy lineage of `gardenadm init`. The later etcd-druid transition then rewrites the Secret with
// managed content.
func (b *GardenadmBotanist) DeleteStaleOperatingSystemConfigSecret(ctx context.Context, realClient client.Client) error {
	if b.operatingSystemConfigSecret == nil {
		return fmt.Errorf("operating system config secret is nil, make sure to call createOperatingSystemConfigSecretForNodeAgent() first")
	}

	return client.IgnoreNotFound(realClient.Delete(ctx, b.operatingSystemConfigSecret))
}

// FinalizeGardenerNodeAgentManagedResource removes the finalizers from the shoot-gardener-node-agent ManagedResource
// restored from the ETCD snapshot and deletes it. During the restore bootstrap no gardener-resource-manager is running
// to remove its finalizer, so a plain delete would leave the ManagedResource stuck in Terminating. It is a no-op if the
// ManagedResource is absent. The ManagedResource must be gone before DeleteStaleOperatingSystemConfigSecret deletes the
// Secret, otherwise a later gardener-resource-manager reconciliation would recreate the stale Secret.
func (b *GardenadmBotanist) FinalizeGardenerNodeAgentManagedResource(ctx context.Context, realClient client.Client) error {
	managedResource := &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      botanist.GardenerNodeAgentManagedResourceName,
			Namespace: b.Shoot.ControlPlaneNamespace,
		},
	}
	if err := realClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource); err != nil {
		return client.IgnoreNotFound(err)
	}

	if len(managedResource.Finalizers) > 0 {
		patch := client.MergeFrom(managedResource.DeepCopy())
		managedResource.SetFinalizers(nil)

		b.Logger.Info("Removing ManagedResource finalizers", "managedResource", client.ObjectKeyFromObject(managedResource))
		if err := realClient.Patch(ctx, managedResource, patch); err != nil {
			return fmt.Errorf("failed removing finalizers from ManagedResource %s: %w", client.ObjectKeyFromObject(managedResource), err)
		}
	}

	b.Logger.Info("Deleting ManagedResource", "managedResource", client.ObjectKeyFromObject(managedResource))
	if err := realClient.Delete(ctx, managedResource); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed deleting ManagedResource %s: %w", client.ObjectKeyFromObject(managedResource), err)
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	b.Logger.Info("Waiting for ManagedResource to be cleaned up", "managedResource", client.ObjectKeyFromObject(managedResource))
	if err := kubernetesutils.WaitUntilResourceDeleted(ctxWithTimeout, realClient, managedResource, 10*time.Second); err != nil {
		return fmt.Errorf("failed waiting until ManagedResource %s is cleaned up: %w", client.ObjectKeyFromObject(managedResource), err)
	}

	return nil
}

// DeleteStaleEtcdMemberLeases deletes the stale etcd member Leases restored from the ETCD snapshot and clears the
// derived Etcd.Status.Members, so etcd-druid sees an empty member set and proceeds. Otherwise its PreSync
// remove-etcd-member hook dials the recorded member endpoint, which is not up yet on a fresh restore (only the
// bootstrap etcd on 127.0.0.1 is), and the reconcile deadlocks before scaling up the managed etcd.
//
// The restored etcd-druid ValidatingWebhookConfiguration is deleted first: it guards these Lease deletions with
// FailurePolicy=Fail but is unreachable while druid is down. The init flow's druid bootstrap recreates it.
//
// Only member Leases are targeted (via druidcorev1alpha1.GetMemberLeaseNames), never the snapshot Leases that
// etcd-backup-restore depends on. No-op when nothing exists.
func (b *GardenadmBotanist) DeleteStaleEtcdMemberLeases(ctx context.Context, realClient client.Client) error {
	webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: etcd.Druid}}
	b.Logger.Info("Deleting etcd-druid ValidatingWebhookConfiguration", "validatingWebhookConfiguration", client.ObjectKeyFromObject(webhookConfig))
	if err := realClient.Delete(ctx, webhookConfig); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed deleting etcd-druid ValidatingWebhookConfiguration %s: %w", client.ObjectKeyFromObject(webhookConfig), err)
	}

	etcdList := &druidcorev1alpha1.EtcdList{}
	if err := realClient.List(ctx, etcdList, client.InNamespace(b.Shoot.ControlPlaneNamespace)); err != nil {
		return fmt.Errorf("failed listing Etcds: %w", err)
	}

	for _, etcd := range etcdList.Items {
		for _, leaseName := range druidcorev1alpha1.GetMemberLeaseNames(&etcd) {
			lease := &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{Name: leaseName, Namespace: etcd.Namespace}}
			b.Logger.Info("Deleting stale etcd member Lease", "lease", client.ObjectKeyFromObject(lease))
			if err := realClient.Delete(ctx, lease); client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("failed deleting etcd member Lease %s: %w", client.ObjectKeyFromObject(lease), err)
			}
		}

		if len(etcd.Status.Members) > 0 {
			patch := client.MergeFrom(etcd.DeepCopy())
			etcd.Status.Members = nil
			b.Logger.Info("Clearing stale Etcd status members", "etcd", client.ObjectKeyFromObject(&etcd))
			if err := realClient.Status().Patch(ctx, &etcd, patch); err != nil {
				return fmt.Errorf("failed clearing status members of Etcd %s: %w", client.ObjectKeyFromObject(&etcd), err)
			}
		}
	}

	return nil
}
