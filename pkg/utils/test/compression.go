// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/andybalholm/brotli"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

// BrotliCompression compressed the passed data with the Brotli compression algorithm.
func BrotliCompression(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := brotli.NewWriter(&buf)

	if _, err := w.Write(data); err != nil {
		return nil, err
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// BrotliDecompression decompressed the passed data with the Brotli compression algorithm.
func BrotliDecompression(data []byte) ([]byte, error) {
	return io.ReadAll(brotli.NewReader(bytes.NewBuffer(data)))
}

// ExtractManifestsFromManagedResourceData extracts the compressed resources from the given data,
// usually used for ManagedResources.
func ExtractManifestsFromManagedResourceData(data map[string][]byte) ([]string, error) {
	compressedData, ok := data[resourcesv1alpha1.CompressedDataKey]
	if !ok {
		return nil, fmt.Errorf("failed to extract manifests, data key %s not found", resourcesv1alpha1.CompressedDataKey)
	}

	uncompressedData, err := BrotliDecompression(compressedData)
	if err != nil {
		return nil, err
	}

	var manifests []string
	for _, manifest := range strings.Split(string(uncompressedData), "---\n") {
		if manifest != "" {
			manifests = append(manifests, manifest)
		}
		continue
	}

	return manifests, nil
}
