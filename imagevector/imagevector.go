// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate ../hack/generate-imagename-constants.sh imagevector containers.yaml Container
//go:generate ../hack/resolve-etcd-version-from-etcd-druid.sh containers.yaml
//go:generate ../hack/generate-imagename-constants.sh imagevector charts.yaml Chart

package imagevector

import (
	_ "embed"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/utils/imagevector"
)

var (
	//go:embed containers.yaml
	containersYAML                string
	containersImageVector         imagevector.ImageVector
	containersCABundle            *imagevector.CABundle
	containersImagePullCredential *imagevector.PullCredentials

	//go:embed charts.yaml
	chartsYAML                string
	chartsImageVector         imagevector.ImageVector
	chartsCABundle            *imagevector.CABundle
	chartsImagePullCredential *imagevector.PullCredentials
)

func init() {
	var err error

	containersImageVector, containersCABundle, containersImagePullCredential, err = imagevector.Read([]byte(containersYAML))
	runtime.Must(err)
	containersImageVector, containersCABundle, containersImagePullCredential, err = imagevector.WithEnvOverride(containersImageVector, containersCABundle, imagevector.OverrideEnv)
	runtime.Must(err)

	chartsImageVector, chartsCABundle, chartsImagePullCredential, err = imagevector.Read([]byte(chartsYAML))
	runtime.Must(err)
	chartsImageVector, chartsCABundle, chartsImagePullCredential, err = imagevector.WithEnvOverride(chartsImageVector, chartsCABundle, imagevector.OverrideChartsEnv)
	runtime.Must(err)
}

// Containers is the image vector that contains all the needed container images.
func Containers() imagevector.ImageVector {
	return containersImageVector
}

// ContainerImagePullCredential returns the global image pull credential for container images, if specified.
func ContainerImagePullCredential() *imagevector.PullCredentials {
	return containersImagePullCredential
}

// Charts is the image vector that contains all the needed Helm chart images.
func Charts() imagevector.ImageVector {
	return chartsImageVector
}

// ContainersCABundle returns the CA bundle defined in the container image vector.
func ContainersCABundle() *imagevector.CABundle {
	return containersCABundle
}

// ChartsCABundle returns the CA bundle defined in the chart image vector.
func ChartsCABundle() *imagevector.CABundle {
	return chartsCABundle
}

// ChartImagePullCredential returns the global image pull credential for Helm chart images, if specified.
func ChartImagePullCredential() *imagevector.PullCredentials {
	return chartsImagePullCredential
}

// AllContainerImagePullCredentials returns all unique image pull credentials (global + per-image) for containers.
func AllContainerImagePullCredentials() []*imagevector.PullCredentials {
	seen := sets.New[string]()
	var result []*imagevector.PullCredentials

	addCred := func(cred *imagevector.PullCredentials) {
		if cred == nil {
			return
		}
		key := imagevector.CredentialKey(cred)
		if !seen.Has(key) {
			seen.Insert(key)
			result = append(result, cred)
		}
	}

	addCred(containersImagePullCredential)
	for _, cred := range containersImageVector.AllPullCredentials() {
		addCred(cred)
	}

	return result
}

// ContainerImagePullCredentialForImage returns the pull credential for a given container image reference.
// It first checks for a per-image credential, then falls back to the global credential.
// Returns nil if no credential is configured for the image.
func ContainerImagePullCredentialForImage(containerImage string) *imagevector.PullCredentials {
	if perImage := containersImageVector.ImagePullCredentialForContainerImage(containerImage); perImage != nil {
		return perImage
	}
	return containersImagePullCredential
}

// OverwriteForImagePullSecretWebhook returns the raw container image vector overwrite to be propagated to a
// gardener-resource-manager so its image-pull-secret webhook can determine which pull secrets to inject.
// It returns nil (no propagation, webhook stays disabled) when no overwrite is configured or when the
// overwrite does not configure any image pull credentials - in that case the webhook would have nothing to
// inject and running it on every pod would be pointless.
func OverwriteForImagePullSecretWebhook() (*string, error) {
	path := os.Getenv(imagevector.OverrideEnv)
	if path == "" || len(AllContainerImagePullCredentials()) == 0 {
		return nil, nil
	}

	overwrite, err := os.ReadFile(path) // #nosec: G304 -- IMAGEVECTOR_OVERWRITE is a trusted operator-provided path.
	if err != nil {
		return nil, fmt.Errorf("failed reading image vector overwrite file %q: %w", path, err)
	}

	return ptr.To(string(overwrite)), nil
}
