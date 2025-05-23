// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/provider-local/apis/local/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

// NewNamespacedCloudProfileMutator returns a new instance of a NamespacedCloudProfile mutator.
func NewNamespacedCloudProfileMutator(mgr manager.Manager) extensionswebhook.Mutator {
	return &namespacedCloudProfile{
		client:  mgr.GetClient(),
		decoder: serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
	}
}

type namespacedCloudProfile struct {
	client  client.Client
	decoder runtime.Decoder
}

// Mutate mutates the given NamespacedCloudProfile object.
func (p *namespacedCloudProfile) Mutate(_ context.Context, newObj, _ client.Object) error {
	profile, ok := newObj.(*gardencorev1beta1.NamespacedCloudProfile)
	if !ok {
		return fmt.Errorf("wrong object type %T", newObj)
	}

	// Ignore NamespacedCloudProfiles being deleted and wait for core mutator to patch the status.
	if profile.DeletionTimestamp != nil || profile.Generation != profile.Status.ObservedGeneration ||
		profile.Spec.ProviderConfig == nil || profile.Status.CloudProfileSpec.ProviderConfig == nil {
		return nil
	}

	specConfig := &v1alpha1.CloudProfileConfig{}
	if _, _, err := p.decoder.Decode(profile.Spec.ProviderConfig.Raw, nil, specConfig); err != nil {
		return fmt.Errorf("could not decode providerConfig of namespacedCloudProfile spec for '%s': %w", profile.Name, err)
	}
	statusConfig := &v1alpha1.CloudProfileConfig{}
	if _, _, err := p.decoder.Decode(profile.Status.CloudProfileSpec.ProviderConfig.Raw, nil, statusConfig); err != nil {
		return fmt.Errorf("could not decode providerConfig of namespacedCloudProfile status for '%s': %w", profile.Name, err)
	}

	statusConfig.MachineImages = mergeMachineImages(specConfig.MachineImages, statusConfig.MachineImages)

	modifiedStatusConfig, err := json.Marshal(statusConfig)
	if err != nil {
		return err
	}
	profile.Status.CloudProfileSpec.ProviderConfig.Raw = modifiedStatusConfig

	return nil
}

func mergeMachineImages(specMachineImages, statusMachineImages []v1alpha1.MachineImages) []v1alpha1.MachineImages {
	specImages := utils.CreateMapFromSlice(specMachineImages, func(mi v1alpha1.MachineImages) string { return mi.Name })
	statusImages := utils.CreateMapFromSlice(statusMachineImages, func(mi v1alpha1.MachineImages) string { return mi.Name })
	for _, specMachineImage := range specImages {
		if _, exists := statusImages[specMachineImage.Name]; !exists {
			statusImages[specMachineImage.Name] = specMachineImage
		} else {
			statusImageVersions := utils.CreateMapFromSlice(statusImages[specMachineImage.Name].Versions, func(v v1alpha1.MachineImageVersion) string { return v.Version })
			specImageVersions := utils.CreateMapFromSlice(specImages[specMachineImage.Name].Versions, func(v v1alpha1.MachineImageVersion) string { return v.Version })
			for _, version := range specImageVersions {
				statusImageVersions[version.Version] = version
			}

			statusImages[specMachineImage.Name] = v1alpha1.MachineImages{
				Name:     specMachineImage.Name,
				Versions: slices.Collect(maps.Values(statusImageVersions)),
			}
		}
	}
	return slices.Collect(maps.Values(statusImages))
}
