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

package etcd

import (
	"github.com/gardener/gardener/pkg/operation/botanist/component"
)

const (
	loggingParserEtcdName          = "etcdParser"
	loggingParserBackupRestoreName = "backupRestoreParser"

	loggingParser = `[PARSER]
    Name        ` + loggingParserEtcdName + `
    Format      regex
    Regex       ^(?<time>\d{4}-\d{2}-\d{2}\s+[^ ]*)\s+(?<severity>\w+)\s+\|\s+(?<source>[^ :]*):\s+(?<log>.*)
    Time_Key    time
    Time_Format %Y-%m-%d %H:%M:%S.%L

[PARSER]
    Name        ` + loggingParserBackupRestoreName + `
    Format      regex
    Regex       ^time="(?<time>\d{4}-\d{2}-\d{2}T[^"]*)"\s+level=(?<severity>\w+)\smsg="(?<log>.*)"
    Time_Key    time
    Time_Format %Y-%m-%dT%H:%M:%S%z
`
	loggingFilter = `[FILTER]
    Name                parser
    Match               kubernetes.*` + statefulSetNamePrefix + `*` + containerNameEtcd + `*
    Key_Name            log
    Parser              ` + loggingParserEtcdName + `
    Reserve_Data        True

[FILTER]
    Name                parser
    Match               kubernetes.*` + statefulSetNamePrefix + `*` + containerNameBackupRestore + `*
    Key_Name            log
    Parser              ` + loggingParserBackupRestoreName + `
    Reserve_Data        True
`
)

// CentralLoggingConfiguration returns a fluent-bit parser and filter for the etcd and backup-restore sidecar logs.
func CentralLoggingConfiguration() (component.CentralLoggingConfig, error) {
	return component.CentralLoggingConfig{Filters: loggingFilter, Parsers: loggingParser}, nil
}
