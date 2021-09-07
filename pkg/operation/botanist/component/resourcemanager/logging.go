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

package resourcemanager

import (
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
)

const (
	loggingParserName = "gardenerResourceManagerParser"
	loggingParser     = `[PARSER]
    Name        ` + loggingParserName + `
    Format      json
    Time_Key    ts
    Time_Format %Y-%m-%dT%H:%M:%S.%L
`

	loggingFilters = `[FILTER]
    Name                parser
    Match               kubernetes.*` + v1beta1constants.DeploymentNameGardenerResourceManager + `*` + containerName + `*
    Key_Name            log
    Parser              ` + loggingParserName + `
    Reserve_Data        True

[FILTER]
    Name                modify
    Match               kubernetes.*` + v1beta1constants.DeploymentNameGardenerResourceManager + `*` + containerName + `*
    Rename              level  severity
    Rename              msg    log
    Rename              logger source
`
)

// CentralLoggingConfiguration returns a fluent-bit parser and filters for the gardener-resource-manager logs.
func CentralLoggingConfiguration() (component.CentralLoggingConfig, error) {
	return component.CentralLoggingConfig{Filters: loggingFilters, Parsers: loggingParser}, nil
}
