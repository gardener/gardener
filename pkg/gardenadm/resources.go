// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// Resources contains the Kubernetes and Gardener resources read from the manifests.
type Resources struct {
	CloudProfile            *gardencorev1beta1.CloudProfile
	Project                 *gardencorev1beta1.Project
	Seed                    *gardencorev1beta1.Seed
	Shoot                   *gardencorev1beta1.Shoot
	ControllerRegistrations []*gardencorev1beta1.ControllerRegistration
	ControllerDeployments   []*gardencorev1.ControllerDeployment
	Secrets                 []*corev1.Secret
	SecretBinding           *gardencorev1beta1.SecretBinding
	CredentialsBinding      *securityv1alpha1.CredentialsBinding
}

// ReadManifests reads Kubernetes and Gardener manifests in YAML or JSON format.
// It returns among others a CloudProfile, Project, and Shoot resource if found, or an error if any issues occur during
// reading or decoding.
func ReadManifests(log logr.Logger, fsys fs.FS) (Resources, error) {
	resources := Resources{Seed: &gardencorev1beta1.Seed{}}
	decoder := serializer.NewCodecFactory(kubernetes.GardenScheme).UniversalDecoder(
		gardencorev1.SchemeGroupVersion,
		gardencorev1beta1.SchemeGroupVersion,
		securityv1alpha1.SchemeGroupVersion,
		corev1.SchemeGroupVersion,
	)

	if err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed walking directory: %w", err)
		}

		if d.IsDir() || !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") && !strings.HasSuffix(path, ".json") {
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
				if resources.CloudProfile != nil {
					return fmt.Errorf("found more than one *gardencorev1beta1.CloudProfile resource, but only one is allowed")
				}
				resources.CloudProfile = typedObj

			case *gardencorev1beta1.Project:
				if resources.Project != nil {
					return fmt.Errorf("found more than one *gardencorev1beta1.Project resource, but only one is allowed")
				}
				resources.Project = typedObj

			case *gardencorev1beta1.Shoot:
				if resources.Shoot != nil {
					return fmt.Errorf("found more than one *gardencorev1beta1.Shoot resource, but only one is allowed")
				}
				resources.Shoot = typedObj

			case *gardencorev1beta1.ControllerRegistration:
				resources.ControllerRegistrations = append(resources.ControllerRegistrations, typedObj)

			case *gardencorev1.ControllerDeployment:
				resources.ControllerDeployments = append(resources.ControllerDeployments, typedObj)

			case *corev1.Secret:
				resources.Secrets = append(resources.Secrets, typedObj)

			case *gardencorev1beta1.SecretBinding:
				if resources.SecretBinding != nil {
					return fmt.Errorf("found more than one *gardencorev1beta1.SecretBinding resource, but only one is allowed")
				}
				resources.SecretBinding = typedObj

			case *securityv1alpha1.CredentialsBinding:
				if resources.CredentialsBinding != nil {
					return fmt.Errorf("found more than one *securityv1alpha1.CredentialsBinding resource, but only one is allowed")
				}
				resources.CredentialsBinding = typedObj
			}
		}

		return nil
	}); err != nil {
		return Resources{}, fmt.Errorf("failed reading Kubernetes resources from config directory: %w", err)
	}

	if resources.CloudProfile == nil {
		return Resources{}, fmt.Errorf("must provide a *gardencorev1beta1.CloudProfile resource, but did not find any")
	}
	if resources.Project == nil {
		return Resources{}, fmt.Errorf("must provide a *gardencorev1beta1.Project resource, but did not find any")
	}
	if resources.Shoot == nil {
		return Resources{}, fmt.Errorf("must provide a *gardencorev1beta1.Shoot resource, but did not find any")
	}

	return resources, nil
}
