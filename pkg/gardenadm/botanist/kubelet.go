// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/afero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"

	"github.com/gardener/gardener/cmd/gardener-node-agent/app/bootstrappers"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/kubernetes/bootstraptoken"
)

const kubeletTokenFilePermission = 0o600

// CreateBootstrapToken creates a bootstrap token for the kubelet and writes it to the file system.
func (b *AutonomousBotanist) CreateBootstrapToken(ctx context.Context) error {
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
func (b *AutonomousBotanist) WriteKubeletBootstrapKubeconfig(ctx context.Context, server string, caBundle []byte) error {
	if err := b.FS.MkdirAll(nodeagentconfigv1alpha1.TempDir, os.ModeDir); err != nil {
		return fmt.Errorf("failed to create temporary directory (%q): %w", nodeagentconfigv1alpha1.TempDir, err)
	}
	if err := b.FS.MkdirAll(nodeagentconfigv1alpha1.CredentialsDir, os.ModeDir); err != nil {
		return fmt.Errorf("failed to create credentials directory (%q): %w", nodeagentconfigv1alpha1.CredentialsDir, err)
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

	kubeletBootstrapKubeconfigCreator := &bootstrappers.KubeletBootstrapKubeconfig{
		Log: b.Logger,
		FS:  b.FS,
		APIServerConfig: nodeagentconfigv1alpha1.APIServer{
			Server:   server,
			CABundle: caBundle,
		},
	}

	return kubeletBootstrapKubeconfigCreator.Start(ctx)
}
