// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	extensionsconfig "github.com/gardener/gardener/extensions/pkg/apis/config"
	"github.com/gardener/gardener/extensions/pkg/util"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// NewHandlerWithShootClient creates a new handler for the given types, using the given mutator, and logger.
func NewHandlerWithShootClient(mgr manager.Manager, types []Type, mutator MutatorWithShootClient, logger logr.Logger) (http.Handler, error) {
	// Build a map of the given types keyed by their GVKs
	typesMap, err := buildTypesMap(mgr.GetScheme(), objectsFromTypes(types))
	if err != nil {
		return nil, err
	}

	// inject RemoteAddr into admission http.Handler, we need it to identify which API server called the webhook server
	// in order to create a client for that shoot cluster.
	return remoteAddrInjectingHandler{
		Handler: &admission.Webhook{
			Handler: &handlerShootClient{
				typesMap: typesMap,
				mutator:  mutator,
				logger:   logger.WithName("handlerShootClient"),
			},
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

// InjectScheme injects the given scheme into the handler.
func (h *handlerShootClient) InjectScheme(s *runtime.Scheme) error {
	h.decoder = serializer.NewCodecFactory(s).UniversalDecoder()
	return nil
}

// InjectFunc injects stuff into the mutator.
func (h *handlerShootClient) InjectFunc(f inject.Func) error {
	return f(h.mutator)
}

// InjectClient injects a client.
func (h *handlerShootClient) InjectClient(client client.Client) error {
	h.client = client
	return nil
}

func (h *handlerShootClient) Handle(ctx context.Context, req admission.Request) admission.Response {
	var mut MutateFunc = func(ctx context.Context, new, old client.Object) error {
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
			return fmt.Errorf("could not find shoot namespace for webhook request")
		}

		_, shootClient, err := util.NewClientForShoot(ctx, h.client, shootNamespace, client.Options{}, extensionsconfig.RESTOptions{})
		if err != nil {
			return fmt.Errorf("could not create shoot client: %w", err)
		}

		return h.mutator.Mutate(ctx, new, old, shootClient)
	}

	// Decode object
	t, ok := h.typesMap[req.AdmissionRequest.Kind]
	if !ok {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unexpected request kind %s", req.AdmissionRequest.Kind))
	}

	return handle(ctx, req, mut, t, h.decoder, h.logger)
}
