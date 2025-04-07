// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/spf13/afero"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/gardener/gardener/cmd/gardener-node-agent/app/bootstrappers"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	nodeagentcomponent "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/nodeagent"
	"github.com/gardener/gardener/pkg/nodeagent"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/kubernetes/bootstraptoken"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

const kubeletTokenFilePermission = 0o600

// WriteBootstrapToken creates a bootstrap token for the kubelet and writes it to the file system.
func (b *AutonomousBotanist) WriteBootstrapToken(ctx context.Context) error {
	bootstrapTokenSecret, err := bootstraptoken.ComputeBootstrapToken(
		ctx,
		b.SeedClientSet.Client(),
		bootstraptoken.TokenID(metav1.ObjectMeta{Name: b.Shoot.GetInfo().Name, Namespace: b.Shoot.GetInfo().Namespace}),
		"kubelet",
		10*time.Minute,
	)
	if err != nil {
		return fmt.Errorf("failed computing bootstrap token: %w", err)
	}

	bootstrapToken := string(bootstrapTokenSecret.Data[bootstraptokenapi.BootstrapTokenIDKey]) +
		"." + string(bootstrapTokenSecret.Data[bootstraptokenapi.BootstrapTokenSecretKey])
	return b.FS.WriteFile(nodeagentconfigv1alpha1.BootstrapTokenFilePath, []byte(bootstrapToken), kubeletTokenFilePermission)
}

// WriteKubeletBootstrapKubeconfig writes the kubelet bootstrap kubeconfig to the file system.
func (b *AutonomousBotanist) WriteKubeletBootstrapKubeconfig(ctx context.Context) error {
	if err := b.ensureGardenerNodeAgentDirectories(); err != nil {
		return fmt.Errorf("failed ensuring gardener-node-agent directories exist: %w", err)
	}

	exists, err := b.FS.Exists(nodeagentconfigv1alpha1.BootstrapTokenFilePath)
	if err != nil {
		return fmt.Errorf("failed to check whether bootstrap token file exists (%q): %w", nodeagentconfigv1alpha1.BootstrapTokenFilePath, err)
	}
	if !exists {
		b.Logger.Info("Writing fake bootstrap token to file to make sure kubelet can start up", "path", nodeagentconfigv1alpha1.BootstrapTokenFilePath)
		// without this, kubelet will complain about an invalid kubeconfig
		if err := b.FS.WriteFile(nodeagentconfigv1alpha1.BootstrapTokenFilePath, []byte("dummy-token-to-make-kubelet-start"), kubeletTokenFilePermission); err != nil {
			return fmt.Errorf("failed to write fake bootstrap token to file (%q): %w", nodeagentconfigv1alpha1.BootstrapTokenFilePath, err)
		}
	}

	if err := b.FS.Remove(kubelet.PathKubeconfigReal); err != nil && !errors.Is(err, afero.ErrFileNotFound) {
		return fmt.Errorf("failed to remove kubelet kubeconfig file (%q): %w", kubelet.PathKubeconfigReal, err)
	}

	caBundleSecret, ok := b.SecretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !ok {
		return fmt.Errorf("failed to retrieve cluster CA secret")
	}

	kubeletBootstrapKubeconfigCreator := &bootstrappers.KubeletBootstrapKubeconfig{
		Log: b.Logger,
		FS:  b.FS,
		APIServerConfig: nodeagentconfigv1alpha1.APIServer{
			Server:   "localhost",
			CABundle: caBundleSecret.Data[secretsutils.DataKeyCertificateBundle],
		},
	}

	return kubeletBootstrapKubeconfigCreator.Start(ctx)
}

// BootstrapKubelet bootstraps the kubelet.
func (b *AutonomousBotanist) BootstrapKubelet(ctx context.Context) error {
	node, err := nodeagent.FetchNodeByHostName(ctx, b.SeedClientSet.Client(), b.HostName)
	if err != nil {
		return fmt.Errorf("failed fetching node object via hostname: %w", err)
	}
	if node != nil {
		b.Logger.Info("Found node for hostname, skipping kubelet bootstrap", "hostName", b.HostName, "nodeName", node.Name)
		return nil
	}

	b.Logger.Info("No node found for hostname, bootstrapping kubelet", "hostName", b.HostName)

	for _, obj := range nodeagentcomponent.GetCertificateSigningRequestClusterRoleBindings() {
		desired := obj.DeepCopyObject()
		if _, err := controllerutil.CreateOrUpdate(ctx, b.SeedClientSet.Client(), obj, func() error {
			obj.(*rbacv1.ClusterRoleBinding).Subjects = desired.(*rbacv1.ClusterRoleBinding).Subjects
			obj.(*rbacv1.ClusterRoleBinding).RoleRef = desired.(*rbacv1.ClusterRoleBinding).RoleRef
			return nil
		}); err != nil {
			return fmt.Errorf("failed reconciling ClusterRoleBinding %q: %w", obj.GetName(), err)
		}
	}

	if err := b.WriteBootstrapToken(ctx); err != nil {
		return fmt.Errorf("failed writing bootstrap token: %w", err)
	}

	if err := b.WriteKubeletBootstrapKubeconfig(ctx); err != nil {
		return fmt.Errorf("failed writing kubelet bootstrap kubeconfig: %w", err)
	}

	return b.DBus.Restart(ctx, nil, nil, kubelet.UnitName)
}

// ApproveKubeletServerCertificateSigningRequest approves the kubelet server certificate signing request.
func (b *AutonomousBotanist) ApproveKubeletServerCertificateSigningRequest(ctx context.Context) error {
	serverCertificateExists, err := b.FS.Exists(filepath.Join(kubelet.PathKubeletDirectory, "pki", "kubelet-server-current.pem"))
	if err != nil {
		return fmt.Errorf("failed checking if kubelet server certificate exists: %w", err)
	}
	if serverCertificateExists {
		return nil
	}

	csrList := &certificatesv1.CertificateSigningRequestList{}
	if err := b.SeedClientSet.Client().List(ctx, csrList); err != nil {
		return fmt.Errorf("failed listing certificate signing requests: %w", err)
	}

	for _, csr := range csrList.Items {
		if csr.Spec.Username == "system:node:"+b.HostName {
			if !slices.ContainsFunc(csr.Status.Conditions, func(condition certificatesv1.CertificateSigningRequestCondition) bool {
				return condition.Type == certificatesv1.CertificateApproved && condition.Status == corev1.ConditionTrue
			}) {
				b.Logger.Info("Approving kubelet server certificate signing request", "csrName", csr.Name, "hostName", b.HostName)
				csr.Status.Conditions = append(csr.Status.Conditions, certificatesv1.CertificateSigningRequestCondition{
					Type:    certificatesv1.CertificateApproved,
					Status:  corev1.ConditionTrue,
					Reason:  "RequestApproved",
					Message: "Approving kubelet server certificate signing request via gardenadm",
				})
				return b.SeedClientSet.Client().SubResource("approval").Update(ctx, &csr)
			}

			return nil
		}
	}

	return fmt.Errorf("no certificate signing request found for node %q", b.HostName)
}
