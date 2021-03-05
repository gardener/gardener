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

package seed

import (
	"strings"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"

	"k8s.io/apiserver/pkg/authentication/user"
)

// Identity returns the seed name and a boolean indicating whether the provided user has the
// gardener.cloud:system:seeds group. If the seed name is ambigious then an empty string will be returned.
func Identity(u user.Info) (string, bool) {
	if u == nil {
		return "", false
	}

	userName := u.GetName()
	if !strings.HasPrefix(userName, v1beta1constants.SeedUserNamePrefix) {
		return "", false
	}

	if !utils.ValueExists(v1beta1constants.SeedsGroup, u.GetGroups()) {
		return "", false
	}

	var seedName string
	if suffix := strings.TrimPrefix(userName, v1beta1constants.SeedUserNamePrefix); suffix != v1beta1constants.SeedUserNameSuffixAmbiguous {
		seedName = suffix
	}

	return seedName, true
}
