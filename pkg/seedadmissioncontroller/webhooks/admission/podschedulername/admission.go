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

package podschedulername

import (
	"context"

	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	// GardenerShootControlPlaneSchedulerName is the name of the scheduler used for
	// shoot control plane pods.
	GardenerShootControlPlaneSchedulerName = "gardener-shoot-controlplane-scheduler"

	// WebhookPath is the HTTP handler path for this admission webhook handler.
	// Note: In the future we might want to have additional scheduler names
	// so lets have the handler be of pattern "/webhooks/default-pod-scheduler-name/{scheduler-name}"
	WebhookPath = "/webhooks/default-pod-scheduler-name/" + GardenerShootControlPlaneSchedulerName
)

var podGVK = metav1.GroupVersionKind{Group: "", Kind: "Pod", Version: "v1"}

var _ = admission.HandlerFunc(DefaultShootControlPlanePodsSchedulerName)

// DefaultShootControlPlanePodsSchedulerName is a webhook handler that sets "gardener-shoot-controlplane-scheduler"
// as schedulerName on shoot control plane Pods.
func DefaultShootControlPlanePodsSchedulerName(_ context.Context, request admission.Request) admission.Response {
	// If the request does not indicate the correct operation (CREATE) we allow the review without further doing.
	if request.Operation != admissionv1.Create {
		return admission.Allowed("operation is not CREATE")
	}

	if request.Kind != podGVK {
		return admission.Allowed("resource is not corev1.Pod")
	}

	if request.SubResource != "" {
		return admission.Allowed("subresources on pods are not supported")
	}

	return admission.Patched(
		"shoot control plane pod created",
		jsonpatch.NewOperation("replace", "/spec/schedulerName", GardenerShootControlPlaneSchedulerName),
	)
}
