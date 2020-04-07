// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"github.com/spf13/pflag"
)

const (
	// IgnoreOperationAnnotationFlag is the name of the command line flag to specify whether the operation annotation
	// is ignored or not.
	IgnoreOperationAnnotationFlag = "ignore-operation-annotation"
)

// ReconcilerOptions are command line options that can be set for controller.Options.
type ReconcilerOptions struct {
	// IgnoreOperationAnnotation defines whether to ignore the operation annotation or not.
	IgnoreOperationAnnotation bool

	config *ReconcilerConfig
}

// AddFlags implements Flagger.AddFlags.
func (c *ReconcilerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&c.IgnoreOperationAnnotation, IgnoreOperationAnnotationFlag, c.IgnoreOperationAnnotation, "Ignore the operation annotation or not.")
}

// Complete implements Completer.Complete.
func (c *ReconcilerOptions) Complete() error {
	c.config = &ReconcilerConfig{c.IgnoreOperationAnnotation}
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
}

// Apply sets the values of this ReconcilerConfig in the given controller.Options.
func (c *ReconcilerConfig) Apply(ignore *bool) {
	*ignore = c.IgnoreOperationAnnotation
}
