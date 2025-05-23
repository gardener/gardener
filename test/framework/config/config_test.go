// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"flag"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/test/framework/config"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Framework config TestSuite")
}

type flagData struct {
	String string
	Bool   bool
	Int    int
}

var _ = Describe("Config", func() {

	var (
		fs      *flag.FlagSet
		data    flagData
		tmpfile *os.File

		configContent = `
string: world
bool: true
int: 5
`
	)

	BeforeEach(func() {
		var err error
		fs = flag.NewFlagSet("test", flag.ContinueOnError)
		data = flagData{}

		fs.StringVar(&data.String, "string", "hello", "")
		fs.BoolVar(&data.Bool, "bool", false, "")
		fs.IntVar(&data.Int, "int", 1, "")

		tmpfile, err = os.CreateTemp("", "configtest-*.yaml")
		Expect(err).ToNot(HaveOccurred())
		_, err = tmpfile.WriteString(configContent)
		Expect(err).ToNot(HaveOccurred())
		Expect(tmpfile.Close()).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.Remove(tmpfile.Name())).ToNot(HaveOccurred())
	})

	It("should apply specified configuration from config file", func() {
		err := config.ParseConfigForFlags(tmpfile.Name(), fs)
		Expect(err).ToNot(HaveOccurred(), "read config file")

		Expect(data.String).To(Equal("world"))
		Expect(data.Bool).To(BeTrue())
		Expect(data.Int).To(Equal(5))
	})

	It("should apply specified configuration only to unspecified flags", func() {
		err := fs.Set("int", "10")
		Expect(err).ToNot(HaveOccurred())

		err = config.ParseConfigForFlags(tmpfile.Name(), fs)
		Expect(err).ToNot(HaveOccurred(), "read config file")

		Expect(data.String).To(Equal("world"))
		Expect(data.Bool).To(BeTrue())
		Expect(data.Int).To(Equal(10))
	})

})
