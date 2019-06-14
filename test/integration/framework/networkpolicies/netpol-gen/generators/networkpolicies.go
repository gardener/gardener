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

package generators

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/huandu/xstrings"

	"github.com/gardener/gardener/test/integration/framework/networkpolicies"

	"k8s.io/gengo/args"
	"k8s.io/gengo/generator"
	"k8s.io/gengo/namer"
	"k8s.io/gengo/types"

	"k8s.io/klog"
)

// NameSystems returns the name system used by the generators in this package.
func NameSystems() namer.NameSystems {
	return namer.NameSystems{
		"public":  namer.NewPublicNamer(0),
		"private": namer.NewPrivateNamer(0),
		"raw":     namer.NewRawNamer("", nil),
	}
}

// DefaultNameSystem returns the default name system for ordering the types to be
// processed by the generators in this package.
func DefaultNameSystem() string {
	return "public"
}

// Packages makes the sets package definition.
func Packages(p *generator.Context, arguments *args.GeneratorArgs) generator.Packages {
	boilerplate, err := arguments.LoadGoBoilerplate()
	if err != nil {
		klog.Fatalf("Failed loading boilerplate: %v", err)
	}

	supportedClouds := defaultRegistry()

	packages := generator.Packages{}

	for _, y := range p.Order {
		if y.Kind == types.Struct && extractBoolTagOrDie("gen-netpoltests", y.CommentLines) {
			values := types.ExtractCommentTags("+", y.CommentLines)["gen-packagename"]
			if values != nil {
				typeFQN := y.Name.String()
				packageName := values[0]

				filterFunc := func(c *generator.Context, t *types.Type) bool {
					switch t.Kind {
					case types.Struct:
						// Only some structs can be keys in a map. This is triggered by the line
						// // +gen-netpoltests=true
						return typeFQN == t.Name.String() && extractBoolTagOrDie("gen-netpoltests", t.CommentLines)
					}
					return false

				}
				pkg := &generator.DefaultPackage{
					PackageName: packageName,
					PackagePath: filepath.Join(arguments.OutputPackagePath, packageName),
					HeaderText:  boilerplate,
					PackageDocumentation: []byte(
						`// Package has auto-generated cloud-specific network policy tests.
			`),
					// GeneratorFunc returns a list of generators. Each generator makes a
					// single file.
					GeneratorFunc: func(c *generator.Context) (generators []generator.Generator) {
						generators = []generator.Generator{
							// Always generate a "doc.go" file.
							generator.DefaultGen{OptionalName: "doc"},
						}
						// Since we want a file per type that we generate a set for, we
						// have to provide a function for this.
						for _, t := range c.Order {
							generators = append(generators, &genTest{
								DefaultGen: generator.DefaultGen{
									OptionalName: fmt.Sprintf("networkpolicy_%s_test", packageName),
								},
								outputPackage: arguments.OutputPackagePath,
								typeToMatch:   t,
								imports:       generator.NewImportTracker(),
								provider:      supportedClouds[typeFQN],
							})
						}
						return generators
					},
					FilterFunc: filterFunc,
				}

				suitePkg := &generator.DefaultPackage{
					PackageName: fmt.Sprintf("%s_test", packageName),
					PackagePath: filepath.Join(arguments.OutputPackagePath, packageName),
					HeaderText:  boilerplate,
					GeneratorFunc: func(c *generator.Context) []generator.Generator {
						return []generator.Generator{
							generator.DefaultGen{
								OptionalName: "networkpolicies_suite_test",
								OptionalBody: []byte(suiteBody),
							},
						}
					},
					FilterFunc: filterFunc,
				}

				packages = append(packages, pkg, suitePkg)

			}
		}
	}
	return packages
}

// genTest produces a file with a set for a single type.
type genTest struct {
	generator.DefaultGen
	outputPackage string
	typeToMatch   *types.Type
	imports       namer.ImportTracker
	provider      networkpolicies.CloudAwarePodInfo
}

// Filter ignores all but one type because we're making a single file per type.
func (g *genTest) Filter(c *generator.Context, t *types.Type) bool { return t == g.typeToMatch }

func (g *genTest) Namers(c *generator.Context) namer.NameSystems {
	return namer.NameSystems{
		"raw": namer.NewRawNamer(g.outputPackage, g.imports),
	}
}

func (g *genTest) Imports(c *generator.Context) (imports []string) {
	return append(g.imports.ImportLines(),
		"context",
		"encoding/json",
		"flag",
		"fmt",
		"io",
		"io/ioutil",
		"sync",
		"time",
		`. "github.com/gardener/gardener/test/integration/framework"`,
		`. "github.com/gardener/gardener/test/integration/shoots"`,
		`. "github.com/onsi/ginkgo"`,
		`. "github.com/onsi/gomega"`,
		"github.com/gardener/gardener/pkg/apis/garden/v1beta1",
		"github.com/gardener/gardener/pkg/client/kubernetes",
		"github.com/gardener/gardener/pkg/logger",
		`networkpolicies "github.com/gardener/gardener/test/integration/framework/networkpolicies"`,
		"github.com/sirupsen/logrus",
		"k8s.io/apimachinery/pkg/api/errors",
		"k8s.io/apimachinery/pkg/labels",
		"k8s.io/apimachinery/pkg/types",
		"k8s.io/apimachinery/pkg/util/sets",
		"sigs.k8s.io/controller-runtime/pkg/client",
		`corev1 "k8s.io/api/core/v1"`,
		`metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"`,
		`networkingv1 "k8s.io/api/networking/v1"`,
	)
}

// args constructs arguments for templates. Usage:
// g.args(t, "key1", value1, "key2", value2, ...)
//
// 't' is loaded with the key 'type'.
//
// We could use t directly as the argument, but doing it this way makes it easy
// to mix in additional parameters.
func (g *genTest) args(t *types.Type, kv ...interface{}) interface{} {
	m := map[interface{}]interface{}{"type": t}
	for i := 0; i < len(kv)/2; i++ {
		m[kv[i*2]] = kv[i*2+1]
	}
	return m
}

func (g *genTest) simpleArgs(kv ...interface{}) interface{} {
	m := map[interface{}]interface{}{}
	for i := 0; i < len(kv)/2; i++ {
		m[kv[i*2]] = kv[i*2+1]
	}
	return m
}

// GenerateType makes the body of a file implementing a set for type t.
func (g *genTest) GenerateType(c *generator.Context, t *types.Type, w io.Writer) error {
	sw := generator.NewSnippetWriter(w, c, "$", "$")
	sw.Do(setHeader, g.args(t))
	g.generateValues(sw)
	sw.Do(setBody, g.args(t))
	g.correctPolicies(sw)
	g.ingressFromOtherNamespaces(sw)
	g.egressToOtherNamespaces(sw)
	g.egressToSeedNodes(sw)
	g.egressForMirroredPods(sw)
	sw.Do("})\n", nil)
	return nil
}

func (g *genTest) correctPolicies(sw *generator.SnippetWriter) {
	sw.Do(`
Context("components are selected by correct policies", func() {
	var (
		assertHasNetworkPolicy = func(sourcePod *networkpolicies.SourcePod) func(context.Context) {
			return func(ctx context.Context) {
				if !sourcePod.Pod.CheckVersion(shootTestOperations.Shoot) {
					Skip("Component doesn't match Shoot version contstraints. Skipping.")
				}
				if !sourcePod.Pod.CheckSeedCluster(sharedResources.SeedCloudProvider) {
					Skip("Component doesn't match Seed Provider contstraints. Skipping.")
				}

				matched := sets.NewString()
				var podLabelSet labels.Set

				By(fmt.Sprintf("Getting first running pod with selectors %q in namespace %q", sourcePod.Pod.Labels, shootTestOperations.ShootSeedNamespace()))
				pod, err := shootTestOperations.GetFirstRunningPodWithLabels(ctx, sourcePod.Pod.Selector(), shootTestOperations.ShootSeedNamespace(), shootTestOperations.SeedClient)
				podLabelSet = pod.GetLabels()
				Expect(err).NotTo(HaveOccurred())

				for _, netPol := range sharedResources.Policies {
					netPolSelector, err := metav1.LabelSelectorAsSelector(&netPol.Spec.PodSelector)
					Expect(err).NotTo(HaveOccurred())

					if netPolSelector.Matches(podLabelSet) {
						matched.Insert(netPol.GetName())
					}
				}
				By(fmt.Sprintf("Matching actual network policies against expected %s", sourcePod.ExpectedPolicies.List()))
				Expect(matched.List()).Should(ConsistOf(sourcePod.ExpectedPolicies.List()))
			}
		}
	)
`, nil)

	for _, s := range g.provider.ToSources() {
		sw.Do("DefaultCIt(`$.sourcePodName$`, assertHasNetworkPolicy($.sourceVarName$))", g.simpleArgs("sourcePodName", s.SourcePod.Pod.Name, "sourceVarName", podToVariableName(&s.SourcePod.Pod)))
		sw.Do("\n", nil)
	}

	sw.Do("})\n", nil)
}

func (g *genTest) generateValues(sw *generator.SnippetWriter) {
	sw.Do("// generated targets\n", nil)
	for _, t := range g.flattenPods() {
		sw.Do("$.targetName$ = &$.targetPod$\n", g.simpleArgs("targetName", t.name, "targetPod", t.value))
	}
}

func (g *genTest) egressForMirroredPods(sw *generator.SnippetWriter) {
	sw.Do(`
Context("egress for mirrored pods", func() {

	var (
		from *networkpolicies.NamespacedSourcePod

		assertEgresssToMirroredPod = func(to *networkpolicies.TargetPod, allowed bool) func(context.Context) {
			return func(ctx context.Context) {
				assertConnectToPod(ctx, from, networkpolicies.NewNamespacedTargetPod(to, sharedResources.Mirror), allowed)
			}
		}

		assertEgresssToHost = func(to *networkpolicies.Host, allowed bool) func(context.Context) {
			return func(ctx context.Context) {
				assertConnectToHost(ctx, from, to, allowed)
			}
		}
	)
`, nil)

	for _, s := range g.provider.ToSources() {

		sw.Do(`
Context("$.podName$", func() {

	BeforeEach(func(){
		from = networkpolicies.NewNamespacedSourcePod($.sourcePod$, sharedResources.Mirror)
	})

		`, g.simpleArgs("podName", s.SourcePod.Pod.Name, "sourcePod", podToVariableName(&s.SourcePod.Pod)))
		for _, t := range s.TargetPods {
			if s.Pod.Name != t.Pod.Name {
				sw.Do("DefaultCIt(`$.description$`, assertEgresssToMirroredPod($.targetVarName$, $.allowed$))", g.simpleArgs("description", t.ToString(), "targetVarName", targetPodToVariableName(&t.TargetPod), "allowed", t.Allowed))
				sw.Do("\n", nil)
			}
		}
		for _, h := range s.TargetHosts {
			sw.Do("DefaultCIt(`$.description$`, assertEgresssToHost($.targetVarName$, $.allowed$))", g.simpleArgs("description", h.ToString(), "targetVarName", hostToVariableName(&h.Host), "allowed", h.Allowed))
			sw.Do("\n", nil)
		}

		sw.Do("})\n", nil)

	}
	sw.Do("})\n", nil)
}

func (g *genTest) egressToSeedNodes(sw *generator.SnippetWriter) {
	sw.Do(`
Context("egress to Seed nodes", func() {

	var (
		assertBlockToSeedNodes = func(from *networkpolicies.SourcePod) func(context.Context) {
			return func(ctx context.Context) {
				assertCannotConnectToHost(ctx, networkpolicies.NewNamespacedSourcePod(from, sharedResources.Mirror), sharedResources.SeedNodeIP, 10250)
			}
		}
	)

	`, nil)
	for _, s := range g.provider.ToSources() {
		sw.Do("DefaultCIt(`should block connectivity from $.podName$`, assertBlockToSeedNodes($.targetVarName$))", g.simpleArgs("podName", s.SourcePod.Pod.Name, "targetVarName", podToVariableName(&s.SourcePod.Pod)))
		sw.Do("\n", nil)
	}
	sw.Do("})\n", nil)
}

func (g *genTest) egressToOtherNamespaces(sw *generator.SnippetWriter) {
	sw.Do(`
Context("egress to other namespaces", func() {

	var (
		assertBlockEgresss = func(from *networkpolicies.SourcePod) func(context.Context) {
			return func(ctx context.Context) {
				assertCannotConnectToPod(ctx, networkpolicies.NewNamespacedSourcePod(from, sharedResources.Mirror), networkpolicies.NewNamespacedTargetPod(networkpolicies.BusyboxInfo.DummyPort(), sharedResources.External))
			}
		}
	)

	`, nil)

	for _, s := range g.provider.ToSources() {
		sw.Do("DefaultCIt(`should block connectivity from $.sourcePodName$ to busybox`, assertBlockEgresss($.sourceVarName$))", g.simpleArgs("sourcePodName", s.SourcePod.Pod.Name, "sourceVarName", podToVariableName(&s.SourcePod.Pod)))
		sw.Do("\n", nil)
	}
	sw.Do("})\n", nil)
}

func (g *genTest) ingressFromOtherNamespaces(sw *generator.SnippetWriter) {
	sw.Do(`
Context("ingress from other namespaces", func() {

	var (
		assertBlockIngress = func(to *networkpolicies.TargetPod, allowed bool) func(context.Context) {
			return func(ctx context.Context) {
				assertConnectToPod(ctx, networkpolicies.NewNamespacedSourcePod(networkpolicies.BusyboxInfo, sharedResources.External), networkpolicies.NewNamespacedTargetPod(to, shootTestOperations.ShootSeedNamespace()), allowed)
			}
		}
	)

	`, nil)

	for _, tp := range g.provider.EgressFromOtherNamespaces(networkpolicies.BusyboxInfo).TargetPods {
		sw.Do("DefaultCIt(`$.description$`, assertBlockIngress($.targetVarName$, $.allowed$))", g.simpleArgs("description", tp.ToString(), "targetVarName", targetPodToVariableName(&tp.TargetPod), "allowed", tp.Allowed))
		sw.Do("\n", nil)
	}
	sw.Do("})\n", nil)

}

type target struct {
	name  string
	value string
}

func (g *genTest) flattenPods() []target {
	fPods := map[string]string{}
	targets := []target{}
	for _, s := range g.provider.ToSources() {

		targets = append(targets, target{name: podToVariableName(&s.SourcePod.Pod), value: prettyPrint(*s.SourcePod)})

		for _, p := range s.TargetPods {
			fPodName := targetPodToVariableName(&p.TargetPod)
			if _, exists := fPods[fPodName]; !exists {
				v := prettyPrint(p.TargetPod)
				fPods[fPodName] = v
				targets = append(targets, target{name: fPodName, value: v})
			}
		}
		for _, h := range s.TargetHosts {
			fHostName := hostToVariableName(&h.Host)
			if _, exists := fPods[fHostName]; !exists {
				v := prettyPrint(h.Host)
				fPods[fHostName] = v
				targets = append(targets, target{name: fHostName, value: v})
			}
		}
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].name < targets[j].name })

	return targets
}

func podToVariableName(p *networkpolicies.Pod) string {
	return xstrings.ToCamelCase(strings.ReplaceAll(p.Name, "-", "_"))
}

func targetPodToVariableName(p *networkpolicies.TargetPod) string {
	return xstrings.ToCamelCase(strings.ReplaceAll(fmt.Sprintf("%s%d", p.Pod.Name, p.Port.Port), "-", "_"))
}

func hostToVariableName(p *networkpolicies.Host) string {
	return xstrings.FirstRuneToUpper(strings.ReplaceAll(fmt.Sprintf("%sPort%d", p.Description, p.Port), " ", ""))
}

var suiteBody = `
import (
	"testing"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
)

func TestNetworkPolicies(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Network Policies Integration Test Suite")
}
`

var setHeader = `
var (
	kubeconfig     = flag.String("kubeconfig", "", "the path to the kubeconfig  of the garden cluster that will be used for integration tests")
	shootName      = flag.String("shootName", "", "the name of the shoot we want to test")
	shootNamespace = flag.String("shootNamespace", "", "the namespace name that the shoot resides in")
	logLevel       = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")
	cleanup        = flag.Bool("cleanup", false, "deletes all created e2e resources after the test suite is done")
)

const (
	InitializationTimeout = 10 * time.Minute
	FinalizationTimeout   = time.Minute
	DefaultTestTimeout    = 10 * time.Second
)

func validateFlags() {
	if !StringSet(*kubeconfig) {
		Fail("you need to specify the correct path for the kubeconfig")
	}

	if !FileExists(*kubeconfig) {
		Fail("kubeconfig path does not exist")
	}
}

var _ = Describe("Network Policy Testing", func() {

	var (
		shootGardenerTest   *ShootGardenerTest
		shootTestOperations *GardenerTestOperation
		shootAppTestLogger  *logrus.Logger
		sharedResources     networkpolicies.SharedResources

		DefaultCIt = func(text string, body func(ctx context.Context)) {
			CIt(text, body, DefaultTestTimeout)
		}

		setGlobals = func(ctx context.Context) {

			validateFlags()
			shootAppTestLogger = logger.AddWriter(logger.NewLogger(*logLevel), GinkgoWriter)

			if StringSet(*shootName) {
				var err error
				shootGardenerTest, err = NewShootGardenerTest(*kubeconfig, nil, shootAppTestLogger)
				Expect(err).NotTo(HaveOccurred())

				shoot := &v1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Namespace: *shootNamespace, Name: *shootName}}
				shootTestOperations, err = NewGardenTestOperation(ctx, shootGardenerTest.GardenClient, shootAppTestLogger, shoot)
				Expect(err).NotTo(HaveOccurred())
			}
		}

		getTargetPod = func(ctx context.Context, targetPod *networkpolicies.NamespacedTargetPod) *corev1.Pod {
			if !targetPod.Pod.CheckVersion(shootTestOperations.Shoot) {
				Skip("Target pod doesn't match Shoot version contstraints. Skipping.")
			}
			if !targetPod.Pod.CheckSeedCluster(sharedResources.SeedCloudProvider) {
				Skip("Component doesn't match Seed Provider contstraints. Skipping.")
			}
			By(fmt.Sprintf("Checking that target Pod: %s is running", targetPod.Pod.Name))
			err := shootTestOperations.WaitUntilPodIsRunningWithLabels(ctx, targetPod.Pod.Selector(), targetPod.Namespace, shootTestOperations.SeedClient)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By(fmt.Sprintf("Get target pod: %s", targetPod.Pod.Name))
			trgPod, err := shootTestOperations.GetFirstRunningPodWithLabels(ctx, targetPod.Pod.Selector(), targetPod.Namespace, shootTestOperations.SeedClient)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			return trgPod
		}


		establishConnectionToHost = func(ctx context.Context, nsp *networkpolicies.NamespacedSourcePod, host string, port int32) (io.Reader, error) {
			if !nsp.Pod.CheckVersion(shootTestOperations.Shoot) {
				Skip("Source pod doesn't match Shoot version contstraints. Skipping.")
			}
			if !nsp.Pod.CheckSeedCluster(sharedResources.SeedCloudProvider) {
				Skip("Component doesn't match Seed Provider contstraints. Skipping.")
			}
			By(fmt.Sprintf("Checking for source Pod: %s is running", nsp.Pod.Name))
			err := shootTestOperations.WaitUntilPodIsRunningWithLabels(ctx, nsp.Pod.Selector(), nsp.Namespace, shootTestOperations.SeedClient)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By(fmt.Sprintf("Executing connectivity command from %s/%s to %s:%d", nsp.Namespace, nsp.Pod.Name, host, port))
			command := fmt.Sprintf("nc -v -z -w 3 %s %d", host, port)

			return shootTestOperations.PodExecByLabel(ctx, nsp.Pod.Selector(), "busybox-0", command, nsp.Namespace, shootTestOperations.SeedClient)
		}

		assertCannotConnectToHost = func(ctx context.Context, sourcePod *networkpolicies.NamespacedSourcePod, host string, port int32) {
			r, err := establishConnectionToHost(ctx, sourcePod, host, port)
			ExpectWithOffset(1, err).To(HaveOccurred())
			bytes, err := ioutil.ReadAll(r)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("Connection message is timed out\n")
			ExpectWithOffset(1, string(bytes)).To(SatisfyAny(ContainSubstring("Connection timed out"), ContainSubstring("nc: bad address")))
		}

		assertCannotConnectToPod = func(ctx context.Context, sourcePod *networkpolicies.NamespacedSourcePod, targetPod *networkpolicies.NamespacedTargetPod) {
			pod := getTargetPod(ctx, targetPod)
			assertCannotConnectToHost(ctx, sourcePod, pod.Status.PodIP, targetPod.Port.Port)
		}

		assertConnectToHost = func(ctx context.Context, sourcePod *networkpolicies.NamespacedSourcePod, targetHost *networkpolicies.Host, allowed bool) {
			r, err := establishConnectionToHost(ctx, sourcePod, targetHost.HostName, targetHost.Port)
			if allowed {
				ExpectWithOffset(1, err).NotTo(HaveOccurred())
				ExpectWithOffset(1, r).NotTo(BeNil())
			} else {
				ExpectWithOffset(1, err).To(HaveOccurred())
				bytes, err := ioutil.ReadAll(r)
				ExpectWithOffset(1, err).NotTo(HaveOccurred())

				By("Connection message is timed out\n")
				ExpectWithOffset(1, string(bytes)).To(SatisfyAny(ContainSubstring("Connection timed out"), ContainSubstring("nc: bad address")))
			}
		}

		assertConnectToPod = func(ctx context.Context, sourcePod *networkpolicies.NamespacedSourcePod, targetPod *networkpolicies.NamespacedTargetPod, allowed bool) {
			pod := getTargetPod(ctx, targetPod)
			assertConnectToHost(ctx, sourcePod, &networkpolicies.Host{
				HostName: pod.Status.PodIP,
				Port:     targetPod.Port.Port,
			}, allowed)
		}

`

var setBody = `
	)

	SynchronizedBeforeSuite(func() []byte {
		ctx, cancel := context.WithTimeout(context.TODO(), InitializationTimeout)
		defer cancel()

		setGlobals(ctx)
		var err error

		By("Getting Seed Cloud Provider")
		sharedResources.SeedCloudProvider, err = shootTestOperations.SeedCloudProvider()
		Expect(err).NotTo(HaveOccurred())

		By("Creating namespace for Ingress testing")
		ns, err := shootTestOperations.SeedClient.CreateNamespace(
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "gardener-e2e-network-policies-",
					Labels: map[string]string{
						"gardener-e2e-test": "networkpolicies",
					},
				},
			}, true)

		Expect(err).NotTo(HaveOccurred())

		sharedResources.External = ns.GetName()

		By("Creating mirror namespace for pod2pod network testing")
		mirrorNamespace, err := shootTestOperations.SeedClient.CreateNamespace(
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "gardener-e2e-mirror-network-policies-",
					Labels: map[string]string{
						"gardener-e2e-test": "networkpolicies",
					},
				},
			}, true)
		Expect(err).NotTo(HaveOccurred())

		sharedResources.Mirror = mirrorNamespace.GetName()

		By(fmt.Sprintf("Getting all network policies in namespace %q", shootTestOperations.ShootSeedNamespace()))
		list := &networkingv1.NetworkPolicyList{}
		err = shootTestOperations.SeedClient.Client().List(ctx, &client.ListOptions{Namespace: shootTestOperations.ShootSeedNamespace()}, list)
		Expect(err).ToNot(HaveOccurred())

		sharedResources.Policies = list.Items

		for _, netPol := range sharedResources.Policies {
			cpy := &networkingv1.NetworkPolicy{}
			cpy.Name = netPol.Name
			cpy.Namespace = sharedResources.Mirror
			cpy.Spec = *netPol.Spec.DeepCopy()
			By(fmt.Sprintf("Copying network policy %s in namespace %q", netPol.Name, sharedResources.Mirror))
			err = shootTestOperations.SeedClient.Client().Create(ctx, cpy)
			Expect(err).NotTo(HaveOccurred())
		}

		By("Getting the current CloudProvider")
		currentProvider, err := shootTestOperations.GetCloudProvider()
		Expect(err).NotTo(HaveOccurred())

		getFirstNodeInternalIP := func(ctx context.Context, cl kubernetes.Interface) (string, error) {
			nodes := &corev1.NodeList{}
			err := cl.Client().List(ctx, &client.ListOptions{Raw: &metav1.ListOptions{Limit: 1}}, nodes)
			if err != nil {
				return "", err
			}

			if len(nodes.Items) > 0 {
				firstNode := nodes.Items[0]
				for _, address := range firstNode.Status.Addresses {
					if address.Type == corev1.NodeInternalIP {
						return address.Address, nil
					}
				}
			}

			return "", ErrNoInternalIPsForNodeWasFound
		}

		By("Getting fist running node")
		sharedResources.SeedNodeIP, err = getFirstNodeInternalIP(ctx, shootTestOperations.SeedClient)
		Expect(err).NotTo(HaveOccurred())

		// this provider is generated
		cloudAwarePodInfo := $.type|raw${}

		if currentProvider != cloudAwarePodInfo.Provider() {
			Fail(fmt.Sprintf("Not suported cloud provider %s", currentProvider))
		}

		createBusyBox := func(ctx context.Context, sourcePod *networkpolicies.NamespacedSourcePod, ports ...corev1.ContainerPort) {
			if len(ports) == 0 {
				Fail(fmt.Sprintf("No ports found for SourcePod %+v", *sourcePod.SourcePod))
			}
			containers := []corev1.Container{}
			for i, port := range ports {
				containers = append(containers, corev1.Container{
					Args:  []string{"nc", "-lk", "-p", fmt.Sprint(port.ContainerPort), "-e", "/bin/echo", "-s", "0.0.0.0"},
					Image: "busybox",
					Name:  fmt.Sprintf("busybox-%d", i),
					Ports: ports,
				})
			}
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sourcePod.Pod.Name,
					Namespace: sourcePod.Namespace,
					Labels:    sourcePod.Pod.Labels,
				},
				Spec: corev1.PodSpec{
					Containers: containers,
				},
			}

			By(fmt.Sprintf("Creating Pod %s/%s", sourcePod.Namespace, sourcePod.Name))
			err := shootTestOperations.SeedClient.Client().Create(ctx, pod)

			By(fmt.Sprintf("Waiting foo Pod %s/%s to be running", sourcePod.Namespace, sourcePod.Name))
			err = shootTestOperations.WaitUntilPodIsRunning(ctx, pod.GetName(), sourcePod.Namespace, shootTestOperations.SeedClient)
			if err != nil {
				Fail(fmt.Sprintf("Couldn't find running busybox %s/%s", sourcePod.Namespace, pod.GetName()))
			}
		}

		sources := cloudAwarePodInfo.ToSources()

		var wg sync.WaitGroup
		// one extra for the busybox Pod bellow.
		wg.Add(len(sources) + 1)

		for _, s := range sources {
			go func(pi *networkpolicies.SourcePod) {
				defer GinkgoRecover()
				defer wg.Done()
				if !pi.Pod.CheckVersion(shootTestOperations.Shoot) || !pi.Pod.CheckSeedCluster(sharedResources.SeedCloudProvider) {
					return
				}
				pod, err := shootTestOperations.GetFirstRunningPodWithLabels(ctx, pi.Pod.Selector(), shootTestOperations.ShootSeedNamespace(), shootTestOperations.SeedClient)
				if err != nil {
					Fail(fmt.Sprintf("Couldn't find running Pod %s/%s with labels: %+v", shootTestOperations.ShootSeedNamespace(), pi.Pod.Name, pi.Pod.Labels))
				}
				cpy := *pi

				targetLabels := make(map[string]string)

				for k, v := range pod.Labels {
					targetLabels[k] = v
				}

				cpy.Pod.Labels = targetLabels
				By(fmt.Sprintf("Mirroring Pod %s to namespace %s", cpy.Pod.Labels.String(), sharedResources.Mirror))

				expectedPorts := sets.Int64{}
				actualPorts := sets.Int64{}
				for _, p := range pi.Ports {
					expectedPorts.Insert(int64(p.Port))
				}
				containerPorts := []corev1.ContainerPort{}
				for _, container := range pod.Spec.Containers {
					if len(container.Ports) > 0 {
						for _, p := range container.Ports {
							actualPorts.Insert(int64(p.ContainerPort))
						}
						containerPorts = append(containerPorts, container.Ports...)
					}
				}

				if !actualPorts.HasAll(expectedPorts.List()...) {
					Fail(fmt.Sprintf("Pod %s doesn't have all ports. Expected %+v, actual %+v", pi.Pod.Name, expectedPorts.List(), actualPorts.List()))
				}
				if len(containerPorts) == 0 {
					// Dummy port for containers which don't have any ports.
					containerPorts = append(containerPorts, corev1.ContainerPort{ContainerPort: 8080})
				}
				createBusyBox(ctx, networkpolicies.NewNamespacedSourcePod(&cpy, sharedResources.Mirror), containerPorts...)
			}(s.SourcePod)
		}
		go func() {
			defer GinkgoRecover()
			defer wg.Done()
			createBusyBox(ctx, networkpolicies.NewNamespacedSourcePod(networkpolicies.BusyboxInfo, ns.GetName()), corev1.ContainerPort{ContainerPort: 8080})
		}()

		wg.Wait()

		b, err := json.Marshal(sharedResources)
		Expect(err).NotTo(HaveOccurred())

		return b
	}, func(data []byte) {
		ctx, cancel := context.WithTimeout(context.TODO(), InitializationTimeout)
		defer cancel()

		sr := &networkpolicies.SharedResources{}
		err := json.Unmarshal(data, sr)
		Expect(err).NotTo(HaveOccurred())

		setGlobals(ctx)

		sharedResources = *sr
	})

	SynchronizedAfterSuite(func() {
		if !*cleanup {
			return
		}

		ctx, cancel := context.WithTimeout(context.TODO(), FinalizationTimeout)
		defer cancel()

		setGlobals(ctx)

		namespaces := &corev1.NamespaceList{}
		selector := &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(labels.Set{
				"gardener-e2e-test": "networkpolicies",
			}),
		}
		err := shootTestOperations.SeedClient.Client().List(ctx, selector, namespaces)
		Expect(err).NotTo(HaveOccurred())

		for _, ns := range namespaces.Items {
			err = shootTestOperations.SeedClient.Client().Delete(ctx, &ns)
			if err != nil && !errors.IsConflict(err) {
				Expect(err).NotTo(HaveOccurred())
			}
		}
	}, func() {})

	Context("Deprecated old policies are removed", func() {

		const (
			deprecatedKubeAPIServerPolicy = "kube-apiserver-default"
			deprecatedMetadataAppPolicy   = "cloud-metadata-service-deny-blacklist-app"
			deprecatedMetadataRolePolicy  = "cloud-metadata-service-deny-blacklist-role"
			deprecatedKibanaLogging       = "kibana-logging"
		)

		var (
			assertPolicyIsGone = func(policyName string) func(ctx context.Context) {
				return func(ctx context.Context) {
					By(fmt.Sprintf("Getting network policy %q in namespace %q", policyName, shootTestOperations.ShootSeedNamespace()))
					getErr := shootTestOperations.SeedClient.Client().Get(ctx, types.NamespacedName{Name: policyName, Namespace: shootTestOperations.ShootSeedNamespace()}, &networkingv1.NetworkPolicy{})
					Expect(getErr).To(HaveOccurred())
					By("error is NotFound")
					Expect(errors.IsNotFound(getErr)).To(BeTrue())
				}
			}
		)

		DefaultCIt(deprecatedKubeAPIServerPolicy, assertPolicyIsGone(deprecatedKubeAPIServerPolicy))
		DefaultCIt(deprecatedMetadataAppPolicy, assertPolicyIsGone(deprecatedMetadataAppPolicy))
		DefaultCIt(deprecatedMetadataRolePolicy, assertPolicyIsGone(deprecatedMetadataRolePolicy))
		DefaultCIt(deprecatedMetadataRolePolicy, assertPolicyIsGone(deprecatedKibanaLogging))
	})
`
