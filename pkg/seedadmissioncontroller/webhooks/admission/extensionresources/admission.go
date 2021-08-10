package extensionresources

import (
	"context"
	"fmt"
	"net/http"

	authenticationapiv1alpha1 "github.com/gardener/gardener/pkg/apis/authentication/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener/pkg/apis/extensions/validation"

	v1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// WebhookPath is the HTTP handler path for this admission webhook handler.
const WebhookPath = "/webhooks/validate-extension-resources"

// TODO: fix this
var (
	gvk = schema.GroupVersionKind{
		Group:   authenticationapiv1alpha1.SchemeGroupVersion.Group,
		Version: authenticationapiv1alpha1.SchemeGroupVersion.Version,
		Kind:    "AdminKubeconfigRequest",
	}

	// todo make this sync.Map
	artifacts = map[string]artifact{
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
	artifact := artifacts[ar.Kind.Kind]

	switch ar.Operation {
	case v1.Create:
		return h.handleValidation(ar.Object, ar.OldObject, artifact.newEntity, artifact.validateResource)
	case v1.Update:
		return h.handleValidation(ar.Object, ar.OldObject, artifact.newEntity, artifact.validateResourceUpdate)
	default:
		return admission.Allowed("operation is not CREATE or UPDATE")
	}

	return admission.Allowed("")
}

type artifact struct {
	// map[ar.Kind.Kind]artifact
	// artifact{ newobjfunc -> return from the correct type (DNSRecord)
	//         ar.Object
	//         ar.OldObject
	//         func() -> ValidateCRD.. ? all have same signiture
	//         func() -> ValidateCRDUpdate.. ? all have same signiture

	newEntity func() interface{}
	//object                 runtime.RawExtension
	//oldObject              runtime.RawExtension
	validateResource       func(o, n interface{}) field.ErrorList
	validateResourceUpdate func(o, n interface{}) field.ErrorList
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
