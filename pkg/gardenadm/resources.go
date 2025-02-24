// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenadm

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// ReadKubernetesResourcesFromConfigDir reads Kubernetes resources from the specified configuration directory.
// It returns a CloudProfile, Project, and Shoot resource if found, or an error if any issues occur during reading or
// decoding.
func ReadKubernetesResourcesFromConfigDir(log logr.Logger, f afero.Afero, configDir string) (*gardencorev1beta1.CloudProfile, *gardencorev1beta1.Project, *gardencorev1beta1.Shoot, error) {
	var (
		cloudProfile *gardencorev1beta1.CloudProfile
		project      *gardencorev1beta1.Project
		shoot        *gardencorev1beta1.Shoot
	)

	if err := afero.Walk(f, configDir, func(path string, fileInfo fs.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed walking directory %s: %w", configDir, err)
		}

		if fileInfo.IsDir() || !(strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".json")) {
			return nil
		}

		content, err := f.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed reading file %s: %w", path, err)
		}

		var (
			decoder    = yaml.NewYAMLOrJSONDecoder(bytes.NewReader(content), 1024)
			decodedObj map[string]any
		)

		for indexInFile := 0; true; indexInFile++ {
			objLog := log.WithValues("path", path, "indexInFile", indexInFile)

			if err := decoder.Decode(&decodedObj); err == io.EOF {
				break
			} else if err != nil {
				return fmt.Errorf("failed decoding resource at index %d in %s", indexInFile, path)
			}

			if decodedObj == nil {
				continue
			}

			obj := &unstructured.Unstructured{Object: decodedObj}
			objLog.V(2).Info("Read resource", "apiVersion", obj.GetAPIVersion(), "kind", obj.GetKind(), "name", obj.GetName(), "namespace", obj.GetNamespace())

			switch gk := obj.GroupVersionKind().GroupKind(); gk {
			case gardencorev1beta1.SchemeGroupVersion.WithKind("CloudProfile").GroupKind():
				if cloudProfile != nil {
					return fmt.Errorf("found more than one *gardencorev1beta1.CloudProfile resource, but only one is allowed")
				}
				cloudProfile = &gardencorev1beta1.CloudProfile{}
				if err := kubernetes.GardenScheme.Convert(obj, cloudProfile, nil); err != nil {
					return fmt.Errorf("failed converting object with group kind %s to CloudProfile: %w", gk, err)
				}

			case gardencorev1beta1.SchemeGroupVersion.WithKind("Project").GroupKind():
				if project != nil {
					return fmt.Errorf("found more than one *gardencorev1beta1.Project resource, but only one is allowed")
				}
				project = &gardencorev1beta1.Project{}
				if err := kubernetes.GardenScheme.Convert(obj, project, nil); err != nil {
					return fmt.Errorf("failed converting object with group kind %s to Project: %w", gk, err)
				}

			case gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot").GroupKind():
				if shoot != nil {
					return fmt.Errorf("found more than one *gardencorev1beta1.Shoot resource, but only one is allowed")
				}
				shoot = &gardencorev1beta1.Shoot{}
				if err := kubernetes.GardenScheme.Convert(obj, shoot, nil); err != nil {
					return fmt.Errorf("failed converting object with group kind %s to Shoot: %w", gk, err)
				}
			}
		}

		return nil
	}); err != nil {
		return nil, nil, nil, err
	}

	if cloudProfile == nil {
		return nil, nil, nil, fmt.Errorf("must provide a *gardencorev1beta1.CloudProfile resource but did not find any")
	}
	if project == nil {
		return nil, nil, nil, fmt.Errorf("must provide a *gardencorev1beta1.Project resource but did not find any")
	}
	if shoot == nil {
		return nil, nil, nil, fmt.Errorf("must provide a *gardencorev1beta1.Shoot resource but did not find any")
	}

	return cloudProfile, project, shoot, nil
}
