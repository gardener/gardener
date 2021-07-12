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

package internaldomainsecret

import (
	"context"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	acadmission "github.com/gardener/gardener/pkg/admissioncontroller/webhooks/admission"
	gardercore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	// HandlerName is the name of this admission webhook handler.
	HandlerName = "internal_domain"
	// WebhookPath is the HTTP handler path for this admission webhook handler.
	WebhookPath = "/webhooks/admission/validate-internal-domain"
)

var secretGVK = metav1.GroupVersionKind{Group: "", Kind: "Secret", Version: "v1"}

// New creates a new handler for validating the immutability of the internal domain secret.
func New(logger logr.Logger) *handler {
	return &handler{logger: logger}
}

type handler struct {
	logger    logr.Logger
	apiReader client.Reader
	decoder   *admission.Decoder
}

var _ admission.Handler = &handler{}

func (h *handler) InjectAPIReader(reader client.Reader) error {
	h.apiReader = reader
	return nil
}

func (h *handler) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}

func (h *handler) Handle(ctx context.Context, request admission.Request) admission.Response {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if request.Kind != secretGVK {
		return acadmission.Allowed("resource is not corev1.Secret")
	}
	if request.SubResource != "" {
		return acadmission.Allowed("subresources on Secrets are not handled")
	}
	seedName := gutil.ComputeSeedName(request.Namespace)
	if request.Namespace != v1beta1constants.GardenNamespace && seedName == "" {
		return acadmission.Allowed("only secrets from the garden and seed namespaces are handled")
	}

	switch request.Operation {
	case admissionv1.Create:
		alreadyExists, err := h.secretAlreadyExists(ctx, request.Name, request.Namespace)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if alreadyExists {
			return admission.Allowed("the secret already exists")
		}
		exists, err := h.internalDomainSecretExists(ctx, request.Namespace)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if exists {
			return admission.Denied("cannot create internal domain secret because there can be only one secret with the 'internal-domain' secret role per namespace")
		}

		secret := &corev1.Secret{}
		if err := h.decoder.Decode(request, secret); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if _, _, _, _, _, err := gutil.GetDomainInfoFromAnnotations(secret.Annotations); err != nil {
			return admission.Errored(http.StatusUnprocessableEntity, err)
		}

		return acadmission.Allowed("internal domain secret is valid")

	case admissionv1.Update:
		secret, oldSecret := &corev1.Secret{}, &corev1.Secret{}
		if err := h.decoder.Decode(request, secret); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if err := h.decoder.DecodeRaw(request.OldObject, oldSecret); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		_, oldDomain, _, _, _, err := gutil.GetDomainInfoFromAnnotations(oldSecret.Annotations)
		if err != nil {
			return admission.Errored(http.StatusUnprocessableEntity, err)
		}
		_, newDomain, _, _, _, err := gutil.GetDomainInfoFromAnnotations(secret.Annotations)
		if err != nil {
			return admission.Errored(http.StatusUnprocessableEntity, err)
		}

		if oldDomain != newDomain {
			atLeastOneShoot, err := h.atLeastOneShootExists(ctx, seedName)
			if err != nil {
				return admission.Errored(http.StatusInternalServerError, err)
			}
			if atLeastOneShoot {
				return admission.Denied("cannot change domain because there are still shoots left in the system")
			}
		}

		return acadmission.Allowed("domain didn't change or no shoot exists")

	case admissionv1.Delete:
		atLeastOneShoot, err := h.atLeastOneShootExists(ctx, seedName)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if atLeastOneShoot {
			return admission.Denied("cannot delete internal domain secret because there are still shoots left in the system")
		}
		return acadmission.Allowed("no shoot exists")

	default:
		return acadmission.Allowed("unknown operation")
	}
}

func (h *handler) atLeastOneShootExists(ctx context.Context, seedName string) (bool, error) {
	var (
		shoots      = &metav1.PartialObjectMetadataList{}
		listOptions = []client.ListOption{client.Limit(1)}
	)

	if seedName != "" {
		listOptions = append(listOptions, client.MatchingFields{
			gardercore.ShootSeedName: seedName,
		})
	}

	shoots.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("ShootList"))

	if err := h.apiReader.List(ctx, shoots, listOptions...); err != nil {
		return false, err
	}

	return len(shoots.Items) > 0, nil
}

func (h *handler) internalDomainSecretExists(ctx context.Context, namespace string) (bool, error) {
	secrets := &metav1.PartialObjectMetadataList{}
	secrets.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("SecretList"))

	if err := h.apiReader.List(
		ctx,
		secrets,
		client.InNamespace(namespace),
		client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleInternalDomain},
		client.Limit(1),
	); err != nil {
		return false, err
	}

	return len(secrets.Items) > 0, nil
}

func (h *handler) secretAlreadyExists(ctx context.Context, name, namespace string) (bool, error) {
	secret := &corev1.Secret{}

	if err := h.apiReader.Get(ctx, kutil.Key(namespace, name), secret); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
