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

package extensionresources

import (
	"context"
	"fmt"
	"net/http"
	"time"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	druidvalidation "github.com/gardener/etcd-druid/api/validation"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/extensions/validation"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	// WebhookPath is the HTTP handler path for this admission webhook handler.
	WebhookPath = "/webhooks/validate-extension-resources"

	// HandlerName is the name of this admission webhook handler.
	HandlerName = "extension_resources"
)

// New creates a new webhook handler validating CREATE and UPDATE requests for extension resources.
func New(logger logr.Logger, allowInvalidExtensionResources bool) *handler {
	artifacts := map[metav1.GroupVersionResource]*artifact{
		gvr("backupbuckets"): {
			newObject: func() client.Object { return new(extensionsv1alpha1.BackupBucket) },
			validateCreateResource: func(n, _ client.Object) field.ErrorList {
				return validation.ValidateBackupBucket(n.(*extensionsv1alpha1.BackupBucket))
			},
			validateUpdateResource: func(n, o client.Object) field.ErrorList {
				return validation.ValidateBackupBucketUpdate(n.(*extensionsv1alpha1.BackupBucket), o.(*extensionsv1alpha1.BackupBucket))
			},
		},

		gvr("backupentries"): {
			newObject: func() client.Object { return new(extensionsv1alpha1.BackupEntry) },
			validateCreateResource: func(n, o client.Object) field.ErrorList {
				return validation.ValidateBackupEntry(n.(*extensionsv1alpha1.BackupEntry))
			},
			validateUpdateResource: func(n, o client.Object) field.ErrorList {
				return validation.ValidateBackupEntryUpdate(n.(*extensionsv1alpha1.BackupEntry), o.(*extensionsv1alpha1.BackupEntry))
			},
		},

		gvr("bastions"): {
			newObject: func() client.Object { return new(extensionsv1alpha1.Bastion) },
			validateCreateResource: func(n, o client.Object) field.ErrorList {
				return validation.ValidateBastion(n.(*extensionsv1alpha1.Bastion))
			},
			validateUpdateResource: func(n, o client.Object) field.ErrorList {
				return validation.ValidateBastionUpdate(n.(*extensionsv1alpha1.Bastion), o.(*extensionsv1alpha1.Bastion))
			},
		},

		gvr("containerruntimes"): {
			newObject: func() client.Object { return new(extensionsv1alpha1.ContainerRuntime) },
			validateCreateResource: func(n, o client.Object) field.ErrorList {
				return validation.ValidateContainerRuntime(n.(*extensionsv1alpha1.ContainerRuntime))
			},
			validateUpdateResource: func(n, o client.Object) field.ErrorList {
				return validation.ValidateContainerRuntimeUpdate(n.(*extensionsv1alpha1.ContainerRuntime), o.(*extensionsv1alpha1.ContainerRuntime))
			},
		},

		gvr("controlplanes"): {
			newObject: func() client.Object { return new(extensionsv1alpha1.ControlPlane) },
			validateCreateResource: func(n, _ client.Object) field.ErrorList {
				return validation.ValidateControlPlane(n.(*extensionsv1alpha1.ControlPlane))
			},
			validateUpdateResource: func(n, o client.Object) field.ErrorList {
				return validation.ValidateControlPlaneUpdate(n.(*extensionsv1alpha1.ControlPlane), o.(*extensionsv1alpha1.ControlPlane))
			},
		},

		gvr("dnsrecords"): {
			newObject: func() client.Object { return new(extensionsv1alpha1.DNSRecord) },
			validateCreateResource: func(n, _ client.Object) field.ErrorList {
				return validation.ValidateDNSRecord(n.(*extensionsv1alpha1.DNSRecord))
			},
			validateUpdateResource: func(n, o client.Object) field.ErrorList {
				return validation.ValidateDNSRecordUpdate(n.(*extensionsv1alpha1.DNSRecord), o.(*extensionsv1alpha1.DNSRecord))
			},
		},

		gvrDruid("etcds"): {
			newObject: func() client.Object { return new(druidv1alpha1.Etcd) },
			validateCreateResource: func(n, _ client.Object) field.ErrorList {
				return druidvalidation.ValidateEtcd(n.(*druidv1alpha1.Etcd))
			},
			validateUpdateResource: func(n, o client.Object) field.ErrorList {
				return druidvalidation.ValidateEtcdUpdate(n.(*druidv1alpha1.Etcd), o.(*druidv1alpha1.Etcd))
			},
		},

		gvr("extensions"): {
			newObject: func() client.Object { return new(extensionsv1alpha1.Extension) },
			validateCreateResource: func(n, _ client.Object) field.ErrorList {
				return validation.ValidateExtension(n.(*extensionsv1alpha1.Extension))
			},
			validateUpdateResource: func(n, o client.Object) field.ErrorList {
				return validation.ValidateExtensionUpdate(n.(*extensionsv1alpha1.Extension), o.(*extensionsv1alpha1.Extension))
			},
		},

		gvr("infrastructures"): {
			newObject: func() client.Object { return new(extensionsv1alpha1.Infrastructure) },
			validateCreateResource: func(n, _ client.Object) field.ErrorList {
				return validation.ValidateInfrastructure(n.(*extensionsv1alpha1.Infrastructure))
			},
			validateUpdateResource: func(n, o client.Object) field.ErrorList {
				return validation.ValidateInfrastructureUpdate(n.(*extensionsv1alpha1.Infrastructure), o.(*extensionsv1alpha1.Infrastructure))
			},
		},

		gvr("networks"): {
			newObject: func() client.Object { return new(extensionsv1alpha1.Network) },
			validateCreateResource: func(n, _ client.Object) field.ErrorList {
				return validation.ValidateNetwork(n.(*extensionsv1alpha1.Network))
			},
			validateUpdateResource: func(n, o client.Object) field.ErrorList {
				return validation.ValidateNetworkUpdate(n.(*extensionsv1alpha1.Network), o.(*extensionsv1alpha1.Network))
			},
		},

		gvr("operatingsystemconfigs"): {
			newObject: func() client.Object { return new(extensionsv1alpha1.OperatingSystemConfig) },
			validateCreateResource: func(n, _ client.Object) field.ErrorList {
				return validation.ValidateOperatingSystemConfig(n.(*extensionsv1alpha1.OperatingSystemConfig))
			},
			validateUpdateResource: func(n, o client.Object) field.ErrorList {
				return validation.ValidateOperatingSystemConfigUpdate(n.(*extensionsv1alpha1.OperatingSystemConfig), o.(*extensionsv1alpha1.OperatingSystemConfig))
			},
		},

		gvr("workers"): {
			newObject: func() client.Object { return new(extensionsv1alpha1.Worker) },
			validateCreateResource: func(n, _ client.Object) field.ErrorList {
				return validation.ValidateWorker(n.(*extensionsv1alpha1.Worker))
			},
			validateUpdateResource: func(n, o client.Object) field.ErrorList {
				return validation.ValidateWorkerUpdate(n.(*extensionsv1alpha1.Worker), o.(*extensionsv1alpha1.Worker))
			},
		},
	}

	h := handler{
		logger:                         logger,
		artifacts:                      artifacts,
		allowInvalidExtensionResources: allowInvalidExtensionResources,
	}

	return &h
}

type handler struct {
	decoder                        *admission.Decoder
	logger                         logr.Logger
	artifacts                      map[metav1.GroupVersionResource]*artifact
	allowInvalidExtensionResources bool
}

var _ admission.Handler = &handler{}

func (h *handler) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}

func (h *handler) Handle(ctx context.Context, request admission.Request) admission.Response {
	_, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	artifact, ok := h.artifacts[request.Resource]
	if !ok {
		return admission.Allowed("validation not found for the given resource")
	}

	switch request.Operation {
	case admissionv1.Create:
		return h.handleValidation(request, artifact.newObject, artifact.validateCreateResource)
	case admissionv1.Update:
		return h.handleValidation(request, artifact.newObject, artifact.validateUpdateResource)
	default:
		return admission.Allowed("operation is not CREATE or UPDATE")
	}
}

type (
	newObjectFunc func() client.Object
	validateFunc  func(new, old client.Object) field.ErrorList
)

// artifact servers as a helper to setup the corresponding function.
type artifact struct {
	// newObject is a simple function that creates and returns a new resource.
	newObject newObjectFunc

	// validateCreateResource is a wrapper function for the different create validation functions.
	validateCreateResource validateFunc

	// validateUpdateResource is a wrapper function for the different update validation functions.
	validateUpdateResource validateFunc
}

func (h handler) handleValidation(request admission.Request, newObject newObjectFunc, validate validateFunc) admission.Response {
	obj := newObject()
	if err := h.decoder.DecodeRaw(request.Object, obj); err != nil {
		h.logger.Error(err, "Could not decode object", "object", request.Object)
		return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("could not decode object %v: %w", request.Object, err))
	}

	h.logger.Info("Validating extension resource", "resource", request.Resource, "name", kutil.ObjectName(obj), "operation", request.Operation)

	var oldObj client.Object
	if len(request.OldObject.Raw) != 0 {
		oldObj = newObject()
		if err := h.decoder.DecodeRaw(request.OldObject, oldObj); err != nil {
			h.logger.Error(err, "Could not decode old object", "old object", oldObj)
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("could not decode old object %v: %v", oldObj, err))
		}
	}

	errors := validate(obj, oldObj)
	if len(errors) != 0 {
		err := apierrors.NewInvalid(schema.GroupKind{
			Group: request.Kind.Group,
			Kind:  request.Kind.Kind,
		}, kutil.ObjectName(obj), errors)

		h.logger.Info("Invalid extension resource detected", "operation", request.Operation, "error", err.Error())
		if h.allowInvalidExtensionResources {
			return admission.Allowed(err.Error())
		}
		return admission.Denied(err.Error())
	}

	return admission.Allowed("validation successful")
}

func gvr(resource string) metav1.GroupVersionResource {
	return metav1.GroupVersionResource{
		Group:    extensionsv1alpha1.SchemeGroupVersion.Group,
		Version:  extensionsv1alpha1.SchemeGroupVersion.Version,
		Resource: resource,
	}
}

func gvrDruid(resource string) metav1.GroupVersionResource {
	return metav1.GroupVersionResource{
		Group:    druidv1alpha1.GroupVersion.Group,
		Version:  druidv1alpha1.GroupVersion.Version,
		Resource: resource,
	}
}
