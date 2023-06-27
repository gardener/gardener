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

package dbus_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/nodeagent/dbus"
)

var _ = Describe("Dbus", func() {
	It("should enable a unit", func() {
		d := &dbus.FakeDbus{}
		Expect(d.Enable(context.Background(), "kubelet")).Should(Succeed())
		Expect(d.Actions).Should(Equal([]dbus.FakeSystemdAction{
			{
				Action:    dbus.FakeEnable,
				UnitNames: []string{"kubelet"},
			},
		}))
	})

	It("should disable a unit", func() {
		d := &dbus.FakeDbus{}
		Expect(d.Disable(context.Background(), "kubelet")).Should(Succeed())
		Expect(d.Actions).Should(Equal([]dbus.FakeSystemdAction{
			{
				Action:    dbus.FakeDisable,
				UnitNames: []string{"kubelet"},
			},
		}))
	})

	It("should restart a unit", func() {
		d := &dbus.FakeDbus{}
		Expect(d.Restart(context.Background(), nil, nil, "kubelet")).Should(Succeed())
		Expect(d.Actions).Should(Equal([]dbus.FakeSystemdAction{
			{
				Action:    dbus.FakeRestart,
				UnitNames: []string{"kubelet"},
			},
		}))
	})

	It("should start a unit", func() {
		d := &dbus.FakeDbus{}
		Expect(d.Start(context.Background(), nil, nil, "kubelet")).Should(Succeed())
		Expect(d.Actions).Should(Equal([]dbus.FakeSystemdAction{
			{
				Action:    dbus.FakeStart,
				UnitNames: []string{"kubelet"},
			},
		}))
	})

	It("should stop a unit", func() {
		d := &dbus.FakeDbus{}
		Expect(d.Stop(context.Background(), nil, nil, "kubelet")).Should(Succeed())
		Expect(d.Actions).Should(Equal([]dbus.FakeSystemdAction{
			{
				Action:    dbus.FakeStop,
				UnitNames: []string{"kubelet"},
			},
		}))
	})

	It("should reload deamon", func() {
		d := &dbus.FakeDbus{}
		Expect(d.DaemonReload(context.Background())).Should(Succeed())
		Expect(d.Actions).Should(Equal([]dbus.FakeSystemdAction{
			{
				Action: dbus.FakeDeamonReload,
			},
		}))
	})
})
