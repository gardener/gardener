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

package seedadmission

const (
	// GardenerShootControlPlaneSchedulerName is the name of the scheduler used for
	// shoot control plane pods.
	GardenerShootControlPlaneSchedulerName = "gardener-shoot-controlplane-scheduler"
	// GardenerShootControlPlaneSchedulerWebhookPath is the path of the webhook server
	// that sets the "gardener-shoot-controlplane-scheduler" schedulerName for Pods.
	GardenerShootControlPlaneSchedulerWebhookPath = "/webhooks/default-pod-scheduler-name/" + GardenerShootControlPlaneSchedulerName
)
