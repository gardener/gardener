// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/remotes/docker/config"
	"github.com/containerd/containerd/snapshots"
	"github.com/opencontainers/image-spec/identity"
	"github.com/spf13/afero"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/files"
)

type containerdExtractor struct{}

// NewExtractor creates a new instance of containerd extractor.
func NewExtractor() Extractor {
	return &containerdExtractor{}
}

// CopyFromImage copies a file from a given image reference to the destination file.
func (e *containerdExtractor) CopyFromImage(ctx context.Context, imageRef string, filePathInImage string, destination string, permissions os.FileMode) error {
	fs := afero.Afero{Fs: afero.NewOsFs()}

	address := os.Getenv("CONTAINERD_ADDRESS")
	if address == "" {
		address = defaults.DefaultAddress
	}
	namespace := os.Getenv(namespaces.NamespaceEnvVar)
	if namespace == "" {
		namespace = namespaces.Default
	}

	client, err := containerd.New(address, containerd.WithDefaultNamespace(namespace))
	if err != nil {
		return fmt.Errorf("error creating containerd client: %w", err)
	}
	ctx, done, err := client.WithLease(ctx)
	if err != nil {
		return fmt.Errorf("error adding lease to containerd client: %w", err)
	}

	defer func() { utilruntime.HandleError(done(ctx)) }()

	resolver := docker.NewResolver(docker.ResolverOptions{
		Hosts: config.ConfigureHosts(ctx, config.HostOptions{HostDir: config.HostDirFromRoot("/etc/containerd/certs.d")}),
	})

	image, err := client.Pull(ctx, imageRef, containerd.WithPullSnapshotter(containerd.DefaultSnapshotter), containerd.WithResolver(resolver), containerd.WithPullUnpack)
	if err != nil {
		return fmt.Errorf("error pulling image: %w", err)
	}

	snapshotter := client.SnapshotService(containerd.DefaultSnapshotter)

	imageMountDirectory, err := fs.TempDir(nodeagentconfigv1alpha1.TempDir, "mount-image-")
	if err != nil {
		return fmt.Errorf("error creating temp directory: %w", err)
	}

	defer func() { utilruntime.HandleError(fs.Remove(imageMountDirectory)) }()

	if err := mountImage(ctx, image, snapshotter, imageMountDirectory); err != nil {
		return err
	}

	defer func() { utilruntime.HandleError(unmountImage(ctx, snapshotter, imageMountDirectory)) }()

	source := path.Join(imageMountDirectory, filePathInImage)
	if err := files.Copy(fs, source, destination, permissions); err != nil {
		return fmt.Errorf("error copying file %s to %s: %w", source, destination, err)
	}

	return nil
}

func mountImage(ctx context.Context, image containerd.Image, snapshotter snapshots.Snapshotter, directory string) error {
	diffIDs, err := image.RootFS(ctx)
	if err != nil {
		return err
	}
	chainID := identity.ChainID(diffIDs).String()

	mounts, err := snapshotter.View(ctx, directory, chainID)
	if err != nil {
		return err
	}

	if err := mount.All(mounts, directory); err != nil {
		if err := snapshotter.Remove(ctx, directory); err != nil && !errdefs.IsNotFound(err) {
			return fmt.Errorf("error cleaning up snapshot after mount error: %w", err)
		}
		return fmt.Errorf("error mounting image: %w", err)
	}

	return nil
}

func unmountImage(ctx context.Context, snapshotter snapshots.Snapshotter, directory string) error {
	if err := mount.UnmountAll(directory, 0); err != nil {
		return err
	}

	if err := snapshotter.Remove(ctx, directory); err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("error removing snapshot: %w", err)
	}

	return nil
}
