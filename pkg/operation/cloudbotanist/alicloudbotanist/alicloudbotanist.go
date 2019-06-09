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

package alicloudbotanist

import (
	"errors"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"

	"github.com/gardener/gardener-extensions/controllers/provider-alicloud/pkg/apis/alicloud"
	alicloudv1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-alicloud/pkg/apis/alicloud/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

// IMPORTANT NOTICE
// The following part is only temporarily needed until we have completed the Extensibility epic
// and moved out all provider specifics.
// IMPORTANT NOTICE

var (
	scheme  *runtime.Scheme
	decoder runtime.Decoder
)

func init() {
	scheme = runtime.NewScheme()

	// Workaround for incompatible kubernetes dependencies in gardener/gardener and
	// gardener/gardener-extensions.
	alicloudSchemeBuilder := runtime.NewSchemeBuilder(func(scheme *runtime.Scheme) error {
		scheme.AddKnownTypes(alicloud.SchemeGroupVersion, &alicloud.InfrastructureConfig{}, &alicloud.InfrastructureStatus{})
		return nil
	})
	alicloudv1alpha1SchemeBuilder := runtime.NewSchemeBuilder(func(scheme *runtime.Scheme) error {
		scheme.AddKnownTypes(alicloudv1alpha1.SchemeGroupVersion, &alicloudv1alpha1.InfrastructureConfig{}, &alicloudv1alpha1.InfrastructureStatus{})
		return nil
	})
	schemeBuilder := runtime.NewSchemeBuilder(
		alicloudv1alpha1SchemeBuilder.AddToScheme,
		alicloudSchemeBuilder.AddToScheme,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))

	decoder = serializer.NewCodecFactory(scheme).UniversalDecoder()
}

// IMPORTANT NOTICE
// The above part is only temporarily needed until we have completed the Extensibility epic
// and moved out all provider specifics.
// IMPORTANT NOTICE

// New takes an operation object <o> and creates a new AlicloudBotanist object.
func New(o *operation.Operation, purpose string) (*AlicloudBotanist, error) {
	var cloudProvider gardenv1beta1.CloudProvider

	switch purpose {
	case common.CloudPurposeShoot:
		cloudProvider = o.Shoot.CloudProvider
	case common.CloudPurposeSeed:
		cloudProvider = o.Seed.CloudProvider
	}

	if cloudProvider != gardenv1beta1.CloudProviderAlicloud {
		return nil, errors.New("cannot instantiate an Alicloud botanist if neither Shoot nor Seed cluster specifies Alicloud")
	}

	return &AlicloudBotanist{
		Operation:         o,
		CloudProviderName: "alicloud",
	}, nil
}

// GetCloudProviderName returns the Kubernetes cloud provider name for this cloud.
func (b *AlicloudBotanist) GetCloudProviderName() string {
	return b.CloudProviderName
}
