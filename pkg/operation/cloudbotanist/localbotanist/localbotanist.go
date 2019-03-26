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

package localbotanist

import (
	"errors"

	"github.com/gardener/gardener/pkg/operation"
)

// New takes an operation object <o> and creates a new LocalBotanist object.
func New(o *operation.Operation) (*LocalBotanist, error) {
	if o.Shoot.Info.Spec.Cloud.Local == nil {
		return nil, errors.New("cannot instantiate an Local botanist if `.spec.cloud.local` is nil")
	}

	vb := &LocalBotanist{
		Operation: o,
		// empty string for no cloud provider
		CloudProviderName: "",
	}
	vb.APIServerAddress = *o.Shoot.Info.Spec.DNS.Domain
	vb.Shoot.InternalClusterDomain = *o.Shoot.Info.Spec.DNS.Domain + ":31443"
	return vb, nil
}

// GetCloudProviderName returns the Kubernetes cloud provider name for this cloud.
func (b *LocalBotanist) GetCloudProviderName() string {
	return b.CloudProviderName
}
