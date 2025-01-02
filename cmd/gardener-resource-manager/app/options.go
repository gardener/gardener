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
	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
	resourcemanagervalidation "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1/validation"
)

var configDecoder runtime.Decoder

func init() {
	configScheme := runtime.NewScheme()
	utilruntime.Must(resourcemanagerconfigv1alpha1.AddToScheme(configScheme))
	configDecoder = serializer.NewCodecFactory(configScheme).UniversalDecoder()
}

type options struct {
	configFile string
	config     *resourcemanagerconfigv1alpha1.ResourceManagerConfiguration
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

	o.config = &resourcemanagerconfigv1alpha1.ResourceManagerConfiguration{}
	if err = runtime.DecodeInto(configDecoder, data, o.config); err != nil {
		return fmt.Errorf("error decoding config: %w", err)
	}

	return nil
}

func (o *options) Validate() error {
	if errs := resourcemanagervalidation.ValidateResourceManagerConfiguration(o.config); len(errs) > 0 {
		return errs.ToAggregate()
	}
	return nil
}

func (o *options) LogConfig() (string, string) {
	return o.config.LogLevel, o.config.LogFormat
}
