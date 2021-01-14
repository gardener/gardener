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

package cloudprovider

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

const (
	// WebhookName is the name of the webhook.
	WebhookName = "cloudprovider"
)

var (
	logger = log.Log.WithName("cloudprovider-webhook")
)

// Args are the requirements to create a cloudprovider webhook.
type Args struct {
	Provider string
	Mutator  extensionswebhook.Mutator
}

// New creates a new cloudprovider webhook.
func New(mgr manager.Manager, args Args) (*extensionswebhook.Webhook, error) {
	logger := logger.WithValues("cloud-provider", args.Provider)

	types := []client.Object{&corev1.Secret{}}
	handler, err := extensionswebhook.NewBuilder(mgr, logger).WithMutator(args.Mutator, types...).Build()
	if err != nil {
		return nil, err
	}

	namespaceSelector := buildSelector(args.Provider)
	logger.Info("Creating webhook")

	return &extensionswebhook.Webhook{
		Name:     WebhookName,
		Target:   extensionswebhook.TargetSeed,
		Provider: args.Provider,
		Types:    types,
		Webhook:  &admission.Webhook{Handler: handler},
		Path:     WebhookName,
		Selector: namespaceSelector,
	}, nil
}

func buildSelector(provider string) *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      v1beta1constants.LabelShootProvider,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{provider},
			},
		},
	}
}
