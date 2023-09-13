// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registry

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	containerregistryv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type remoteExtractor struct{}

func (remoteExtractor) ExtractFromLayer(image, pathSuffix, dest string) error {
	// In the local environment, we pull Gardener images built via skaffold from the local registry running in the kind
	// cluster. However, on local machine pods, `localhost:5001` does obviously not lead to this registry. Hence, we
	// have to replace it with `garden.local.gardener.cloud:5001` which allows accessing the registry from both local
	// machine and machine pods.
	image = strings.ReplaceAll(image, "localhost:5001", "garden.local.gardener.cloud:5001")

	imageRef, err := name.ParseReference(image)
	if err != nil {
		return fmt.Errorf("unable to parse reference: %w", err)
	}

	remoteImage, err := remote.Image(imageRef, remote.WithPlatform(containerregistryv1.Platform{OS: "linux", Architecture: runtime.GOARCH}))
	if err != nil {
		return fmt.Errorf("unable access remote image reference: %w", err)
	}

	layers, err := remoteImage.Layers()
	if err != nil {
		return fmt.Errorf("unable retrieve image layers: %w", err)
	}

	lastLayer := len(layers) - 1
	for i := range layers {
		layer := layers[lastLayer-i]
		buffer, err := layer.Uncompressed()
		if err != nil {
			return fmt.Errorf("unable to get reader for uncompressed layer: %w", err)
		}

		if err = extractFileFromTar(buffer, pathSuffix, dest); err != nil {
			if errors.Is(err, errNotFound) {
				continue
			}
			return fmt.Errorf("unable to extract tarball to file system: %w", err)
		}

		return nil
	}

	return fmt.Errorf("did not find file %q in layer", pathSuffix)
}

var errNotFound = errors.New("file not contained in tar")

func extractFileFromTar(uncompressedStream io.Reader, searchSuffix, targetFile string) error {
	var (
		tarReader = tar.NewReader(uncompressedStream)
		header    *tar.Header
		err       error
	)

	for header, err = tarReader.Next(); err == nil; header, err = tarReader.Next() {
		switch header.Typeflag {
		case tar.TypeReg:
			if !strings.HasSuffix(header.Name, searchSuffix) {
				continue
			}

			tmpDest := targetFile + ".tmp"

			outFile, err := os.OpenFile(tmpDest, os.O_CREATE|os.O_RDWR, 0755)
			if err != nil {
				return fmt.Errorf("create file failed: %w", err)
			}

			defer outFile.Close()

			if _, err := io.Copy(outFile, tarReader); err != nil {
				return fmt.Errorf("copying file from tarball failed: %w", err)
			}

			return os.Rename(tmpDest, targetFile)
		default:
			continue
		}
	}
	if err != io.EOF {
		return fmt.Errorf("iterating tar files failed: %w", err)
	}

	return errNotFound
}
