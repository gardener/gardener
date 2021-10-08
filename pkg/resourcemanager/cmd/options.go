// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Option is a compound interface type for an option for the resource-manager.
type Option interface {
	Flagger
	Completer
}

// Flagger can add needed flags to a FlagSet.
type Flagger interface {
	AddFlags(*pflag.FlagSet)
}

// Completer can complete some configuration options.
type Completer interface {
	Complete() error
}

// AddAllFlags adds all Flaggers to the given FlagSet.
func AddAllFlags(fs *pflag.FlagSet, f ...Flagger) {
	for _, flagger := range f {
		flagger.AddFlags(fs)
	}
}

// AddToMangerFunc is a function which will add a controller or something similar to a Manager.
type AddToMangerFunc func(manager.Manager) error

// AddAllToManager calls all AddToManagerFuncs.
func AddAllToManager(mgr manager.Manager, all ...AddToMangerFunc) error {
	for _, add := range all {
		if err := add(mgr); err != nil {
			return err
		}
	}
	return nil
}

// CompleteAll completes all Completers.
func CompleteAll(c ...Completer) error {
	for _, completer := range c {
		if err := completer.Complete(); err != nil {
			return err
		}
	}
	return nil
}
