// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registry

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

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

	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
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

	imageMountDirectory, err := fs.TempDir(nodeagentv1alpha1.TempDir, "mount-image-")
	if err != nil {
		return fmt.Errorf("error creating temp directory: %w", err)
	}

	defer func() { utilruntime.HandleError(fs.Remove(imageMountDirectory)) }()

	if err := mountImage(ctx, image, snapshotter, imageMountDirectory); err != nil {
		return err
	}

	source := path.Join(imageMountDirectory, filePathInImage)
	if err := CopyFile(fs, source, destination, permissions); err != nil {
		return fmt.Errorf("error copying file %s to %s: %w", source, destination, err)
	}

	return unmountImage(ctx, snapshotter, imageMountDirectory)
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

// CopyFile copies a source file to destination file and sets the given permissions.
func CopyFile(fs afero.Afero, source, destination string, permissions os.FileMode) error {
	if destinationFileStat, err := fs.Stat(destination); err == nil {
		if !destinationFileStat.Mode().IsRegular() {
			return fmt.Errorf("destination %q exists but is not a regular file", destination)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := fs.MkdirAll(path.Dir(destination), permissions); err != nil {
		return fmt.Errorf("destination directory %q could not be created", path.Dir(destination))
	}

	tempDir, err := fs.TempDir(nodeagentv1alpha1.TempDir, "copy-image-")
	if err != nil {
		return fmt.Errorf("error creating temp directory: %w", err)
	}

	defer func() { utilruntime.HandleError(fs.Remove(tempDir)) }()

	tmpFilePath := filepath.Join(tempDir, filepath.Base(destination))

	if err := copyFileAndSync(fs, source, tmpFilePath); err != nil {
		return err
	}

	if err := fs.Chmod(tmpFilePath, permissions); err != nil {
		return err
	}

	return MoveFile(fs, tmpFilePath, destination)
}

func copyFileAndSync(fs afero.Afero, source, destination string) error {
	sourceFileStat, err := fs.Stat(source)
	if err != nil {
		return err
	}
	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("source %q is not a regular file", source)
	}

	if destinationFileStat, err := fs.Stat(destination); err == nil {
		if !destinationFileStat.Mode().IsRegular() {
			return fmt.Errorf("destination %q exists but is not a regular file", destination)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	srcFile, err := fs.Open(source)
	if err != nil {
		return err
	}
	defer func() { utilruntime.HandleError(srcFile.Close()) }()

	if err := fs.MkdirAll(path.Dir(destination), os.ModeDir); err != nil {
		return fmt.Errorf("destination directory %q could not be created", path.Dir(destination))
	}

	dstFile, err := fs.OpenFile(destination, os.O_CREATE|os.O_RDWR, 0755)
	if err != nil {
		return err
	}
	defer func() { utilruntime.HandleError(dstFile.Close()) }()

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	if err := fs.Chmod(dstFile.Name(), sourceFileStat.Mode()); err != nil {
		return err
	}

	return dstFile.Sync()
}

// CrossDeviceModeOnly is exposed for testing cross-device moving mode
var CrossDeviceModeOnly = false

// MoveFile moves files on the same or across different devices
func MoveFile(fs afero.Afero, source, destination string) error {
	sourceFileStat, err := fs.Stat(source)
	if err != nil {
		return err
	}
	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("source %q is not a regular file", source)
	}

	if destinationFileStat, err := fs.Stat(destination); err == nil {
		if !destinationFileStat.Mode().IsRegular() {
			return fmt.Errorf("destination %q exists but is not a regular file", destination)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if CrossDeviceModeOnly {
		return moveCrossDevice(fs, source, destination)
	}
	err = fs.Rename(source, destination)
	if err != nil && strings.Contains(err.Error(), "cross-device link") {
		return moveCrossDevice(fs, source, destination)
	}
	return err
}

func moveCrossDevice(fs afero.Afero, source, destination string) error {
	tmpFilePath := destination + ".tmp"
	if err := copyFileAndSync(fs, source, tmpFilePath); err != nil {
		return err
	}
	if err := fs.Rename(tmpFilePath, destination); err != nil {
		return err
	}

	return fs.Remove(source)
}
