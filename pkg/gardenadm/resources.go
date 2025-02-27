// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenadm

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// ReadManifests reads Kubernetes and Gardener manifests in YAML or JSON format.
// It returns a CloudProfile, Project, and Shoot resource if found, or an error if any issues occur during reading or
// decoding.
func ReadManifests(log logr.Logger, fsys fs.FS) (*gardencorev1beta1.CloudProfile, *gardencorev1beta1.Project, *gardencorev1beta1.Shoot, error) {
	var (
		cloudProfile *gardencorev1beta1.CloudProfile
		project      *gardencorev1beta1.Project
		shoot        *gardencorev1beta1.Shoot

		decoder = serializer.NewCodecFactory(kubernetes.GardenScheme).UniversalDecoder(gardencorev1.SchemeGroupVersion, gardencorev1beta1.SchemeGroupVersion)
	)

	if err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed walking directory: %w", err)
		}

		if d.IsDir() || !(strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".json")) {
			return nil
		}

		file, err := fsys.Open(path)
		if err != nil {
			return fmt.Errorf("failed opening file %s: %w", path, err)
		}
		defer file.Close()

		reader := yaml.NewYAMLReader(bufio.NewReader(file))

		for indexInFile := 0; true; indexInFile++ {
			content, err := reader.Read()
			if err == io.EOF {
				break
			} else if err != nil {
				return fmt.Errorf("failed reading resource at index %d in %s: %w", indexInFile, path, err)
			}

			o, err := runtime.Decode(decoder, content)
			if err != nil {
				return fmt.Errorf("failed decoding resource at index %d in %s: %w", indexInFile, path, err)
			}

			obj, ok := o.(client.Object)
			if !ok {
				return fmt.Errorf("expected client.Object but got %T at index %d in %s", o, indexInFile, path)
			}

			objLog := log.WithValues("path", path, "indexInFile", indexInFile)
			objLog.V(2).Info("Read resource", "gvk", obj.GetObjectKind().GroupVersionKind(), "name", obj.GetName(), "namespace", obj.GetNamespace())

			switch typedObj := obj.(type) {
			case *gardencorev1beta1.CloudProfile:
				if cloudProfile != nil {
					return fmt.Errorf("found more than one *gardencorev1beta1.CloudProfile resource, but only one is allowed")
				}
				cloudProfile = typedObj

			case *gardencorev1beta1.Project:
				if project != nil {
					return fmt.Errorf("found more than one *gardencorev1beta1.Project resource, but only one is allowed")
				}
				project = typedObj

			case *gardencorev1beta1.Shoot:
				if shoot != nil {
					return fmt.Errorf("found more than one *gardencorev1beta1.Shoot resource, but only one is allowed")
				}
				shoot = typedObj
			}
		}

		return nil
	}); err != nil {
		return nil, nil, nil, fmt.Errorf("failed reading Kubernetes resources from config directory: %w", err)
	}

	if cloudProfile == nil {
		return nil, nil, nil, fmt.Errorf("must provide a *gardencorev1beta1.CloudProfile resource, but did not find any")
	}
	if project == nil {
		return nil, nil, nil, fmt.Errorf("must provide a *gardencorev1beta1.Project resource, but did not find any")
	}
	if shoot == nil {
		return nil, nil, nil, fmt.Errorf("must provide a *gardencorev1beta1.Shoot resource, but did not find any")
	}

	return cloudProfile, project, shoot, nil
}
