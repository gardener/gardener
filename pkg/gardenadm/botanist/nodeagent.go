// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/afero"
	certificatesv1 "k8s.io/api/certificates/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	"github.com/gardener/gardener/pkg/nodeagent"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/bootstraptoken"
	"github.com/gardener/gardener/pkg/utils/retry"
)

// WriteBootstrapToken creates a bootstrap token for the gardener-node-agent and kubelet, and writes it to the file
// system.
func (b *AutonomousBotanist) WriteBootstrapToken(ctx context.Context) error {
	bootstrapTokenSecret, err := bootstraptoken.ComputeBootstrapToken(
		ctx,
		b.SeedClientSet.Client(),
		bootstraptoken.TokenID(metav1.ObjectMeta{Name: b.Shoot.GetInfo().Name, Namespace: b.Shoot.GetInfo().Namespace}),
		b.HostName,
		10*time.Minute,
	)
	if err != nil {
		return fmt.Errorf("failed computing bootstrap token: %w", err)
	}

	bootstrapToken := string(bootstrapTokenSecret.Data[bootstraptokenapi.BootstrapTokenIDKey]) +
		"." + string(bootstrapTokenSecret.Data[bootstraptokenapi.BootstrapTokenSecretKey])
	return b.FS.WriteFile(nodeagentconfigv1alpha1.BootstrapTokenFilePath, []byte(bootstrapToken), kubeletTokenFilePermission)
}

func emptyTemporaryClusterAdminBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "zzz-temporary-cluster-admin-access-for-bootstrapping"}}
}

func (b *AutonomousBotanist) reconcileTemporaryClusterAdminBindingForBootstrapping(ctx context.Context) error {
	clusterRoleBinding := emptyTemporaryClusterAdminBinding()
	_, err := controllerutil.CreateOrUpdate(ctx, b.SeedClientSet.Client(), clusterRoleBinding, func() error {
		clusterRoleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		}
		clusterRoleBinding.Subjects = []rbacv1.Subject{
			// This is needed such that both gardener-node-agent and kubelet can create CertificateSigningRequests to
			// get to their client certificates.
			{
				APIGroup: rbacv1.GroupName,
				Kind:     rbacv1.GroupKind,
				Name:     bootstraptokenapi.BootstrapDefaultGroup,
			},
			// This is needed such that gardener-node-agent can act while the node-agent authorizer webhook is not yet
			// running.
			{
				APIGroup: rbacv1.GroupName,
				Kind:     rbacv1.GroupKind,
				Name:     v1beta1constants.NodeAgentsGroup,
			},
		}
		return nil
	})
	return err
}

// ActivateGardenerNodeAgent activates the gardener-node-agent. In order to do so, it first writes the machine name
// file and a real bootstrap token for communication with the kube-apiserver. Then, it creates a temporary
// ClusterRoleBinding that grants the system:bootstrappers and gardener.cloud:node-agents group all access. This is
// needed such that gardener-node-agent and kubelet can create CertificateSigningRequests, and also to let
// gardener-node-agent act while the node-agent auhorizer webhook is not yet running.
// Note that the Secret containing the OperatingSystemConfig reconciled by gardener-node-agent is already in the system.
// It got imported automatically from the fake client.
// Finally, as the kubelet has already been started earlier such that it can run the static control plane pods, this
// made it create the real kubeconfig file. We have to actively delete it in order to re-trigger its bootstrap process
// w/ the bootstrap token (otherwise, it tries to use this real kubeconfig file and fails).
func (b *AutonomousBotanist) ActivateGardenerNodeAgent(ctx context.Context) error {
	alreadyBootstrapped, err := b.FS.Exists(nodeagentconfigv1alpha1.KubeconfigFilePath)
	if err != nil {
		return fmt.Errorf("failed checking whether gardener-node-agent's kubeconfig %s exists: %w", nodeagentconfigv1alpha1.KubeconfigFilePath, err)
	}
	if alreadyBootstrapped {
		return nil
	}

	if err := b.FS.WriteFile(nodeagentconfigv1alpha1.MachineNameFilePath, []byte(b.HostName), 0600); err != nil {
		return fmt.Errorf("failed writing machine name file: %w", err)
	}

	if err := b.WriteBootstrapToken(ctx); err != nil {
		return fmt.Errorf("failed writing bootstrap token: %w", err)
	}

	if err := b.reconcileTemporaryClusterAdminBindingForBootstrapping(ctx); err != nil {
		return fmt.Errorf("failed reconciling temporary cluster-admin ClusterRoleBinding for bootstrapping gardener-node-agent and kubelet: %w", err)
	}

	if err := b.FS.Remove(kubelet.PathKubeconfigReal); err != nil && !errors.Is(err, afero.ErrFileNotFound) {
		return fmt.Errorf("failed to remove kubelet kubeconfig file (%q): %w", kubelet.PathKubeconfigReal, err)
	}

	return b.DBus.Start(ctx, nil, nil, nodeagentconfigv1alpha1.UnitName)
}

// ApproveNodeAgentCertificateSigningRequest approves the node agent certificate signing request.
func (b *AutonomousBotanist) ApproveNodeAgentCertificateSigningRequest(ctx context.Context) error {
	bootstrapToken, err := b.FS.ReadFile(nodeagentconfigv1alpha1.BootstrapTokenFilePath)
	if err != nil {
		if !errors.Is(err, afero.ErrFileNotFound) {
			return fmt.Errorf("failed to read bootstrap token file: %w", err)
		}
		// bootstrap token already deleted, nothing to do
		return nil
	}

	tokenUsername := strings.Split(string(bootstrapToken), ".")[0]
	username := "system:bootstrap:" + tokenUsername

	csrList := &certificatesv1.CertificateSigningRequestList{}
	if err := b.SeedClientSet.Client().List(ctx, csrList); err != nil {
		return fmt.Errorf("failed listing certificate signing requests: %w", err)
	}

	for _, csr := range csrList.Items {
		if csr.Spec.Username == username && csr.Spec.SignerName == certificatesv1.KubeAPIServerClientSignerName {
			x509cr, err := utils.DecodeCertificateRequest(csr.Spec.Request)
			if err != nil {
				return fmt.Errorf("failed decoding certificate signing request: %w", err)
			}

			if !strings.HasPrefix(x509cr.Subject.CommonName, v1beta1constants.NodeAgentUserNamePrefix) {
				continue
			}

			if !slices.ContainsFunc(csr.Status.Conditions, func(condition certificatesv1.CertificateSigningRequestCondition) bool {
				return condition.Type == certificatesv1.CertificateApproved && condition.Status == corev1.ConditionTrue
			}) {
				b.Logger.Info("Approving gardener-node-agent client certificate signing request", "csrName", csr.Name, "username", username)
				csr.Status.Conditions = append(csr.Status.Conditions, certificatesv1.CertificateSigningRequestCondition{
					Type:    certificatesv1.CertificateApproved,
					Status:  corev1.ConditionTrue,
					Reason:  "RequestApproved",
					Message: "Approving gardener-node-agent client certificate signing request via gardenadm",
				})
				if err := b.SeedClientSet.Client().SubResource("approval").Update(ctx, &csr); err != nil {
					return fmt.Errorf("failed approving certificate signing request: %w", err)
				}
			}

			return nil
		}
	}

	return fmt.Errorf("no certificate signing request found for gardener-node-agent from username %q", username)
}

// FinalizeGardenerNodeAgentBootstrapping deletes the temporary cluster-admin ClusterRoleBinding for
// gardener-node-agent.
func (b *AutonomousBotanist) FinalizeGardenerNodeAgentBootstrapping(ctx context.Context) error {
	return kubernetesutils.DeleteObject(ctx, b.SeedClientSet.Client(), emptyTemporaryClusterAdminBinding())
}

var (
	// WaitForNodeAgentLeaseInterval is the interval at which we check whether the gardener-node-agent lease is renewed.
	// Exposed for testing.
	WaitForNodeAgentLeaseInterval = 2 * time.Second
	// WaitForNodeAgentLeaseTimeout is the timeout after which we give up waiting for the gardener-node-agent lease to
	// be renewed. Exposed for testing.
	WaitForNodeAgentLeaseTimeout = 5 * time.Minute
)

// WaitUntilGardenerNodeAgentLeaseIsRenewed waits until the gardener-node-agent lease is renewed, which indicates that
// it is ready to be used (and that it still has the needed permissions, even though its cluster-admin binding has been
// removed).
func (b *AutonomousBotanist) WaitUntilGardenerNodeAgentLeaseIsRenewed(ctx context.Context) error {
	node, err := nodeagent.FetchNodeByHostName(ctx, b.SeedClientSet.Client(), b.HostName)
	if err != nil {
		return fmt.Errorf("failed fetching node object by hostname %q: %w", b.HostName, err)
	}
	if node == nil {
		return fmt.Errorf("node for host %q was not created yet", b.HostName)
	}
	lease := &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{Name: gardenerutils.NodeAgentLeaseName(node.GetName()), Namespace: metav1.NamespaceSystem}}

	timeoutCtx, cancel := context.WithTimeout(ctx, WaitForNodeAgentLeaseTimeout)
	defer cancel()

	now := b.Clock.Now()
	return retry.Until(timeoutCtx, WaitForNodeAgentLeaseInterval, func(ctx context.Context) (bool, error) {
		if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(lease), lease); err != nil {
			if !apierrors.IsNotFound(err) {
				return retry.MinorError(fmt.Errorf("failed fetching lease %s: %w", client.ObjectKeyFromObject(lease), err))
			}
			return retry.MinorError(fmt.Errorf("lease %s not found, gardener-node-agent might not be ready yet", client.ObjectKeyFromObject(lease)))
		}

		if lease.Spec.RenewTime != nil && lease.Spec.RenewTime.UTC().After(now) {
			return retry.Ok()
		}

		return retry.MinorError(fmt.Errorf("lease %s not renewed yet, gardener-node-agent might not be ready yet", client.ObjectKeyFromObject(lease)))
	})
}
