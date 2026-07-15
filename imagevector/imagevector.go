// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate ../hack/generate-imagename-constants.sh imagevector containers.yaml Container
//go:generate ../hack/resolve-etcd-version-from-etcd-druid.sh containers.yaml
//go:generate ../hack/generate-imagename-constants.sh imagevector charts.yaml Chart

package imagevector

import (
	_ "embed"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/gardener/gardener/pkg/utils/imagevector"
)

var (
	//go:embed containers.yaml
	containersYAML                string
	containersImageVector         imagevector.ImageVector
	containersCABundle            *imagevector.CABundle
	containersImagePullCredential *imagevector.ImagePullCredential

	//go:embed charts.yaml
	chartsYAML                string
	chartsImageVector         imagevector.ImageVector
	chartsCABundle            *imagevector.CABundle
	chartsImagePullCredential *imagevector.ImagePullCredential
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

// ContainerImagePullCredential returns the global image pull credential for container images, if specified.
func ContainerImagePullCredential() *imagevector.ImagePullCredential {
	return containersImagePullCredential
}

// ChartImagePullCredential returns the global image pull credential for Helm chart images, if specified.
func ChartImagePullCredential() *imagevector.ImagePullCredential {
	return chartsImagePullCredential
}

// AllContainerImagePullCredentials returns all unique image pull credentials (global + per-image) for containers.
func AllContainerImagePullCredentials() []*imagevector.ImagePullCredential {
	seen := sets.New[string]()
	var result []*imagevector.ImagePullCredential

	addCred := func(cred *imagevector.ImagePullCredential) {
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
	for _, cred := range containersImageVector.AllImagePullCredentials() {
		addCred(cred)
	}
	return result
}

// ContainerImagePullCredentialForImage returns the pull credential for a given container image reference.
// It first checks for a per-image credential, then falls back to the global credential.
// Returns nil if no credential is configured for the image.
func ContainerImagePullCredentialForImage(containerImage string) *imagevector.ImagePullCredential {
	if perImage := containersImageVector.ImagePullCredentialForContainerImage(containerImage); perImage != nil {
		return perImage
	}
	return containersImagePullCredential
}
