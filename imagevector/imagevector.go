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

	"github.com/gardener/gardener/pkg/utils/imagevector"
)

var (
	//go:embed containers.yaml
	containersYAML               string
	containersImageVector        imagevector.ImageVector
	containersImagePullCredential *imagevector.ImagePullCredential

	//go:embed charts.yaml
	chartsYAML               string
	chartsImageVector        imagevector.ImageVector
	chartsImagePullCredential *imagevector.ImagePullCredential
)

func init() {
	var err error

	containersImageVector, containersImagePullCredential, err = imagevector.Read([]byte(containersYAML))
	runtime.Must(err)
	containersImageVector, containersImagePullCredential, err = imagevector.WithEnvOverride(containersImageVector, imagevector.OverrideEnv)
	runtime.Must(err)

	chartsImageVector, chartsImagePullCredential, err = imagevector.Read([]byte(chartsYAML))
	runtime.Must(err)
	chartsImageVector, chartsImagePullCredential, err = imagevector.WithEnvOverride(chartsImageVector, imagevector.OverrideChartsEnv)
	runtime.Must(err)
}

// Containers is the image vector that contains all the needed container images.
func Containers() imagevector.ImageVector {
	return containersImageVector
}

// ContainerImagePullCredential returns the global image pull credential for container images, if specified.
func ContainerImagePullCredential() *imagevector.ImagePullCredential {
	return containersImagePullCredential
}

// Charts is the image vector that contains all the needed Helm chart images.
func Charts() imagevector.ImageVector {
	return chartsImageVector
}

// ChartImagePullCredential returns the global image pull credential for Helm chart images, if specified.
func ChartImagePullCredential() *imagevector.ImagePullCredential {
	return chartsImagePullCredential
}

// AllContainerImagePullCredentials returns all unique image pull credentials (global + per-image) for containers.
func AllContainerImagePullCredentials() []*imagevector.ImagePullCredential {
	seen := make(map[string]struct{})
	var result []*imagevector.ImagePullCredential

	addCred := func(cred *imagevector.ImagePullCredential) {
		if cred == nil {
			return
		}
		key := credentialKey(cred)
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
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

// credentialKey returns a deduplication key for an ImagePullCredential.
func credentialKey(c *imagevector.ImagePullCredential) string {
	if c.SecretName != nil {
		return string(c.Type) + ":" + *c.SecretName
	}
	return string(c.Type)
}
