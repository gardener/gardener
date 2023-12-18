// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package csinode

import (
	"golang.org/x/exp/maps"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	storagev1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
)

const (
	// WebhookName is the name of the webhook.
	WebhookName = "csinode"
)

// Args are the requirements to create a csinode webhook.
type Args struct {
	// Drivers is a list that maps a csidriver to the mutating function. CSIDrivers without an entry in this map will be ignored by the webhook.
	Drivers  map[string]CSINodeMutateFunc
	Provider string
}

// New creates a new cloudprovider webhook.
func New(mgr manager.Manager, args *Args) (*extensionswebhook.Webhook, error) {
	logger := mgr.GetLogger().WithName("csinode-webhook")

	logger.Info("Adding webhook to manager")
	logger.Info("Monitoring CSI Drivers to mutate", "driver-names", maps.Keys(args.Drivers))
	types := []extensionswebhook.Type{extensionswebhook.Type{
		Obj: &storagev1.CSINode{},
	}}

	handler, err := extensionswebhook.NewHandlerWithShootClient(mgr, types, NewMutator(mgr, logger, args), logger)
	if err != nil {
		return nil, err
	}

	wh := &extensionswebhook.Webhook{
		Name:          WebhookName,
		Path:          WebhookName,
		Target:        extensionswebhook.TargetShoot,
		Types:         types,
		Handler:       handler,
		FailurePolicy: toPtr(admissionregistrationv1.Fail),
	}

	return wh, nil
}

func toPtr[T any](t T) *T {
	return &t
}
