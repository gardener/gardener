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

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/extensions/validation"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	// WebhookPath is the HTTP handler path for this admission webhook handler.
	WebhookPath = "/webhooks/validate-extension-resources"

	// HandlerName is the name of this admission webhook handler.
	HandlerName = "extension_resources"
)

var gvk = schema.GroupVersionKind{
	Group:   extensionsv1alpha1.SchemeGroupVersion.Group,
	Version: extensionsv1alpha1.SchemeGroupVersion.Version,
	Kind:    "ValidatingWebhookForExternalResources",
}

// New creates a new webhook handler validating CREATE and UPDATE requests for extension resources.
func New(logger logr.Logger) *handler {
	h := handler{
		logger:    logger,
		artifacts: make(map[metav1.GroupVersionResource]artifact),
	}

	h.addResource(metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "backupbuckets"},
		artifact{
			newEntity: func() client.Object { return new(extensionsv1alpha1.BackupBucket) },
			validateCreateResource: func(n, _ client.Object) field.ErrorList {
				new := n.(*extensionsv1alpha1.BackupBucket)
				return validation.ValidateBackupBucket(new)
			},
			validateUpdateResource: func(n, o client.Object) field.ErrorList {
				new := n.(*extensionsv1alpha1.BackupBucket)
				old := o.(*extensionsv1alpha1.BackupBucket)
				return validation.ValidateBackupBucketUpdate(new, old)
			}})

	h.addResource(metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "backupentries"},
		artifact{
			newEntity: func() client.Object { return new(extensionsv1alpha1.BackupEntry) },
			validateCreateResource: func(n, o client.Object) field.ErrorList {
				new := n.(*extensionsv1alpha1.BackupEntry)
				return validation.ValidateBackupEntry(new)
			},
			validateUpdateResource: func(n, o client.Object) field.ErrorList {
				new := n.(*extensionsv1alpha1.BackupEntry)
				old := o.(*extensionsv1alpha1.BackupEntry)
				return validation.ValidateBackupEntryUpdate(new, old)
			}})

	h.addResource(metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "bastions"},
		artifact{
			newEntity: func() client.Object { return new(extensionsv1alpha1.Bastion) },
			validateCreateResource: func(n, o client.Object) field.ErrorList {
				new := n.(*extensionsv1alpha1.Bastion)
				return validation.ValidateBastion(new)
			},
			validateUpdateResource: func(n, o client.Object) field.ErrorList {
				new := n.(*extensionsv1alpha1.Bastion)
				old := o.(*extensionsv1alpha1.Bastion)
				return validation.ValidateBastionUpdate(new, old)
			}})

	h.addResource(metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "containerruntimes"},
		artifact{
			newEntity: func() client.Object { return new(extensionsv1alpha1.ContainerRuntime) },
			validateCreateResource: func(n, o client.Object) field.ErrorList {
				new := n.(*extensionsv1alpha1.ContainerRuntime)
				return validation.ValidateContainerRuntime(new)
			},
			validateUpdateResource: func(n, o client.Object) field.ErrorList {
				new := n.(*extensionsv1alpha1.ContainerRuntime)
				old := o.(*extensionsv1alpha1.ContainerRuntime)
				return validation.ValidateContainerRuntimeUpdate(new, old)
			},
		})

	h.addResource(metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "controlplanes"},
		artifact{
			newEntity: func() client.Object { return new(extensionsv1alpha1.ControlPlane) },
			validateCreateResource: func(n, _ client.Object) field.ErrorList {
				new := n.(*extensionsv1alpha1.ControlPlane)
				return validation.ValidateControlPlane(new)
			},
			validateUpdateResource: func(n, o client.Object) field.ErrorList {
				new := n.(*extensionsv1alpha1.ControlPlane)
				old := o.(*extensionsv1alpha1.ControlPlane)
				return validation.ValidateControlPlaneUpdate(new, old)
			}})

	h.addResource(metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "dnsrecords"},
		artifact{
			newEntity: func() client.Object { return new(extensionsv1alpha1.DNSRecord) },
			validateCreateResource: func(n, _ client.Object) field.ErrorList {
				new := n.(*extensionsv1alpha1.DNSRecord)
				return validation.ValidateDNSRecord(new)
			},
			validateUpdateResource: func(n, o client.Object) field.ErrorList {
				new := n.(*extensionsv1alpha1.DNSRecord)
				old := o.(*extensionsv1alpha1.DNSRecord)
				return validation.ValidateDNSRecordUpdate(new, old)
			}})

	h.addResource(metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "extensions"},
		artifact{
			newEntity: func() client.Object { return new(extensionsv1alpha1.Extension) },
			validateCreateResource: func(n, _ client.Object) field.ErrorList {
				new := n.(*extensionsv1alpha1.Extension)
				return validation.ValidateExtension(new)
			},
			validateUpdateResource: func(n, o client.Object) field.ErrorList {
				new := n.(*extensionsv1alpha1.Extension)
				old := o.(*extensionsv1alpha1.Extension)
				return validation.ValidateExtensionUpdate(new, old)
			}})

	h.addResource(metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "infrastructures"},
		artifact{
			newEntity: func() client.Object { return new(extensionsv1alpha1.Infrastructure) },
			validateCreateResource: func(n, _ client.Object) field.ErrorList {
				new := n.(*extensionsv1alpha1.Infrastructure)
				return validation.ValidateInfrastructure(new)
			},
			validateUpdateResource: func(n, o client.Object) field.ErrorList {
				new := n.(*extensionsv1alpha1.Infrastructure)
				old := o.(*extensionsv1alpha1.Infrastructure)
				return validation.ValidateInfrastructureUpdate(new, old)
			}})

	h.addResource(metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "networks"},
		artifact{
			newEntity: func() client.Object { return new(extensionsv1alpha1.Network) },
			validateCreateResource: func(n, _ client.Object) field.ErrorList {
				new := n.(*extensionsv1alpha1.Network)
				return validation.ValidateNetwork(new)
			},
			validateUpdateResource: func(n, o client.Object) field.ErrorList {
				new := n.(*extensionsv1alpha1.Network)
				old := o.(*extensionsv1alpha1.Network)
				return validation.ValidateNetworkUpdate(new, old)
			}})

	h.addResource(metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "operatingsystemconfigs"},
		artifact{
			newEntity: func() client.Object { return new(extensionsv1alpha1.OperatingSystemConfig) },
			validateCreateResource: func(n, _ client.Object) field.ErrorList {
				new := n.(*extensionsv1alpha1.OperatingSystemConfig)
				return validation.ValidateOperatingSystemConfig(new)
			},
			validateUpdateResource: func(n, o client.Object) field.ErrorList {
				new := n.(*extensionsv1alpha1.OperatingSystemConfig)
				old := o.(*extensionsv1alpha1.OperatingSystemConfig)
				return validation.ValidateOperatingSystemConfigUpdate(new, old)
			}})

	h.addResource(metav1.GroupVersionResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Version: extensionsv1alpha1.SchemeGroupVersion.Version, Resource: "workers"},
		artifact{
			newEntity: func() client.Object { return new(extensionsv1alpha1.Worker) },
			validateCreateResource: func(n, _ client.Object) field.ErrorList {
				new := n.(*extensionsv1alpha1.Worker)
				return validation.ValidateWorker(new)
			},
			validateUpdateResource: func(n, o client.Object) field.ErrorList {
				new := n.(*extensionsv1alpha1.Worker)
				old := o.(*extensionsv1alpha1.Worker)
				return validation.ValidateWorkerUpdate(new, old)
			}})

	return &h
}

type handler struct {
	decoder   *admission.Decoder
	logger    logr.Logger
	artifacts map[metav1.GroupVersionResource]artifact
}

func (h *handler) addResource(gvr metav1.GroupVersionResource, art artifact) {
	h.artifacts[gvr] = art
}

var _ admission.Handler = &handler{}

func (h *handler) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}

func (h *handler) Handle(ctx context.Context, ar admission.Request) admission.Response {
	h.logger.Info(fmt.Sprintf("validating resource of type %s for operation %s", ar.Resource, ar.Operation))

	_, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	artifact, ok := h.artifacts[ar.Resource]
	if !ok {
		return admission.Allowed("validation not found for the given resource")
	}

	switch ar.Operation {
	case admissionv1.Create:
		return h.handleValidation(ar.Object, ar.OldObject, artifact.newEntity, artifact.validateCreateResource)
	case admissionv1.Update:
		return h.handleValidation(ar.Object, ar.OldObject, artifact.newEntity, artifact.validateUpdateResource)
	default:
		return admission.Allowed("operation is not CREATE or UPDATE")
	}
}

type newEntity func() client.Object
type validate func(new, old client.Object) field.ErrorList

// artifact servers as a helper to setup the corresponding function.
type artifact struct {
	// newEntity is a simple function that creates and returns a new resource.
	newEntity newEntity

	// validateCreateResource is a wrapper function for the different create validation functions.
	validateCreateResource validate

	// validateUpdateResource is a wrapper function for the different update validation functions.
	validateUpdateResource validate
}

func (h handler) handleValidation(object, oldObject runtime.RawExtension, newEntity newEntity, validate validate) admission.Response {
	obj := newEntity()
	if err := h.decoder.DecodeRaw(object, obj); err != nil {
		h.logger.Error(err, "could not decode object", "object", object)
		return admission.Errored(http.StatusUnprocessableEntity, fmt.Errorf("could not decode object %v: %w", object, err))
	}

	h.logger.Info(fmt.Sprintf("handle validation for %s", obj.GetName()))

	oldObj := newEntity()
	if len(oldObject.Raw) != 0 {
		if err := h.decoder.DecodeRaw(oldObject, oldObj); err != nil {
			h.logger.Error(err, "could not decode old object", "old object", oldObj)
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("could not decode old object %v: %v", oldObj, err))
		}
	}

	errors := validate(obj, oldObj)
	if len(errors) != 0 {
		err := apierrors.NewInvalid(gvk.GroupKind(), obj.GetName(), errors)
		return admission.Denied(err.Error())
	}

	return admission.Allowed("validation successful")
}
