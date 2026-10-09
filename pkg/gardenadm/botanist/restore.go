// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"strings"
	"time"

	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/api/indexer"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
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
