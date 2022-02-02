// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

// Disclaimer: This controller is here only temporarily until the gardenlet no longer creates the `dnsv1alpha1.DNSProvider`
// resources.

package dnsprovider

import (
	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ControllerName is the name of the controller.
const ControllerName = "dnsprovider_controller"

// DefaultAddOptions are the default AddOptions for AddToManager.
var DefaultAddOptions = AddOptions{
	Controller: controller.Options{
		RecoverPanic: true,
	},
}

// AddOptions are options to apply when adding the local infrastructure controller to the manager.
type AddOptions struct {
	// Controller are the controller.Options.
	Controller controller.Options
}

// AddToManagerWithOptions adds a controller with the given Options to the given manager.
// The opts.Reconciler is being set with a newly instantiated actuator.
func AddToManagerWithOptions(mgr manager.Manager, opts AddOptions) error {
	opts.Controller.Reconciler = NewReconciler()

	ctrl, err := controller.New(ControllerName, mgr, opts.Controller)
	if err != nil {
		return err
	}

	return ctrl.Watch(&source.Kind{Type: &dnsv1alpha1.DNSProvider{}}, &handler.EnqueueRequestForObject{})
}

// AddToManager adds a controller with the default Options.
func AddToManager(mgr manager.Manager) error {
	return AddToManagerWithOptions(mgr, DefaultAddOptions)
}
