// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/extensions/pkg/util"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	api "github.com/gardener/gardener/pkg/provider-local/apis/local"
	validation "github.com/gardener/gardener/pkg/provider-local/apis/local/validation"
)

// NewCloudProfileValidator returns a new instance of a cloud profile validator.
func NewCloudProfileValidator(mgr manager.Manager) extensionswebhook.Validator {
	return &cloudProfile{
		decoder: serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
	}
}

type cloudProfile struct {
	decoder runtime.Decoder
}

// Validate validates the given cloud profile objects.
func (cp *cloudProfile) Validate(_ context.Context, newObj, _ client.Object) error {
	cloudProfile, ok := newObj.(*core.CloudProfile)
	if !ok {
		return fmt.Errorf("wrong object type %T", newObj)
	}

	providerConfigPath := field.NewPath("spec").Child("providerConfig")
	if cloudProfile.Spec.ProviderConfig == nil {
		return field.Required(providerConfigPath, "providerConfig must be set for local cloud profiles")
	}

	cpConfig, err := decodeCloudProfileConfig(cp.decoder, cloudProfile.Spec.ProviderConfig)
	if err != nil {
		return err
	}

	return validation.ValidateCloudProfileConfig(cpConfig, cloudProfile.Spec.MachineImages, cloudProfile.Spec.Capabilities, providerConfigPath).ToAggregate()
}

func decodeCloudProfileConfig(decoder runtime.Decoder, config *runtime.RawExtension) (*api.CloudProfileConfig, error) {
	cloudProfileConfig := &api.CloudProfileConfig{}
	if err := util.Decode(decoder, config.Raw, cloudProfileConfig); err != nil {
		return nil, err
	}
	return cloudProfileConfig, nil
}
