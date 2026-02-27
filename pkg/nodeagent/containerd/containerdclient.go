// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/defaults"
	"github.com/containerd/containerd/v2/pkg/namespaces"
)

// Client defines the containerd client Interface exported for testing.
type Client interface {
	Version(context.Context) (containerd.Version, error)
}

// envContainerdAddress is the name of the environment variable holding the containerd address
const envContainerdAddress = "CONTAINERD_ADDRESS"

var version2dot2 *semver.Version

func init() {
	version2dot2 = semver.MustParse("2.2")
}

// NewClient returns a new client to connect to containerd
func NewClient() (*containerd.Client, error) {
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
func VersionGreaterThanEqual22(ctx context.Context, client Client) (bool, error) {
	return versionGreaterThanEqual(ctx, client, version2dot2)
}

func versionGreaterThanEqual(ctx context.Context, client Client, s *semver.Version) (bool, error) {
	containerdVersion, err := client.Version(ctx)
	if err != nil {
		return false, err
	}

	// on some Debian based distros, internal build info can spill over to the patch version of containerd
	// e.g. the version could be 1.7.23~ds2
	// this is not valid semver and therefore, we need to strip any ~ and what follows from the patch
	// we also strip any pre-release or build info from the version string as these might also not be
	// semver compliant with certain Debian builds of containerd
	sanitizedVersion := strings.Split(containerdVersion.Version, ".")
	if len(sanitizedVersion) < 3 {
		return false, fmt.Errorf("containerd version %s is not semver compliant and does not consist of <major>.<minor>.<patch>", containerdVersion.Version)
	}

	for _, c := range []string{"-", "+", "~"} {
		if before, _, found := strings.Cut(sanitizedVersion[2], c); found {
			sanitizedVersion[2] = before
		}
	}

	v, err := semver.NewVersion(strings.Join(sanitizedVersion[:3], "."))
	if err != nil {
		return false, err
	}

	return v.GreaterThanEqual(s), nil
}
