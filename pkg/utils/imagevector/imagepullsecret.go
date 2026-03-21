// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package imagevector

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/docker/cli/cli/config/configfile"
	corev1 "k8s.io/api/core/v1"
)

const defaultRegistryHost = "index.docker.io"

// CredentialsFromDockerConfigJSON extracts the username and password for the registry
// host matching the given imageRef (e.g. "myregistry.io/repo/img:tag") from a
// kubernetes.io/dockerconfigjson Secret.
func CredentialsFromDockerConfigJSON(secret *corev1.Secret, imageRef string) (string, string, error) {
	configfile, err := ConfigFileFromImagePullSecret(secret)
	if err != nil {
		return "", "", err
	}

	return CredentialsFromDockerConfigFile(configfile, imageRef)
}

// ConfigFileFromImagePullSecret creates a docker config file from the given kubernetes.io/dockerconfigjson Secret.
func ConfigFileFromImagePullSecret(secret *corev1.Secret) (*configfile.ConfigFile, error) {
	raw, ok := secret.Data[corev1.DockerConfigJsonKey]
	if !ok {
		return nil, fmt.Errorf("secret %q is missing key %s", secret.Name, corev1.DockerConfigJsonKey)
	}

	cf := configfile.New("")
	if err := cf.LoadFromReader(bytes.NewReader(raw)); err != nil {
		return nil, fmt.Errorf("failed to parse dockerconfigjson: %w", err)
	}
	return cf, nil
}

// CredentialsFromDockerConfigFile extracts the username and password from the given docker config file for the registry host matching the given imageRef.
func CredentialsFromDockerConfigFile(cf *configfile.ConfigFile, imageRef string) (string, string, error) {
	host := registryHostFromImageRef(imageRef)
	authConfig, err := cf.GetAuthConfig(host)
	if err != nil {
		return "", "", err
	}

	return authConfig.Username, authConfig.Password, nil
}

func registryHostFromImageRef(imageRef string) string {
	if isDefaultRegistryMatch(imageRef) {
		return defaultRegistryHost
	}
	host, _, _ := strings.Cut(imageRef, "/")
	return host
}

// isDefaultRegistryMatch determines whether the given image will pull from the
// default registry (DockerHub) based on the characteristics of its name.
//
// Copied from https://github.com/kubernetes/kubernetes/blob/v1.35.0/pkg/credentialprovider/keyring.go#L190-L215
func isDefaultRegistryMatch(image string) bool {
	parts := strings.SplitN(image, "/", 2)

	if len(parts[0]) == 0 {
		return false
	}

	if len(parts) == 1 {
		// e.g. library/ubuntu
		return true
	}

	if parts[0] == "docker.io" || parts[0] == "index.docker.io" {
		// resolve docker.io/image and index.docker.io/image as default registry
		return true
	}

	// From: http://blog.docker.com/2013/07/how-to-use-your-own-registry/
	// Docker looks for either a “.” (domain separator) or “:” (port separator)
	// to learn that the first part of the repository name is a location and not
	// a user name.
	return !strings.ContainsAny(parts[0], ".:")
}
