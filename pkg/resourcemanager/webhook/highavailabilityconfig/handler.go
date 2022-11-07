// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package highavailabilityconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

// Handler handles admission requests and sets the following fields based on the failure tolerance type and the
// component type:
// - `.spec.replicas`
// - `.spec.template.spec.affinity`
// - `.spec.template.spec.topologySpreadConstraints`
type Handler struct {
	Logger       logr.Logger
	TargetClient client.Reader

	decoder *admission.Decoder
}

// InjectDecoder injects the decoder.
func (h *Handler) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}

// Handle defaults the high availability settings of the provided resource.
func (h *Handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	var (
		requestGK = schema.GroupKind{Group: req.Kind.Group, Kind: req.Kind.Kind}
		obj       runtime.Object
		err       error
	)

	namespace := &corev1.Namespace{}
	if err := h.TargetClient.Get(ctx, client.ObjectKey{Name: req.Namespace}, namespace); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	var (
		failureToleranceType *gardencorev1beta1.FailureToleranceType
		replicaCriteria      = resourcesv1alpha1.HighAvailabilityConfigCriteriaZones
		zones                []string
	)

	if v, ok := namespace.Annotations[resourcesv1alpha1.HighAvailabilityConfigFailureToleranceType]; ok {
		value := gardencorev1beta1.FailureToleranceType(v)
		failureToleranceType = &value
	}

	if v, ok := namespace.Annotations[resourcesv1alpha1.HighAvailabilityConfigReplicaCriteria]; ok {
		replicaCriteria = v
	}

	if v, ok := namespace.Annotations[resourcesv1alpha1.HighAvailabilityConfigZones]; ok {
		zones = sets.NewString(strings.Split(v, ",")...).Delete("").List()
	}

	switch requestGK {
	case appsv1.SchemeGroupVersion.WithKind("Deployment").GroupKind():
		obj, err = h.handleDeployment(req, failureToleranceType, replicaCriteria, zones)
	case appsv1.SchemeGroupVersion.WithKind("StatefulSet").GroupKind():
		obj, err = h.handleStatefulSet(req, failureToleranceType, replicaCriteria, zones)
	default:
		return admission.Allowed(fmt.Sprintf("unexpected resource: %s", requestGK))
	}

	if err != nil {
		var apiStatus apierrors.APIStatus
		if errors.As(err, &apiStatus) {
			result := apiStatus.Status()
			return admission.Response{AdmissionResponse: admissionv1.AdmissionResponse{Allowed: false, Result: &result}}
		}
		return admission.Denied(err.Error())
	}

	marshalled, err := json.Marshal(obj)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshalled)
}

func (h *Handler) handleDeployment(
	req admission.Request,
	failureToleranceType *gardencorev1beta1.FailureToleranceType,
	replicaCriteria string,
	zones []string,
) (
	runtime.Object,
	error,
) {
	deployment := &appsv1.Deployment{}
	if err := h.decoder.Decode(req, deployment); err != nil {
		return nil, err
	}

	return deployment, nil
}

func (h *Handler) handleStatefulSet(
	req admission.Request,
	failureToleranceType *gardencorev1beta1.FailureToleranceType,
	replicaCriteria string,
	zones []string,
) (
	runtime.Object,
	error,
) {
	statefulSet := &appsv1.StatefulSet{}
	if err := h.decoder.Decode(req, statefulSet); err != nil {
		return nil, err
	}

	return statefulSet, nil
}
