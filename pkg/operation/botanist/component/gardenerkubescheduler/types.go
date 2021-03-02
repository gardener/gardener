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
	"github.com/Masterminds/semver"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var (
	versionConstraintEqual118 *semver.Constraints
	versionConstraintEqual119 *semver.Constraints
	versionConstraintEqual120 *semver.Constraints
)

const (
	// Name is the name of kubernetes resources associated with gardener-kube-scheduler.
	Name = "gardener-kube-scheduler"
)

func init() {
	var err error

	versionConstraintEqual118, err = semver.NewConstraint("1.18.x")
	utilruntime.Must(err)

	versionConstraintEqual119, err = semver.NewConstraint("1.19.x")
	utilruntime.Must(err)

	versionConstraintEqual120, err = semver.NewConstraint("1.20.x")
	utilruntime.Must(err)
}
