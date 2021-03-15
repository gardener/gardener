// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package cmd

import (
	"errors"
	"fmt"

	mockcmd "github.com/gardener/gardener/extensions/pkg/controller/cmd/mock"
	mockcontroller "github.com/gardener/gardener/extensions/pkg/controller/mock"
	"github.com/gardener/gardener/pkg/utils/test"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var _ = Describe("Options", func() {
	var (
		ctrl *gomock.Controller
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#LeaderElectionNameID", func() {
		It("should return a leader election with the name", func() {
			Expect(LeaderElectionNameID("foo")).To(Equal("foo-leader-election"))
		})
	})

	Describe("#PrefixFlagger", func() {
		const (
			cmdName  = "test"
			prefix   = "foo"
			flagName = "bar"
			value    = "x"
		)
		command := test.NewCommandBuilder(cmdName).
			Flags(
				test.StringFlag(fmt.Sprintf("%s%s", prefix, flagName), value),
			).
			Command().
			Slice()

		It("should add the prefix to the flags", func() {
			var bar string
			flagger := mockcmd.NewMockFlagger(ctrl)
			flagger.EXPECT().AddFlags(gomock.Any()).Do(func(fs *pflag.FlagSet) {
				fs.StringVar(&bar, flagName, "", "bar")
			})
			fs := pflag.NewFlagSet(cmdName, pflag.ExitOnError)

			prefixedFlagger := PrefixFlagger(prefix, flagger)
			prefixedFlagger.AddFlags(fs)

			err := fs.Parse(command)
			Expect(err).NotTo(HaveOccurred())
			Expect(bar).To(Equal(value))
		})
	})

	Describe("#PrefixOption", func() {
		const (
			cmdName  = "test"
			prefix   = "foo"
			flagName = "bar"
			value    = "x"
		)
		command := test.NewCommandBuilder(cmdName).
			Flags(
				test.StringFlag(fmt.Sprintf("%s%s", prefix, flagName), value),
			).
			Command().
			Slice()

		It("should add the prefix to the flags", func() {
			var bar string
			option := mockcmd.NewMockOption(ctrl)
			option.EXPECT().AddFlags(gomock.Any()).Do(func(fs *pflag.FlagSet) {
				fs.StringVar(&bar, flagName, "", "bar")
			})
			option.EXPECT().Complete()

			fs := pflag.NewFlagSet(cmdName, pflag.ExitOnError)

			prefixedOption := PrefixOption(prefix, option)
			prefixedOption.AddFlags(fs)

			err := fs.Parse(command)
			Expect(err).NotTo(HaveOccurred())
			Expect(bar).To(Equal(value))
			Expect(prefixedOption.Complete()).NotTo(HaveOccurred())
		})
	})

	Context("OptionAggregator", func() {
		Describe("#NewOptionAggregator", func() {
			It("should register the options correctly", func() {
				o1 := mockcmd.NewMockOption(ctrl)
				o2 := mockcmd.NewMockOption(ctrl)

				aggregated := NewOptionAggregator(o1, o2)

				Expect(aggregated).To(Equal(OptionAggregator{o1, o2}))
			})
		})

		Describe("#Register", func() {
			It("should register the options correctly", func() {
				o1 := mockcmd.NewMockOption(ctrl)
				o2 := mockcmd.NewMockOption(ctrl)

				aggregated := NewOptionAggregator()
				aggregated.Register(o1, o2)

				Expect(aggregated).To(Equal(OptionAggregator{o1, o2}))
			})

			It("should append the newly added options", func() {
				o1 := mockcmd.NewMockOption(ctrl)
				o2 := mockcmd.NewMockOption(ctrl)
				o3 := mockcmd.NewMockOption(ctrl)

				aggregated := NewOptionAggregator(o1)
				aggregated.Register(o2, o3)

				Expect(aggregated).To(Equal(OptionAggregator{o1, o2, o3}))
			})
		})

		Describe("#AddFlags", func() {
			It("should add the flags of all options", func() {
				fs := pflag.NewFlagSet("", pflag.ExitOnError)
				o1 := mockcmd.NewMockOption(ctrl)
				o2 := mockcmd.NewMockOption(ctrl)
				gomock.InOrder(
					o1.EXPECT().AddFlags(fs),
					o2.EXPECT().AddFlags(fs),
				)

				aggregated := NewOptionAggregator(o1, o2)

				aggregated.AddFlags(fs)
			})
		})

		Describe("#Complete", func() {
			It("should call complete on all options", func() {
				o1 := mockcmd.NewMockOption(ctrl)
				o2 := mockcmd.NewMockOption(ctrl)
				gomock.InOrder(
					o1.EXPECT().Complete(),
					o2.EXPECT().Complete(),
				)

				aggregated := NewOptionAggregator(o1, o2)

				Expect(aggregated.Complete()).NotTo(HaveOccurred())
			})

			It("should return abort after the first error and return it", func() {
				o1 := mockcmd.NewMockOption(ctrl)
				o2 := mockcmd.NewMockOption(ctrl)
				err := errors.New("error")
				gomock.InOrder(
					o1.EXPECT().Complete().Return(err),
				)

				aggregated := NewOptionAggregator(o1, o2)

				Expect(aggregated.Complete()).To(BeIdenticalTo(err))
			})
		})
	})

	Context("ManagerOptions", func() {
		const (
			name                       = "foo"
			leaderElectionResourceLock = "leases"
			leaderElectionID           = "id"
			leaderElectionNamespace    = "namespace"
		)
		command := test.NewCommandBuilder(name).
			Flags(
				test.BoolFlag("leader-election", true),
				test.StringFlag("leader-election-resource-lock", leaderElectionResourceLock),
				test.StringFlag("leader-election-id", leaderElectionID),
				test.StringFlag("leader-election-namespace", leaderElectionNamespace),
			).
			Command().
			Slice()

		Describe("#AddFlags", func() {
			It("should add all flags", func() {
				fs := pflag.NewFlagSet(name, pflag.ExitOnError)
				opts := ManagerOptions{}

				opts.AddFlags(fs)

				Expect(fs.Parse(command)).NotTo(HaveOccurred())
				Expect(opts).To(Equal(ManagerOptions{
					LeaderElection:             true,
					LeaderElectionResourceLock: leaderElectionResourceLock,
					LeaderElectionID:           leaderElectionID,
					LeaderElectionNamespace:    leaderElectionNamespace,
				}))
			})

			It("should default resource lock to configmapsleases", func() {
				fs := pflag.NewFlagSet(name, pflag.ExitOnError)
				opts := ManagerOptions{}

				opts.AddFlags(fs)

				Expect(fs.Parse(
					test.NewCommandBuilder(name).
						Flags(
							test.BoolFlag("leader-election", true),
							test.StringFlag("leader-election-id", leaderElectionID),
							test.StringFlag("leader-election-namespace", leaderElectionNamespace),
						).
						Command().
						Slice(),
				)).NotTo(HaveOccurred())
				Expect(opts).To(Equal(ManagerOptions{
					LeaderElection:             true,
					LeaderElectionResourceLock: "configmapsleases",
					LeaderElectionID:           leaderElectionID,
					LeaderElectionNamespace:    leaderElectionNamespace,
				}))
			})
		})

		Describe("#Complete", func() {
			It("should complete without error after the flags have been parsed", func() {
				fs := pflag.NewFlagSet(name, pflag.ExitOnError)
				opts := ManagerOptions{}

				opts.AddFlags(fs)

				Expect(fs.Parse(command)).NotTo(HaveOccurred())
				Expect(opts.Complete()).NotTo(HaveOccurred())
			})
		})

		Describe("#Completed", func() {
			It("should yield a correct ManagerConfig after completion", func() {
				fs := pflag.NewFlagSet(name, pflag.ExitOnError)
				opts := ManagerOptions{}

				opts.AddFlags(fs)

				Expect(fs.Parse(command)).NotTo(HaveOccurred())
				Expect(opts.Complete()).NotTo(HaveOccurred())
				Expect(opts.Completed()).To(Equal(&ManagerConfig{
					LeaderElection:             true,
					LeaderElectionResourceLock: leaderElectionResourceLock,
					LeaderElectionID:           leaderElectionID,
					LeaderElectionNamespace:    leaderElectionNamespace,
				}))
			})
		})
	})

	Context("ControllerOptions", func() {
		const (
			name                    = "foo"
			maxConcurrentReconciles = 5
		)
		command := test.NewCommandBuilder(name).
			Flags(test.IntFlag(MaxConcurrentReconcilesFlag, maxConcurrentReconciles)).
			Command().
			Slice()

		Describe("#AddFlags", func() {
			It("should add all flags", func() {
				fs := pflag.NewFlagSet(name, pflag.ExitOnError)
				opts := ControllerOptions{}

				opts.AddFlags(fs)

				Expect(fs.Parse(command)).NotTo(HaveOccurred())
				Expect(opts).To(Equal(ControllerOptions{
					MaxConcurrentReconciles: maxConcurrentReconciles,
				}))
			})
		})

		Describe("#Complete", func() {
			It("should complete without error after the flags have been parsed", func() {
				fs := pflag.NewFlagSet(name, pflag.ExitOnError)
				opts := ControllerOptions{}

				opts.AddFlags(fs)

				Expect(fs.Parse(command)).NotTo(HaveOccurred())
				Expect(opts.Complete()).NotTo(HaveOccurred())
			})
		})

		Describe("#Completed", func() {
			It("should yield a correct ManagerConfig after completion", func() {
				fs := pflag.NewFlagSet(name, pflag.ExitOnError)
				opts := ControllerOptions{}

				opts.AddFlags(fs)

				Expect(fs.Parse(command)).NotTo(HaveOccurred())
				Expect(opts.Complete()).NotTo(HaveOccurred())
				Expect(opts.Completed()).To(Equal(&ControllerConfig{
					MaxConcurrentReconciles: maxConcurrentReconciles,
				}))
			})
		})
	})

	Context("RESTOptions", func() {
		const (
			name       = "foo"
			kubeconfig = "kubeconfig"
			masterURL  = "masterURL"
		)
		command := test.NewCommandBuilder(name).
			Flags(
				test.StringFlag(KubeconfigFlag, kubeconfig),
				test.StringFlag(MasterURLFlag, masterURL),
			).
			Command().
			Slice()

		Describe("#AddFlags", func() {
			It("should add all flags", func() {
				fs := pflag.NewFlagSet(name, pflag.ExitOnError)
				opts := RESTOptions{}

				opts.AddFlags(fs)

				Expect(fs.Parse(command)).NotTo(HaveOccurred())
				Expect(opts).To(Equal(RESTOptions{
					Kubeconfig: kubeconfig,
					MasterURL:  masterURL,
				}))
			})
		})

		Describe("#Complete", func() {
			It("should return errors encountered during completing", func() {
				called := false
				err := errors.New("error")
				defer test.WithVar(&BuildConfigFromFlags, func(actualMasterURL, actualKubeconfig string) (*rest.Config, error) {
					called = true
					Expect(actualMasterURL).To(Equal(masterURL))
					Expect(actualKubeconfig).To(Equal(kubeconfig))
					return nil, err
				})()

				opts := RESTOptions{
					Kubeconfig: kubeconfig,
					MasterURL:  masterURL,
				}

				Expect(opts.Complete()).To(BeIdenticalTo(err))
				Expect(called).To(BeTrue())
			})

			It("should complete without error calling BuildConfigFromFlags", func() {
				called := false
				defer test.WithVar(&BuildConfigFromFlags, func(actualMasterURL, actualKubeconfig string) (*rest.Config, error) {
					called = true
					Expect(actualMasterURL).To(Equal(masterURL))
					Expect(actualKubeconfig).To(Equal(kubeconfig))
					return &rest.Config{}, nil
				})()

				opts := RESTOptions{
					Kubeconfig: kubeconfig,
					MasterURL:  masterURL,
				}

				Expect(opts.Complete()).NotTo(HaveOccurred())
				Expect(called).To(BeTrue())
			})

			It("should complete without error calling BuildConfigFromFlags with kubeconfig from Getenv", func() {
				buildConfigFromFlagsCalled := false
				getenvCalled := true
				defer test.WithVar(&Getenv, func(key string) string {
					getenvCalled = true
					Expect(key).To(Equal(clientcmd.RecommendedConfigPathEnvVar))
					return kubeconfig
				})()
				defer test.WithVar(&BuildConfigFromFlags, func(actualMasterURL, actualKubeconfig string) (*rest.Config, error) {
					buildConfigFromFlagsCalled = true
					Expect(actualMasterURL).To(Equal(masterURL))
					Expect(actualKubeconfig).To(Equal(kubeconfig))
					return &rest.Config{}, nil
				})()

				opts := RESTOptions{
					MasterURL: masterURL,
				}

				Expect(opts.Complete()).NotTo(HaveOccurred())
				Expect(buildConfigFromFlagsCalled).To(BeTrue())
				Expect(getenvCalled).To(BeTrue())
			})

			It("should complete without error calling InClusterConfig", func() {
				inClusterConfigCalled := false
				getenvCalled := true
				defer test.WithVar(&Getenv, func(key string) string {
					getenvCalled = true
					Expect(key).To(Equal(clientcmd.RecommendedConfigPathEnvVar))
					return ""
				})()
				defer test.WithVar(&InClusterConfig, func() (*rest.Config, error) {
					inClusterConfigCalled = true
					return &rest.Config{}, nil
				})()

				opts := RESTOptions{}

				Expect(opts.Complete()).NotTo(HaveOccurred())
				Expect(inClusterConfigCalled).To(BeTrue())
				Expect(getenvCalled).To(BeTrue())
			})

			It("should complete without error calling BuildConfigFromFlags with the recommended home file", func() {
				inClusterConfigCalled := false
				getenvCalled := true
				buildConfigFromFlagsCalled := true
				recommendedHomeFile := "home/file"
				defer test.WithVar(&RecommendedHomeFile, recommendedHomeFile)()
				defer test.WithVar(&Getenv, func(key string) string {
					getenvCalled = true
					Expect(key).To(Equal(clientcmd.RecommendedConfigPathEnvVar))
					return ""
				})()
				defer test.WithVar(&InClusterConfig, func() (*rest.Config, error) {
					inClusterConfigCalled = true
					return nil, errors.New("some error")
				})()
				defer test.WithVar(&BuildConfigFromFlags, func(actualMasterURL, actualKubeconfig string) (*rest.Config, error) {
					buildConfigFromFlagsCalled = true
					Expect(actualMasterURL).To(BeEmpty())
					Expect(actualKubeconfig).To(Equal(RecommendedHomeFile))
					return &rest.Config{}, nil
				})()

				opts := RESTOptions{}

				Expect(opts.Complete()).NotTo(HaveOccurred())
				Expect(inClusterConfigCalled).To(BeTrue())
				Expect(getenvCalled).To(BeTrue())
				Expect(buildConfigFromFlagsCalled).To(BeTrue())
			})
		})

		DescribeTable("#Completed",
			func(setup func(config *rest.Config, opts *RESTOptions) func()) {
				opts := RESTOptions{}
				config := &rest.Config{}
				defer setup(config, &opts)()

				Expect(opts.Complete()).NotTo(HaveOccurred())
				Expect(opts.Completed()).To(Equal(&RESTConfig{
					Config: config,
				}))
			},
			Entry("defined kubeconfig", func(config *rest.Config, opts *RESTOptions) func() {
				opts.Kubeconfig = kubeconfig
				return test.WithVar(&BuildConfigFromFlags, func(string, string) (*rest.Config, error) {
					return config, nil
				})
			}),
			Entry("kubeconfig from env", func(config *rest.Config, opts *RESTOptions) func() {
				resetConfigFromFlags := test.WithVar(&BuildConfigFromFlags, func(string, string) (*rest.Config, error) {
					return config, nil
				})
				resetGetenv := test.WithVar(&Getenv, func(string) string { return kubeconfig })
				return func() { resetConfigFromFlags(); resetGetenv() }
			}),
			Entry("in-cluster", func(config *rest.Config, opts *RESTOptions) func() {
				resetInClusterConfig := test.WithVar(&InClusterConfig, func() (*rest.Config, error) {
					return config, nil
				})
				resetGetenv := test.WithVar(&Getenv, func(string) string { return "" })
				return func() { resetInClusterConfig(); resetGetenv() }
			}),
			Entry("recommended kubeconfig", func(config *rest.Config, opts *RESTOptions) func() {
				resetInClusterConfig := test.WithVar(&InClusterConfig, func() (*rest.Config, error) {
					return nil, errors.New("error")
				})
				resetConfigFromFlags := test.WithVar(&BuildConfigFromFlags, func(string, string) (*rest.Config, error) {
					return config, nil
				})
				resetGetenv := test.WithVar(&Getenv, func(string) string { return "" })
				return func() { resetConfigFromFlags(); resetInClusterConfig(); resetGetenv() }
			}),
		)
	})

	Context("ManagerConfig", func() {
		const (
			leaderElectionResourceLock = "leases"
			leaderElectionID           = "id"
			leaderElectionNamespace    = "namespace"
		)

		Describe("#Apply", func() {
			It("should apply the values to the given manager.Options", func() {
				cfg := &ManagerConfig{
					LeaderElection:             true,
					LeaderElectionResourceLock: leaderElectionResourceLock,
					LeaderElectionID:           leaderElectionID,
					LeaderElectionNamespace:    leaderElectionNamespace,
				}

				opts := manager.Options{}
				cfg.Apply(&opts)

				Expect(opts).To(Equal(manager.Options{
					LeaderElection:             true,
					LeaderElectionResourceLock: leaderElectionResourceLock,
					LeaderElectionID:           leaderElectionID,
					LeaderElectionNamespace:    leaderElectionNamespace,
				}))
			})
		})

		Describe("#Options", func() {
			It("should return manager.Options with the given values set", func() {
				cfg := &ManagerConfig{
					LeaderElection:             true,
					LeaderElectionResourceLock: leaderElectionResourceLock,
					LeaderElectionID:           leaderElectionID,
					LeaderElectionNamespace:    leaderElectionNamespace,
				}

				opts := cfg.Options()

				Expect(opts.LeaderElection).To(BeTrue())
				Expect(opts.LeaderElectionResourceLock).To(Equal(leaderElectionResourceLock))
				Expect(opts.LeaderElectionID).To(Equal(leaderElectionID))
				Expect(opts.LeaderElectionNamespace).To(Equal(leaderElectionNamespace))
			})
		})
	})

	Context("ControllerConfig", func() {
		const (
			maxConcurrentReconciles = 5
		)

		Describe("#Apply", func() {
			It("should apply the values to the given controller.Options", func() {
				cfg := &ControllerConfig{
					MaxConcurrentReconciles: maxConcurrentReconciles,
				}

				opts := controller.Options{}
				cfg.Apply(&opts)

				Expect(opts).To(Equal(controller.Options{
					MaxConcurrentReconciles: maxConcurrentReconciles,
				}))
			})
		})

		Describe("#Options", func() {
			It("should return controller.Options with the given values set", func() {
				cfg := &ControllerConfig{
					MaxConcurrentReconciles: maxConcurrentReconciles,
				}

				opts := cfg.Options()

				Expect(opts).To(Equal(controller.Options{
					MaxConcurrentReconciles: maxConcurrentReconciles,
				}))
			})
		})
	})

	Context("SwitchOptions", func() {
		const commandName = "test"

		Describe("#AddFlags", func() {
			It("should correctly parse the flags", func() {
				var (
					name1    = "foo"
					name2    = "bar"
					switches = NewSwitchOptions(
						Switch(name1, nil),
						Switch(name2, nil),
					)
				)

				fs := pflag.NewFlagSet(commandName, pflag.ContinueOnError)
				switches.AddFlags(fs)

				err := fs.Parse(test.NewCommandBuilder(commandName).
					Flags(
						test.StringSliceFlag(DisableFlag, name1, name2),
					).
					Command().
					Slice())

				Expect(err).NotTo(HaveOccurred())
				Expect(switches.Complete()).To(Succeed())

				Expect(switches.Disabled).To(Equal([]string{name1, name2}))
			})

			It("should error on an unknown controller", func() {
				switches := NewSwitchOptions()

				fs := pflag.NewFlagSet(commandName, pflag.ContinueOnError)
				switches.AddFlags(fs)

				err := fs.Parse(test.NewCommandBuilder(commandName).
					Flags(
						test.StringSliceFlag(DisableFlag, "unknown"),
					).
					Command().
					Slice())

				Expect(err).NotTo(HaveOccurred())
				Expect(switches.Complete()).To(HaveOccurred())
			})
		})

		Describe("#AddToManager", func() {
			It("should return a configuration that does not add the disabled controllers", func() {
				var (
					f1 = mockcontroller.NewMockAddToManager(ctrl)
					f2 = mockcontroller.NewMockAddToManager(ctrl)

					name1 = "name1"
					name2 = "name2"

					switches = NewSwitchOptions(
						Switch(name1, f1.Do),
						Switch(name2, f2.Do),
					)
				)

				f1.EXPECT().Do(nil)

				switches.Disabled = []string{name2}

				Expect(switches.Complete()).To(Succeed())
				Expect(switches.Completed().AddToManager(nil)).To(Succeed())
			})
		})
	})
})
