// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"github.com/coreos/go-systemd/v22/unit"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("UnitSerializer", func() {
	var (
		options = []*unit.UnitOption{
			{
				Section: "Unit",
				Name:    "Description",
				Value:   "kubelet daemon",
			},
			{
				Section: "Unit",
				Name:    "Documentation",
				Value:   "https://kubernetes.io/docs/admin/kubelet",
			},
			{
				Section: "Unit",
				Name:    "After",
				Value:   "docker.service",
			},
			{
				Section: "Unit",
				Name:    "Wants",
				Value:   "docker.socket rpc-statd.service",
			},
			{
				Section: "Install",
				Name:    "WantedBy",
				Value:   "multi-user.target",
			},
			{
				Section: "Service",
				Name:    "Restart",
				Value:   "always",
			},
			{
				Section: "Service",
				Name:    "RestartSec",
				Value:   "5",
			},
			{
				Section: "Service",
				Name:    "EnvironmentFile",
				Value:   "/etc/environment",
			},
			{
				Section: "Service",
				Name:    "ExecStartPre",
				Value:   "/bin/docker run --rm -v /opt/bin:/opt/bin:rw registry.k8s.io/hyperkube:v1.18.0 cp /hyperkube /opt/bin/",
			},
			{
				Section: "Service",
				Name:    "ExecStart",
				Value: `/opt/bin/hyperkube kubelet \
    --cloud-provider=aws`,
			},
		}

		content = `[Unit]
Description=kubelet daemon
Documentation=https://kubernetes.io/docs/admin/kubelet
After=docker.service
Wants=docker.socket rpc-statd.service

[Install]
WantedBy=multi-user.target

[Service]
Restart=always
RestartSec=5
EnvironmentFile=/etc/environment
ExecStartPre=/bin/docker run --rm -v /opt/bin:/opt/bin:rw registry.k8s.io/hyperkube:v1.18.0 cp /hyperkube /opt/bin/
ExecStart=/opt/bin/hyperkube kubelet \
    --cloud-provider=aws
`
	)

	Describe("#Serialize", func() {
		It("should serialize the given unit options into a string appropriately", func() {
			// Create serializer
			us := NewUnitSerializer()

			// Call Serialize and check result
			s, err := us.Serialize(options)
			Expect(err).NotTo(HaveOccurred())
			Expect(s).To(Equal(content))
		})
	})

	Describe("#Deserialize", func() {
		It("should deserialize unit options from the given string appropriately", func() {
			// Create serializer
			us := NewUnitSerializer()

			// Call Deserialize and check result
			opts, err := us.Deserialize(content)
			Expect(err).NotTo(HaveOccurred())
			Expect(opts).To(Equal(options))
		})
	})
})
