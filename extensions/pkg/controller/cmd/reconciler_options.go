// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/pflag"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

const (
	// IgnoreOperationAnnotationFlag is the name of the command line flag to specify whether the operation annotation
	// is ignored or not.
	IgnoreOperationAnnotationFlag = "ignore-operation-annotation"
	// ExtensionClassFlag is the name of the extension class this extension is responsible for.
	ExtensionClassFlag = "extension-class"
)

// ReconcilerOptions are command line options that can be set for controller.Options.
type ReconcilerOptions struct {
	// IgnoreOperationAnnotation defines whether to ignore the operation annotation or not.
	IgnoreOperationAnnotation bool
	// ExtensionClass defines the extension class this extension is responsible for.
	ExtensionClass string

	config *ReconcilerConfig
}

// AddFlags implements Flagger.AddFlags.
func (c *ReconcilerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&c.IgnoreOperationAnnotation, IgnoreOperationAnnotationFlag, c.IgnoreOperationAnnotation, "Ignore the operation annotation or not.")
	fs.StringVar(&c.ExtensionClass, ExtensionClassFlag, "", "Extension class this extension is responsible for.")
}

// Complete implements Completer.Complete.
func (c *ReconcilerOptions) Complete() error {
	c.config = &ReconcilerConfig{
		IgnoreOperationAnnotation: c.IgnoreOperationAnnotation,
		ExtensionClass:            (extensionsv1alpha1.ExtensionClass)(c.ExtensionClass),
	}
	return nil
}

// Completed returns the completed ReconcilerConfig. Only call this if `Complete` was successful.
func (c *ReconcilerOptions) Completed() *ReconcilerConfig {
	return c.config
}

// ReconcilerConfig is a completed controller configuration.
type ReconcilerConfig struct {
	// IgnoreOperationAnnotation defines whether to ignore the operation annotation or not.
	IgnoreOperationAnnotation bool
	// ExtensionClass defines the extension class this extension is responsible for.
	ExtensionClass extensionsv1alpha1.ExtensionClass
}

// Apply sets the values of this ReconcilerConfig in the given controller.Options.
func (c *ReconcilerConfig) Apply(ignore *bool, class *extensionsv1alpha1.ExtensionClass) {
	if ignore != nil {
		*ignore = c.IgnoreOperationAnnotation
	}
	if class != nil {
		*class = c.ExtensionClass
	}
}
