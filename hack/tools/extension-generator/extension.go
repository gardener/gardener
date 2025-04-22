// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

// GenerateExtension generates the extension in YAML format.
func GenerateExtension(opts *Options) ([]byte, error) {
	extension := newBaseExtension(opts)

	if opts.AdmissionRuntimeOCIRepository != "" && opts.AdmissionApplicationOCIRepository != "" {
		ensureAdmission(extension, opts)
	}

	if err := ensureResources(extension, opts); err != nil {
		return nil, err
	}

	yamlBytes, err := yaml.Marshal(extension)
	if err != nil {
		return nil, err
	}

	// Remove known empty values as a workaround for https://github.com/kubernetes/kubernetes/issues/67610.
	yamlBytes = bytes.Replace(yamlBytes, []byte("  creationTimestamp: null\n"), []byte(""), -1)
	yamlBytes = bytes.Replace(yamlBytes, []byte("status: {}\n"), []byte(""), -1)

	return yamlBytes, nil
}

func newBaseExtension(opts *Options) *operatorv1alpha1.Extension {
	return &operatorv1alpha1.Extension{
		TypeMeta: metav1.TypeMeta{
			APIVersion: operatorv1alpha1.SchemeGroupVersion.String(),
			Kind:       "Extension",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: opts.ExtensionName,
		},
		Spec: operatorv1alpha1.ExtensionSpec{
			Deployment: &operatorv1alpha1.Deployment{
				ExtensionDeployment: &operatorv1alpha1.ExtensionDeploymentSpec{
					DeploymentSpec: operatorv1alpha1.DeploymentSpec{
						Helm: &operatorv1alpha1.ExtensionHelm{
							OCIRepository: &gardencorev1.OCIRepository{
								Ref: &opts.ExtensionOCIRepository,
							},
						},
					},
				},
			},
		},
	}
}

func ensureAdmission(extension *operatorv1alpha1.Extension, opts *Options) {
	extension.Spec.Deployment.AdmissionDeployment = &operatorv1alpha1.AdmissionDeploymentSpec{
		RuntimeCluster: &operatorv1alpha1.DeploymentSpec{
			Helm: &operatorv1alpha1.ExtensionHelm{
				OCIRepository: &gardencorev1.OCIRepository{
					Ref: &opts.AdmissionRuntimeOCIRepository,
				},
			},
		},
		VirtualCluster: &operatorv1alpha1.DeploymentSpec{
			Helm: &operatorv1alpha1.ExtensionHelm{
				OCIRepository: &gardencorev1.OCIRepository{
					Ref: &opts.AdmissionApplicationOCIRepository,
				},
			},
		},
	}
}

func ensureResources(extension *operatorv1alpha1.Extension, opts *Options) error {
	for _, category := range opts.ComponentCategories {
		fn, ok := categoryToEnsurer[category]
		if !ok {
			return fmt.Errorf("unsupported component category: %s", category)
		}

		fn(extension, opts)
	}
	return nil
}

var categoryToEnsurer = map[string]func(extension *operatorv1alpha1.Extension, opts *Options){
	"container-runtime":  ensureContainerRuntimeExtension,
	"extension":          ensureGenericExtension,
	"network":            ensureNetworkExtension,
	"operating-system":   ensureOperatingSystemExtension,
	"provider-extension": ensureProviderExtension,
}

func ensureContainerRuntimeExtension(extension *operatorv1alpha1.Extension, opts *Options) {
	extension.Spec.Resources = append(extension.Spec.Resources,
		gardencorev1beta1.ControllerResource{Kind: extensionsv1alpha1.ContainerRuntimeResource, Type: opts.ProviderType},
	)
}

func ensureGenericExtension(extension *operatorv1alpha1.Extension, opts *Options) {
	extension.Spec.Resources = append(extension.Spec.Resources,
		gardencorev1beta1.ControllerResource{Kind: extensionsv1alpha1.ExtensionResource, Type: opts.ProviderType},
	)
}

func ensureNetworkExtension(extension *operatorv1alpha1.Extension, opts *Options) {
	extension.Spec.Resources = append(extension.Spec.Resources,
		gardencorev1beta1.ControllerResource{Kind: extensionsv1alpha1.NetworkResource, Type: opts.ProviderType},
	)
}

func ensureOperatingSystemExtension(extension *operatorv1alpha1.Extension, opts *Options) {
	extension.Spec.Resources = append(extension.Spec.Resources,
		gardencorev1beta1.ControllerResource{Kind: extensionsv1alpha1.OperatingSystemConfigResource, Type: opts.ProviderType},
	)
}

func ensureProviderExtension(extension *operatorv1alpha1.Extension, opts *Options) {
	extension.Spec.Resources = append(extension.Spec.Resources,
		gardencorev1beta1.ControllerResource{Kind: extensionsv1alpha1.BackupBucketResource, Type: opts.ProviderType},
		gardencorev1beta1.ControllerResource{Kind: extensionsv1alpha1.BackupEntryResource, Type: opts.ProviderType},
		gardencorev1beta1.ControllerResource{Kind: extensionsv1alpha1.BastionResource, Type: opts.ProviderType},
		gardencorev1beta1.ControllerResource{Kind: extensionsv1alpha1.ControlPlaneResource, Type: opts.ProviderType},
		gardencorev1beta1.ControllerResource{Kind: extensionsv1alpha1.DNSRecordResource, Type: opts.ProviderType},
		gardencorev1beta1.ControllerResource{Kind: extensionsv1alpha1.InfrastructureResource, Type: opts.ProviderType},
		gardencorev1beta1.ControllerResource{Kind: extensionsv1alpha1.WorkerResource, Type: opts.ProviderType},
	)
	extension.Spec.Deployment.ExtensionDeployment.InjectGardenKubeconfig = ptr.To(true)
}
