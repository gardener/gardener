// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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

// BrotliCompression compresses the passed data with the Brotli compression algorithm.
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

// BrotliCompressionForManifests compresses the passed data with the Brotli compression algorithm.
func BrotliCompressionForManifests(manifests ...string) ([]byte, error) {
	var data bytes.Buffer

	for i, manifest := range manifests {
		if _, err := data.WriteString(manifest); err != nil {
			return nil, err
		}

		if !strings.HasSuffix(manifest, "\n") {
			if _, err := data.WriteString("\n"); err != nil {
				return nil, err
			}
		}
		if !strings.HasSuffix(manifest, "---\n") && i < len(manifests)-1 {
			if _, err := data.WriteString("---\n"); err != nil {
				return nil, err
			}
		}
	}

	return BrotliCompression(data.Bytes())
}

// BrotliDecompression decompresses the passed data with the Brotli compression algorithm.
func BrotliDecompression(data []byte) ([]byte, error) {
	return io.ReadAll(brotli.NewReader(bytes.NewBuffer(data)))
}

// ExtractManifestsFromManagedResourceData extracts the resources from the given compressed data,
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
	}

	return manifests, nil
}
