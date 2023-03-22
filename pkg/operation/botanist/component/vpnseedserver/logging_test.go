// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package vpnseedserver_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
)

var _ = Describe("Logging", func() {
	Describe("#CentralLoggingConfiguration", func() {
		It("should return the expected logging parser and filter", func() {
			loggingConfig, err := CentralLoggingConfiguration()

			Expect(err).NotTo(HaveOccurred())
			Expect(loggingConfig.Parsers).To(Equal(`[PARSER]
    Name        vpnSeedServerParser
    Format      regex
    Regex       ^(?<time>\d{4}-\d{2}-\d{2}\s+[^\s]+)\s+(?<log>.*)$
    Time_Key    time
    Time_Format %Y-%m-%d %H:%M:%S

[PARSER]
    Name        vpnSeedServerEnvoyProxyParser
    Format      regex
    Regex       ^\[(?<time>\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+\S+?)]\s+(?<log>.*)$
    Time_Key    time
    Time_Format %Y-%m-%dT%H:%M:%S.%L%z
`))

			Expect(loggingConfig.Filters).To(Equal(`[FILTER]
    Name                parser
    Match               kubernetes.*vpn-seed-server*vpn-seed-server*
    Key_Name            log
    Parser              vpnSeedServerParser
    Reserve_Data        True

[FILTER]
    Name                parser
    Match               kubernetes.*vpn-seed-server*envoy-proxy*
    Key_Name            log
    Parser              vpnSeedServerEnvoyProxyParser
    Reserve_Data        True
`))
		})
	})
})
