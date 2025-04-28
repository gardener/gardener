// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardener/cmd/utils/initrun"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/features"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gardenletvalidation "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1/validation"
)

var configDecoder runtime.Decoder

func init() {
	configScheme := runtime.NewScheme()
	schemeBuilder := runtime.NewSchemeBuilder(
		gardenletconfigv1alpha1.AddToScheme,
		gardencore.AddToScheme,
		gardencorev1beta1.AddToScheme,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(configScheme))
	configDecoder = serializer.NewCodecFactory(configScheme).UniversalDecoder()
}

type options struct {
	configFile string
	config     *gardenletconfigv1alpha1.GardenletConfiguration
}

var _ initrun.Options = &options{}

func (o *options) addFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.configFile, "config", o.configFile, "Path to configuration file.")
}

func (o *options) Complete() error {
	if len(o.configFile) == 0 {
		return fmt.Errorf("missing config file")
	}

	data, err := os.ReadFile(o.configFile)
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	o.config = &gardenletconfigv1alpha1.GardenletConfiguration{}
	if err = runtime.DecodeInto(configDecoder, data, o.config); err != nil {
		return fmt.Errorf("error decoding config: %w", err)
	}

	// Set feature gates immediately after decoding the config.
	// Feature gates might influence the next steps, e.g., validating the config.
	return features.DefaultFeatureGate.SetFromMap(o.config.FeatureGates)
}

func (o *options) Validate() error {
	// TODO(vpnachev): Remove once the backup.secretRef field is removed.
	syncBackupSecretRefAndCredentialsRef(o.config.SeedConfig.Spec.Backup)

	if errs := gardenletvalidation.ValidateGardenletConfiguration(o.config, nil, false); len(errs) > 0 {
		return errs.ToAggregate()
	}
	return nil
}

func (o *options) LogConfig() (string, string) {
	return o.config.LogLevel, o.config.LogFormat
}
