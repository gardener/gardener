// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig_test

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pelletier/go-toml"
	"github.com/spf13/afero"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/controller/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/utils/structuredmap"
)

var (
	r   operatingsystemconfig.Reconciler
	osc *extensionsv1alpha1.OperatingSystemConfig

	ctx context.Context
	log logr.Logger
)

const (
	containerdConfigFilePath = "/etc/containerd/config.toml"

	certdsDirValue    = "/etc/containerd/certs.d"
	importsValue      = "/etc/containerd/conf.d/*.toml"
	sandBoxImageValue = "foo:1.23"
	cniBinDirValue    = "/opt/cni/bin"
)

func init() {
	ctx = context.Background()
	log = logr.Discard()

	r = operatingsystemconfig.Reconciler{}

	osc = &extensionsv1alpha1.OperatingSystemConfig{
		Spec: extensionsv1alpha1.OperatingSystemConfigSpec{
			CRIConfig: &extensionsv1alpha1.CRIConfig{
				Name:         extensionsv1alpha1.CRINameContainerD,
				CgroupDriver: ptr.To(extensionsv1alpha1.CgroupDriverSystemd),
				Containerd: &extensionsv1alpha1.ContainerdConfig{
					SandboxImage: sandBoxImageValue,
				},
			},
			Units: []extensionsv1alpha1.Unit{},
		},
	}
}

func getMapEntry(m map[string]any, path structuredmap.Path) (any, error) {
	if m == nil {
		return nil, nil
	}

	key := path[0]

	if len(path) == 1 {
		return m[key], nil
	}

	entry, ok := m[key]
	if !ok {
		return nil, fmt.Errorf("unable to traverse into data structure because key %q does not exist", key)
	}

	childMap, ok := entry.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unable to traverse into data structure because value at %q is not a map", key)
	}

	r, err := getMapEntry(childMap, path[1:])
	if err != nil {
		return nil, err
	}

	return r, nil
}

func getContainerdConfigValue(fs afero.Afero, path structuredmap.Path) (any, error) {
	containerdConfigfile, err := fs.ReadFile(containerdConfigFilePath)
	if err != nil {
		return nil, err
	}

	containerdConfigContent := map[string]any{}

	if err = toml.Unmarshal(containerdConfigfile, &containerdConfigContent); err != nil {
		return nil, err
	}

	return getMapEntry(containerdConfigContent, path)
}

func loadContainerdConfig(source string, fs afero.Afero) error {
	containerdConfig, err := os.ReadFile(source)
	if err != nil {
		return err
	}

	err = fs.WriteFile("/etc/containerd/config.toml", containerdConfig, 0644)
	if err != nil {
		return err
	}

	return nil
}

var _ = Describe("containerd configuration file tests", func() {

	BeforeEach(func() {
		r.FS = afero.Afero{Fs: afero.NewMemMapFs()}
		Expect(r.FS.MkdirAll("/etc/containerd", 0755)).To(Succeed())
	})

	Describe("static containerd configuration paths set by gardener-node-agent", func() {

		When("containerd configuration file version is v2", func() {
			BeforeEach(func() {
				Expect(loadContainerdConfig("testfiles/containerd-config.toml-v2", r.FS)).To(Succeed())
				Expect(r.ReconcileContainerdConfig(ctx, log, osc)).To(Succeed())
			})

			It("should set the imports", func() {
				path := structuredmap.Path{"imports"}
				configValue, err := getContainerdConfigValue(r.FS, path)
				Expect(err).ToNot(HaveOccurred())

				rawImports, ok := configValue.([]any)
				Expect(ok).To(BeTrue())
				Expect(rawImports).To(HaveLen(1))

				v, ok := rawImports[0].(string)
				Expect(ok).To(BeTrue())
				Expect(v).To(Equal(importsValue))
			})

			It("should set the sandbox image", func() {
				path := structuredmap.Path{"plugins", "io.containerd.grpc.v1.cri", "sandbox_image"}
				configValue, err := getContainerdConfigValue(r.FS, path)
				Expect(err).ToNot(HaveOccurred())

				v, ok := configValue.(string)
				Expect(ok).To(BeTrue())
				Expect(v).To(Equal(sandBoxImageValue))
			})

			It("should set the registry config path", func() {
				path := structuredmap.Path{"plugins", "io.containerd.grpc.v1.cri", "registry", "config_path"}
				configValue, err := getContainerdConfigValue(r.FS, path)
				Expect(err).ToNot(HaveOccurred())

				v, ok := configValue.(string)
				Expect(ok).To(BeTrue())
				Expect(v).To(Equal(certdsDirValue))
			})

			It("should set the cgroup driver", func() {
				path := structuredmap.Path{"plugins", "io.containerd.grpc.v1.cri", "containerd", "runtimes", "runc", "options", "SystemdCgroup"}
				configValue, err := getContainerdConfigValue(r.FS, path)
				Expect(err).ToNot(HaveOccurred())

				v, ok := configValue.(bool)
				Expect(ok).To(BeTrue())
				Expect(v).To(BeTrue())
			})

			It("should set the CNI plugin dir", func() {
				path := structuredmap.Path{"plugins", "io.containerd.grpc.v1.cri", "cni", "bin_dir"}
				configValue, err := getContainerdConfigValue(r.FS, path)
				Expect(err).ToNot(HaveOccurred())

				v, ok := configValue.(string)
				Expect(ok).To(BeTrue())
				Expect(v).To(Equal(cniBinDirValue))
			})
		})

		When("containerd configuration file version is v3", func() {
			BeforeEach(func() {
				Expect(loadContainerdConfig("testfiles/containerd-config.toml-v3", r.FS)).To(Succeed())
				Expect(r.ReconcileContainerdConfig(ctx, log, osc)).To(Succeed())
			})

			It("should set the imports", func() {
				path := structuredmap.Path{"imports"}
				configValue, err := getContainerdConfigValue(r.FS, path)
				Expect(err).ToNot(HaveOccurred())

				rawImports, ok := configValue.([]any)
				Expect(ok).To(BeTrue())
				Expect(rawImports).To(HaveLen(1))

				v, ok := rawImports[0].(string)
				Expect(ok).To(BeTrue())
				Expect(v).To(Equal(importsValue))
			})

			It("should set the sandbox image", func() {
				path := structuredmap.Path{"plugins", "io.containerd.cri.v1.runtime", "sandbox_image"}
				configValue, err := getContainerdConfigValue(r.FS, path)
				Expect(err).ToNot(HaveOccurred())

				v, ok := configValue.(string)
				Expect(ok).To(BeTrue())
				Expect(v).To(Equal(sandBoxImageValue))
			})

			It("should set the registry config path", func() {
				path := structuredmap.Path{"plugins", "io.containerd.cri.v1.images", "registry", "config_path"}
				configValue, err := getContainerdConfigValue(r.FS, path)
				Expect(err).ToNot(HaveOccurred())

				v, ok := configValue.(string)
				Expect(ok).To(BeTrue())
				Expect(v).To(Equal(certdsDirValue))
			})

			It("should set the cgroup driver", func() {
				path := structuredmap.Path{"plugins", "io.containerd.cri.v1.runtime", "containerd", "runtimes", "runc", "options", "SystemdCgroup"}
				configValue, err := getContainerdConfigValue(r.FS, path)
				Expect(err).ToNot(HaveOccurred())

				v, ok := configValue.(bool)
				Expect(ok).To(BeTrue())
				Expect(v).To(BeTrue())
			})

			It("should set the CNI plugin dir", func() {
				path := structuredmap.Path{"plugins", "io.containerd.cri.v1.runtime", "cni", "bin_dir"}
				configValue, err := getContainerdConfigValue(r.FS, path)
				Expect(err).ToNot(HaveOccurred())

				v, ok := configValue.(string)
				Expect(ok).To(BeTrue())
				Expect(v).To(Equal(cniBinDirValue))
			})
		})
	})

	Describe("plugin configuration paths inserted by osc plugin config", func() {

		When("containerd configuration file version is v2", func() {
			BeforeEach(func() {
				Expect(loadContainerdConfig("testfiles/containerd-config.toml-v2", r.FS)).To(Succeed())
			})

			It("should not translate anything", func() {
				osc.Spec.CRIConfig.Containerd.Plugins = []extensionsv1alpha1.PluginConfig{
					{
						Op:   ptr.To(extensionsv1alpha1.AddPluginPathOperation),
						Path: []string{"io.containerd.grpc.v1.cri", "containerd", "runtimes", "foo"},
						Values: &apiextensionsv1.JSON{
							Raw: []byte("{\"runtime_type\": \"bar.123\"}"),
						},
					},
				}

				Expect(r.ReconcileContainerdConfig(ctx, log, osc)).To(Succeed())

				wrongPath := structuredmap.Path{"plugins", "io.containerd.cri.v1.runtime", "containerd", "runtimes", "foo", "runtime_type"}
				_, err := getContainerdConfigValue(r.FS, wrongPath)
				Expect(err).To(HaveOccurred())

				goodPath := structuredmap.Path{"plugins", "io.containerd.grpc.v1.cri", "containerd", "runtimes", "foo", "runtime_type"}
				configValue, err := getContainerdConfigValue(r.FS, goodPath)
				Expect(err).ToNot(HaveOccurred())

				v, ok := configValue.(string)
				Expect(ok).To(BeTrue())
				Expect(v).To(Equal("bar.123"))
			})
		})

		When("containerd configuration file version is v3", func() {
			BeforeEach(func() {
				Expect(loadContainerdConfig("testfiles/containerd-config.toml-v3", r.FS)).To(Succeed())
			})

			It("should translate a v2 compliant runtime path to its v3 equivalent", func() {
				osc.Spec.CRIConfig.Containerd.Plugins = []extensionsv1alpha1.PluginConfig{
					{
						Op:   ptr.To(extensionsv1alpha1.AddPluginPathOperation),
						Path: []string{"io.containerd.grpc.v1.cri", "containerd", "runtimes", "foo"},
						Values: &apiextensionsv1.JSON{
							Raw: []byte("{\"runtime_type\": \"bar.123\"}"),
						},
					},
				}

				Expect(r.ReconcileContainerdConfig(ctx, log, osc)).To(Succeed())

				wrongPath := structuredmap.Path{"plugins", "io.containerd.grpc.v1.cri", "containerd", "runtimes", "foo", "runtime_type"}
				_, err := getContainerdConfigValue(r.FS, wrongPath)
				Expect(err).To(HaveOccurred())

				goodPath := structuredmap.Path{"plugins", "io.containerd.cri.v1.runtime", "containerd", "runtimes", "foo", "runtime_type"}
				configValue, err := getContainerdConfigValue(r.FS, goodPath)
				Expect(err).ToNot(HaveOccurred())

				v, ok := configValue.(string)
				Expect(ok).To(BeTrue())
				Expect(v).To(Equal("bar.123"))
			})

			It("should not translate a path that is not in the translation map", func() {
				osc.Spec.CRIConfig.Containerd.Plugins = []extensionsv1alpha1.PluginConfig{
					{
						Op:   ptr.To(extensionsv1alpha1.AddPluginPathOperation),
						Path: []string{"io.containerd.grpc.v1.cri", "containerd", "foobar", "foo"},
						Values: &apiextensionsv1.JSON{
							Raw: []byte("{\"runtime_type\": \"bar.123\"}"),
						},
					},
				}

				Expect(r.ReconcileContainerdConfig(ctx, log, osc)).To(Succeed())

				wrongPath := structuredmap.Path{"plugins", "io.containerd.cri.v1.runtime", "containerd", "foobar", "foo", "runtime_type"}
				_, err := getContainerdConfigValue(r.FS, wrongPath)
				Expect(err).To(HaveOccurred())

				goodPath := structuredmap.Path{"plugins", "io.containerd.grpc.v1.cri", "containerd", "foobar", "foo", "runtime_type"}
				configValue, err := getContainerdConfigValue(r.FS, goodPath)
				Expect(err).ToNot(HaveOccurred())

				v, ok := configValue.(string)
				Expect(ok).To(BeTrue())
				Expect(v).To(Equal("bar.123"))
			})
		})
	})
})
