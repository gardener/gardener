// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	extensionsconfigv1alpha1 "github.com/gardener/gardener/extensions/pkg/apis/config/v1alpha1"
	"github.com/gardener/gardener/extensions/pkg/util"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// remoteAddrContextKey is a context key. It will be filled by the remoteAddrInjectingHandler with the received
// request's RemoteAddr field value.
// The associated value will be of type string.
var remoteAddrContextKey = struct{}{}

// NewHandlerWithShootClient creates a new handler for the given types, using the given mutator, and logger.
func NewHandlerWithShootClient(mgr manager.Manager, types []Type, mutator MutatorWithShootClient, logger logr.Logger) (admission.Handler, error) {
	// Build a map of the given types keyed by their GVKs
	typesMap, err := buildTypesMap(mgr.GetScheme(), objectsFromTypes(types))
	if err != nil {
		return nil, err
	}

	return &admission.Webhook{
		Handler: &handlerShootClient{
			typesMap: typesMap,
			mutator:  mutator,
			client:   mgr.GetClient(),
			decoder:  serializer.NewCodecFactory(mgr.GetScheme()).UniversalDecoder(),
			logger:   logger.WithName("handlerShootClient"),
		},
		RecoverPanic: ptr.To(true),
		WithContextFunc: func(ctx context.Context, r *http.Request) context.Context {
			if r != nil {
				ctx = context.WithValue(ctx, remoteAddrContextKey, r.RemoteAddr) //nolint:staticcheck
			}
			return ctx
		},
	}, nil
}

type handlerShootClient struct {
	typesMap map[metav1.GroupVersionKind]client.Object
	mutator  MutatorWithShootClient
	client   client.Client
	decoder  runtime.Decoder
	logger   logr.Logger
}

func (h *handlerShootClient) Handle(ctx context.Context, req admission.Request) admission.Response {
	var mut actionFunc = func(ctx context.Context, new, old client.Object) error {
		// TODO: replace this logic with a proper authentication mechanism
		// see https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#authenticate-apiservers
		// API servers should authenticate against webhooks servers using TLS client certs, from which the webhook
		// can identify from which shoot cluster the webhook call is coming
		remoteAddrValue := ctx.Value(remoteAddrContextKey)
		if remoteAddrValue == nil {
			return fmt.Errorf("didn't receive remote address")
		}

		remoteAddr, ok := remoteAddrValue.(string)
		if !ok {
			return fmt.Errorf("remote address expected to be string, got %T", remoteAddrValue)
		}

		ipPort := strings.Split(remoteAddr, ":")
		if len(ipPort) < 1 {
			return fmt.Errorf("remote address not parseable: %s", remoteAddr)
		}
		ip := ipPort[0]

		podList := &corev1.PodList{}
		if err := h.client.List(ctx, podList, client.MatchingLabels{
			v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
			v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer,
		}); err != nil {
			return fmt.Errorf("error while listing all pods: %w", err)
		}

		var shootNamespace string
		for _, pod := range podList.Items {
			if pod.Status.PodIP == ip {
				shootNamespace = pod.Namespace
				break
			}
		}

		if len(shootNamespace) == 0 {
			return fmt.Errorf("could not find shoot namespace for webhook request from remote address %s", remoteAddr)
		}

		_, shootClient, err := util.NewClientForShoot(ctx, h.client, shootNamespace, client.Options{}, extensionsconfigv1alpha1.RESTOptions{})
		if err != nil {
			return fmt.Errorf("could not create shoot client: %w", err)
		}

		return h.mutator.Mutate(ctx, new, old, shootClient)
	}

	// Decode object
	t, ok := h.typesMap[req.Kind]
	if !ok {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected request kind %s", req.Kind))
	}

	return handle(ctx, req, mut, t, h.decoder, h.logger)
}
