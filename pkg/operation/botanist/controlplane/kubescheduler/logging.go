// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubescheduler

import (
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

const (
	loggingParserName = "kubeSchedulerParser"
	loggingParser     = `[PARSER]
    Name        ` + loggingParserName + `
    Format      regex
    Regex       ^(?<severity>\w)(?<time>\d{4} [^\s]*)\s+(?<pid>\d+)\s+(?<source>[^ \]]+)\] (?<log>.*)$
    Time_Key    time
    Time_Format %m%d %H:%M:%S.%L
`
	loggingFilter = `[FILTER]
    Name                parser
    Match               kubernetes.*` + v1beta1constants.DeploymentNameKubeScheduler + `*` + v1beta1constants.DeploymentNameKubeScheduler + `*
    Key_Name            log
    Parser              ` + loggingParserName + `
    Reserve_Data        True
`
)

// LoggingConfiguration returns a fluent-bit parser and filter for the kube-scheduler logs.
func LoggingConfiguration() (string, string, error) {
	return loggingParser, loggingFilter, nil
}
