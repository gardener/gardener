package kubernetes

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// ValidDeploymentContainerImageVersion validates compliance of a deployment container image to a minimum version
func ValidDeploymentContainerImageVersion(deploymentToCheck *appsv1.Deployment, containerName, minimumVersion string) (bool, error) {
	containers := deploymentToCheck.Spec.Template.Spec.Containers
	getContainer := func(container string) (*corev1.Container, error) {
		for _, container := range containers {
			if container.Name == containerName {
				return &container, nil
			}
		}
		return nil, fmt.Errorf("Container %q does not belong to this deployment", container)
	}

	containerToCheck, err := getContainer(containerName)
	if err != nil {
		return false, err
	}
	actualVersion, err := semver.NewVersion(strings.Split(containerToCheck.Image, ":")[1])
	if err != nil {
		return false, err
	}
	minVersion, err := semver.NewVersion(minimumVersion)
	if err != nil {
		return false, err
	}
	if actualVersion.LessThan(minVersion) {
		return false, nil
	}

	return true, nil
}
