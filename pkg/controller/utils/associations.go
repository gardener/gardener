// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package utils

import (
	"fmt"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/gardener/gardener/pkg/logger"
	"k8s.io/apimachinery/pkg/labels"
)

// DetermineShootAssociations gets a <shootLister> to determine the Shoots resources which are associated
// to given <obj> (either a CloudProfile a or a Seed object).
func DetermineShootAssociations(obj interface{}, shootLister gardenlisters.ShootLister) ([]string, error) {
	var associatedShoots []string
	shoots, err := shootLister.List(labels.Everything())
	if err != nil {
		logger.Logger.Info(err.Error())
		return nil, err
	}

	for _, shoot := range shoots {
		switch t := obj.(type) {
		case *gardenv1beta1.CloudProfile:
			cloudProfile := obj.(*gardenv1beta1.CloudProfile)
			if shoot.Spec.Cloud.Profile == cloudProfile.Name {
				associatedShoots = append(associatedShoots, fmt.Sprintf("%s/%s", shoot.Namespace, shoot.Name))
			}
		case *gardenv1beta1.Seed:
			seed := obj.(*gardenv1beta1.Seed)
			if shoot.Spec.Cloud.Seed != nil && *shoot.Spec.Cloud.Seed == seed.Name {
				associatedShoots = append(associatedShoots, fmt.Sprintf("%s/%s", shoot.Namespace, shoot.Name))
			}
		case *gardenv1beta1.SecretBinding:
			binding := obj.(*gardenv1beta1.SecretBinding)
			if shoot.Spec.Cloud.SecretBindingRef.Name == binding.Name && shoot.Namespace == binding.Namespace {
				associatedShoots = append(associatedShoots, fmt.Sprintf("%s/%s", shoot.Namespace, shoot.Name))
			}
		default:
			return nil, fmt.Errorf("Unable to determine Shoot associations, due to unknown type %t", t)
		}
	}
	return associatedShoots, nil
}

// DetermineSecretBindingAssociations gets a <bindingLister> to determine the SecretBinding
// resources which are associated to given Quota <obj>.
func DetermineSecretBindingAssociations(quota *gardenv1beta1.Quota, bindingLister gardenlisters.SecretBindingLister) ([]string, error) {
	var associatedBindings []string
	bindings, err := bindingLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	for _, binding := range bindings {
		for _, quotaRef := range binding.Quotas {
			if quotaRef.Name == quota.Name && quotaRef.Namespace == quota.Namespace {
				associatedBindings = append(associatedBindings, fmt.Sprintf("%s/%s", binding.Namespace, binding.Name))
			}
		}
	}
	return associatedBindings, nil
}
