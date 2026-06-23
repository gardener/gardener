// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

// KubeconfigSecretName is the name of the secret in the shoot namespace of the bootstrap cluster containing the
// kubeconfig of the self-hosted shoot.
const KubeconfigSecretName = "kubeconfig"

// NewClientSetFromFile creates a client set from the specified kubeconfig file.
func NewClientSetFromFile(kubeconfigPath string, scheme *runtime.Scheme) (kubernetes.Interface, error) {
	return kubernetes.NewClientFromFile("", kubeconfigPath,
		kubernetes.WithClientOptions(client.Options{Scheme: scheme}),
		kubernetes.WithClientConnectionOptions(componentbaseconfigv1alpha1.ClientConnectionConfiguration{QPS: 100, Burst: 130}),
		kubernetes.WithDisabledCachedClient(),
	)
}

// CreateClientSet creates a client set for the control plane.
func (b *GardenadmBotanist) CreateClientSet(ctx context.Context) (kubernetes.Interface, error) {
	pathKubeconfig := botanist.PathKubeconfig
	if path := os.Getenv("KUBECONFIG"); path != "" {
		pathKubeconfig = path
	}

	clientSet, err := NewClientSetFromFile(pathKubeconfig, kubernetes.SeedScheme)
	if err != nil {
		b.Logger.Info("Waiting for kube-apiserver to start", "error", err.Error())
		return nil, fmt.Errorf("failed creating client set: %w", err)
	}

	clientSet.Start(ctx)

	waitContext, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if !clientSet.WaitForCacheSync(waitContext) {
		return nil, fmt.Errorf("timed out waiting for caches")
	}

	result := clientSet.RESTClient().Get().AbsPath("/readyz").Do(ctx)
	if result.Error() != nil {
		return nil, fmt.Errorf("failed to GET /readyz endpoint of kube-apiserver: %w", result.Error())
	}

	var statusCode int
	result.StatusCode(&statusCode)
	if statusCode != http.StatusOK {
		b.Logger.Info("The kube-apiserver does not report readiness yet", "statusCode", statusCode)
		return nil, fmt.Errorf("kube-apiserver does not report readiness yet")
	}

	return clientSet, nil
}

// DiscoverKubernetesVersion discovers the Kubernetes version of the control plane.
func (b *GardenadmBotanist) DiscoverKubernetesVersion(clientSet kubernetes.Interface) (*semver.Version, error) {
	version, err := semver.NewVersion(clientSet.Version())
	if err != nil {
		return nil, fmt.Errorf("failed parsing semver version %q: %w", clientSet.Version(), err)
	}

	return version, nil
}

// FetchKubeconfig fetches the kubeconfig of the self-hosted shoot from the control plane machine via SSH and stores it
// in a secret in the shoot namespace of the bootstrap cluster for later retrieval. It also writes the kubeconfig to the
// given output writer, if any.
func (b *GardenadmBotanist) FetchKubeconfig(ctx context.Context, output io.Writer) error {
	kubeconfigBuffer := &bytes.Buffer{}
	if err := b.sshConnection.SCP.CopyFromRemotePassThru(ctx, kubeconfigBuffer, botanist.PathKubeconfig, nil); err != nil {
		return fmt.Errorf("error fetching kubeconfig: %w", err)
	}
	kubeconfigBytes := kubeconfigBuffer.Bytes()

	kubeconfigSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: KubeconfigSecretName, Namespace: b.Shoot.ControlPlaneNamespace}}
	_, err := controllerutils.CreateOrGetAndMergePatch(ctx, b.SeedClientSet.Client(), kubeconfigSecret, func() error {
		kubeconfigSecret.Data = map[string][]byte{
			secretsutils.DataKeyKubeconfig: kubeconfigBytes,
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error writing kubeconfig secret: %w", err)
	}
	b.Logger.Info("Stored kubeconfig of the self-hosted shoot in secret", "namespace", kubeconfigSecret.Namespace, "name", kubeconfigSecret.Name)

	if output != nil {
		if outputFile, ok := output.(*os.File); ok {
			b.Logger.Info("Writing kubeconfig of the self-hosted shoot to file", "path", outputFile.Name())
		}
		if _, err := output.Write(kubeconfigBytes); err != nil {
			return fmt.Errorf("error writing kubeconfig to output: %w", err)
		}
	}

	return nil
}
