// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardener/pkg/scheduler/apis/config"
	"github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
)

var scheme *runtime.Scheme

func init() {
	scheme = runtime.NewScheme()
	utilruntime.Must(config.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
}

// ConvertSchedulerConfiguration converts the given object to an internal SchedulerConfiguration version.
func ConvertSchedulerConfiguration(obj runtime.Object) (*config.SchedulerConfiguration, error) {
	obj, err := scheme.ConvertToVersion(obj, config.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	result, ok := obj.(*config.SchedulerConfiguration)
	if !ok {
		return nil, fmt.Errorf("could not convert SchedulerConfiguration to the internal version")
	}
	return result, nil
}

// ConvertSchedulerConfigurationExternal converts the given object to an external SchedulerConfiguration version.
func ConvertSchedulerConfigurationExternal(obj runtime.Object) (*v1alpha1.SchedulerConfiguration, error) {
	obj, err := scheme.ConvertToVersion(obj, v1alpha1.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	result, ok := obj.(*v1alpha1.SchedulerConfiguration)
	if !ok {
		return nil, fmt.Errorf("could not convert SchedulerConfiguration to the external version")
	}
	return result, nil
}
