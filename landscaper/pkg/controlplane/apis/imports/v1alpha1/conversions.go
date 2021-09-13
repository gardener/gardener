// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package v1alpha1

import (
	"fmt"

	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	admissioncontrollerencoding "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/encoding"
	admissioncontrollerconfigv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	controllermanagerencoding "github.com/gardener/gardener/pkg/controllermanager/apis/config/encoding"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	schedulerencoding "github.com/gardener/gardener/pkg/scheduler/apis/config/encoding"
	schedulerconfigv1alpha1 "github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
	"k8s.io/apimachinery/pkg/conversion"
)

func Convert_v1alpha1_GardenerAdmissionController_To_imports_GardenerAdmissionController(in *GardenerAdmissionController, out *imports.GardenerAdmissionController, s conversion.Scope) error {
	// if the object is not set, then we need to manually convert the Raw bytes from runtime.RawExtension to a runtime.Object
	// then set it on the internal version. Otherwise the component config gets lost.
	if in.ComponentConfiguration != nil && in.ComponentConfiguration.Configuration != nil && in.ComponentConfiguration.Configuration.ComponentConfiguration.Object == nil {
		cfg, err := admissioncontrollerencoding.DecodeAdmissionControllerConfigurationFromBytes(in.ComponentConfiguration.Configuration.ComponentConfiguration.Raw, false)
		if err != nil {
			return err
		}
		in.ComponentConfiguration.Configuration.ComponentConfiguration.Object = cfg
	}

	return autoConvert_v1alpha1_GardenerAdmissionController_To_imports_GardenerAdmissionController(in, out, s)
}

func Convert_imports_GardenerAdmissionController_To_v1alpha1_GardenerAdmissionController(in *imports.GardenerAdmissionController, out *GardenerAdmissionController, s conversion.Scope) error {
	if err := autoConvert_imports_GardenerAdmissionController_To_v1alpha1_GardenerAdmissionController(in, out, s); err != nil {
		return err
	}

	// if the rawBytes are not set on the component configuration (runtime.RawExtension), then we need to manually convert the existing configuration of type
	// runtime.Object to bytes.
	if out.ComponentConfiguration != nil && out.ComponentConfiguration.Configuration != nil && out.ComponentConfiguration.Configuration.ComponentConfiguration.Raw == nil {
		cfg, ok := out.ComponentConfiguration.Configuration.ComponentConfiguration.Object.(*admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration)
		if !ok {
			return fmt.Errorf("unknown AdmissionController config object type")
		}
		raw, err := admissioncontrollerencoding.EncodeAdmissionControllerConfigurationToBytes(cfg)
		if err != nil {
			return err
		}
		out.ComponentConfiguration.Configuration.ComponentConfiguration.Raw = raw
	}
	return nil
}

func Convert_v1alpha1_GardenerControllerManager_To_imports_GardenerControllerManager(in *GardenerControllerManager, out *imports.GardenerControllerManager, s conversion.Scope) error {
	if in.ComponentConfiguration != nil && in.ComponentConfiguration.Configuration != nil && in.ComponentConfiguration.Configuration.ComponentConfiguration.Object == nil {
		cfg, err := controllermanagerencoding.DecodeControllerManagerConfigurationFromBytes(in.ComponentConfiguration.Configuration.ComponentConfiguration.Raw, false)
		if err != nil {
			return err
		}
		in.ComponentConfiguration.Configuration.ComponentConfiguration.Object = cfg
	}

	return autoConvert_v1alpha1_GardenerControllerManager_To_imports_GardenerControllerManager(in, out, s)
}

func Convert_imports_GardenerControllerManager_To_v1alpha1_GardenerControllerManager(in *imports.GardenerControllerManager, out *GardenerControllerManager, s conversion.Scope) error {
	if err := autoConvert_imports_GardenerControllerManager_To_v1alpha1_GardenerControllerManager(in, out, s); err != nil {
		return err
	}

	if out.ComponentConfiguration != nil && out.ComponentConfiguration.Configuration != nil && out.ComponentConfiguration.Configuration.ComponentConfiguration.Raw == nil {
		cfg, ok := out.ComponentConfiguration.Configuration.ComponentConfiguration.Object.(*controllermanagerconfigv1alpha1.ControllerManagerConfiguration)
		if !ok {
			return fmt.Errorf("unknown ControllerManager config object type")
		}
		raw, err := controllermanagerencoding.EncodeControllerManagerConfigurationToBytes(cfg)
		if err != nil {
			return err
		}
		out.ComponentConfiguration.Configuration.ComponentConfiguration.Raw = raw
	}
	return nil
}

func Convert_v1alpha1_GardenerScheduler_To_imports_GardenerScheduler(in *GardenerScheduler, out *imports.GardenerScheduler, s conversion.Scope) error {
	if in.ComponentConfiguration != nil && in.ComponentConfiguration.Configuration != nil && in.ComponentConfiguration.Configuration.ComponentConfiguration.Object == nil {
		cfg, err := schedulerencoding.DecodeSchedulerConfigurationFromBytes(in.ComponentConfiguration.Configuration.ComponentConfiguration.Raw, false)
		if err != nil {
			return err
		}
		in.ComponentConfiguration.Configuration.ComponentConfiguration.Object = cfg
	}

	return autoConvert_v1alpha1_GardenerScheduler_To_imports_GardenerScheduler(in, out, s)
}

func Convert_imports_GardenerScheduler_To_v1alpha1_GardenerScheduler(in *imports.GardenerScheduler, out *GardenerScheduler, s conversion.Scope) error {
	if err := autoConvert_imports_GardenerScheduler_To_v1alpha1_GardenerScheduler(in, out, s); err != nil {
		return err
	}

	if out.ComponentConfiguration != nil && out.ComponentConfiguration.Configuration != nil && out.ComponentConfiguration.Configuration.ComponentConfiguration.Raw == nil {
		cfg, ok := out.ComponentConfiguration.Configuration.ComponentConfiguration.Object.(*schedulerconfigv1alpha1.SchedulerConfiguration)
		if !ok {
			return fmt.Errorf("unknown Scheduler config object type")
		}
		raw, err := schedulerencoding.EncodeSchedulerConfigurationToBytes(cfg)
		if err != nil {
			return err
		}
		out.ComponentConfiguration.Configuration.ComponentConfiguration.Raw = raw
	}
	return nil
}
