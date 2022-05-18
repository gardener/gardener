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

package node

import (
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/provider-local/local"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// WebhookName is the name of the node webhook.
const WebhookName = "node"

var (
	logger = log.Log.WithName("local-node-webhook")

	// DefaultAddOptions are the default AddOptions for AddToManager.
	DefaultAddOptions = AddOptions{}
)

// AddOptions are options to apply when adding the local exposure webhook to the manager.
type AddOptions struct{}

// AddToManagerWithOptions creates a webhook with the given options and adds it to the manager.
func AddToManagerWithOptions(mgr manager.Manager, opts AddOptions) (*extensionswebhook.Webhook, error) {
	logger.Info("Adding webhook to manager")

	var (
		name          = "node"
		provider      = local.Type
		types         = []extensionswebhook.Type{{Obj: &corev1.Node{}, Subresource: pointer.String("status")}}
		failurePolicy = admissionregistrationv1.Fail
	)

	logger = logger.WithValues("provider", provider)

	handler, err := extensionswebhook.NewBuilder(mgr, logger).WithMutator(&mutator{}, types...).Build()
	if err != nil {
		return nil, err
	}

	logger.Info("Creating webhook", "name", name)

	return &extensionswebhook.Webhook{
		Name:           name,
		Provider:       provider,
		Types:          types,
		Target:         extensionswebhook.TargetSeed,
		Path:           name,
		Webhook:        &admission.Webhook{Handler: handler},
		ObjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/hostname": "gardener-local-control-plane"}},
		FailurePolicy:  &failurePolicy,
		TimeoutSeconds: pointer.Int32(1),
	}, nil
}

// AddToManager creates a webhook with the default options and adds it to the manager.
func AddToManager(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	return AddToManagerWithOptions(mgr, DefaultAddOptions)
}
