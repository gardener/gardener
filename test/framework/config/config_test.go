// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
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
		Expect(data.Bool).To(Equal(true))
		Expect(data.Int).To(Equal(5))
	})

	It("should apply specified configuration only to unspecified flags", func() {
		err := fs.Set("int", "10")
		Expect(err).ToNot(HaveOccurred())

		err = config.ParseConfigForFlags(tmpfile.Name(), fs)
		Expect(err).ToNot(HaveOccurred(), "read config file")

		Expect(data.String).To(Equal("world"))
		Expect(data.Bool).To(Equal(true))
		Expect(data.Int).To(Equal(10))
	})

})
