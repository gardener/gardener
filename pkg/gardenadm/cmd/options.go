// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/klog/v2"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/gardener/gardener/pkg/logger"
)

// LogfSetLogger is an alias for logf.SetLogger for testing purposes.
// logf.SetLogger discards all calls after the first invocation and there is no way of telling from the outside whether
// it was called at all. Without a func alias, we cannot verify that Options.Complete correctly initialized the global
// controller-runtime logger.
var LogfSetLogger = logf.SetLogger

// Options contains persistent options for all commands.
type Options struct {
	genericiooptions.IOStreams

	// Log is the global logger.
	Log logr.Logger
	// LogLevel is the log level (one of [info,debug,error]).
	LogLevel string
	// LogFormat is the log format (one of [json,text]).
	LogFormat string
}

// Validate validates the options.
func (o *Options) Validate() error {
	if !sets.New(logger.AllLogLevels...).Has(o.LogLevel) {
		return fmt.Errorf("log-level must be one of %v", logger.AllLogLevels)
	}

	if !sets.New(logger.AllLogFormats...).Has(o.LogFormat) {
		return fmt.Errorf("log-format must be one of %v", logger.AllLogFormats)
	}

	return nil
}

// Complete completes the options.
func (o *Options) Complete() error {
	var err error
	o.Log, err = logger.NewZapLogger(o.LogLevel, o.LogFormat, logzap.WriteTo(o.ErrOut))
	if err != nil {
		return fmt.Errorf("error instantiating zap logger: %w", err)
	}

	LogfSetLogger(o.Log)
	klog.SetLogger(o.Log)
	return nil
}

// AddFlags adds the flags to the flag set.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVarP(&o.LogLevel, "log-level", "", "info", fmt.Sprintf("The level/severity for the logs. Must be one of %v", logger.AllLogLevels))
	fs.StringVarP(&o.LogFormat, "log-format", "", "text", fmt.Sprintf("The format for the logs. Must be one of %v", logger.AllLogFormats))
}
