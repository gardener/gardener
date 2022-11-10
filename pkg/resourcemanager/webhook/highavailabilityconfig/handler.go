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
	"strconv"
	"strings"

	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Handler handles admission requests and sets the following fields based on the failure tolerance type and the
// component type:
// - `.spec.replicas`
// - `.spec.template.spec.affinity`
// - `.spec.template.spec.topologySpreadConstraints`
type Handler struct {
	Logger                       logr.Logger
	TargetClient                 client.Reader
	TargetVersionGreaterEqual123 bool

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
		zones                []string
	)

	if v, ok := namespace.Annotations[resourcesv1alpha1.HighAvailabilityConfigFailureToleranceType]; ok {
		value := gardencorev1beta1.FailureToleranceType(v)
		failureToleranceType = &value
	}

	if v, ok := namespace.Annotations[resourcesv1alpha1.HighAvailabilityConfigZones]; ok {
		zones = sets.NewString(strings.Split(v, ",")...).Delete("").List()
	}

	isHorizontallyScaled, maxReplicas, err := h.isHorizontallyScaled(ctx, req.Namespace, schema.GroupVersion{Group: req.Kind.Group, Version: req.Kind.Version}.String(), req.Kind.Kind, req.Name)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	switch requestGK {
	case appsv1.SchemeGroupVersion.WithKind("Deployment").GroupKind():
		obj, err = h.handleDeployment(req, failureToleranceType, zones, isHorizontallyScaled, maxReplicas)
	case appsv1.SchemeGroupVersion.WithKind("StatefulSet").GroupKind():
		obj, err = h.handleStatefulSet(req, failureToleranceType, zones, isHorizontallyScaled, maxReplicas)
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
	zones []string,
	isHorizontallyScaled bool,
	maxReplicas int32,
) (
	runtime.Object,
	error,
) {
	deployment := &appsv1.Deployment{}
	if err := h.decoder.Decode(req, deployment); err != nil {
		return nil, err
	}

	log := h.Logger.WithValues("deployment", kutil.ObjectKeyForCreateWebhooks(deployment, req))

	if err := h.mutateReplicas(
		log,
		failureToleranceType,
		isHorizontallyScaled,
		deployment,
		deployment.Spec.Replicas,
		func(replicas *int32) { deployment.Spec.Replicas = replicas },
	); err != nil {
		return nil, err
	}

	h.mutateNodeAffinity(
		failureToleranceType,
		zones,
		&deployment.Spec.Template,
	)

	h.mutateTopologySpreadConstraints(
		failureToleranceType,
		zones,
		isHorizontallyScaled,
		deployment.Spec.Replicas,
		maxReplicas,
		&deployment.Spec.Template,
	)

	return deployment, nil
}

func (h *Handler) handleStatefulSet(
	req admission.Request,
	failureToleranceType *gardencorev1beta1.FailureToleranceType,
	zones []string,
	isHorizontallyScaled bool,
	maxReplicas int32,
) (
	runtime.Object,
	error,
) {
	statefulSet := &appsv1.StatefulSet{}
	if err := h.decoder.Decode(req, statefulSet); err != nil {
		return nil, err
	}

	log := h.Logger.WithValues("statefulSet", kutil.ObjectKeyForCreateWebhooks(statefulSet, req))

	if err := h.mutateReplicas(
		log,
		failureToleranceType,
		isHorizontallyScaled,
		statefulSet,
		statefulSet.Spec.Replicas,
		func(replicas *int32) { statefulSet.Spec.Replicas = replicas },
	); err != nil {
		return nil, err
	}

	h.mutateNodeAffinity(
		failureToleranceType,
		zones,
		&statefulSet.Spec.Template,
	)

	h.mutateTopologySpreadConstraints(
		failureToleranceType,
		zones,
		isHorizontallyScaled,
		statefulSet.Spec.Replicas,
		maxReplicas,
		&statefulSet.Spec.Template,
	)

	return statefulSet, nil
}

func (h *Handler) mutateReplicas(
	log logr.Logger,
	failureToleranceType *gardencorev1beta1.FailureToleranceType,
	isHorizontallyScaled bool,
	obj client.Object,
	currentReplicas *int32,
	mutateReplicas func(*int32),
) error {
	// do not mutate replicas if they are set to 0 (hibernation case)
	if pointer.Int32Deref(currentReplicas, 0) == 0 {
		return nil
	}

	replicas := kutil.GetReplicaCount(failureToleranceType, obj.GetLabels()[resourcesv1alpha1.HighAvailabilityConfigType])
	if replicas == nil {
		return nil
	}

	// check if custom replica overwrite is desired
	if replicasOverwrite := obj.GetAnnotations()[resourcesv1alpha1.HighAvailabilityConfigReplicas]; replicasOverwrite != "" {
		v, err := strconv.Atoi(replicasOverwrite)
		if err != nil {
			return err
		}
		replicas = pointer.Int32(int32(v))
	}

	// only mutate replicas if object is not horizontally scaled or if current replica count is lower than what we have
	// computed
	if !isHorizontallyScaled || pointer.Int32Deref(currentReplicas, 0) < *replicas {
		log.Info("Mutating replicas", "replicas", *replicas)
		mutateReplicas(replicas)
	}

	return nil
}

func (h *Handler) isHorizontallyScaled(ctx context.Context, namespace, targetAPIVersion, targetKind, targetName string) (bool, int32, error) {
	if h.TargetVersionGreaterEqual123 {
		hpaList := &autoscalingv2.HorizontalPodAutoscalerList{}
		if err := h.TargetClient.List(ctx, hpaList, client.InNamespace(namespace)); err != nil {
			return false, 0, fmt.Errorf("failed to list all HPAs: %w", err)
		}

		for _, hpa := range hpaList.Items {
			if targetRef := hpa.Spec.ScaleTargetRef; targetRef.APIVersion == targetAPIVersion &&
				targetRef.Kind == targetKind && targetRef.Name == targetName {
				return true, hpa.Spec.MaxReplicas, nil
			}
		}
	} else {
		hpaList := &autoscalingv2beta1.HorizontalPodAutoscalerList{}
		if err := h.TargetClient.List(ctx, hpaList, client.InNamespace(namespace)); err != nil {
			return false, 0, fmt.Errorf("failed to list all HPAs: %w", err)
		}

		for _, hpa := range hpaList.Items {
			if targetRef := hpa.Spec.ScaleTargetRef; targetRef.APIVersion == targetAPIVersion &&
				targetRef.Kind == targetKind && targetRef.Name == targetName {
				return true, hpa.Spec.MaxReplicas, nil
			}
		}
	}

	hvpaList := &hvpav1alpha1.HvpaList{}
	if err := h.TargetClient.List(ctx, hvpaList); err != nil && !meta.IsNoMatchError(err) {
		return false, 0, fmt.Errorf("failed to list all HVPAs: %w", err)
	}

	for _, hvpa := range hvpaList.Items {
		if targetRef := hvpa.Spec.TargetRef; targetRef != nil && targetRef.APIVersion == targetAPIVersion &&
			targetRef.Kind == targetKind && targetRef.Name == targetName && hvpa.Spec.Hpa.Deploy {
			return true, hvpa.Spec.Hpa.Template.Spec.MaxReplicas, nil
		}
	}

	return false, 0, nil
}

func (h *Handler) mutateNodeAffinity(
	failureToleranceType *gardencorev1beta1.FailureToleranceType,
	zones []string,
	podTemplateSpec *corev1.PodTemplateSpec,
) {
	if nodeSelectorTerms := kutil.GetNodeAffinitySelectorTermsForZones(failureToleranceType, zones); nodeSelectorTerms != nil {
		if podTemplateSpec.Spec.Affinity == nil {
			podTemplateSpec.Spec.Affinity = &corev1.Affinity{}
		}

		if podTemplateSpec.Spec.Affinity.NodeAffinity == nil {
			podTemplateSpec.Spec.Affinity.NodeAffinity = &corev1.NodeAffinity{}
		}

		if podTemplateSpec.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
			podTemplateSpec.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{}
		}

		// Filter existing terms with the same expression key to prevent that we are trying to add an expression with
		// the same key multiple times.
		var filteredNodeSelectorTerms []corev1.NodeSelectorTerm
		for _, term := range podTemplateSpec.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
			for _, expr := range term.MatchExpressions {
				if expr.Key != corev1.LabelTopologyZone {
					filteredNodeSelectorTerms = append(filteredNodeSelectorTerms, term)
				}
			}
		}

		podTemplateSpec.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = append(filteredNodeSelectorTerms, nodeSelectorTerms...)
	}
}

func (h *Handler) mutateTopologySpreadConstraints(
	failureToleranceType *gardencorev1beta1.FailureToleranceType,
	zones []string,
	isHorizontallyScaled bool,
	currentReplicas *int32,
	maxReplicas int32,
	podTemplateSpec *corev1.PodTemplateSpec,
) {
	replicas := pointer.Int32Deref(currentReplicas, 0)
	if !isHorizontallyScaled {
		maxReplicas = replicas
	}

	if constraints := kutil.GetTopologySpreadConstraints(replicas, maxReplicas, metav1.LabelSelector{MatchLabels: podTemplateSpec.Labels}, int32(len(zones)), failureToleranceType); constraints != nil {
		// Filter existing constraints with the same topology key to prevent that we are trying to add a constraint with
		// the same key multiple times.
		var filteredConstraints []corev1.TopologySpreadConstraint
		for _, constraint := range podTemplateSpec.Spec.TopologySpreadConstraints {
			if constraint.TopologyKey != corev1.LabelHostname && constraint.TopologyKey != corev1.LabelTopologyZone {
				filteredConstraints = append(filteredConstraints, constraint)
			}
		}

		podTemplateSpec.Spec.TopologySpreadConstraints = append(filteredConstraints, constraints...)
	}
}
