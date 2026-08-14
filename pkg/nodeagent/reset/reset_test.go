// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reset_test

import (
	"context"
	"io/fs"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/apimachinery/pkg/runtime"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	cri "k8s.io/cri-api/pkg/apis"
	fakecriclient "k8s.io/cri-client/pkg/fake"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	fakedbus "github.com/gardener/gardener/pkg/nodeagent/dbus/fake"
	. "github.com/gardener/gardener/pkg/nodeagent/reset"
)

var _ = Describe("Reset", func() {
	Describe("#Reset", func() {
		var (
			ctx = context.Background()
			log = logr.Discard()

			fakeFS   afero.Afero
			fakeDBus *fakedbus.DBus

			testEncoder runtime.Encoder

			folders = []string{
				"/var/lib/gardener-node-agent/sub-dir",
				"/var/lib/kubelet/some-dir",
			}
			files = []string{
				"/opt/bin/gardener-node-agent",
				"/opt/bin/kubelet",
				"/my/random/path/file",
				"/var/lib/gardener-node-agent/random-file",
				"/var/lib/kubelet/kubeconfig",
			}

			config = extensionsv1alpha1.OperatingSystemConfig{
				Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
					Units: []extensionsv1alpha1.Unit{
						{Name: "kubelet.service"},
						{Name: "gardener-node-agent.service"},
						{Name: "my-unit.service"},
					},
					Files: []extensionsv1alpha1.File{
						{Path: "/opt/bin/gardener-node-agent"},
						{Path: "/opt/bin/kubelet"},
					},
				},
				Status: extensionsv1alpha1.OperatingSystemConfigStatus{
					ExtensionUnits: []extensionsv1alpha1.Unit{
						{Name: "extension.service"},
					},
					ExtensionFiles: []extensionsv1alpha1.File{
						{Path: "/my/random/path/file"},
					},
				},
			}

			expectedSystemdActions []fakedbus.SystemdAction
		)

		BeforeEach(func() {
			fakeFS = afero.Afero{Fs: afero.NewMemMapFs()}
			fakeDBus = fakedbus.New()

			testEncoder = &jsonserializer.Serializer{}

			NewRemoteRuntimeService = func(_ context.Context, _ string, _ time.Duration, _ trace.TracerProvider, _ bool) (cri.RuntimeService, error) {
				return fakecriclient.NewFakeRemoteRuntime().RuntimeService, nil
			}

			Expect(fakeFS.MkdirAll("/opt/bin", fs.ModeDir)).To(Succeed())
			Expect(fakeFS.MkdirAll("/my/random/path", fs.ModeDir)).To(Succeed())
			Expect(fakeFS.MkdirAll("/proc", fs.ModeDir)).To(Succeed())

			for i, f := range folders {
				ExpectWithOffset(i, fakeFS.MkdirAll(f, fs.ModeDir)).To(Succeed())
			}
			for i, f := range files {
				_, err := fakeFS.Create(f)
				ExpectWithOffset(i, err).NotTo(HaveOccurred())
			}

			_, err := fakeFS.Create("/proc/mounts")
			Expect(err).NotTo(HaveOccurred())

			data, err := runtime.Encode(testEncoder, &config)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeFS.WriteFile("/var/lib/gardener-node-agent/last-applied-osc.yaml", data, 0644)).To(Succeed())

			expectedSystemdActions = []fakedbus.SystemdAction{
				{Action: fakedbus.ActionStop, UnitNames: []string{"gardener-node-agent.service"}},
				{Action: fakedbus.ActionStop, UnitNames: []string{"kubelet.service"}},
			}
			for _, unit := range append(config.Spec.Units, config.Status.ExtensionUnits...) {
				expectedSystemdActions = append(expectedSystemdActions,
					fakedbus.SystemdAction{Action: fakedbus.ActionStop, UnitNames: []string{unit.Name}},
					fakedbus.SystemdAction{Action: fakedbus.ActionDisable, UnitNames: []string{unit.Name}},
				)
			}
		})

		It("should reset the node", func() {
			Expect(Reset(ctx, log, fakeFS, fakeDBus)).To(Succeed())

			for i, f := range folders {
				exists, err := fakeFS.DirExists(f)
				ExpectWithOffset(i, err).NotTo(HaveOccurred())
				ExpectWithOffset(i, exists).To(BeFalse())
			}
			for _, f := range files {
				exists, err := fakeFS.Exists(f)
				Expect(err).NotTo(HaveOccurred())
				Expect(exists).To(BeFalse())
			}

			Expect(fakeDBus.Actions).To(Equal(expectedSystemdActions))
		})
	})
})
