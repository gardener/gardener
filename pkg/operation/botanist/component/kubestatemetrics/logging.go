// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubestatemetrics

import (
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
)

const (
	loggingParserName = "kubeStateMetricsParser"
	loggingParser     = `[PARSER]
    Name        ` + loggingParserName + `
    Format      regex
    Regex       ^(?<severity>\w)(?<time>\d{4} [^\s]*)\s+(?<pid>\d+)\s+(?<source>[^ \]]+)\] (?<log>.*)$
    Time_Key    time
    Time_Format %m%d %H:%M:%S.%L
`
	loggingFilter = `[FILTER]
    Name                parser
    Match               kubernetes.*` + v1beta1constants.DeploymentNameKubeStateMetrics + `*` + containerName + `*
    Key_Name            log
    Parser              ` + loggingParserName + `
    Reserve_Data        True
`
)

// CentralLoggingConfiguration returns a fluent-bit parser and filter for the kube-state-metrics logs.
func CentralLoggingConfiguration() (component.CentralLoggingConfig, error) {
	return component.CentralLoggingConfig{Filters: loggingFilter, Parsers: loggingParser}, nil
}
