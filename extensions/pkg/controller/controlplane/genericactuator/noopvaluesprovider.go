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

var _ ValuesProvider = &NoopValuesProvider{}

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

// GetControlPlaneShootCRDsChartValues returns the values for the control plane shoot CRDs chart applied by this actuator.
func (vp *NoopValuesProvider) GetControlPlaneShootCRDsChartValues(context.Context, *extensionsv1alpha1.ControlPlane, *extensionscontroller.Cluster) (map[string]interface{}, error) {
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
