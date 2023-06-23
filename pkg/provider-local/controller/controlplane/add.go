// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controlplane

import (
	"context"
	"sync/atomic"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	"github.com/gardener/gardener/extensions/pkg/util"
	kubernetesclient "github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/provider-local/imagevector"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

// DefaultAddOptions are the default AddOptions for AddToManager.
var DefaultAddOptions = AddOptions{}

// AddOptions are options to apply when adding the local controlplane controller to the manager.
type AddOptions struct {
	// Controller are the controller.Options.
	Controller controller.Options
	// IgnoreOperationAnnotation specifies whether to ignore the operation annotation or not.
	IgnoreOperationAnnotation bool
	// ShootWebhookConfig specifies the desired Shoot MutatingWebhooksConfiguration.
	ShootWebhookConfig *atomic.Value
	// WebhookServerNamespace is the namespace in which the webhook server runs.
	WebhookServerNamespace string
}

// AddToManagerWithOptions adds a controller with the given Options to the given manager.
// The opts.Reconciler is being set with a newly instantiated actuator.
func AddToManagerWithOptions(ctx context.Context, mgr manager.Manager, opts AddOptions) error {
	gardenerClientset, err := kubernetesclient.NewWithConfig(kubernetesclient.WithRESTConfig(mgr.GetConfig()))
	if err != nil {
		return err
	}

	return controlplane.Add(ctx, mgr, controlplane.AddArgs{
		Actuator: genericactuator.NewActuator(mgr, local.Name, getSecretConfigs, nil, nil, nil, nil, nil, controlPlaneShootChart,
			nil, storageClassChart, nil, NewValuesProvider(), extensionscontroller.ChartRendererFactoryFunc(util.NewChartRendererForShoot),
			imagevector.ImageVector(), "", opts.ShootWebhookConfig, opts.WebhookServerNamespace, mgr.GetWebhookServer().(*webhook.DefaultServer).Options.Port, gardenerClientset),
		ControllerOptions: opts.Controller,
		Predicates:        controlplane.DefaultPredicates(ctx, mgr, opts.IgnoreOperationAnnotation),
		Type:              local.Type,
	})
}

// AddToManager adds a controller with the default Options.
func AddToManager(ctx context.Context, mgr manager.Manager) error {
	return AddToManagerWithOptions(ctx, mgr, DefaultAddOptions)
}
