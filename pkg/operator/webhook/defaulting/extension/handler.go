// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// Handler performs defaulting.
type Handler struct {
	Decoder admission.Decoder
}

// Handle performs the defaulting.
func (h *Handler) Handle(_ context.Context, req admission.Request) admission.Response {
	extension := &operatorv1alpha1.Extension{}
	if err := h.Decoder.Decode(req, extension); err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("unable to decode request to extension object: %w", err))
	}

	if slices.ContainsFunc(extension.Spec.Resources, func(resource gardencorev1beta1.ControllerResource) bool {
		return resource.Kind == extensionsv1alpha1.WorkerResource
	}) && extension.Spec.Deployment != nil && extension.Spec.Deployment.ExtensionDeployment != nil && extension.Spec.Deployment.ExtensionDeployment.InjectGardenKubeconfig == nil {
		extension.Spec.Deployment.ExtensionDeployment.InjectGardenKubeconfig = ptr.To(true)
	}

	if req.Operation == admissionv1.Update {
		extensionOld := &operatorv1alpha1.Extension{}
		if err := h.Decoder.DecodeRaw(req.OldObject, extensionOld); err != nil {
			return admission.Errored(http.StatusInternalServerError, fmt.Errorf("unable to decode request to old extension object: %w", err))
		}

		handleAutoEnabledResources(extensionOld, extension)
	}

	for i, resource := range extension.Spec.Resources {
		if resource.Primary == nil {
			extension.Spec.Resources[i].Primary = ptr.To(true)
		}

		if resource.Kind == extensionsv1alpha1.ExtensionResource {
			if ptr.Deref(resource.GloballyEnabled, false) && !slices.Contains(resource.AutoEnable, gardencorev1beta1.ClusterTypeShoot) {
				extension.Spec.Resources[i].AutoEnable = append(extension.Spec.Resources[i].AutoEnable, gardencorev1beta1.ClusterTypeShoot)
			}

			// only maintain GloballyEnabled if it was not set before
			if resource.GloballyEnabled != nil {
				extension.Spec.Resources[i].GloballyEnabled = ptr.To(slices.Contains(extension.Spec.Resources[i].AutoEnable, gardencorev1beta1.ClusterTypeShoot))
			}

			if len(resource.ClusterCompatibility) == 0 && slices.Contains(extension.Spec.Resources[i].AutoEnable, gardencorev1beta1.ClusterTypeShoot) {
				extension.Spec.Resources[i].ClusterCompatibility = []gardencorev1beta1.ClusterType{gardencorev1beta1.ClusterTypeShoot}
			}
		}
	}

	marshalled, err := json.Marshal(extension)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshalled)
}

func handleAutoEnabledResources(oldExtension, extension *operatorv1alpha1.Extension) {
	resourceKindTypeToResource := map[string]gardencorev1beta1.ControllerResource{}
	for _, resource := range oldExtension.Spec.Resources {
		if resource.Kind != extensionsv1alpha1.ExtensionResource {
			continue
		}

		resourceKindTypeToResource[gardenerutils.ExtensionsID(resource.Kind, resource.Type)] = resource
	}

	for i, resource := range extension.Spec.Resources {
		var (
			oldResource gardencorev1beta1.ControllerResource
			ok          bool
		)

		if oldResource, ok = resourceKindTypeToResource[gardenerutils.ExtensionsID(resource.Kind, resource.Type)]; !ok {
			continue
		}

		// When globallyEnabled was set from true to false, the shoot type must be removed from the autoEnable list.
		// Don't do anything if globallyEnabled was not set before, as this means a conflict and field AutoEnable should take precedence in this case.
		if oldResource.GloballyEnabled != nil && *oldResource.GloballyEnabled && resource.GloballyEnabled != nil && !*resource.GloballyEnabled {
			extension.Spec.Resources[i].AutoEnable = slices.DeleteFunc(resource.AutoEnable, func(clusterType gardencorev1beta1.ClusterType) bool {
				return clusterType == gardencorev1beta1.ClusterTypeShoot
			})
		}
	}
}
