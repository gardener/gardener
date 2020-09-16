// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package genericactuator

import (
	"context"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/common"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// NoopValuesProvider provides no-op implementation of ValuesProvider. This can be anonymously composed by actual ValuesProviders for convenience.
type NoopValuesProvider struct {
	common.ClientContext
}

// GetConfigChartValues returns the values for the config chart applied by this actuator.
func (vp *NoopValuesProvider) GetConfigChartValues(context.Context, *extensionsv1alpha1.ControlPlane, *extensionscontroller.Cluster) (map[string]interface{}, error) {
	return nil, nil
}

// GetControlPlaneChartValues returns the values for the control plane chart applied by this actuator.
func (vp *NoopValuesProvider) GetControlPlaneChartValues(context.Context, *extensionsv1alpha1.ControlPlane, *extensionscontroller.Cluster, map[string]string, bool) (map[string]interface{}, error) {
	return nil, nil
}

// GetControlPlaneShootChartValues returns the values for the control plane shoot chart applied by this actuator.
func (vp *NoopValuesProvider) GetControlPlaneShootChartValues(context.Context, *extensionsv1alpha1.ControlPlane, *extensionscontroller.Cluster, map[string]string) (map[string]interface{}, error) {
	return nil, nil
}

// GetStorageClassesChartValues returns the values for the storage classes chart applied by this actuator.
func (vp *NoopValuesProvider) GetStorageClassesChartValues(context.Context, *extensionsv1alpha1.ControlPlane, *extensionscontroller.Cluster) (map[string]interface{}, error) {
	return nil, nil
}

// GetControlPlaneExposureChartValues returns the values for the control plane exposure chart applied by this actuator.
func (vp *NoopValuesProvider) GetControlPlaneExposureChartValues(context.Context, *extensionsv1alpha1.ControlPlane, *extensionscontroller.Cluster, map[string]string) (map[string]interface{}, error) {
	return nil, nil
}
