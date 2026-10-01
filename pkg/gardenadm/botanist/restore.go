// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/api/indexer"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component/gardener/resourcemanager"
	"github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/utils"
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
		return nil
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

// DeleteNodeAgentCertificateSigningRequests deletes the gardener-node-agent client CSRs restored from the etcd
// snapshot. A CSR is considered a gardener-node-agent CSR if it targets the kube-apiserver-client signer and its
// embedded x509 CommonName carries the gardener-node-agent user-name prefix (the same predicate the init flow's
// approval step uses). Removing them ensures the approval step only ever sees the CSR created during the current run.
func (b *GardenadmBotanist) DeleteNodeAgentCertificateSigningRequests(ctx context.Context, realClient client.Client) error {
	csrList := &certificatesv1.CertificateSigningRequestList{}
	if err := realClient.List(ctx, csrList); err != nil {
		return fmt.Errorf("failed listing CertificateSigningRequests: %w", err)
	}

	for _, csr := range csrList.Items {
		if csr.Spec.SignerName != certificatesv1.KubeAPIServerClientSignerName {
			continue
		}

		x509cr, err := utils.DecodeCertificateRequest(csr.Spec.Request)
		if err != nil {
			return fmt.Errorf("failed decoding CertificateSigningRequest %s: %w", client.ObjectKeyFromObject(&csr), err)
		}
		if !strings.HasPrefix(x509cr.Subject.CommonName, v1beta1constants.NodeAgentUserNamePrefix) {
			continue
		}

		b.Logger.Info("Deleting gardener-node-agent CertificateSigningRequest", "certificateSigningRequest", client.ObjectKeyFromObject(&csr))
		if err := realClient.Delete(ctx, &csr); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed deleting CertificateSigningRequest %s: %w", client.ObjectKeyFromObject(&csr), err)
		}
	}

	return nil
}

// DeleteGardenerResourceManagers deletes the gardener-resource-manager owner chain (Deployment, ReplicaSets and Pods)
// in both the kube-system (ForShootOrVirtualGarden) and garden (ForRuntime) namespaces. These are restored from the
// etcd snapshot and must be removed so that they neither approve the gardener-node-agent CSR nor reconcile resources
// out-of-band before the init flow has re-established its bringup invariants.
//
// The Deployment and ReplicaSets are deleted with Orphan propagation, and the Pods are then force-deleted directly.
// This tears the chain down without relying on the kube-controller-manager garbage collector (which may not be running
// mid-restore) and, by removing the ReplicaSets explicitly, prevents them from respawning Pods in between. We do not
// wait for the Pods to be gone: in a real disaster the worker kubelets may be dead, so their Pods would never finish
// terminating. Dropping the Pod API objects immediately is also what unblocks the fresh GRM Deployment's Recreate
// strategy, which otherwise refuses to create the new control-plane Pod while an old Pod still exists.
func (b *GardenadmBotanist) DeleteGardenerResourceManagers(ctx context.Context, realClient client.Client) error {
	matchingLabels := client.MatchingLabels{v1beta1constants.LabelApp: resourcemanager.LabelValue}
	orphan := client.PropagationPolicy(metav1.DeletePropagationOrphan)
	forceDelete := &client.DeleteAllOfOptions{DeleteOptions: client.DeleteOptions{GracePeriodSeconds: new(int64(0)), PropagationPolicy: new(metav1.DeletePropagationBackground)}}

	for _, namespace := range []string{metav1.NamespaceSystem, v1beta1constants.GardenNamespace} {
		inNamespace := client.InNamespace(namespace)

		b.Logger.Info("Deleting gardener-resource-manager Deployments", "namespace", namespace)
		if err := realClient.DeleteAllOf(ctx, &appsv1.Deployment{}, inNamespace, matchingLabels, orphan); err != nil {
			return fmt.Errorf("failed deleting gardener-resource-manager Deployments in namespace %s: %w", namespace, err)
		}

		b.Logger.Info("Deleting gardener-resource-manager ReplicaSets", "namespace", namespace)
		if err := realClient.DeleteAllOf(ctx, &appsv1.ReplicaSet{}, inNamespace, matchingLabels, orphan); err != nil {
			return fmt.Errorf("failed deleting gardener-resource-manager ReplicaSets in namespace %s: %w", namespace, err)
		}

		b.Logger.Info("Force deleting gardener-resource-manager Pods", "namespace", namespace)
		if err := realClient.DeleteAllOf(ctx, &corev1.Pod{}, inNamespace, matchingLabels, forceDelete); err != nil {
			return fmt.Errorf("failed force deleting gardener-resource-manager Pods in namespace %s: %w", namespace, err)
		}
	}

	return nil
}

// DeleteExtensionWorkloads deletes the extension controller workloads (Deployments, ReplicaSets and Pods) restored from
// the etcd snapshot in every extension namespace (identified by the gardener.cloud/role=extension label). These are
// orphaned mid-restore: finalizeManagedResources removes the extension ManagedResources, but with the
// gardener-resource-manager (ManagedResource controller) torn down nothing cascades that deletion to the underlying
// Deployments. Their Pods were bound to surviving worker Nodes in the prior cluster, where the restored apiserver's Node
// authorizer graph has no edge for them, so the worker kubelets can never finish terminating them.
//
// The Deployments and ReplicaSets are deleted with Orphan propagation, and the Pods are then force-deleted directly.
// This tears the chain down without relying on the kube-controller-manager garbage collector (which may not be running
// mid-restore) and, by removing the ReplicaSets explicitly, prevents them from respawning Pods in between. We do not
// wait for the Pods to be gone: dropping the Pod API objects immediately is what unblocks the fresh extension
// Deployments' Recreate strategy, which otherwise refuses to create the new control-plane Pod while an old Pod exists.
func (b *GardenadmBotanist) DeleteExtensionWorkloads(ctx context.Context, realClient client.Client) error {
	namespaceList := &corev1.NamespaceList{}
	if err := realClient.List(ctx, namespaceList, client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleExtension}); err != nil {
		return fmt.Errorf("failed listing extension namespaces: %w", err)
	}

	orphan := client.PropagationPolicy(metav1.DeletePropagationOrphan)
	forceDelete := &client.DeleteAllOfOptions{DeleteOptions: client.DeleteOptions{GracePeriodSeconds: new(int64(0)), PropagationPolicy: new(metav1.DeletePropagationBackground)}}

	for _, namespace := range namespaceList.Items {
		inNamespace := client.InNamespace(namespace.Name)

		b.Logger.Info("Deleting extension Deployments", "namespace", namespace.Name)
		if err := realClient.DeleteAllOf(ctx, &appsv1.Deployment{}, inNamespace, orphan); err != nil {
			return fmt.Errorf("failed deleting extension Deployments in namespace %s: %w", namespace.Name, err)
		}

		b.Logger.Info("Deleting extension ReplicaSets", "namespace", namespace.Name)
		if err := realClient.DeleteAllOf(ctx, &appsv1.ReplicaSet{}, inNamespace, orphan); err != nil {
			return fmt.Errorf("failed deleting extension ReplicaSets in namespace %s: %w", namespace.Name, err)
		}

		b.Logger.Info("Force deleting extension Pods", "namespace", namespace.Name)
		if err := realClient.DeleteAllOf(ctx, &corev1.Pod{}, inNamespace, forceDelete); err != nil {
			return fmt.Errorf("failed force deleting extension Pods in namespace %s: %w", namespace.Name, err)
		}
	}

	return nil
}
