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

package shoot

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

const (
	// WebhookName is the name of the shoot webhook.
	WebhookName = "shoot"
	// KindSystem is used for webhooks which should only apply to the to the kube-system namespace.
	KindSystem = "system"
)

var logger = log.Log.WithName("shoot-webhook")

// Args are arguments for creating a webhook targeting a shoot.
type Args struct {
	// Types is a list of resource types.
	Types []extensionswebhook.Type
	// Mutator is a mutator to be used by the admission handler. It doesn't need the shoot client.
	Mutator extensionswebhook.Mutator
	// MutatorWithShootClient is a mutator to be used by the admission handler. It needs the shoot client.
	MutatorWithShootClient extensionswebhook.MutatorWithShootClient
}

// New creates a new webhook with the shoot as target cluster.
func New(mgr manager.Manager, args Args) (*extensionswebhook.Webhook, error) {
	logger.Info("Creating webhook", "name", WebhookName)

	// Build namespace selector from the webhook kind and provider
	namespaceSelector, err := buildSelector()
	if err != nil {
		return nil, err
	}

	wh := &extensionswebhook.Webhook{
		Name:     WebhookName,
		Types:    args.Types,
		Path:     WebhookName,
		Target:   extensionswebhook.TargetShoot,
		Selector: namespaceSelector,
	}

	switch {
	case args.Mutator != nil:
		handler, err := extensionswebhook.NewBuilder(mgr, logger).WithMutator(args.Mutator, args.Types...).Build()
		if err != nil {
			return nil, err
		}

		wh.Webhook = &admission.Webhook{Handler: handler}
		return wh, nil

	case args.MutatorWithShootClient != nil:
		handler, err := extensionswebhook.NewHandlerWithShootClient(mgr, args.Types, args.MutatorWithShootClient, logger)
		if err != nil {
			return nil, err
		}

		wh.Handler = handler
		return wh, nil
	}

	return nil, fmt.Errorf("neither mutator nor mutator with shoot client is set")
}

// buildSelector creates and returns a LabelSelector for the given webhook kind and provider.
func buildSelector() (*metav1.LabelSelector, error) {
	// Create and return LabelSelector
	return &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: v1beta1constants.GardenerPurpose, Operator: metav1.LabelSelectorOpIn, Values: []string{metav1.NamespaceSystem}},
		},
	}, nil
}
