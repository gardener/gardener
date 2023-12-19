// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
)

var scheme *runtime.Scheme

func init() {
	scheme = runtime.NewScheme()
	utilruntime.Must(config.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
}

// ConvertControllerManagerConfiguration converts the given object to an internal ControllerManagerConfiguration version.
func ConvertControllerManagerConfiguration(obj runtime.Object) (*config.ControllerManagerConfiguration, error) {
	obj, err := scheme.ConvertToVersion(obj, config.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	result, ok := obj.(*config.ControllerManagerConfiguration)
	if !ok {
		return nil, fmt.Errorf("could not convert ControllerManagerConfiguration to the internal version")
	}
	return result, nil
}

// ConvertControllerManagerConfigurationExternal converts the given object to an external ControllerManagerConfiguration.
func ConvertControllerManagerConfigurationExternal(obj runtime.Object) (*v1alpha1.ControllerManagerConfiguration, error) {
	obj, err := scheme.ConvertToVersion(obj, v1alpha1.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	result, ok := obj.(*v1alpha1.ControllerManagerConfiguration)
	if !ok {
		return nil, fmt.Errorf("could not convert ControllerManagerConfiguration to the external version")
	}
	return result, nil
}
