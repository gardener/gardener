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
	"errors"
	"fmt"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/gardenerkubescheduler/configurator"
	schedulerconfigv18 "github.com/gardener/gardener/pkg/operation/botanist/component/gardenerkubescheduler/v18"
	schedulerconfigv19 "github.com/gardener/gardener/pkg/operation/botanist/component/gardenerkubescheduler/v19"
	schedulerconfigv20 "github.com/gardener/gardener/pkg/operation/botanist/component/gardenerkubescheduler/v20"
	schedulerconfigv21 "github.com/gardener/gardener/pkg/operation/botanist/component/gardenerkubescheduler/v21"
	schedulerconfigv22 "github.com/gardener/gardener/pkg/operation/botanist/component/gardenerkubescheduler/v22"
	schedulerconfigv23 "github.com/gardener/gardener/pkg/operation/botanist/component/gardenerkubescheduler/v23"
	"github.com/gardener/gardener/pkg/operation/botanist/component/seedadmissioncontroller"
	"github.com/gardener/gardener/pkg/seedadmissioncontroller/webhooks/admission/podschedulername"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/version"
	schedulerconfigv18v1alpha2 "github.com/gardener/gardener/third_party/kube-scheduler/v18/v1alpha2"
	schedulerconfigv19v1beta1 "github.com/gardener/gardener/third_party/kube-scheduler/v19/v1beta1"
	schedulerconfigv20v1beta1 "github.com/gardener/gardener/third_party/kube-scheduler/v20/v1beta1"
	schedulerconfigv21v1beta1 "github.com/gardener/gardener/third_party/kube-scheduler/v21/v1beta1"
	schedulerconfigv22v1beta2 "github.com/gardener/gardener/third_party/kube-scheduler/v22/v1beta2"
	schedulerconfigv23v1beta3 "github.com/gardener/gardener/third_party/kube-scheduler/v23/v1beta3"

	"github.com/Masterminds/semver"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Bootstrap is used to bootstrap gardener-kube-scheduler in Seed clusters.
func Bootstrap(
	c client.Client,
	secretsManager secretsmanager.Interface,
	seedAdmissionControllerNamespace string,
	image *imagevector.Image,
	seedVersion *semver.Version,
) (
	component.DeployWaiter,
	error,
) {
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
	case version.ConstraintK8sEqual118.Check(seedVersion):
		schedulerConfig := &schedulerconfigv18v1alpha2.KubeSchedulerConfiguration{
			Profiles: []schedulerconfigv18v1alpha2.KubeSchedulerProfile{
				{
					SchedulerName: pointer.String(podschedulername.GardenerShootControlPlaneSchedulerName),
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
				},
			},
		}
		config, err = schedulerconfigv18.NewConfigurator(Name, Name, schedulerConfig)
	case version.ConstraintK8sEqual119.Check(seedVersion):
		schedulerConfig := &schedulerconfigv19v1beta1.KubeSchedulerConfiguration{
			Profiles: []schedulerconfigv19v1beta1.KubeSchedulerProfile{
				{
					SchedulerName: pointer.String(podschedulername.GardenerShootControlPlaneSchedulerName),
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
				},
			},
		}
		config, err = schedulerconfigv19.NewConfigurator(Name, Name, schedulerConfig)
	case version.ConstraintK8sEqual120.Check(seedVersion):
		schedulerConfig := &schedulerconfigv20v1beta1.KubeSchedulerConfiguration{
			Profiles: []schedulerconfigv20v1beta1.KubeSchedulerProfile{
				{
					SchedulerName: pointer.String(podschedulername.GardenerShootControlPlaneSchedulerName),
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
				},
			},
		}
		config, err = schedulerconfigv20.NewConfigurator(Name, Name, schedulerConfig)
	case version.ConstraintK8sEqual121.Check(seedVersion):
		schedulerConfig := &schedulerconfigv21v1beta1.KubeSchedulerConfiguration{
			Profiles: []schedulerconfigv21v1beta1.KubeSchedulerProfile{
				{
					SchedulerName: pointer.String(podschedulername.GardenerShootControlPlaneSchedulerName),
					Plugins: &schedulerconfigv21v1beta1.Plugins{
						Score: &schedulerconfigv21v1beta1.PluginSet{
							Disabled: []schedulerconfigv21v1beta1.Plugin{
								{Name: "NodeResourcesLeastAllocated"},
								{Name: "NodeResourcesBalancedAllocation"},
							},
							Enabled: []schedulerconfigv21v1beta1.Plugin{
								{Name: "NodeResourcesMostAllocated"},
							},
						},
					},
				},
			},
		}
		config, err = schedulerconfigv21.NewConfigurator(Name, Name, schedulerConfig)
	case version.ConstraintK8sEqual122.Check(seedVersion):
		schedulerConfig := &schedulerconfigv22v1beta2.KubeSchedulerConfiguration{
			Profiles: []schedulerconfigv22v1beta2.KubeSchedulerProfile{
				{
					SchedulerName: pointer.String(podschedulername.GardenerShootControlPlaneSchedulerName),
					PluginConfig: []schedulerconfigv22v1beta2.PluginConfig{
						{
							Name: "NodeResourcesFit",
							Args: runtime.RawExtension{
								Object: &schedulerconfigv22v1beta2.NodeResourcesFitArgs{
									ScoringStrategy: &schedulerconfigv22v1beta2.ScoringStrategy{
										Type: schedulerconfigv22v1beta2.MostAllocated,
									},
								},
							},
						},
					},
					Plugins: &schedulerconfigv22v1beta2.Plugins{
						Score: schedulerconfigv22v1beta2.PluginSet{
							Disabled: []schedulerconfigv22v1beta2.Plugin{
								{Name: "NodeResourcesBalancedAllocation"},
							},
						},
					},
				},
			},
		}
		config, err = schedulerconfigv22.NewConfigurator(Name, Name, schedulerConfig)
	case version.ConstraintK8sEqual123.Check(seedVersion),
		// There aren't any significant changes in the kube-scheduler config types between 1.23 and 1.24,
		// that's why the 1.23 types are used for 1.24 as well.
		version.ConstraintK8sEqual124.Check(seedVersion):
		schedulerConfig := &schedulerconfigv23v1beta3.KubeSchedulerConfiguration{
			Profiles: []schedulerconfigv23v1beta3.KubeSchedulerProfile{
				{
					SchedulerName: pointer.String(podschedulername.GardenerShootControlPlaneSchedulerName),
					PluginConfig: []schedulerconfigv23v1beta3.PluginConfig{
						{
							Name: "NodeResourcesFit",
							Args: runtime.RawExtension{
								Object: &schedulerconfigv23v1beta3.NodeResourcesFitArgs{
									ScoringStrategy: &schedulerconfigv23v1beta3.ScoringStrategy{
										Type: schedulerconfigv23v1beta3.MostAllocated,
									},
								},
							},
						},
					},
					Plugins: &schedulerconfigv23v1beta3.Plugins{
						Score: schedulerconfigv23v1beta3.PluginSet{
							Disabled: []schedulerconfigv23v1beta3.Plugin{
								{Name: "NodeResourcesBalancedAllocation"},
							},
						},
					},
				},
			},
		}
		config, err = schedulerconfigv23.NewConfigurator(Name, Name, schedulerConfig)
	default:
		supportedVersion = false
	}

	if err != nil {
		return nil, err
	}

	var caBundleData []byte
	if secretsManager != nil {
		caSecret, found := secretsManager.Get(v1beta1constants.SecretNameCASeed)
		if !found {
			return nil, fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCASeed)
		}
		caBundleData = caSecret.Data[secretutils.DataKeyCertificateBundle]
	}

	scheduler, err := New(
		c,
		Name,
		image,
		seedVersion,
		config,
		&admissionregistrationv1.WebhookClientConfig{
			Service: &admissionregistrationv1.ServiceReference{
				Name:      seedadmissioncontroller.Name,
				Namespace: seedAdmissionControllerNamespace,
				Path:      pointer.String(podschedulername.WebhookPath),
			},
			CABundle: caBundleData,
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
