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

package kubeapiserver

import (
	"fmt"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
)

const (
	loggingParserNameAPIServer = "kubeAPIServerParser"
	loggingParserAPIServer     = `[PARSER]
    Name        ` + loggingParserNameAPIServer + `
    Format      regex
    Regex       ^(?<severity>\w)(?<time>\d{4} [^\s]*)\s+(?<pid>\d+)\s+(?<source>[^ \]]+)\] (?<log>.*)$
    Time_Key    time
    Time_Format %m%d %H:%M:%S.%L
`
	loggingFilterAPIServer = `[FILTER]
    Name                parser
    Match               kubernetes.*` + v1beta1constants.DeploymentNameKubeAPIServer + `*` + ContainerNameKubeAPIServer + `*
    Key_Name            log
    Parser              ` + loggingParserNameAPIServer + `
    Reserve_Data        True
`

	loggingParserNameVPNSeed = "vpnSeedParser"
	loggingParserVPNSeed     = `[PARSER]
    Name        ` + loggingParserNameVPNSeed + `
    Format      regex
    Regex       ^(?<time>[^0-9]*\d{1,2}\s+[^\s]+\s+\d{4})\s+(?<log>.*)
    Time_Key    time
    Time_Format %a %b%t%d %H:%M:%S %Y
`
	loggingFilterVPNSeed = `[FILTER]
    Name                parser
    Match               kubernetes.*` + v1beta1constants.DeploymentNameKubeAPIServer + `*` + containerNameVPNSeed + `*
    Key_Name            log
    Parser              ` + loggingParserNameVPNSeed + `
    Reserve_Data        True
`

	loggingParserNameAPIProxyMutator = "apiProxyMutatorParser"
	loggingParserAPIProxyMutator     = `[PARSER]
    Name        ` + loggingParserNameAPIProxyMutator + `
    Format      json
    Time_Key    ts
`
	loggingFilterAPIProxyMutator = `[FILTER]
    Name                parser
    Match               kubernetes.*` + v1beta1constants.DeploymentNameKubeAPIServer + `*` + containerNameAPIServerProxyPodMutator + `*
    Key_Name            log
    Parser              ` + loggingParserNameAPIProxyMutator + `
    Reserve_Data        True
`
	loggingModifyFilterAPIProxyMutator = `[FILTER]
    Name                modify
    Match               kubernetes.*` + v1beta1constants.DeploymentNameKubeAPIServer + `*` + containerNameAPIServerProxyPodMutator + `*
    Copy                level    severity
`
)

// CentralLoggingConfiguration returns a fluent-bit parser and filter for the kube-apiserver logs.
func CentralLoggingConfiguration() (component.CentralLoggingConfig, error) {
	return component.CentralLoggingConfig{
		Filters:     fmt.Sprintf("%s\n%s\n%s\n%s", loggingFilterAPIServer, loggingFilterVPNSeed, loggingFilterAPIProxyMutator, loggingModifyFilterAPIProxyMutator),
		Parsers:     fmt.Sprintf("%s\n%s\n%s", loggingParserAPIServer, loggingParserVPNSeed, loggingParserAPIProxyMutator),
		UserExposed: true,
		PodPrefixes: []string{v1beta1constants.DeploymentNameKubeAPIServer},
	}, nil
}
