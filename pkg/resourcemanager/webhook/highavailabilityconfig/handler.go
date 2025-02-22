// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package highavailabilityconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Handler handles admission requests and sets the following fields based on the failure tolerance type and the
// component type:
// - `.spec.replicas`
// - `.spec.template.spec.affinity`
// - `.spec.template.spec.topologySpreadConstraints`
type Handler struct {
	Logger       logr.Logger
	TargetClient client.Reader
	Config       resourcemanagerconfigv1alpha1.HighAvailabilityConfigWebhookConfig
	Decoder      admission.Decoder
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
		isZonePinningEnabled bool
	)

	if v, ok := namespace.Annotations[resourcesv1alpha1.HighAvailabilityConfigFailureToleranceType]; ok {
		value := gardencorev1beta1.FailureToleranceType(v)
		failureToleranceType = &value
	}

	if v, ok := namespace.Annotations[resourcesv1alpha1.HighAvailabilityConfigZones]; ok {
		zones = sets.List(sets.New(strings.Split(v, ",")...).Delete(""))
	}

	if v, err := strconv.ParseBool(namespace.Annotations[resourcesv1alpha1.HighAvailabilityConfigZonePinning]); err == nil {
		isZonePinningEnabled = v
	}

	isHorizontallyScaled, maxReplicas, err := h.isHorizontallyScaled(ctx, req.Namespace, schema.GroupVersion{Group: req.Kind.Group, Version: req.Kind.Version}.String(), req.Kind.Kind, req.Name)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	switch requestGK {
	case appsv1.SchemeGroupVersion.WithKind("Deployment").GroupKind():
		obj, err = h.handleDeployment(req, failureToleranceType, zones, isHorizontallyScaled, maxReplicas, isZonePinningEnabled)
	case appsv1.SchemeGroupVersion.WithKind("StatefulSet").GroupKind():
		obj, err = h.handleStatefulSet(req, failureToleranceType, zones, isHorizontallyScaled, maxReplicas, isZonePinningEnabled)
	case autoscalingv2.SchemeGroupVersion.WithKind("HorizontalPodAutoscaler").GroupKind():
		obj, err = h.handleHorizontalPodAutoscaler(req, failureToleranceType)
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
	isZonePinningEnabled bool,
) (
	runtime.Object,
	error,
) {
	deployment := &appsv1.Deployment{}
	if err := h.Decoder.Decode(req, deployment); err != nil {
		return nil, err
	}

	log := h.Logger.WithValues("deployment", kubernetesutils.ObjectKeyForCreateWebhooks(deployment, req))

	if err := mutateReplicas(
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
		// TODO(ScheererJ): Remove "failureToleranceType != nil" after the shoot namespaces have been annotated with
		//  "zone-pinning=enabled" as well (today, only the istio-ingress namespaces have this annotation).
		failureToleranceType != nil || isZonePinningEnabled,
		zones,
		&deployment.Spec.Template,
	)

	h.mutateTopologySpreadConstraints(
		"Deployment",
		failureToleranceType,
		zones,
		isHorizontallyScaled,
		deployment.Spec.Replicas,
		maxReplicas,
		&deployment.Spec.Template,
		deployment.Annotations,
		metav1.LabelSelector{MatchLabels: deployment.Spec.Template.Labels},
	)

	h.mutatePodTolerationSeconds(
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
	isZonePinningEnabled bool,
) (
	runtime.Object,
	error,
) {
	statefulSet := &appsv1.StatefulSet{}
	if err := h.Decoder.Decode(req, statefulSet); err != nil {
		return nil, err
	}

	log := h.Logger.WithValues("statefulSet", kubernetesutils.ObjectKeyForCreateWebhooks(statefulSet, req))

	if err := mutateReplicas(
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
		// TODO(ScheererJ): Remove "failureToleranceType != nil" after the shoot namespaces have been annotated with
		//  "zone-pinning=enabled" as well (today, only the istio-ingress namespaces have this annotation).
		failureToleranceType != nil || isZonePinningEnabled,
		zones,
		&statefulSet.Spec.Template,
	)

	// The label selector for TSCs must be stable, so we use the immutable field 'statefulSet.Spec.Selector'.
	// If labels ('statefulSet.Spec.Template.Labels') change over time, the TSCs must still select the pods which
	// were not updated yet.
	// Please see https://github.com/gardener/etcd-druid/issues/899 for why this is especially important for StatefulSets.
	h.mutateTopologySpreadConstraints(
		"StatefulSet",
		failureToleranceType,
		zones,
		isHorizontallyScaled,
		statefulSet.Spec.Replicas,
		maxReplicas,
		&statefulSet.Spec.Template,
		statefulSet.Annotations,
		*statefulSet.Spec.Selector,
	)

	h.mutatePodTolerationSeconds(
		&statefulSet.Spec.Template,
	)

	return statefulSet, nil
}

func (h *Handler) handleHorizontalPodAutoscaler(req admission.Request, failureToleranceType *gardencorev1beta1.FailureToleranceType) (runtime.Object, error) {
	switch req.Kind.Version {
	case autoscalingv2beta1.SchemeGroupVersion.Version:
		hpa := &autoscalingv2beta1.HorizontalPodAutoscaler{}
		if err := h.Decoder.Decode(req, hpa); err != nil {
			return nil, err
		}

		log := h.Logger.WithValues("hpa", kubernetesutils.ObjectKeyForCreateWebhooks(hpa, req))

		if err := mutateAutoscalingReplicas(
			log,
			failureToleranceType,
			hpa,
			func() *int32 { return hpa.Spec.MinReplicas },
			func(n *int32) { hpa.Spec.MinReplicas = n },
			func() int32 { return hpa.Spec.MaxReplicas },
			func(n int32) { hpa.Spec.MaxReplicas = n },
		); err != nil {
			return nil, err
		}

		return hpa, nil
	case autoscalingv2.SchemeGroupVersion.Version:
		hpa := &autoscalingv2.HorizontalPodAutoscaler{}
		if err := h.Decoder.Decode(req, hpa); err != nil {
			return nil, err
		}

		log := h.Logger.WithValues("hpa", kubernetesutils.ObjectKeyForCreateWebhooks(hpa, req))

		if err := mutateAutoscalingReplicas(
			log,
			failureToleranceType,
			hpa,
			func() *int32 { return hpa.Spec.MinReplicas },
			func(n *int32) { hpa.Spec.MinReplicas = n },
			func() int32 { return hpa.Spec.MaxReplicas },
			func(n int32) { hpa.Spec.MaxReplicas = n },
		); err != nil {
			return nil, err
		}

		return hpa, nil
	default:
		return nil, fmt.Errorf("autoscaling version %q in request is not supported", req.Kind.Version)
	}
}

func mutateReplicas(
	log logr.Logger,
	failureToleranceType *gardencorev1beta1.FailureToleranceType,
	isHorizontallyScaled bool,
	obj client.Object,
	currentReplicas *int32,
	setReplicas func(*int32),
) error {
	replicas, err := getReplicaCount(obj, currentReplicas, failureToleranceType)
	if err != nil {
		return err
	}
	if replicas == nil {
		return nil
	}

	// only mutate replicas if object is not horizontally scaled or if current replica count is lower than what we have
	// computed
	if !isHorizontallyScaled || ptr.Deref(currentReplicas, 0) < *replicas {
		log.Info("Mutating replicas", "replicas", *replicas)
		setReplicas(replicas)
	}

	return nil
}

func getReplicaCount(obj client.Object, currentOrMinReplicas *int32, failureToleranceType *gardencorev1beta1.FailureToleranceType) (*int32, error) {
	// do not mutate replicas if they are set to 0 (hibernation case or HPA disabled)
	if ptr.Deref(currentOrMinReplicas, 0) == 0 {
		return nil, nil
	}

	replicas := kubernetesutils.GetReplicaCount(failureToleranceType, obj.GetLabels()[resourcesv1alpha1.HighAvailabilityConfigType])
	if replicas == nil {
		return nil, nil
	}

	// check if custom replica overwrite is desired
	if replicasOverwrite := obj.GetAnnotations()[resourcesv1alpha1.HighAvailabilityConfigReplicas]; replicasOverwrite != "" {
		v, err := strconv.Atoi(replicasOverwrite)
		if err != nil {
			return nil, err
		}
		replicas = ptr.To(int32(v)) // #nosec: G109 G115 - There is a validation for `replicas` in `Deployments` and `StatefulSets` which limits their value range.
	}
	return replicas, nil
}

func (h *Handler) isHorizontallyScaled(ctx context.Context, namespace, targetAPIVersion, targetKind, targetName string) (bool, int32, error) {
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

	return false, 0, nil
}

func (h *Handler) mutateNodeAffinity(
	isZonePinningEnabled bool,
	zones []string,
	podTemplateSpec *corev1.PodTemplateSpec,
) {
	if nodeSelectorRequirement := kubernetesutils.GetNodeSelectorRequirementForZones(isZonePinningEnabled, zones); nodeSelectorRequirement != nil {
		if podTemplateSpec.Spec.Affinity == nil {
			podTemplateSpec.Spec.Affinity = &corev1.Affinity{}
		}

		if podTemplateSpec.Spec.Affinity.NodeAffinity == nil {
			podTemplateSpec.Spec.Affinity.NodeAffinity = &corev1.NodeAffinity{}
		}

		if podTemplateSpec.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
			podTemplateSpec.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{}
		}

		if len(podTemplateSpec.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms) == 0 {
			podTemplateSpec.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = []corev1.NodeSelectorTerm{{}}
		}

		for i, term := range podTemplateSpec.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
			filteredExpressions := make([]corev1.NodeSelectorRequirement, 0, len(term.MatchExpressions))
			// Remove existing expressions for `topology.kubernetes.io/zone` to
			// - avoid duplicates for the same key
			// - remove expressions that prevent zone pinning
			for _, expr := range term.MatchExpressions {
				if expr.Key != corev1.LabelTopologyZone {
					filteredExpressions = append(filteredExpressions, expr)
				}
			}

			// Add remaining expressions with intended zone expression.
			podTemplateSpec.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[i].MatchExpressions = append(filteredExpressions, *nodeSelectorRequirement)
		}
	}
}

func (h *Handler) mutateTopologySpreadConstraints(
	kind string,
	failureToleranceType *gardencorev1beta1.FailureToleranceType,
	zones []string,
	isHorizontallyScaled bool,
	currentReplicas *int32,
	maxReplicas int32,
	podTemplateSpec *corev1.PodTemplateSpec,
	annotations map[string]string,
	labelSelector metav1.LabelSelector,
) {
	replicas := ptr.Deref(currentReplicas, 0)

	// Set maxReplicas to replicas if component is not scaled horizontally or of the replica count is higher than maxReplicas
	// which can happen if the involved HPA object is not mutated yet.
	if !isHorizontallyScaled || replicas > maxReplicas {
		maxReplicas = replicas
	}

	enforceSpreadAcrossHosts := false
	if b, err := strconv.ParseBool(annotations[resourcesv1alpha1.HighAvailabilityConfigHostSpread]); err == nil {
		enforceSpreadAcrossHosts = b
	}

	if constraints := kubernetesutils.GetTopologySpreadConstraints(
		replicas,
		maxReplicas,
		labelSelector,
		int32(len(zones)), // #nosec G115 -- `len(zones)` cannot be higher than max int32. Zones come from shoot spec and there is a validation that there cannot be more zones than worker.Maximum which is int32.
		failureToleranceType,
		enforceSpreadAcrossHosts,
	); constraints != nil {
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

	if kind == "Deployment" {
		kubernetesutils.MutateMatchLabelKeys(podTemplateSpec.Spec.TopologySpreadConstraints)
	}
}

func (h *Handler) mutatePodTolerationSeconds(podTemplateSpec *corev1.PodTemplateSpec) {
	var (
		toleratesNodeNotReady    bool
		toleratesNodeUnreachable bool
	)

	// Check if toleration is already specific in podTemplate.
	for _, toleration := range podTemplateSpec.Spec.Tolerations {
		if len(toleration.Effect) > 0 && toleration.Effect != corev1.TaintEffectNoExecute {
			continue
		}

		switch toleration.Key {
		case corev1.TaintNodeNotReady:
			toleratesNodeNotReady = true
		case corev1.TaintNodeUnreachable:
			toleratesNodeUnreachable = true
		}
	}

	if !toleratesNodeNotReady && h.Config.DefaultNotReadyTolerationSeconds != nil {
		podTemplateSpec.Spec.Tolerations = append(podTemplateSpec.Spec.Tolerations, corev1.Toleration{
			Key:               corev1.TaintNodeNotReady,
			Operator:          corev1.TolerationOpExists,
			Effect:            corev1.TaintEffectNoExecute,
			TolerationSeconds: h.Config.DefaultNotReadyTolerationSeconds,
		})
	}

	if !toleratesNodeUnreachable && h.Config.DefaultUnreachableTolerationSeconds != nil {
		podTemplateSpec.Spec.Tolerations = append(podTemplateSpec.Spec.Tolerations, corev1.Toleration{
			Key:               corev1.TaintNodeUnreachable,
			Operator:          corev1.TolerationOpExists,
			Effect:            corev1.TaintEffectNoExecute,
			TolerationSeconds: h.Config.DefaultUnreachableTolerationSeconds,
		})
	}
}

func mutateAutoscalingReplicas(
	log logr.Logger,
	failureToleranceType *gardencorev1beta1.FailureToleranceType,
	obj client.Object,
	getMinReplicas func() *int32,
	setMinReplicas func(*int32),
	getMaxReplicas func() int32,
	setMaxReplicas func(int32),
) error {
	replicas, err := getReplicaCount(obj, getMinReplicas(), failureToleranceType)
	if err != nil {
		return err
	}
	if replicas == nil {
		return nil
	}

	// For compatibility reasons, only overwrite minReplicas if the current count is lower than the calculated count.
	if ptr.Deref(getMinReplicas(), 0) < *replicas {
		log.Info("Mutating minReplicas", "minReplicas", replicas)
		setMinReplicas(replicas)
	}

	if getMaxReplicas() < ptr.Deref(getMinReplicas(), 0) {
		log.Info("Mutating maxReplicas", "maxReplicas", replicas)
		setMaxReplicas(*replicas)
	}

	return nil
}
