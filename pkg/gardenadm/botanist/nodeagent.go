// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"slices"
	"strings"

	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/nodeagent"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

// RequestAndStoreKubeconfig is an alias for `nodeagent.RequestAndStoreKubeconfig`. Exposed for tests.
var RequestAndStoreKubeconfig = nodeagent.RequestAndStoreKubeconfig

// WriteNodeAgentKubeconfig writes the node-agent kubeconfig to the file system.
func (b *AutonomousBotanist) WriteNodeAgentKubeconfig(ctx context.Context) error {
	exists, err := b.FS.Exists(nodeagentconfigv1alpha1.KubeconfigFilePath)
	if err != nil {
		return fmt.Errorf("failed to check whether gardener-node-agent kubeconfig file exists (%q): %w", nodeagentconfigv1alpha1.KubeconfigFilePath, err)
	}
	// Kubeconfig for gardener-node-agent already exists, no need to write it again.
	if exists {
		return nil
	}

	if err := b.ensureGardenerNodeAgentDirectories(); err != nil {
		return fmt.Errorf("failed ensuring gardener-node-agent directories exist: %w", err)
	}

	exists, err = b.FS.Exists(nodeagentconfigv1alpha1.BootstrapTokenFilePath)
	if err != nil {
		return fmt.Errorf("failed to check whether bootstrap token file exists (%q): %w", nodeagentconfigv1alpha1.BootstrapTokenFilePath, err)
	}
	// Kubeconfig for gardener-node-agent can only be written if the real bootstrap token file exists.
	if !exists {
		return nil
	}

	if err := b.FS.WriteFile(nodeagentconfigv1alpha1.MachineNameFilePath, []byte(b.HostName), 0600); err != nil {
		return fmt.Errorf("failed writing machine name file: %w", err)
	}

	caBundleSecret, ok := b.SecretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !ok {
		return fmt.Errorf("failed to retrieve cluster CA secret")
	}

	clientConnectionConfig := &componentbaseconfigv1alpha1.ClientConnectionConfiguration{}
	componentbaseconfigv1alpha1.RecommendedDefaultClientConnectionConfiguration(clientConnectionConfig)

	restConfig := &rest.Config{
		Burst: int(clientConnectionConfig.Burst),
		QPS:   clientConnectionConfig.QPS,
		ContentConfig: rest.ContentConfig{
			AcceptContentTypes: clientConnectionConfig.AcceptContentTypes,
			ContentType:        clientConnectionConfig.ContentType,
		},
		Host:            "localhost",
		TLSClientConfig: rest.TLSClientConfig{CAData: caBundleSecret.Data[secretsutils.DataKeyCertificateBundle]},
		BearerTokenFile: nodeagentconfigv1alpha1.BootstrapTokenFilePath,
	}

	return RequestAndStoreKubeconfig(ctx, b.Logger, b.FS, restConfig, b.HostName)
}

// ApproveNodeAgentCertificateSigningRequest approves the node agent certificate signing request.
func (b *AutonomousBotanist) ApproveNodeAgentCertificateSigningRequest(ctx context.Context) error {
	bootstrapToken, err := b.FS.ReadFile(nodeagentconfigv1alpha1.BootstrapTokenFilePath)
	if err != nil {
		return fmt.Errorf("failed to read bootstrap token file: %w", err)
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
				err := b.SeedClientSet.Client().SubResource("approval").Update(ctx, &csr)
				if err != nil {
					return fmt.Errorf("failed approving certificate signing request: %w", err)
				}
			}

			return nil
		}
	}

	return fmt.Errorf("no certificate signing request found for gardener-node-agent from username %q", username)
}
