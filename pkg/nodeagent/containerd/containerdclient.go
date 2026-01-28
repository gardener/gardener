// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	"context"
	"fmt"
	"os"

	"github.com/Masterminds/semver/v3"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/defaults"
	"github.com/containerd/containerd/v2/pkg/namespaces"
)

// ContainerdClient defines the containerd client Interface exported for testing.
type ContainerdClient interface {
	Version(context.Context) (containerd.Version, error)
}

const (
	// envContainerdAddress is the name of the environment variable holding the containerd address
	envContainerdAddress = "CONTAINERD_ADDRESS"
)

var (
	version2dot2 *semver.Version
)

func init() {
	version2dot2 = semver.MustParse("2.2")
}

// NewContainerdClient returns a new client to connect to containerd
func NewContainerdClient() (*containerd.Client, error) {
	address := os.Getenv(envContainerdAddress)
	if address == "" {
		address = defaults.DefaultAddress
	}

	namespace := os.Getenv(namespaces.NamespaceEnvVar)
	if namespace == "" {
		namespace = namespaces.Default
	}

	client, err := containerd.New(address, containerd.WithDefaultNamespace(namespace))
	if err != nil {
		return nil, fmt.Errorf("error creating containerd client: %w", err)
	}

	return client, err
}

// VersionGreaterThanEqual22 checks if the running containerd version is greater or equal to 2.2
func VersionGreaterThanEqual22(ctx context.Context, client ContainerdClient) (bool, error) {
	return versionGreaterThanEqual(ctx, client, version2dot2)
}

func versionGreaterThanEqual(ctx context.Context, client ContainerdClient, s *semver.Version) (bool, error) {
	containerdVersion, err := client.Version(ctx)
	if err != nil {
		return false, err
	}

	v, err := semver.NewVersion(containerdVersion.Version)
	if err != nil {
		return false, err
	}

	return v.GreaterThanEqual(s), nil
}
