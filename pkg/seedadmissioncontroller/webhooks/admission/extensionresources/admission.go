package extensionresources

import (
	"context"
	"fmt"
	"net/http"
	"time"

	authenticationapiv1alpha1 "github.com/gardener/gardener/pkg/apis/authentication/v1alpha1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/extensions/validation"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// WebhookPath is the HTTP handler path for this admission webhook handler.
const WebhookPath = "/webhooks/validate-extension-resources"

var (
	gvk = schema.GroupVersionKind{
		Group:   authenticationapiv1alpha1.SchemeGroupVersion.Group,
		Version: authenticationapiv1alpha1.SchemeGroupVersion.Version,
		Kind:    "ValidatingWebhookForExternalResources",
	}

	artifacts = map[string]artifact{
		extensionsv1alpha1.BackupBucketResource: {
			newEntity: func() interface{} { return new(extensionsv1alpha1.BackupBucket) },
			validateResource: func(n, _ interface{}) field.ErrorList {
				new := n.(*extensionsv1alpha1.BackupBucket)
				return validation.ValidateBackupBucket(new)
			},
			validateResourceUpdate: func(n, o interface{}) field.ErrorList {
				new := n.(*extensionsv1alpha1.BackupBucket)
				old := o.(*extensionsv1alpha1.BackupBucket)
				return validation.ValidateBackupBucketUpdate(new, old)
			},
		},
		extensionsv1alpha1.BackupEntryResource: {
			newEntity: func() interface{} { return new(extensionsv1alpha1.BackupEntry) },
			validateResource: func(n, o interface{}) field.ErrorList {
				new := n.(*extensionsv1alpha1.BackupEntry)
				return validation.ValidateBackupEntry(new)
			},
			validateResourceUpdate: func(n, o interface{}) field.ErrorList {
				new := n.(*extensionsv1alpha1.BackupEntry)
				old := n.(*extensionsv1alpha1.BackupEntry)
				return validation.ValidateBackupEntryUpdate(new, old)
			},
		},
		extensionsv1alpha1.ControlPlaneResource: {
			newEntity: func() interface{} { return new(extensionsv1alpha1.ControlPlane) },
			validateResource: func(n, _ interface{}) field.ErrorList {
				new := n.(*extensionsv1alpha1.ControlPlane)
				return validation.ValidateControlPlane(new)
			},
			validateResourceUpdate: func(n, o interface{}) field.ErrorList {
				new := n.(*extensionsv1alpha1.ControlPlane)
				old := o.(*extensionsv1alpha1.ControlPlane)
				return validation.ValidateControlPlaneUpdate(new, old)
			},
		},
		extensionsv1alpha1.DNSRecordResource: {
			newEntity: func() interface{} { return new(extensionsv1alpha1.DNSRecord) },
			validateResource: func(n, _ interface{}) field.ErrorList {
				new := n.(*extensionsv1alpha1.DNSRecord)
				return validation.ValidateDNSRecord(new)
			},
			validateResourceUpdate: func(n, o interface{}) field.ErrorList {
				new := n.(*extensionsv1alpha1.DNSRecord)
				old := o.(*extensionsv1alpha1.DNSRecord)
				return validation.ValidateDNSRecordUpdate(new, old)
			},
		},
		extensionsv1alpha1.ExtensionResource: {
			newEntity: func() interface{} { return new(extensionsv1alpha1.Extension) },
			validateResource: func(n, _ interface{}) field.ErrorList {
				new := n.(*extensionsv1alpha1.Extension)
				return validation.ValidateExtension(new)
			},
			validateResourceUpdate: func(n, o interface{}) field.ErrorList {
				new := n.(*extensionsv1alpha1.Extension)
				old := o.(*extensionsv1alpha1.Extension)
				return validation.ValidateExtensionUpdate(new, old)
			},
		},
		extensionsv1alpha1.InfrastructureResource: {
			newEntity: func() interface{} { return new(extensionsv1alpha1.Infrastructure) },
			validateResource: func(n, _ interface{}) field.ErrorList {
				new := n.(*extensionsv1alpha1.Infrastructure)
				return validation.ValidateInfrastructure(new)
			},
			validateResourceUpdate: func(n, o interface{}) field.ErrorList {
				new := n.(*extensionsv1alpha1.Infrastructure)
				old := o.(*extensionsv1alpha1.Infrastructure)
				return validation.ValidateInfrastructureUpdate(new, old)
			},
		},
		extensionsv1alpha1.NetworkResource: {
			newEntity: func() interface{} { return new(extensionsv1alpha1.Network) },
			validateResource: func(n, _ interface{}) field.ErrorList {
				new := n.(*extensionsv1alpha1.Network)
				return validation.ValidateNetwork(new)
			},
			validateResourceUpdate: func(n, o interface{}) field.ErrorList {
				new := n.(*extensionsv1alpha1.Network)
				old := o.(*extensionsv1alpha1.Network)
				return validation.ValidateNetworkUpdate(new, old)
			},
		},
		extensionsv1alpha1.OperatingSystemConfigResource: {
			newEntity: func() interface{} { return new(extensionsv1alpha1.OperatingSystemConfig) },
			validateResource: func(n, _ interface{}) field.ErrorList {
				new := n.(*extensionsv1alpha1.OperatingSystemConfig)
				return validation.ValidateOperatingSystemConfig(new)
			},
			validateResourceUpdate: func(n, o interface{}) field.ErrorList {
				new := n.(*extensionsv1alpha1.OperatingSystemConfig)
				old := o.(*extensionsv1alpha1.OperatingSystemConfig)
				return validation.ValidateOperatingSystemConfigUpdate(new, old)
			},
		},
		extensionsv1alpha1.WorkerResource: {
			newEntity: func() interface{} { return new(extensionsv1alpha1.Worker) },
			validateResource: func(n, _ interface{}) field.ErrorList {
				new := n.(*extensionsv1alpha1.Worker)
				return validation.ValidateWorker(new)
			},
			validateResourceUpdate: func(n, o interface{}) field.ErrorList {
				new := n.(*extensionsv1alpha1.Worker)
				old := o.(*extensionsv1alpha1.Worker)
				return validation.ValidateWorkerUpdate(new, old)
			},
		},
	}
)

// New creates a new webhook handler validating DELETE requests for extension CRDs and extension resources, that are
// marked for deletion protection (`gardener.cloud/deletion-protected`).
func New(logger logr.Logger) *handler {
	return &handler{logger: logger}
}

type handler struct {
	reader  client.Reader
	decoder *admission.Decoder
	logger  logr.Logger
}

var _ admission.Handler = &handler{}

func (h *handler) InjectAPIReader(reader client.Reader) error {
	h.reader = reader
	return nil
}

func (h *handler) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}

func (h *handler) Handle(ctx context.Context, ar admission.Request) admission.Response {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	artifact := artifacts[ar.Kind.Kind]

	switch ar.Operation {
	case v1.Create:
		return h.handleValidation(ar.Object, ar.OldObject, artifact.newEntity, artifact.validateResource)
	case v1.Update:
		return h.handleValidation(ar.Object, ar.OldObject, artifact.newEntity, artifact.validateResourceUpdate)
	default:
		return admission.Allowed("operation is not CREATE or UPDATE")
	}
}

type artifact struct {
	newEntity              func() interface{}
	validateResource       func(n, o interface{}) field.ErrorList
	validateResourceUpdate func(n, o interface{}) field.ErrorList
}

func (h handler) handleValidation(object, oldObject runtime.RawExtension, newEntity func() interface{}, validate func(o, n interface{}) field.ErrorList) admission.Response {
	obj := newEntity()
	if err := h.decoder.DecodeRaw(object, obj.(runtime.Object)); err != nil {
		h.logger.Error(err, "could not decode ar", "ar", object)
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("could not decode ar %v: %w", object, err))
	}

	oldObj := newEntity()
	if len(oldObject.Raw) != 0 {
		if err := h.decoder.DecodeRaw(oldObject, oldObj.(runtime.Object)); err != nil {
			h.logger.Error(err, "could not decode old object", "object", oldObj)
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("could not decode old object %v: %v", oldObj, err))
		}
	}

	errors := validate(obj, oldObj)
	if len(errors) == 0 {
		return admission.Allowed("")
	}

	var err = apierrors.NewInvalid(gvk.GroupKind(), "", errors)

	return admission.Denied(err.Error())
}
