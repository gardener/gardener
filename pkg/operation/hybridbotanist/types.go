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

package hybridbotanist

import (
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/cloudbotanist"
)

// HybridBotanist is a struct which contains the "normal" Botanist as well as the CloudBotanist.
// It is used to execute the work for which input from both is required or functionalities from
// both must be used.
type HybridBotanist struct {
	*operation.Operation
	Botanist           *botanist.Botanist
	SeedCloudBotanist  cloudbotanist.CloudBotanist
	ShootCloudBotanist cloudbotanist.CloudBotanist
}
