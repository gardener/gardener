// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	extensionsconfigv1alpha1 "github.com/gardener/gardener/extensions/pkg/apis/config/v1alpha1"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

type handlerActionWithTypes struct {
	mutator   Mutator
	validator Validator
	types     []Type
}

// HandlerBuilder contains information which are required to create an admission handler.
type HandlerBuilder struct {
	handlerActions []handlerActionWithTypes
	predicates     []predicate.Predicate
	scheme         *runtime.Scheme
	logger         logr.Logger
	client         client.Client
}

// NewBuilder creates a new HandlerBuilder.
func NewBuilder(mgr manager.Manager, logger logr.Logger) *HandlerBuilder {
	return &HandlerBuilder{
		scheme: mgr.GetScheme(),
		logger: logger.WithName("handler"),
		client: mgr.GetClient(),
	}
}

// WithMutator adds the given mutator for the given types to the HandlerBuilder.
func (b *HandlerBuilder) WithMutator(mutator Mutator, types ...Type) *HandlerBuilder {
	b.handlerActions = append(b.handlerActions, handlerActionWithTypes{mutator: mutator, types: types})
	return b
}

// WithValidator adds the given validator for the given types to the HandlerBuilder.
func (b *HandlerBuilder) WithValidator(validator Validator, types ...Type) *HandlerBuilder {
	b.handlerActions = append(b.handlerActions, handlerActionWithTypes{validator: validator, types: types})
	return b
}

// WithPredicates adds the given predicates to the HandlerBuilder.
func (b *HandlerBuilder) WithPredicates(predicates ...predicate.Predicate) *HandlerBuilder {
	b.predicates = append(b.predicates, predicates...)
	return b
}

// Build creates a new admission.Handler with the settings previously specified with the HandlerBuilder's functions.
func (b *HandlerBuilder) Build() (admission.Handler, error) {
	h := &handler{
		gvkToHandlerAction: make(map[metav1.GroupVersionKind]handlerAction),
		predicates:         b.predicates,
		scheme:             b.scheme,
		logger:             b.logger,
		client:             b.client,
	}

	for _, action := range b.handlerActions {
		gvkToObjectType, err := buildTypesMap(b.scheme, objectsFromTypes(action.types))
		if err != nil {
			return nil, err
		}
		for gvk, obj := range gvkToObjectType {
			h.gvkToHandlerAction[gvk] = handlerAction{
				mutator:   action.mutator,
				validator: action.validator,
				object:    obj,
			}
		}
	}
	h.decoder = serializer.NewCodecFactory(b.scheme).UniversalDecoder()

	return h, nil
}

type handlerAction struct {
	mutator   Mutator
	validator Validator
	object    client.Object
}

type handler struct {
	client  client.Client
	logger  logr.Logger
	scheme  *runtime.Scheme
	decoder runtime.Decoder

	gvkToHandlerAction map[metav1.GroupVersionKind]handlerAction
	predicates         []predicate.Predicate
}

// Handle handles the given admission request.
func (h *handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	ar := req.AdmissionRequest

	action, ok := h.gvkToHandlerAction[ar.Kind]
	if !ok {
		// check if we can find an internal type
		for gvk, action2 := range h.gvkToHandlerAction {
			if gvk.Version == runtime.APIVersionInternal && gvk.Group == ar.Kind.Group && gvk.Kind == ar.Kind.Kind {
				action = action2
				break
			}
		}
		if action.mutator == nil && action.validator == nil {
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected request kind %s", ar.Kind.String()))
		}
	}

	return h.handle(ctx, req, action)
}

func (h *handler) handle(ctx context.Context, req admission.Request, action handlerAction) admission.Response {
	ar := req.AdmissionRequest

	// Decode object
	obj := action.object.DeepCopyObject().(client.Object)
	_, _, err := h.decoder.Decode(req.Object.Raw, nil, obj)
	if err != nil {
		h.logger.Error(err, "Could not decode request", "request", ar)
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("could not decode request %v: %w", ar, err))
	}

	var oldObj client.Object

	// Only UPDATE and DELETE operations have oldObjects.
	if len(req.OldObject.Raw) != 0 {
		oldObj = action.object.DeepCopyObject().(client.Object)
		if _, _, err := h.decoder.Decode(ar.OldObject.Raw, nil, oldObj); err != nil {
			h.logger.Error(err, "Could not decode old object", "object", oldObj)
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("could not decode old object %v: %v", oldObj, err))
		}
	}

	// Run object through predicates
	if !predicateutils.EvalGeneric(obj, h.predicates...) {
		return admission.ValidationResponse(true, "")
	}

	if ctx, err = h.addAdditionalValuesToContext(ctx, action); err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("could not add additional values to context: %w", err))
	}

	newObj := obj.DeepCopyObject().(client.Object)

	switch {
	case action.mutator != nil:
		if err = action.mutator.Mutate(ctx, newObj, oldObj); err != nil {
			h.logger.Error(fmt.Errorf("could not process: %w", err), "Admission denied", "kind", ar.Kind.Kind, "namespace", obj.GetNamespace(), "name", obj.GetName())
			return admission.Errored(http.StatusUnprocessableEntity, err)
		}

		// Return a patch response if the resource should be changed
		if !equality.Semantic.DeepEqual(obj, newObj) {
			oldObjMarshaled, err := json.Marshal(obj)
			if err != nil {
				return admission.Errored(http.StatusInternalServerError, err)
			}
			newObjMarshaled, err := json.Marshal(newObj)
			if err != nil {
				return admission.Errored(http.StatusInternalServerError, err)
			}

			return admission.PatchResponseFromRaw(oldObjMarshaled, newObjMarshaled)
		}

	case action.validator != nil:
		if err = action.validator.Validate(ctx, newObj, oldObj); err != nil {
			h.logger.Error(fmt.Errorf("could not process: %w", err), "Admission denied", "kind", ar.Kind.Kind, "namespace", obj.GetNamespace(), "name", obj.GetName())
			return admission.Errored(http.StatusUnprocessableEntity, err)
		}
	}

	// Return a validation response if the resource should not be changed
	return admission.ValidationResponse(true, "")
}

func (h *handler) addAdditionalValuesToContext(ctx context.Context, action handlerAction) (context.Context, error) {
	var obj interface{}

	switch {
	case action.mutator != nil:
		obj = action.mutator
	case action.validator != nil:
		obj = action.validator
	default:
		return nil, fmt.Errorf("neither mutator nor validator is set")
	}

	if wantsShootClient, ok := obj.(WantsShootClient); ok && wantsShootClient.WantsShootClient() {
		shootClient, err := h.constructShootClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed building shoot client: %w", err)
		}
		ctx = context.WithValue(ctx, ShootClientContextKey{}, shootClient) //nolint:staticcheck
	}

	if wantsClusterObject, ok := obj.(WantsClusterObject); ok && wantsClusterObject.WantsClusterObject() {
		cluster, err := h.constructClusterObject(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed building Cluster object: %w", err)
		}
		ctx = context.WithValue(ctx, ClusterObjectContextKey{}, cluster) //nolint:staticcheck
	}

	return ctx, nil
}

func objectsFromTypes(in []Type) []client.Object {
	out := make([]client.Object, 0, len(in))
	for _, t := range in {
		out = append(out, t.Obj)
	}
	return out
}

// buildTypesMap builds a map of the given types keyed by their GroupVersionKind, using the scheme from the given Manager.
func buildTypesMap(scheme *runtime.Scheme, types []client.Object) (map[metav1.GroupVersionKind]client.Object, error) {
	typesMap := make(map[metav1.GroupVersionKind]client.Object)
	for _, t := range types {
		// Get GVK from the type
		gvk, err := apiutil.GVKForObject(t, scheme)
		if err != nil {
			return nil, fmt.Errorf("could not get GroupVersionKind from object %v: %w", t, err)
		}

		// Add the type to the types map
		typesMap[metav1.GroupVersionKind(gvk)] = t
	}
	return typesMap, nil
}

type (
	// RemoteAddrContextKey is a context key. It will be filled by with the received HTTP request's RemoteAddr field
	// value.
	// The associated value will be of type string.
	RemoteAddrContextKey struct{}
	// ShootClientContextKey is a context key. In case the WantsShootClient interface is implemented, it will be filled
	// with an uncached Shoot client.
	// The associated value will be of type client.Client.
	ShootClientContextKey struct{}
	// ClusterObjectContextKey is a context key. In case the WantsClusterObject interface is implemented, it will be
	// filled with a Cluster object containing the Shoot object for the request, as well as the corresponding Seed and
	// CloudProfile objects.
	// The associated value will be of type extensions.Cluster.
	ClusterObjectContextKey struct{}
)

// WantsShootClient can be implemented if a mutator wants a client for the shoot cluster. The client will always be
// an uncached client.
type WantsShootClient interface {
	// WantsShootClient returns true if the mutator wants a shoot client to be injected into the context.
	WantsShootClient() bool
}

// WantsClusterObject can be implemented if a mutator wants a Cluster object for the shoot cluster. It can be used to
// read information about the Shoot, e.g., the specification, or information about the CloudProfile or Seed.
type WantsClusterObject interface {
	// WantsClusterObject returns true if the mutator wants a Cluster object to be injected into the context.
	WantsClusterObject() bool
}

func (h *handler) determineShootNamespaceInSeed(ctx context.Context) (string, error) {
	// TODO: replace this logic with a proper authentication mechanism
	// see https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#authenticate-apiservers
	// API servers should authenticate against webhooks servers using TLS client certs, from which the webhook
	// can identify from which shoot cluster the webhook call is coming
	remoteAddrValue := ctx.Value(RemoteAddrContextKey{})
	if remoteAddrValue == nil {
		return "", fmt.Errorf("didn't receive remote address")
	}

	remoteAddr, ok := remoteAddrValue.(string)
	if !ok {
		return "", fmt.Errorf("remote address expected to be string, got %T", remoteAddrValue)
	}

	ipPort := strings.Split(remoteAddr, ":")
	if len(ipPort) < 1 {
		return "", fmt.Errorf("remote address not parseable: %s", remoteAddr)
	}
	ip := ipPort[0]

	podList := &corev1.PodList{}
	if err := h.client.List(ctx, podList, client.MatchingLabels{
		v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
		v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer,
	}); err != nil {
		return "", fmt.Errorf("error while listing all kube-apiserver pods: %w", err)
	}

	var shootNamespace string
	for _, pod := range podList.Items {
		if pod.Status.PodIP == ip {
			shootNamespace = pod.Namespace
			break
		}
	}

	if len(shootNamespace) == 0 {
		return "", fmt.Errorf("could not find shoot namespace for webhook request from remote address %s", remoteAddr)
	}

	return shootNamespace, nil
}

func (h *handler) constructShootClient(ctx context.Context) (client.Client, error) {
	shootNamespace, err := h.determineShootNamespaceInSeed(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed determining shoot namespace for shoot client creation: %w", err)
	}

	_, shootClient, err := util.NewClientForShoot(ctx, h.client, shootNamespace, client.Options{}, extensionsconfigv1alpha1.RESTOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not create shoot client: %w", err)
	}

	return shootClient, nil
}

func (h *handler) constructClusterObject(ctx context.Context) (*extensionscontroller.Cluster, error) {
	shootNamespace, err := h.determineShootNamespaceInSeed(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed determining shoot namespace for shoot client creation: %w", err)
	}

	return extensionscontroller.GetCluster(ctx, h.client, shootNamespace)
}
