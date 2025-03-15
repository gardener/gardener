// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package init

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
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/kubernetes/bootstraptoken"
)

const (
	kubeletBootstrapKubeconfigPath = "/var/lib/kubelet/kubeconfig-real"
	kubeletTokenDescription        = "kubelet"
	kubeletTokenFilePermission     = 0o600
)

// CreateBootstrapToken creates a bootstrap token for the kubelet and writes it to the file system.
func CreateBootstrapToken(ctx context.Context, b *botanistpkg.Botanist, fs afero.Afero) error {
	bootstrapTokenSecret, err := bootstraptoken.ComputeBootstrapToken(
		ctx,
		b.SeedClientSet.Client(),
		tokenID(metav1.ObjectMeta{Name: b.Shoot.GetInfo().Name, Namespace: b.Shoot.GetInfo().Namespace}),
		kubeletTokenDescription,
		10*time.Minute)
	if err != nil {
		return fmt.Errorf("failed to create bootstrap token: %w", err)
	}

	bootstrapToken := string(bootstrapTokenSecret.Data[bootstraptokenapi.BootstrapTokenIDKey]) +
		"." + string(bootstrapTokenSecret.Data[bootstraptokenapi.BootstrapTokenSecretKey])
	return fs.WriteFile(nodeagentv1alpha1.BootstrapTokenFilePath, []byte(bootstrapToken), kubeletTokenFilePermission)
}

// WriteKubeletBootstrapKubeconfig writes the kubelet bootstrap kubeconfig to the file system.
func WriteKubeletBootstrapKubeconfig(
	ctx context.Context,
	b *botanistpkg.Botanist,
	fs afero.Afero,
	server string,
	caBundle []byte,
) error {
	if err := fs.MkdirAll(nodeagentv1alpha1.TempDir, os.ModeDir); err != nil {
		return fmt.Errorf("failed to create temporary directory ('%s'): %w", nodeagentv1alpha1.TempDir, err)
	}
	if err := fs.MkdirAll(nodeagentv1alpha1.CredentialsDir, os.ModeDir); err != nil {
		return fmt.Errorf("failed to create credentials directory ('%s'): %w", nodeagentv1alpha1.CredentialsDir, err)
	}

	exists, err := fs.Exists(nodeagentv1alpha1.BootstrapTokenFilePath)
	if err != nil {
		return fmt.Errorf("failed to check whether bootstrap token file exists ('%s'): %w", nodeagentv1alpha1.BootstrapTokenFilePath, err)
	}
	if !exists {
		b.Logger.Info("Writing fake bootstrap token to file to make sure kubelet can start up", "path", nodeagentv1alpha1.BootstrapTokenFilePath)
		// without this, kubelet will complain about an invalid kubeconfig
		token := tokenID(metav1.ObjectMeta{Name: b.Shoot.GetInfo().Name, Namespace: b.Shoot.GetInfo().Namespace})
		if err := fs.WriteFile(nodeagentv1alpha1.BootstrapTokenFilePath, []byte(token), kubeletTokenFilePermission); err != nil {
			return fmt.Errorf("failed to write fake bootstrap token to file ('%s'): %w", nodeagentv1alpha1.BootstrapTokenFilePath, err)
		}
	}

	if err := fs.Remove(kubeletBootstrapKubeconfigPath); err != nil && !errors.Is(err, afero.ErrFileNotFound) {
		return fmt.Errorf("failed to remove kubelet bootstrap kubeconfig file ('%s'): %w", kubeletBootstrapKubeconfigPath, err)
	}

	kubeletBootstrapKubeconfigCreator := &bootstrappers.KubeletBootstrapKubeconfig{
		Log: b.Logger,
		FS:  fs,
		APIServerConfig: nodeagentconfigv1alpha1.APIServer{
			Server:   server,
			CABundle: caBundle,
		},
	}

	return kubeletBootstrapKubeconfigCreator.Start(ctx)
}

func tokenID(meta metav1.ObjectMeta) string {
	value := meta.Name
	if meta.Namespace != "" {
		value = meta.Namespace + "--" + meta.Name
	}

	return utils.ComputeSHA256Hex([]byte(value))[:6]
}
