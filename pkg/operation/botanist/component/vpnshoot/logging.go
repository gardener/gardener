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

package vpnshoot

import (
	"github.com/gardener/gardener/pkg/operation/botanist/component"
)

const (
	loggingParserName = "vpnShootParser"
	loggingParser     = `[PARSER]
    Name        ` + loggingParserName + `
    Format      regex
    Regex       ^(?<time>[^0-9]*\d{1,2}\s+[^\s]+\s+\d{4})\s+(?<log>.*)
    Time_Key    time
    Time_Format %a %b%t%d %H:%M:%S %Y
`
	loggingFilter = `[FILTER]
    Name                parser
    Match               kubernetes.*` + deploymentName + `*` + containerName + `*
    Key_Name            log
    Parser              ` + loggingParserName + `
    Reserve_Data        True
`
)

// CentralLoggingConfiguration returns a fluent-bit parser and filter for the VPNShoot logs.
func CentralLoggingConfiguration() (component.CentralLoggingConfig, error) {
	return component.CentralLoggingConfig{Filters: loggingFilter, Parsers: loggingParser}, nil
}
