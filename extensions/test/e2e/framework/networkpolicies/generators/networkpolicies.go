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
	"sort"
	"strings"

	"github.com/gardener/gardener/extensions/test/e2e/framework/networkpolicies"

	"github.com/huandu/xstrings"
	"k8s.io/gengo/args"
	"k8s.io/gengo/generator"
	"k8s.io/gengo/namer"
	"k8s.io/klog"
)

const (
	outPkgName = "networkpolies"
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

type cloudAwarePackage struct {
	cloud networkpolicies.CloudAware
}

func NewPackages(cloud networkpolicies.CloudAware) func(p *generator.Context, arguments *args.GeneratorArgs) generator.Packages {
	return (&cloudAwarePackage{cloud: cloud}).Packages
}

// Packages makes the sets package definition.
func (a *cloudAwarePackage) Packages(p *generator.Context, arguments *args.GeneratorArgs) generator.Packages {
	boilerplate, err := arguments.LoadGoBoilerplate()
	if err != nil {
		klog.Fatalf("Failed loading boilerplate: %v", err)
	}

	pkg := &generator.DefaultPackage{
		PackageName: outPkgName,
		PackagePath: arguments.OutputPackagePath,
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
				generator.DefaultGen{
					OptionalName: "networkpolicies_suite_test",
					OptionalBody: []byte(suiteBody),
				},
				&genTest{
					DefaultGen: generator.DefaultGen{
						OptionalName: "networkpolicy_test",
					},
					outputPackage: outPkgName,
					imports:       generator.NewImportTracker(),
					provider:      a.cloud,
				},
			}

			return generators
		},
	}

	return generator.Packages{pkg}
}

// genTest produces a file with a set for a single type.
type genTest struct {
	generator.DefaultGen
	outputPackage string
	imports       namer.ImportTracker
	provider      networkpolicies.CloudAware
}

func (g *genTest) Imports(c *generator.Context) (imports []string) {
	return append(g.imports.ImportLines(),
		`context`,
		`encoding/json`,
		`flag`,
		`fmt`,
		`strings`,
		`sync`,
		`time`,

		`"github.com/gardener/gardener/test/framework"`,
		`. "github.com/onsi/ginkgo"`,
		`. "github.com/onsi/gomega"`,
		`corev1 "k8s.io/api/core/v1"`,
		`github.com/gardener/gardener/pkg/apis/core/v1beta1`,
		`github.com/gardener/gardener/pkg/client/kubernetes`,
		`github.com/gardener/gardener/pkg/logger`,
		`github.com/gardener/gardener/extensions/test/e2e/framework/executor`,
		`github.com/sirupsen/logrus`,
		`k8s.io/apimachinery/pkg/api/errors`,
		`k8s.io/apimachinery/pkg/labels`,
		`k8s.io/apimachinery/pkg/types`,
		`k8s.io/apimachinery/pkg/util/sets`,
		`metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"`,
		`networkingv1 "k8s.io/api/networking/v1"`,
		`networkpolicies "github.com/gardener/gardener/extensions/test/e2e/framework/networkpolicies"`,
		`sigs.k8s.io/controller-runtime/pkg/client`,
	)
}

func (g *genTest) simpleArgs(kv ...interface{}) interface{} {
	m := map[interface{}]interface{}{}
	for i := 0; i < len(kv)/2; i++ {
		m[kv[i*2]] = kv[i*2+1]
	}
	return m
}

// GenerateType makes the body of a file implementing a set for type t.
func (g *genTest) Init(c *generator.Context, w io.Writer) error {
	sw := generator.NewSnippetWriter(w, c, "$", "$")
	sw.Do(setHeader, nil)
	g.generateValues(sw)
	sw.Do(setBody, g.simpleArgs("providerName", g.provider.Provider(), "sourceVarName", g.sources()))
	g.correctPolicies(sw)
	g.ingressFromOtherNamespaces(sw)
	g.egressToOtherNamespaces(sw)
	g.egressToSeedNodes(sw)
	g.egressForMirroredPods(sw)
	sw.Do("})\n", nil)
	return nil
}

func (g *genTest) sources() string {
	sourceNames := g.provider.Sources()
	names := []string{}
	for _, source := range sourceNames {
		names = append(names, podToVariableName(&source.Pod))
	}
	return strings.Join(names, ",\n")
}

func (g *genTest) correctPolicies(sw *generator.SnippetWriter) {
	sw.Do(`
Context("components are selected by correct policies", func() {
	var (
		assertHasNetworkPolicy = func(sourcePod *networkpolicies.SourcePod) func(context.Context) {
			return func(ctx context.Context) {
				if !sourcePod.Pod.CheckVersion(f.Shoot) {
					Skip("Component doesn't match Shoot version constraints. Skipping.")
				}
				if !sourcePod.Pod.CheckSeedCluster(sharedResources.SeedCloudProvider) {
					Skip("Component doesn't match Seed Provider constraints. Skipping.")
				}

				matched := sets.NewString()
				var podLabelSet labels.Set

				By(fmt.Sprintf("Getting first running pod with selectors %q in namespace %q", sourcePod.Pod.Labels, f.ShootSeedNamespace()))
				pod, err := framework.GetFirstRunningPodWithLabels(ctx, sourcePod.Pod.Selector(), f.ShootSeedNamespace(), f.SeedClient)
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

	for _, s := range g.provider.Rules() {
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

	for _, s := range g.provider.Rules() {

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
	for _, s := range g.provider.Rules() {
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
				assertCannotConnectToPod(ctx, networkpolicies.NewNamespacedSourcePod(from, sharedResources.Mirror), networkpolicies.NewNamespacedTargetPod(agnostic.Busybox().DummyPort(), sharedResources.External))
			}
		}
	)

	`, nil)

	for _, s := range g.provider.Rules() {
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
				assertConnectToPod(ctx, networkpolicies.NewNamespacedSourcePod(agnostic.Busybox(), sharedResources.External), networkpolicies.NewNamespacedTargetPod(to, f.ShootSeedNamespace()), allowed)
			}
		}
	)

	`, nil)
	for _, tp := range g.provider.EgressFromOtherNamespaces((&networkpolicies.Agnostic{}).Busybox()).TargetPods {
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
	for _, s := range g.provider.Rules() {

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
	ginkgo.RunSpecs(t, "Network Policies e2e Test Suite")
}
`

var setHeader = `
var (
	cleanup        = flag.Bool("cleanup", false, "deletes all created e2e resources after the test suite is done")
)

const (
	InitializationTimeout = 10 * time.Minute
	FinalizationTimeout   = time.Minute
	DefaultTestTimeout    = 10 * time.Second
)

func init() {
	framework.RegisterShootFrameworkFlags()
}

var _ = Describe("Network Policy Testing", func() {

	var (
		f               = framework.NewShootFramework(nil)
		sharedResources networkpolicies.SharedResources

		agnostic   = &networkpolicies.Agnostic{}
		DefaultCIt = func(text string, body func(ctx context.Context)) {
			f.Default().CIt(text, body, DefaultTestTimeout)
		}

		getTargetPod = func(ctx context.Context, targetPod *networkpolicies.NamespacedTargetPod) *corev1.Pod {
			if !targetPod.Pod.CheckVersion(f.Shoot) {
				Skip("Target pod doesn't match Shoot version constraints. Skipping.")
			}
			if !targetPod.Pod.CheckSeedCluster(sharedResources.SeedCloudProvider) {
				Skip("Component doesn't match Seed Provider constraints. Skipping.")
			}
			By(fmt.Sprintf("Checking that target Pod: %s is running", targetPod.Pod.Name))
			err := f.WaitUntilPodIsRunningWithLabels(ctx, targetPod.Pod.Selector(), targetPod.Namespace, f.SeedClient)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By(fmt.Sprintf("Get target pod: %s", targetPod.Pod.Name))
			trgPod, err := framework.GetFirstRunningPodWithLabels(ctx, targetPod.Pod.Selector(), targetPod.Namespace, f.SeedClient)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			return trgPod
		}


		establishConnectionToHost = func(ctx context.Context, nsp *networkpolicies.NamespacedSourcePod, host string, port int32) (stdout, stderr string, err error) {
			if !nsp.Pod.CheckVersion(f.Shoot) {
				Skip("Source pod doesn't match Shoot version constraints. Skipping.")
			}
			if !nsp.Pod.CheckSeedCluster(sharedResources.SeedCloudProvider) {
				Skip("Component doesn't match Seed Provider constraints. Skipping.")
			}
			By(fmt.Sprintf("Checking for source Pod: %s is running", nsp.Pod.Name))
			ExpectWithOffset(1, f.WaitUntilPodIsRunningWithLabels(ctx, nsp.Pod.Selector(), nsp.Namespace, f.SeedClient)).NotTo(HaveOccurred())

			command := []string{"nc", "-vznw", "3", host, fmt.Sprint(port)}
			By(fmt.Sprintf("Executing connectivity command in %s/%s to %s", nsp.Namespace, nsp.Pod.Name, strings.Join(command, " ")))

			return executor.NewExecutor(f.SeedClient).
				ExecCommandInContainerWithFullOutput(ctx, nsp.Namespace, nsp.Pod.Name, "busybox-0", command...)
		}

		assertCannotConnectToHost = func(ctx context.Context, sourcePod *networkpolicies.NamespacedSourcePod, host string, port int32) {
			_, stderr, err := establishConnectionToHost(ctx, sourcePod, host, port)
			ExpectWithOffset(1, err).To(HaveOccurred())
			By("Connection message is timed out\n")
			ExpectWithOffset(1, stderr).To(SatisfyAny(ContainSubstring("Connection timed out"), ContainSubstring("nc: bad address")))
		}

		assertConnectToHost = func(ctx context.Context, sourcePod *networkpolicies.NamespacedSourcePod, targetHost *networkpolicies.Host, allowed bool) {
			_, stderr, err := establishConnectionToHost(ctx, sourcePod, targetHost.HostName, targetHost.Port)
			if allowed {
				ExpectWithOffset(1, err).NotTo(HaveOccurred())
			} else {
				ExpectWithOffset(1, err).To(HaveOccurred())
				ExpectWithOffset(1, stderr).To(SatisfyAny(BeEmpty(), ContainSubstring("Connection timed out"), ContainSubstring("nc: bad address")), "stderr has correct message")
			}
		}

		assertCannotConnectToPod = func(ctx context.Context, sourcePod *networkpolicies.NamespacedSourcePod, targetPod *networkpolicies.NamespacedTargetPod) {
			pod := getTargetPod(ctx, targetPod)
			assertCannotConnectToHost(ctx, sourcePod, pod.Status.PodIP, targetPod.Port.Port)
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

		var err error

		// The framework has to be manually initialized as BeforeEach is not allowed to be called inside a SynchronizedBeforeSuite
		f = &framework.ShootFramework{
			GardenerFramework: framework.NewGardenerFrameworkFromConfig(nil),
			TestDescription:   framework.NewTestDescription("SHOOT"),
			Config:            nil,
		}
		f.CommonFramework.BeforeEach()
		f.GardenerFramework.BeforeEach()
		f.BeforeEach(ctx)

		By("Getting Seed Cloud Provider")
		sharedResources.SeedCloudProvider = f.Seed.Spec.Provider.Type

		By("Creating namespace for Ingress testing")
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "gardener-e2e-network-policies-",
				Labels: map[string]string{
					"gardener-e2e-test": "networkpolicies",
				},
			},
		}
		err = f.SeedClient.Client().Create(ctx, ns)
		Expect(err).NotTo(HaveOccurred())

		sharedResources.External = ns.GetName()

		By("Creating mirror namespace for pod2pod network testing")
		mirrorNamespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "gardener-e2e-mirror-network-policies-",
				Labels: map[string]string{
					"gardener-e2e-test": "networkpolicies",
				},
			},
		}
		err = f.SeedClient.Client().Create(ctx, mirrorNamespace)
		Expect(err).NotTo(HaveOccurred())

		sharedResources.Mirror = mirrorNamespace.GetName()

		By(fmt.Sprintf("Getting all network policies in namespace %q", f.ShootSeedNamespace()))
		list := &networkingv1.NetworkPolicyList{}
		err = f.SeedClient.Client().List(ctx, list, client.InNamespace(f.ShootSeedNamespace()))
		Expect(err).ToNot(HaveOccurred())

		sharedResources.Policies = list.Items

		for _, netPol := range sharedResources.Policies {
			cpy := &networkingv1.NetworkPolicy{}
			cpy.Name = netPol.Name
			cpy.Namespace = sharedResources.Mirror
			cpy.Spec = *netPol.Spec.DeepCopy()
			By(fmt.Sprintf("Copying network policy %s in namespace %q", netPol.Name, sharedResources.Mirror))
			err = f.SeedClient.Client().Create(ctx, cpy)
			Expect(err).NotTo(HaveOccurred())
		}

		By("Getting the current CloudProvider")
		currentProvider := f.Shoot.Spec.Provider.Type

		getFirstNodeInternalIP := func(ctx context.Context, cl kubernetes.Interface) (string, error) {
			nodes := &corev1.NodeList{}
			err := cl.Client().List(ctx, nodes, client.Limit(1))
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

			return "", framework.ErrNoInternalIPsForNodeWasFound
		}

		By("Getting fist running node")
		sharedResources.SeedNodeIP, err = getFirstNodeInternalIP(ctx, f.SeedClient)
		Expect(err).NotTo(HaveOccurred())

		if currentProvider != "$.providerName$" {
			Fail(fmt.Sprintf("Not supported cloud provider %s", currentProvider))
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
					Ports: []corev1.ContainerPort{port},
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
			err := f.SeedClient.Client().Create(ctx, pod)
			Expect(err).NotTo(HaveOccurred())

			By(fmt.Sprintf("Waiting foo Pod %s/%s to be running", sourcePod.Namespace, sourcePod.Name))
			err = framework.WaitUntilPodIsRunning(ctx, f.Logger, pod.GetName(), sourcePod.Namespace, f.SeedClient)
			if err != nil {
				Fail(fmt.Sprintf("Couldn't find running busybox %s/%s", sourcePod.Namespace, pod.GetName()))
			}
		}

		sources := []*networkpolicies.SourcePod{
			$.sourceVarName$,
		}

		var wg sync.WaitGroup
		// one extra for the busybox Pod bellow.
		wg.Add(len(sources) + 1)

		for _, s := range sources {
			go func(pi *networkpolicies.SourcePod) {
				defer GinkgoRecover()
				defer wg.Done()
				if !pi.Pod.CheckVersion(f.Shoot) || !pi.Pod.CheckSeedCluster(sharedResources.SeedCloudProvider) {
					return
				}
				pod, err := framework.GetFirstRunningPodWithLabels(ctx, pi.Pod.Selector(), f.ShootSeedNamespace(), f.SeedClient)
				if err != nil {
					Fail(fmt.Sprintf("Couldn't find running Pod %s/%s with labels: %+v", f.ShootSeedNamespace(), pi.Pod.Name, pi.Pod.Labels))
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
			}(s)
		}
		go func() {
			defer GinkgoRecover()
			defer wg.Done()
			createBusyBox(ctx, networkpolicies.NewNamespacedSourcePod(agnostic.Busybox(), ns.GetName()), corev1.ContainerPort{ContainerPort: 8080})
		}()

		wg.Wait()

		b, err := json.Marshal(sharedResources)
		Expect(err).NotTo(HaveOccurred())

		return b
	}, func(data []byte) {
		sr := &networkpolicies.SharedResources{}
		err := json.Unmarshal(data, sr)
		Expect(err).NotTo(HaveOccurred())

		sharedResources = *sr
	})

	SynchronizedAfterSuite(func() {
		if !*cleanup {
			return
		}

		ctx, cancel := context.WithTimeout(context.TODO(), FinalizationTimeout)
		defer cancel()

		namespaces := &corev1.NamespaceList{}
		selector := labels.SelectorFromSet(labels.Set{
			"gardener-e2e-test": "networkpolicies",
		})
		err := f.SeedClient.Client().List(ctx, namespaces, client.MatchingLabelsSelector{Selector: selector})
		Expect(err).NotTo(HaveOccurred())

		for _, ns := range namespaces.Items {
			err = f.SeedClient.Client().Delete(ctx, &ns)
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
		)

		var (
			assertPolicyIsGone = func(policyName string) func(ctx context.Context) {
				return func(ctx context.Context) {
					By(fmt.Sprintf("Getting network policy %q in namespace %q", policyName, f.ShootSeedNamespace()))
					getErr := f.SeedClient.Client().Get(ctx, types.NamespacedName{Name: policyName, Namespace: f.ShootSeedNamespace()}, &networkingv1.NetworkPolicy{})
					Expect(getErr).To(HaveOccurred())
					By("error is NotFound")
					Expect(errors.IsNotFound(getErr)).To(BeTrue())
				}
			}
		)

		DefaultCIt(deprecatedKubeAPIServerPolicy, assertPolicyIsGone(deprecatedKubeAPIServerPolicy))
		DefaultCIt(deprecatedMetadataAppPolicy, assertPolicyIsGone(deprecatedMetadataAppPolicy))
		DefaultCIt(deprecatedMetadataRolePolicy, assertPolicyIsGone(deprecatedMetadataRolePolicy))
	})
`
