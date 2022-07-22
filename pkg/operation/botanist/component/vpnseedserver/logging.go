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

package vpnseedserver

import (
	"fmt"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
)

const (
	containerNameVPNSeedServer = "vpn-seed-server"
	containerNameEnvoyProxy    = "envoy-proxy"

	loggingParserNameVPNSeedServer = "vpnSeedServerParser"
	loggingParserVPNSeedServer     = `[PARSER]
    Name        ` + loggingParserNameVPNSeedServer + `
    Format      regex
    Regex       ^(?<time>\d{4}-\d{2}-\d{2}\s+[^\s]+)\s+(?<log>.*)$
    Time_Key    time
    Time_Format %Y-%m-%d %H:%M:%S
`
	loggingFilterVpnSeedServer = `[FILTER]
    Name                parser
    Match               kubernetes.*` + v1beta1constants.DeploymentNameVPNSeedServer + `*` + containerNameVPNSeedServer + `*
    Key_Name            log
    Parser              ` + loggingParserNameVPNSeedServer + `
    Reserve_Data        True
`

	loggingParserNameEnvoyProxy = "vpnSeedServerEnvoyProxyParser"
	loggingParserEnvoyProxy     = `[PARSER]
    Name        ` + loggingParserNameEnvoyProxy + `
    Format      regex
    Regex       ^\[(?<time>\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+\S+?)]\s+(?<log>.*)$
    Time_Key    time
    Time_Format %Y-%m-%dT%H:%M:%S.%L%z
`
	loggingFilterEnvoyProxy = `[FILTER]
    Name                parser
    Match               kubernetes.*` + v1beta1constants.DeploymentNameVPNSeedServer + `*` + containerNameEnvoyProxy + `*
    Key_Name            log
    Parser              ` + loggingParserNameEnvoyProxy + `
    Reserve_Data        True
`
)

// CentralLoggingConfiguration returns a fluent-bit parser and filter for the kube-apiserver logs.
func CentralLoggingConfiguration() (component.CentralLoggingConfig, error) {
	return component.CentralLoggingConfig{
		Filters: fmt.Sprintf("%s\n%s", loggingFilterVpnSeedServer, loggingFilterEnvoyProxy),
		Parsers: fmt.Sprintf("%s\n%s", loggingParserVPNSeedServer, loggingParserEnvoyProxy),
	}, nil
}
