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

package dnsrecord_test

import (
	"fmt"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/provider-local/controller/dnsrecord"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

func init() {
	format.CharactersAroundMismatchToInclude = 500
}

var _ = Describe("Actuator", func() {
	var (
		hostname  = "foo.bar.com"
		value1    = "1.2.3.4"
		value2    = "5.6.7.8"
		dnsRecord = &extensionsv1alpha1.DNSRecord{
			Spec: extensionsv1alpha1.DNSRecordSpec{
				Name:   hostname,
				Values: []string{value1, value2},
			},
		}

		etcdHostsContentWithoutSection = []byte(`##
# Host Database
#
# localhost is used to configure the loopback interface
# when the system is booting.  Do not change this entry.
##
127.0.0.1	localhost
255.255.255.255	broadcasthost
::1             localhost
# Added by Docker Desktop
# To allow the same kube context to work on the host and the container:
127.0.0.1 kubernetes.docker.internal
# End of section
`)

		etcdHostsContentWithEmptySection = []byte(`##
# Host Database
#
# localhost is used to configure the loopback interface
# when the system is booting.  Do not change this entry.
##
127.0.0.1	localhost
255.255.255.255	broadcasthost
::1             localhost
# Begin of gardener-extension-provider-local section
# End of gardener-extension-provider-local section
# Added by Docker Desktop
# To allow the same kube context to work on the host and the container:
127.0.0.1 kubernetes.docker.internal
# End of section
`)

		etcdHostsContentWithUpToDateSection = []byte(fmt.Sprintf(`##
# Host Database
#
# localhost is used to configure the loopback interface
# when the system is booting.  Do not change this entry.
##
127.0.0.1	localhost
255.255.255.255	broadcasthost
::1             localhost
# Begin of gardener-extension-provider-local section
%s %s
%s %s
# End of gardener-extension-provider-local section
# Added by Docker Desktop
# To allow the same kube context to work on the host and the container:
127.0.0.1 kubernetes.docker.internal
# End of section
`, value1, hostname, value2, hostname))

		etcdHostsContentWithUpToDateSectionAtFileEnd = []byte(fmt.Sprintf(`%s# Begin of gardener-extension-provider-local section
%s %s
%s %s
# End of gardener-extension-provider-local section
`, etcdHostsContentWithoutSection, value1, hostname, value2, hostname))

		etcdHostsContentWithUpToDateSectionAndAdditionalValues = []byte(fmt.Sprintf(`##
# Host Database
#
# localhost is used to configure the loopback interface
# when the system is booting.  Do not change this entry.
##
127.0.0.1	localhost
255.255.255.255	broadcasthost
::1             localhost
# Begin of gardener-extension-provider-local section
%s %s
%s %s
bar baz
baz foo
foo bar
# End of gardener-extension-provider-local section
# Added by Docker Desktop
# To allow the same kube context to work on the host and the container:
127.0.0.1 kubernetes.docker.internal
# End of section
`, value1, hostname, value2, hostname))
	)

	Describe("#CreateOrUpdateValuesInEtcHostsFile", func() {
		Context("section does not exist", func() {
			It("should add the provided values", func() {
				Expect(CreateOrUpdateValuesInEtcHostsFile(etcdHostsContentWithoutSection, dnsRecord)).To(Equal(etcdHostsContentWithUpToDateSectionAtFileEnd))
			})
		})

		Context("section exists but empty", func() {
			It("should add the provided values", func() {
				Expect(CreateOrUpdateValuesInEtcHostsFile(etcdHostsContentWithEmptySection, dnsRecord)).To(Equal(etcdHostsContentWithUpToDateSection))
			})
		})

		Context("section exists with different hostnames", func() {
			etcdHostsContent := []byte(`##
# Host Database
#
# localhost is used to configure the loopback interface
# when the system is booting.  Do not change this entry.
##
127.0.0.1	localhost
255.255.255.255	broadcasthost
::1             localhost
# Begin of gardener-extension-provider-local section
foo bar
baz foo
bar baz
# End of gardener-extension-provider-local section
# Added by Docker Desktop
# To allow the same kube context to work on the host and the container:
127.0.0.1 kubernetes.docker.internal
# End of section
`)

			It("should add the provided values", func() {
				Expect(CreateOrUpdateValuesInEtcHostsFile(etcdHostsContent, dnsRecord)).To(Equal(etcdHostsContentWithUpToDateSectionAndAdditionalValues))
			})
		})

		Context("section exists with same hostnames", func() {
			etcdHostsContent := []byte(`##
# Host Database
#
# localhost is used to configure the loopback interface
# when the system is booting.  Do not change this entry.
##
127.0.0.1	localhost
255.255.255.255	broadcasthost
::1             localhost
# Begin of gardener-extension-provider-local section
foo bar
baz foo
bar baz
oldvalue ` + hostname + `
# End of gardener-extension-provider-local section
# Added by Docker Desktop
# To allow the same kube context to work on the host and the container:
127.0.0.1 kubernetes.docker.internal
# End of section
`)

			It("should add the provided values", func() {
				Expect(CreateOrUpdateValuesInEtcHostsFile(etcdHostsContent, dnsRecord)).To(Equal(etcdHostsContentWithUpToDateSectionAndAdditionalValues))
			})
		})
	})

	Describe("#DeleteValuesInEtcHostsFile", func() {
		Context("section does not exist", func() {
			It("should do nothing", func() {
				Expect(DeleteValuesInEtcHostsFile(etcdHostsContentWithoutSection, dnsRecord)).To(Equal(etcdHostsContentWithoutSection))
			})
		})

		Context("section exists but empty", func() {
			It("should drop the section", func() {
				Expect(DeleteValuesInEtcHostsFile(etcdHostsContentWithEmptySection, dnsRecord)).To(Equal(etcdHostsContentWithoutSection))
			})
		})

		Context("section exists with different hostnames", func() {
			etcdHostsContent := []byte(`##
# Host Database
#
# localhost is used to configure the loopback interface
# when the system is booting.  Do not change this entry.
##
127.0.0.1	localhost
255.255.255.255	broadcasthost
::1             localhost
# Added by Docker Desktop
# To allow the same kube context to work on the host and the container:
127.0.0.1 kubernetes.docker.internal
# End of section
# Begin of gardener-extension-provider-local section
bar baz
baz foo
foo bar
# End of gardener-extension-provider-local section
`)

			It("should do nothing", func() {
				Expect(DeleteValuesInEtcHostsFile(etcdHostsContent, dnsRecord)).To(Equal(etcdHostsContent))
			})
		})

		Context("section exists with same hostnames", func() {
			etcdHostsContent := []byte(fmt.Sprintf(`##
# Host Database
#
# localhost is used to configure the loopback interface
# when the system is booting.  Do not change this entry.
##
127.0.0.1	localhost
255.255.255.255	broadcasthost
::1             localhost
# Begin of gardener-extension-provider-local section
%s %s
oldvalue %s
bar baz
baz foo
foo bar
# End of gardener-extension-provider-local section
# Added by Docker Desktop
# To allow the same kube context to work on the host and the container:
127.0.0.1 kubernetes.docker.internal
# End of section
`, value1, hostname, hostname))

			It("should delete the provided values", func() {
				Expect(DeleteValuesInEtcHostsFile(etcdHostsContent, dnsRecord)).To(Equal([]byte(`##
# Host Database
#
# localhost is used to configure the loopback interface
# when the system is booting.  Do not change this entry.
##
127.0.0.1	localhost
255.255.255.255	broadcasthost
::1             localhost
# Begin of gardener-extension-provider-local section
bar baz
baz foo
foo bar
# End of gardener-extension-provider-local section
# Added by Docker Desktop
# To allow the same kube context to work on the host and the container:
127.0.0.1 kubernetes.docker.internal
# End of section
`)))
			})
		})

		Context("section exists with only hostnames and ips", func() {
			It("should delete the provided values", func() {
				Expect(DeleteValuesInEtcHostsFile(etcdHostsContentWithUpToDateSection, dnsRecord)).To(Equal(etcdHostsContentWithoutSection))
			})
		})

		Context("section exists with only hostnames and ips at the end of the file", func() {
			It("should delete the provided values", func() {
				Expect(DeleteValuesInEtcHostsFile(etcdHostsContentWithUpToDateSectionAtFileEnd, dnsRecord)).To(Equal(etcdHostsContentWithoutSection))
			})
		})
	})
})
