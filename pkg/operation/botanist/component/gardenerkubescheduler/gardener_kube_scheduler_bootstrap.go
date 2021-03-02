// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardenerkubescheduler

import (
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/gardenerkubescheduler/configurator"
	schedulerconfigv18 "github.com/gardener/gardener/pkg/operation/botanist/component/gardenerkubescheduler/v18"
	schedulerconfigv19 "github.com/gardener/gardener/pkg/operation/botanist/component/gardenerkubescheduler/v19"
	schedulerconfigv20 "github.com/gardener/gardener/pkg/operation/botanist/component/gardenerkubescheduler/v20"
	"github.com/gardener/gardener/pkg/operation/botanist/component/seedadmissioncontroller"
	"github.com/gardener/gardener/pkg/seedadmissioncontroller/webhooks/admission/podschedulername"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	schedulerconfigv18v1alpha2 "github.com/gardener/gardener/third_party/kube-scheduler/v18/v1alpha2"
	schedulerconfigv19v1beta1 "github.com/gardener/gardener/third_party/kube-scheduler/v19/v1beta1"
	schedulerconfigv20v1beta1 "github.com/gardener/gardener/third_party/kube-scheduler/v20/v1beta1"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Bootstrap is used to bootstrap gardener-kube-scheduler in Seed clusters.
func Bootstrap(
	c client.Client,
	seedAdmissionControllerNamespace string,
	image *imagevector.Image,
	seedVersion *semver.Version,
) (
	component.DeployWaiter,
	error,
) {
	if c == nil {
		return nil, errors.New("client is required")
	}

	if len(seedAdmissionControllerNamespace) == 0 {
		return nil, errors.New("seedAdmissionControllerNamespace is required")
	}

	if seedVersion == nil {
		return nil, errors.New("seedVersion is required")
	}

	var (
		config           = configurator.NoOp()
		err              error
		supportedVersion = true
	)

	switch {
	case versionConstraintEqual118.Check(seedVersion):
		config, err = schedulerconfigv18.NewConfigurator(Name, Name, &schedulerconfigv18v1alpha2.KubeSchedulerConfiguration{
			Profiles: []schedulerconfigv18v1alpha2.KubeSchedulerProfile{{
				SchedulerName: pointer.StringPtr(podschedulername.GardenerShootControlPlaneSchedulerName),
				Plugins: &schedulerconfigv18v1alpha2.Plugins{
					Score: &schedulerconfigv18v1alpha2.PluginSet{
						Disabled: []schedulerconfigv18v1alpha2.Plugin{
							{Name: "NodeResourcesLeastAllocated"},
							{Name: "NodeResourcesBalancedAllocation"},
						},
						Enabled: []schedulerconfigv18v1alpha2.Plugin{
							{Name: "NodeResourcesMostAllocated"},
						},
					},
				},
			}},
		})
	case versionConstraintEqual119.Check(seedVersion):
		config, err = schedulerconfigv19.NewConfigurator(Name, Name, &schedulerconfigv19v1beta1.KubeSchedulerConfiguration{
			Profiles: []schedulerconfigv19v1beta1.KubeSchedulerProfile{{
				SchedulerName: pointer.StringPtr(podschedulername.GardenerShootControlPlaneSchedulerName),
				Plugins: &schedulerconfigv19v1beta1.Plugins{
					Score: &schedulerconfigv19v1beta1.PluginSet{
						Disabled: []schedulerconfigv19v1beta1.Plugin{
							{Name: "NodeResourcesLeastAllocated"},
							{Name: "NodeResourcesBalancedAllocation"},
						},
						Enabled: []schedulerconfigv19v1beta1.Plugin{
							{Name: "NodeResourcesMostAllocated"},
						},
					},
				},
			}},
		})
	case versionConstraintEqual120.Check(seedVersion):
		config, err = schedulerconfigv20.NewConfigurator(Name, Name, &schedulerconfigv20v1beta1.KubeSchedulerConfiguration{
			Profiles: []schedulerconfigv20v1beta1.KubeSchedulerProfile{{
				SchedulerName: pointer.StringPtr(podschedulername.GardenerShootControlPlaneSchedulerName),
				Plugins: &schedulerconfigv20v1beta1.Plugins{
					Score: &schedulerconfigv20v1beta1.PluginSet{
						Disabled: []schedulerconfigv20v1beta1.Plugin{
							{Name: "NodeResourcesLeastAllocated"},
							{Name: "NodeResourcesBalancedAllocation"},
						},
						Enabled: []schedulerconfigv20v1beta1.Plugin{
							{Name: "NodeResourcesMostAllocated"},
						},
					},
				},
			}},
		})
	default:
		supportedVersion = false
	}

	if err != nil {
		return nil, err
	}

	scheduler, err := New(
		c,
		Name,
		image,
		config,
		&admissionregistrationv1beta1.WebhookClientConfig{
			Service: &admissionregistrationv1beta1.ServiceReference{
				Name:      seedadmissioncontroller.Name,
				Namespace: seedAdmissionControllerNamespace,
				Path:      pointer.StringPtr(podschedulername.WebhookPath),
			},
			CABundle: []byte(seedadmissioncontroller.TLSCACert),
		},
	)
	if err != nil {
		return nil, err
	}

	if supportedVersion && gardenletfeatures.FeatureGate.Enabled(features.SeedKubeScheduler) {
		return scheduler, nil
	}

	return component.OpDestroy(scheduler), nil
}
