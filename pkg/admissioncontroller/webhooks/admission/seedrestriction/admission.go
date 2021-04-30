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

package seedrestriction

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gardener/gardener/pkg/admissioncontroller/seedidentity"
	acadmission "github.com/gardener/gardener/pkg/admissioncontroller/webhooks/admission"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	// HandlerName is the name of this admission webhook handler.
	HandlerName = "seedrestriction"
	// WebhookPath is the HTTP handler path for this admission webhook handler.
	WebhookPath = "/webhooks/admission/seedrestriction"
)

var (
	// Only take v1beta1 for the core.gardener.cloud API group because the Authorize function only checks the resource
	// group and the resource (but it ignores the version).
	backupBucketResource = gardencorev1beta1.Resource("backupbuckets")
	backupEntryResource  = gardencorev1beta1.Resource("backupentries")
	leaseResource        = coordinationv1.Resource("leases")
	shootStateResource   = gardencorev1beta1.Resource("shootstates")
)

// New creates a new webhook handler restricting requests by gardenlets. It allows all requests.
func New(ctx context.Context, logger logr.Logger, cache cache.Cache) (*handler, error) {
	// Initialize caches here to ensure the readyz informer check will only succeed once informers required for this
	// handler have synced so that http requests can be served quicker with pre-syncronized caches.
	if _, err := cache.GetInformer(ctx, &gardencorev1beta1.BackupBucket{}); err != nil {
		return nil, err
	}
	if _, err := cache.GetInformer(ctx, &gardencorev1beta1.Shoot{}); err != nil {
		return nil, err
	}

	return &handler{
		logger:      logger,
		cacheReader: cache,
	}, nil
}

type handler struct {
	logger      logr.Logger
	cacheReader client.Reader
	decoder     *admission.Decoder
}

var _ admission.Handler = &handler{}

func (h *handler) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}

func (h *handler) Handle(ctx context.Context, request admission.Request) admission.Response {
	seedName, isSeed := seedidentity.FromAuthenticationV1UserInfo(request.UserInfo)
	if !isSeed {
		return acadmission.Allowed("")
	}

	requestResource := schema.GroupResource{Group: request.Resource.Group, Resource: request.Resource.Resource}
	switch requestResource {
	case backupBucketResource:
		return h.admitBackupBucket(seedName, request)
	case backupEntryResource:
		return h.admitBackupEntry(ctx, seedName, request)
	case leaseResource:
		return h.admitLease(seedName, request)
	case shootStateResource:
		return h.admitShootState(ctx, seedName, request)
	}

	return acadmission.Allowed("")
}

func (h *handler) admitBackupBucket(seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	backupBucket := &gardencorev1beta1.BackupBucket{}
	if err := h.decoder.Decode(request, backupBucket); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	return h.admit(seedName, backupBucket.Spec.SeedName)
}

func (h *handler) admitBackupEntry(ctx context.Context, seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	backupEntry := &gardencorev1beta1.BackupEntry{}
	if err := h.decoder.Decode(request, backupEntry); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if resp := h.admit(seedName, backupEntry.Spec.SeedName); !resp.Allowed {
		return resp
	}

	backupBucket := &gardencorev1beta1.BackupBucket{}
	if err := h.cacheReader.Get(ctx, kutil.Key(backupEntry.Spec.BucketName), backupBucket); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return h.admit(seedName, backupBucket.Spec.SeedName)
}

func (h *handler) admitLease(seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	if request.Name == "gardenlet-leader-election" {
		return admission.Allowed("")
	}

	return h.admit(seedName, &request.Name)
}

func (h *handler) admitShootState(ctx context.Context, seedName string, request admission.Request) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected operation: %q", request.Operation))
	}

	shoot := &gardencorev1beta1.Shoot{}
	if err := h.cacheReader.Get(ctx, kutil.Key(request.Namespace, request.Name), shoot); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return h.admit(seedName, shoot.Spec.SeedName)
}

func (h *handler) admit(seedName string, seedNameForObject *string) admission.Response {
	// Allow request if seed name is not known (ambiguous case).
	if seedName == "" {
		return admission.Allowed("")
	}

	// Allow request if seed name of object matches the seed name of the requesting user.
	if seedNameForObject != nil && *seedNameForObject == seedName {
		return admission.Allowed("")
	}

	return admission.Errored(http.StatusForbidden, fmt.Errorf("object does not belong to seed %q", seedName))
}
