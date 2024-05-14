// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
package oci

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	helmRegistry "helm.sh/helm/v3/pkg/registry"

	"github.com/gardener/gardener/pkg/utils"
)

func TestUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OCI Suite")
}

var (
	registryAddress    string
	exampleChartDigest string
	rawChart           []byte
)

var _ = BeforeSuite(func() {
	var cancel context.CancelFunc
	ctx, cancel := context.WithCancel(context.Background())
	DeferCleanup(cancel)

	var err error
	registryAddress, err = startTestRegistry(ctx)
	Expect(err).NotTo(HaveOccurred())

	c, err := helmRegistry.NewClient()
	Expect(err).NotTo(HaveOccurred())
	rawChart, err = os.ReadFile("./testdata/example-0.1.0.tgz")
	Expect(err).NotTo(HaveOccurred())
	res, err := c.Push(rawChart, fmt.Sprintf("%s/charts/example:0.1.0", registryAddress))
	Expect(err).NotTo(HaveOccurred())
	exampleChartDigest = res.Manifest.Digest
})

func startTestRegistry(ctx context.Context) (string, error) {
	config := &configuration.Configuration{}
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}

	port, err := utils.FindFreePort()
	if err != nil {
		return "", err
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	config.HTTP.Addr = registryAddress
	config.HTTP.DrainTimeout = 3 * time.Second

	// setup logger options
	config.Log.AccessLog.Disabled = true
	config.Log.Level = "error"
	// logrus.SetOutput(io.Discard)

	reg, err := registry.NewRegistry(ctx, config)
	if err != nil {
		return "", err
	}
	go func() {
		_ = reg.ListenAndServe()
	}()
	return addr, nil
}
