// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetesdashboard

import (
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
)

const (
	loggingParserName = "kubernetesDashboardParser"
	loggingParser     = `[PARSER]
    Name        ` + loggingParserName + `
    Format      regex
    Regex       ^(?<time>\d{4}\/\d{2}\/\d{2}\s+[^\s]*)\s+(?<log>.*)
    Time_Key    time
    Time_Format %Y/%m/%d %H:%M:%S
`
	loggingFilter = `[FILTER]
    Name                parser
    Match               kubernetes.*addons-kubernetes-dashboard*kubernetes-dashboard*
    Key_Name            log
    Parser              ` + loggingParserName + `
    Reserve_Data        True
`
)

// CentralLoggingConfiguration returns a fluent-bit parser and filter for the kubernetesDashboard logs.
func CentralLoggingConfiguration() (component.CentralLoggingConfig, error) {
	return component.CentralLoggingConfig{
		Filters:     loggingFilter,
		Parsers:     loggingParser,
		UserExposed: true,
		PodPrefixes: []string{v1beta1constants.DeploymentNameKubernetesDashboard, v1beta1constants.DeploymentNameDashboardMetricsScraper},
	}, nil
}
