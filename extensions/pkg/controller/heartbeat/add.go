// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package heartbeat

import (
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// DefaultAddOptions are the default AddOptions for AddToManager.
var DefaultAddOptions = AddOptions{
	RenewIntervalSeconds: 30,
}

// AddOptions are options to apply when adding the heartbeat controller to the manager.
type AddOptions struct {
	// ControllerOptions are the controller.Options.
	ControllerOptions controller.Options
	// ExtensionName is the name of the extension that this heartbeat controller is part of.
	ExtensionName string
	// Namespace is the namespace which will be used for the heart beat lease resource.
	Namespace string
	// RenewIntervalSeconds defines how often the heart beat lease is renewed.
	RenewIntervalSeconds int32
}

// AddToManager adds the heartbeat controller with the default Options to the manager.
func AddToManager(mgr manager.Manager) error {
	return Add(mgr, AddArgs{
		ControllerOptions:    DefaultAddOptions.ControllerOptions,
		ExtensionName:        DefaultAddOptions.ExtensionName,
		Namespace:            DefaultAddOptions.Namespace,
		RenewIntervalSeconds: DefaultAddOptions.RenewIntervalSeconds,
		Clock:                clock.RealClock{},
	})
}
