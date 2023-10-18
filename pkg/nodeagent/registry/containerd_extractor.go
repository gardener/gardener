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

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/snapshots"
	"github.com/opencontainers/image-spec/identity"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

type containerdExtractor struct{}

// NewExtractor creates a new instance of containerd extractor.
func NewExtractor() Extractor {
	return &containerdExtractor{}
}

// CopyFromImage copies files from a given image reference to the destination folder.
func (e *containerdExtractor) CopyFromImage(ctx context.Context, imageRef string, files []string, destination string) error {
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

	image, err := client.Pull(ctx, imageRef, containerd.WithPullSnapshotter(containerd.DefaultSnapshotter), containerd.WithPullUnpack)
	if err != nil {
		return fmt.Errorf("error pulling image: %w", err)
	}

	snapshotter := client.SnapshotService(containerd.DefaultSnapshotter)

	imageMountDirectory, err := os.MkdirTemp("", "node-agent-")
	if err != nil {
		return fmt.Errorf("error creating temp image: %w", err)
	}
	defer os.Remove(imageMountDirectory)

	if err := mountImage(ctx, image, snapshotter, imageMountDirectory); err != nil {
		return err
	}

	for _, file := range files {
		sourceFile := path.Join(imageMountDirectory, file)
		destinationFile := path.Join(destination, path.Base(file))
		if err := CopyFile(sourceFile, destinationFile); err != nil {
			return fmt.Errorf("error copying file %s to %s: %w", sourceFile, destinationFile, err)
		}
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

// CopyFile copies a source file to destination file and sets the executable flag.
func CopyFile(sourceFile, destinationFile string) error {
	sourceFileStat, err := os.Stat(sourceFile)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("source %q is not a regular file", sourceFile)
	}

	if destinationFileStat, err := os.Stat(destinationFile); err == nil {
		if !destinationFileStat.Mode().IsRegular() {
			return fmt.Errorf("destination %q exists but is not a regular file", destinationFile)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	srcFile, err := os.Open(sourceFile)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	if err := os.MkdirAll(path.Dir(destinationFile), 0755); err != nil {
		return fmt.Errorf("destination directory %q could not be created", path.Dir(destinationFile))
	}

	dstFile, err := os.OpenFile(destinationFile, os.O_CREATE|os.O_RDWR, 0755)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	return dstFile.Chmod(0755)
}
