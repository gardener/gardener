// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	"github.com/gardener/gardener/pkg/features"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	nodeagenthelper "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1/helper"
	nodeagentvalidation "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1/validation"
)

var configDecoder runtime.Decoder

func init() {
	configScheme := runtime.NewScheme()
	utilruntime.Must(nodeagentconfigv1alpha1.AddToScheme(configScheme))
	configDecoder = serializer.NewCodecFactory(configScheme).UniversalDecoder()
}

type options struct {
	configDir string
	config    *nodeagentconfigv1alpha1.NodeAgentConfiguration
}

var _ initrun.Options = &options{}

func (o *options) addFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.configDir, "config-dir", o.configDir, "Path to the directory containing the configuration file.")
}

func (o *options) Complete() error {
	if len(o.configDir) == 0 {
		return fmt.Errorf("missing config dir")
	}

	data, err := os.ReadFile(nodeagenthelper.GetConfigFilePath(o.configDir))
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	o.config = &nodeagentconfigv1alpha1.NodeAgentConfiguration{}
	if err = runtime.DecodeInto(configDecoder, data, o.config); err != nil {
		return fmt.Errorf("error decoding config: %w", err)
	}

	// Set feature gates immediately after decoding the config.
	// Feature gates might influence the next steps, e.g., validating the config.
	return features.DefaultFeatureGate.SetFromMap(o.config.FeatureGates)
}

func (o *options) Validate() error {
	if errs := nodeagentvalidation.ValidateNodeAgentConfiguration(o.config); len(errs) > 0 {
		return errs.ToAggregate()
	}
	return nil
}

func (o *options) LogConfig() (string, string) {
	return o.config.LogLevel, o.config.LogFormat
}
