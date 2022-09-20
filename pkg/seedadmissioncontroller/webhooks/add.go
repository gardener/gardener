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

package webhooks

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/seedadmissioncontroller/webhooks/admission/extensioncrds"
	"github.com/gardener/gardener/pkg/seedadmissioncontroller/webhooks/admission/extensionresources"
	"github.com/gardener/gardener/pkg/seedadmissioncontroller/webhooks/admission/podschedulername"
)

// AddWebhookHandlersToManager adds all webhook handlers to the given manager.
func AddWebhookHandlersToManager(mgr manager.Manager) error {
	var (
		log    = mgr.GetLogger().WithName("webhook")
		server = mgr.GetWebhookServer()
	)

	if err := extensionresources.AddWebhooks(mgr); err != nil {
		return err
	}

	server.Register(extensioncrds.WebhookPath, &webhook.Admission{Handler: extensioncrds.New(log.WithName(extensioncrds.HandlerName)), RecoverPanic: true})
	server.Register(podschedulername.WebhookPath, &webhook.Admission{Handler: admission.HandlerFunc(podschedulername.DefaultShootControlPlanePodsSchedulerName), RecoverPanic: true})

	return nil
}
