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

package controlplane

import (
	"fmt"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	// WebhookName is the webhook name.
	WebhookName = "controlplane"
	// ExposureWebhookName is the exposure webhook name.
	ExposureWebhookName = "controlplaneexposure"
	// BackupWebhookName is the backup webhook name.
	BackupWebhookName = "controlplanebackup"

	// KindSeed - A controlplane seed webhook is applied only to those shoot namespaces that have the correct Seed provider label.
	KindSeed = "seed"
	// KindShoot - A controlplane shoot webhook is applied only to those shoot namespaces that have the correct Shoot provider label.
	KindShoot = "shoot"
	// KindBackup - A controlplane backup webhook is applied only to those shoot namespaces that have the correct Backup provider label.
	KindBackup = "backup"
)

var logger = log.Log.WithName("controlplane-webhook")

// AddArgs are arguments for adding a controlplane webhook to a manager.
type AddArgs struct {
	// Kind is the kind of this webhook
	Kind string
	// Provider is the provider of this webhook.
	Provider string
	// Types is a list of resource types.
	Types []runtime.Object
	// Mutator is a mutator to be used by the admission handler.
	Mutator extensionswebhook.Mutator
}

// Add creates a new controlplane webhook and adds it to the given Manager.
func Add(mgr manager.Manager, args AddArgs) (*extensionswebhook.Webhook, error) {
	logger := logger.WithValues("kind", args.Kind, "provider", args.Provider)

	// Create handler
	handler, err := extensionswebhook.NewHandler(mgr, args.Types, args.Mutator, logger)
	if err != nil {
		return nil, err
	}

	// Create webhook
	logger.Info("Creating webhook", "name", getName(args.Kind))

	// Build namespace selector from the webhook kind and provider
	namespaceSelector, err := buildSelector(args.Kind, args.Provider)
	if err != nil {
		return nil, err
	}

	return &extensionswebhook.Webhook{
		Name:     getName(args.Kind),
		Kind:     args.Kind,
		Provider: args.Provider,
		Types:    args.Types,
		Target:   extensionswebhook.TargetSeed,
		Path:     getName(args.Kind),
		Webhook:  &admission.Webhook{Handler: handler},
		Selector: namespaceSelector,
	}, nil
}

func getName(kind string) string {
	switch kind {
	case KindSeed:
		return ExposureWebhookName
	case KindBackup:
		return BackupWebhookName
	default:
		return WebhookName
	}
}

// buildSelector creates and returns a LabelSelector for the given webhook kind and provider.
func buildSelector(kind, provider string) (*metav1.LabelSelector, error) {
	// Determine label selector key from the kind
	var key string
	switch kind {
	case KindSeed:
		key = v1beta1constants.LabelSeedProvider
	case KindShoot:
		key = v1beta1constants.LabelShootProvider
	case KindBackup:
		key = v1beta1constants.LabelBackupProvider
	default:
		return nil, fmt.Errorf("invalid webhook kind '%s'", kind)
	}

	// Create and return LabelSelector
	return &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: key, Operator: metav1.LabelSelectorOpIn, Values: []string{provider}},
		},
	}, nil
}
