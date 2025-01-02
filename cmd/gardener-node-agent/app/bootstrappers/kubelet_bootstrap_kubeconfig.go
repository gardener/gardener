// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrappers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"

	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// KubeletBootstrapKubeconfig is a runnable for creating a bootstrap kubeconfig for kubelet.
type KubeletBootstrapKubeconfig struct {
	Log             logr.Logger
	FS              afero.Afero
	APIServerConfig nodeagentconfigv1alpha1.APIServer
}

// Start performs creation of the bootstrap kubeconfig.
func (k *KubeletBootstrapKubeconfig) Start(_ context.Context) error {
	k.Log.Info("Checking whether kubelet bootstrap kubeconfig needs to be created")

	bootstrapToken, err := k.FS.ReadFile(nodeagentconfigv1alpha1.BootstrapTokenFilePath)
	if err != nil {
		if !errors.Is(err, afero.ErrFileNotFound) {
			return fmt.Errorf("failed checking whether bootstrap token file %q already exists: %w", nodeagentconfigv1alpha1.BootstrapTokenFilePath, err)
		}
		k.Log.Info("Bootstrap token file does not exist, nothing to be done", "path", nodeagentconfigv1alpha1.BootstrapTokenFilePath)
		return nil
	}

	if _, err := k.FS.Stat(kubelet.PathKubeconfigReal); err != nil && !errors.Is(err, afero.ErrFileNotFound) {
		return fmt.Errorf("failed checking whether kubelet kubeconfig file %q already exists: %w", kubelet.PathKubeconfigReal, err)
	} else if err == nil {
		k.Log.Info("Kubelet kubeconfig with client certificates already exists, nothing to be done", "path", kubelet.PathKubeconfigReal)
		return nil
	}

	kubeletClientCertificatePath := filepath.Join(kubelet.PathKubeletDirectory, "pki", "kubelet-client-current.pem")
	if _, err := k.FS.Stat(kubeletClientCertificatePath); err != nil && !errors.Is(err, afero.ErrFileNotFound) {
		return fmt.Errorf("failed checking whether kubelet client certificate file %q already exists: %w", kubeletClientCertificatePath, err)
	} else if err == nil {
		k.Log.Info("Kubelet client certificates file already exists, nothing to be done", "path", kubeletClientCertificatePath)
		return nil
	}

	k.Log.Info("Creating kubelet directory", "path", kubelet.PathKubeletDirectory)
	if err := k.FS.MkdirAll(kubelet.PathKubeletDirectory, os.ModeDir); err != nil {
		return fmt.Errorf("unable to create kubelet directory %q: %w", kubelet.PathKubeletDirectory, err)
	}

	kubeconfig, err := runtime.Encode(clientcmdlatest.Codec, kubernetesutils.NewKubeconfig(
		"kubelet-bootstrap",
		clientcmdv1.Cluster{Server: k.APIServerConfig.Server, CertificateAuthorityData: k.APIServerConfig.CABundle},
		clientcmdv1.AuthInfo{Token: strings.TrimSpace(string(bootstrapToken))},
	))
	if err != nil {
		return fmt.Errorf("unable to encode kubeconfig: %w", err)
	}

	k.Log.Info("Writing kubelet bootstrap kubeconfig file", "path", kubelet.PathKubeconfigBootstrap)
	return k.FS.WriteFile(kubelet.PathKubeconfigBootstrap, kubeconfig, 0600)
}
