// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagent

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

// OperatingSystemConfigSecret returns a Kubernetes secret object containing the OperatingSystemConfig that
// gardener-node-agent will read and reconcile on the worker machines.
func OperatingSystemConfigSecret(
	ctx context.Context,
	seedClient client.Client,
	osc *extensionsv1alpha1.OperatingSystemConfig,
	secretName string,
	workerPoolName string,
) (
	*corev1.Secret,
	error,
) {
	// This OperatingSystemConfig object should only contain the data relevant for gardener-node-agent reconciliation to
	// prevent undesired changes of the computed checksum of this object.
	operatingSystemConfig := &extensionsv1alpha1.OperatingSystemConfig{
		Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
			Units:          osc.Spec.Units,
			Files:          osc.Spec.Files,
			CRIConfig:      osc.Spec.CRIConfig,
			InPlaceUpdates: osc.Spec.InPlaceUpdates,
		},
		Status: extensionsv1alpha1.OperatingSystemConfigStatus{
			ExtensionUnits: osc.Status.ExtensionUnits,
			ExtensionFiles: osc.Status.ExtensionFiles,
			InPlaceUpdates: osc.Status.InPlaceUpdates,
		},
	}

	// The OperatingSystemConfig will be deployed to the shoot to get processed by gardener-node-agent. It doesn't
	// have access to the referenced secrets (stored in the shoot namespace in the seed), hence we have to translate
	// all references into inline content.
	for i, file := range operatingSystemConfig.Spec.Files {
		if file.Content.SecretRef == nil {
			continue
		}

		secret := &corev1.Secret{}
		if err := seedClient.Get(ctx, client.ObjectKey{Name: file.Content.SecretRef.Name, Namespace: osc.Namespace}, secret); err != nil {
			return nil, fmt.Errorf("cannot resolve secret ref from osc: %w", err)
		}

		operatingSystemConfig.Spec.Files[i].Content.SecretRef = nil
		operatingSystemConfig.Spec.Files[i].Content.Inline = &extensionsv1alpha1.FileContentInline{
			Encoding: "b64",
			Data:     utils.EncodeBase64(secret.Data[file.Content.SecretRef.DataKey]),
		}
	}

	operatingSystemConfigRaw, err := runtime.Encode(codec, operatingSystemConfig)
	if err != nil {
		return nil, fmt.Errorf("failed encoding OperatingSystemConfig: %w", err)
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: metav1.NamespaceSystem,
			Annotations: map[string]string{
				nodeagentconfigv1alpha1.AnnotationKeyChecksumDownloadedOperatingSystemConfig: utils.ComputeSHA256Hex(operatingSystemConfigRaw),
			},
			Labels: map[string]string{
				v1beta1constants.GardenRole:      v1beta1constants.GardenRoleOperatingSystemConfig,
				v1beta1constants.LabelWorkerPool: workerPoolName,
			},
		},
		Data: map[string][]byte{nodeagentconfigv1alpha1.DataKeyOperatingSystemConfig: operatingSystemConfigRaw},
	}, nil
}
