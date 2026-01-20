// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"slices"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/logging/fluentbit"
	"github.com/gardener/gardener/pkg/features"
)

// NewFluentBit instantiates a new `Fluent-bit` component.
func NewFluentBit(
	c client.Client,
	gardenNamespaceName string,
	enabled bool,
	valiEnabled bool,
	priorityClassName string,
) (
	deployer component.DeployWaiter,
	err error,
) {
	fluentBitImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameFluentBit)
	if err != nil {
		return nil, err
	}

	fluentBitInitImageName, err := getFluentBitInitImageName()
	if err != nil {
		return nil, err
	}

	deployer = fluentbit.New(
		c,
		gardenNamespaceName,
		fluentbit.Values{
			Image:              fluentBitImage.String(),
			InitContainerImage: fluentBitInitImageName,
			ValiEnabled:        valiEnabled,
			PriorityClassName:  priorityClassName,
		},
	)

	if !enabled {
		deployer = component.OpDestroyAndWait(deployer)
	}

	return deployer, nil
}

// getFluentBitInitImageName returns the Fluent Bit init container image name based on the OpenTelemetryCollector feature gate.
// When the feature gate is disabled, it returns the FluentBitPluginInstaller image, which contains valitail plugin.
// When enabled, it returns the FluentBitPlugin image, which contains OpenTelemetry related plugin.
func getFluentBitInitImageName() (string, error) {
	imageName := imagevector.ContainerImageNameFluentBitPluginInstaller // default image is vali plugin installer

	if slices.ContainsFunc(features.DefaultFeatureGate.KnownFeatures(), func(s string) bool {
		return strings.HasPrefix(s, string(features.OpenTelemetryCollector)+"=")
	}) && features.DefaultFeatureGate.Enabled(features.OpenTelemetryCollector) {
		imageName = imagevector.ContainerImageNameFluentBitPlugin
	}

	image, err := imagevector.Containers().FindImage(imageName)
	if err != nil {
		return "", err
	}

	return image.String(), nil
}
