// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"
	"go.uber.org/mock/gomock"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	extensionsmockcmd "github.com/gardener/gardener/extensions/pkg/controller/cmd/mock"
	extensionsmockcontroller "github.com/gardener/gardener/extensions/pkg/controller/mock"
	"github.com/gardener/gardener/pkg/utils/test"
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
			flagger := extensionsmockcmd.NewMockFlagger(ctrl)
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
			option := extensionsmockcmd.NewMockOption(ctrl)
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
				o1 := extensionsmockcmd.NewMockOption(ctrl)
				o2 := extensionsmockcmd.NewMockOption(ctrl)

				aggregated := NewOptionAggregator(o1, o2)

				Expect(aggregated).To(Equal(OptionAggregator{o1, o2}))
			})
		})

		Describe("#Register", func() {
			It("should register the options correctly", func() {
				o1 := extensionsmockcmd.NewMockOption(ctrl)
				o2 := extensionsmockcmd.NewMockOption(ctrl)

				aggregated := NewOptionAggregator()
				aggregated.Register(o1, o2)

				Expect(aggregated).To(Equal(OptionAggregator{o1, o2}))
			})

			It("should append the newly added options", func() {
				o1 := extensionsmockcmd.NewMockOption(ctrl)
				o2 := extensionsmockcmd.NewMockOption(ctrl)
				o3 := extensionsmockcmd.NewMockOption(ctrl)

				aggregated := NewOptionAggregator(o1)
				aggregated.Register(o2, o3)

				Expect(aggregated).To(Equal(OptionAggregator{o1, o2, o3}))
			})
		})

		Describe("#AddFlags", func() {
			It("should add the flags of all options", func() {
				fs := pflag.NewFlagSet("", pflag.ExitOnError)
				o1 := extensionsmockcmd.NewMockOption(ctrl)
				o2 := extensionsmockcmd.NewMockOption(ctrl)
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
				o1 := extensionsmockcmd.NewMockOption(ctrl)
				o2 := extensionsmockcmd.NewMockOption(ctrl)
				gomock.InOrder(
					o1.EXPECT().Complete(),
					o2.EXPECT().Complete(),
				)

				aggregated := NewOptionAggregator(o1, o2)

				Expect(aggregated.Complete()).NotTo(HaveOccurred())
			})

			It("should return abort after the first error and return it", func() {
				o1 := extensionsmockcmd.NewMockOption(ctrl)
				o2 := extensionsmockcmd.NewMockOption(ctrl)
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
			name                    = "foo"
			leaderElectionID        = "id"
			leaderElectionNamespace = "namespace"
			metricsBindAddress      = ":8080"
			healthBindAddress       = ":8081"
			logLevel                = "debug"
			logFormat               = "text"
			logLevelDefault         = "info"
			logFormatDefault        = "json"
		)
		command := test.NewCommandBuilder(name).
			Flags(
				test.BoolFlag("leader-election", true),
				test.StringFlag("leader-election-id", leaderElectionID),
				test.StringFlag("leader-election-namespace", leaderElectionNamespace),
				test.StringFlag("metrics-bind-address", metricsBindAddress),
				test.StringFlag("health-bind-address", healthBindAddress),
				test.StringFlag("log-level", logLevel),
				test.StringFlag("log-format", logFormat),
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
					LeaderElection:          true,
					LeaderElectionID:        leaderElectionID,
					LeaderElectionNamespace: leaderElectionNamespace,
					MetricsBindAddress:      metricsBindAddress,
					HealthBindAddress:       healthBindAddress,
					LogLevel:                logLevel,
					LogFormat:               logFormat,
				}))
			})

			It("should default resource lock to leases", func() {
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
					LeaderElection:          true,
					LeaderElectionID:        leaderElectionID,
					LeaderElectionNamespace: leaderElectionNamespace,
					MetricsBindAddress:      metricsBindAddress,
					HealthBindAddress:       healthBindAddress,
					LogLevel:                logLevelDefault,
					LogFormat:               logFormatDefault,
				}))
			})
		})

		Describe("#Complete", func() {
			It("should fail on invalid log-level", func() {
				fs := pflag.NewFlagSet(name, pflag.ExitOnError)
				opts := ManagerOptions{}

				opts.AddFlags(fs)

				Expect(fs.Parse(
					test.NewCommandBuilder(name).
						Flags(
							test.StringFlag("log-level", "foo"),
						).
						Command().
						Slice(),
				)).NotTo(HaveOccurred())
				Expect(opts.Complete()).To(MatchError("invalid --log-level: foo"))
			})

			It("should fail on invalid log-format", func() {
				fs := pflag.NewFlagSet(name, pflag.ExitOnError)
				opts := ManagerOptions{}

				opts.AddFlags(fs)

				Expect(fs.Parse(
					test.NewCommandBuilder(name).
						Flags(
							test.StringFlag("log-format", "bar"),
						).
						Command().
						Slice(),
				)).NotTo(HaveOccurred())
				Expect(opts.Complete()).To(MatchError("invalid --log-format: bar"))
			})

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
				Expect(opts.Completed()).To(HaveField("LeaderElection", true))
				Expect(opts.Completed()).To(HaveField("LeaderElectionID", leaderElectionID))
				Expect(opts.Completed()).To(HaveField("LeaderElectionNamespace", leaderElectionNamespace))
				Expect(opts.Completed()).To(HaveField("MetricsBindAddress", metricsBindAddress))
				Expect(opts.Completed()).To(HaveField("HealthBindAddress", healthBindAddress))
			})

			It("should yield an enabled Logger after completion", func() {
				fs := pflag.NewFlagSet(name, pflag.ExitOnError)
				opts := ManagerOptions{}

				opts.AddFlags(fs)

				Expect(fs.Parse(command)).NotTo(HaveOccurred())
				Expect(opts.Complete()).NotTo(HaveOccurred())
				Expect(opts.Completed().Logger.Enabled()).To(BeTrue())
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
			Entry("kubeconfig from env", func(config *rest.Config, _ *RESTOptions) func() {
				resetConfigFromFlags := test.WithVar(&BuildConfigFromFlags, func(string, string) (*rest.Config, error) {
					return config, nil
				})
				resetGetenv := test.WithVar(&Getenv, func(string) string { return kubeconfig })
				return func() { resetConfigFromFlags(); resetGetenv() }
			}),
			Entry("in-cluster", func(config *rest.Config, _ *RESTOptions) func() {
				resetInClusterConfig := test.WithVar(&InClusterConfig, func() (*rest.Config, error) {
					return config, nil
				})
				resetGetenv := test.WithVar(&Getenv, func(string) string { return "" })
				return func() { resetInClusterConfig(); resetGetenv() }
			}),
			Entry("recommended kubeconfig", func(config *rest.Config, _ *RESTOptions) func() {
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
			leaderElectionID        = "id"
			leaderElectionNamespace = "namespace"
		)

		Describe("#Apply", func() {
			It("should apply the values to the given manager.Options", func() {
				cfg := &ManagerConfig{
					LeaderElection:          true,
					LeaderElectionID:        leaderElectionID,
					LeaderElectionNamespace: leaderElectionNamespace,
				}

				opts := manager.Options{}
				cfg.Apply(&opts)

				Expect(opts).To(Equal(manager.Options{
					LeaderElection:             true,
					LeaderElectionResourceLock: "leases",
					LeaderElectionID:           leaderElectionID,
					LeaderElectionNamespace:    leaderElectionNamespace,
					Controller: controllerconfig.Controller{
						RecoverPanic: ptr.To(true),
					},
					WebhookServer: &webhook.DefaultServer{},
				}))
			})
		})

		Describe("#Options", func() {
			It("should return manager.Options with the given values set", func() {
				cfg := &ManagerConfig{
					LeaderElection:          true,
					LeaderElectionID:        leaderElectionID,
					LeaderElectionNamespace: leaderElectionNamespace,
				}

				opts := cfg.Options()

				Expect(opts.LeaderElection).To(BeTrue())
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
			var (
				name1    = "foo"
				name2    = "bar"
				switches = NewSwitchOptions(
					Switch(name1, nil),
					Switch(name2, nil),
				)
			)

			It("should correctly parse the flags", func() {
				fs := pflag.NewFlagSet(commandName, pflag.ContinueOnError)
				switches.AddFlags(fs)

				err := fs.Parse(test.NewCommandBuilder(commandName).
					Flags(
						test.StringSliceFlag(ControllersFlag, name1),
						test.StringSliceFlag(DisableFlag, name1, name2),
					).
					Command().
					Slice())

				Expect(err).NotTo(HaveOccurred())
				Expect(switches.Complete()).To(Succeed())

				Expect(switches.Enabled).To(Equal([]string{name1}))
				Expect(switches.Disabled).To(Equal([]string{name1, name2}))
			})

			It("should error on an unknown controller to disable", func() {
				fs := pflag.NewFlagSet(commandName, pflag.ContinueOnError)
				switches.AddFlags(fs)

				err := fs.Parse(test.NewCommandBuilder(commandName).
					Flags(
						test.StringSliceFlag(DisableFlag, "unknown"),
					).
					Command().
					Slice())

				Expect(err).NotTo(HaveOccurred())
				Expect(switches.Complete()).To(MatchError(`cannot disable unknown controller "unknown"`))
			})

			It("should error on an unknown controller to enable", func() {
				fs := pflag.NewFlagSet(commandName, pflag.ContinueOnError)
				switches.AddFlags(fs)

				err := fs.Parse(test.NewCommandBuilder(commandName).
					Flags(
						test.StringSliceFlag(ControllersFlag, "unknown"),
					).
					Command().
					Slice())

				Expect(err).NotTo(HaveOccurred())
				Expect(switches.Complete()).To(MatchError(`cannot enable unknown controller "unknown"`))
			})
		})

		Describe("#AddToManager", func() {
			It("should return a configuration that does not add the disabled controllers", func() {
				var (
					f1 = extensionsmockcontroller.NewMockAddToManager(ctrl)
					f2 = extensionsmockcontroller.NewMockAddToManager(ctrl)

					name1 = "name1"
					name2 = "name2"

					switches = NewSwitchOptions(
						Switch(name1, f1.Do),
						Switch(name2, f2.Do),
					)
				)

				f1.EXPECT().Do(nil, nil)

				switches.Enabled = []string{name1, name2}
				switches.Disabled = []string{name2}

				Expect(switches.Complete()).To(Succeed())
				Expect(switches.Completed().AddToManager(nil, nil)).To(Succeed())
			})
		})
	})

	Context("GeneralOptions", func() {
		const (
			name            = "foo"
			gardenerVersion = "v1.2.3"
		)

		command := test.NewCommandBuilder(name).
			Flags(
				test.StringFlag(GardenerVersionFlag, gardenerVersion),
			).
			Command().
			Slice()

		Describe("#AddFlags", func() {
			It("should add all flags", func() {
				fs := pflag.NewFlagSet(name, pflag.ExitOnError)
				opts := GeneralOptions{}

				opts.AddFlags(fs)

				Expect(fs.Parse(command)).NotTo(HaveOccurred())
				Expect(opts).To(Equal(GeneralOptions{
					GardenerVersion: gardenerVersion,
				}))
			})
		})

		Describe("#Complete", func() {
			It("should complete without error calling BuildConfigFromFlags", func() {
				opts := GeneralOptions{
					GardenerVersion: gardenerVersion,
				}

				Expect(opts.Complete()).NotTo(HaveOccurred())
			})
		})

		Describe("#Completed", func() {
			It("should yield the expected result", func() {
				opts := GeneralOptions{
					GardenerVersion: gardenerVersion,
				}

				Expect(opts.Complete()).NotTo(HaveOccurred())
				Expect(opts.Completed()).To(Equal(&GeneralConfig{
					GardenerVersion: gardenerVersion,
				}))
			})
		})
	})
})
