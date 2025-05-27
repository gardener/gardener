// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package genericactuator

import (
	"context"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// NoopValuesProvider provides no-op implementation of ValuesProvider. This can be anonymously composed by actual ValuesProviders for convenience.
type NoopValuesProvider struct{}

var _ ValuesProvider = NoopValuesProvider{}

// GetConfigChartValues returns the values for the config chart applied by this actuator.
func (vp NoopValuesProvider) GetConfigChartValues(context.Context, *extensionsv1alpha1.ControlPlane, *extensionscontroller.Cluster) (map[string]any, error) {
	return nil, nil
}

// GetControlPlaneChartValues returns the values for the control plane chart applied by this actuator.
func (vp NoopValuesProvider) GetControlPlaneChartValues(context.Context, *extensionsv1alpha1.ControlPlane, *extensionscontroller.Cluster, secretsmanager.Reader, map[string]string, bool) (map[string]any, error) {
	return nil, nil
}

// GetControlPlaneShootChartValues returns the values for the control plane shoot chart applied by this actuator.
func (vp NoopValuesProvider) GetControlPlaneShootChartValues(context.Context, *extensionsv1alpha1.ControlPlane, *extensionscontroller.Cluster, secretsmanager.Reader, map[string]string) (map[string]any, error) {
	return nil, nil
}

// GetControlPlaneShootCRDsChartValues returns the values for the control plane shoot CRDs chart applied by this actuator.
func (vp NoopValuesProvider) GetControlPlaneShootCRDsChartValues(context.Context, *extensionsv1alpha1.ControlPlane, *extensionscontroller.Cluster) (map[string]any, error) {
	return nil, nil
}

// GetStorageClassesChartValues returns the values for the storage classes chart applied by this actuator.
func (vp NoopValuesProvider) GetStorageClassesChartValues(context.Context, *extensionsv1alpha1.ControlPlane, *extensionscontroller.Cluster) (map[string]any, error) {
	return nil, nil
}

// GetControlPlaneExposureChartValues returns the values for the control plane exposure chart applied by this actuator.
//
// Deprecated: Control plane with purpose `exposure` is being deprecated and will be removed in gardener v1.123.0.
// TODO(theoddora): Remove this function in v1.123.0 when the Purpose field is removed.
func (vp NoopValuesProvider) GetControlPlaneExposureChartValues(context.Context, *extensionsv1alpha1.ControlPlane, *extensionscontroller.Cluster, secretsmanager.Reader, map[string]string) (map[string]any, error) {
	return nil, nil
}
