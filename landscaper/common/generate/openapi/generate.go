// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package main

import (
	"bufio"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/sets"
)

// filterPackagesDelimiter is the delimiter for packages that should not be searched for imports
const filterPackagesDelimiter = ";"

var (
	// flags
	outputDirectory, inputDirectory, rootDirectory, rootPackage, licensePath, verbosity, filterPackages string

	rootCommand = &cobra.Command{
		Use:  "generate-openapi",
		Long: `Wraps the kube-openapi binary to automatically parse the necessary import directories from a given import path`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return execute()
		},
	}

	// filteredFileNames are the name of files that are not searched for imports
	filteredFileNames = sets.NewString("conversions.go", "defaults.go", "doc.go", "register.go")
)

func init() {
	rootCommand.Flags().StringVar(
		&outputDirectory,
		"output-path",
		"",
		"the output path for the generated OpenAPI code")

	rootCommand.Flags().StringVar(
		&inputDirectory,
		"input-directory",
		"",
		"the absolute input directory which should contain the go types to generate OpenAPI code for. Example: /Users/<superuser>/go/src/github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports/v1alpha1")

	rootCommand.Flags().StringVar(
		&rootDirectory,
		"root-directory",
		"",
		"the absolute vendor directory to parse dependent packages")

	rootCommand.Flags().StringVar(
		&rootPackage,
		"package",
		"",
		"the root golang package to generate OpenAPI code for. Example: github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports/v1alpha1")

	rootCommand.Flags().StringVar(
		&licensePath,
		"license-path",
		"",
		"path to the license boilerplate. Defaulted to <root-directory>/hack/LICENSE_BOILERPLATE.txt")

	rootCommand.Flags().StringVar(
		&filterPackages,
		"filter-packages",
		"",
		"semicolon seperated list of packages for whose types no OpenAPI go-code should be generated")

	rootCommand.Flags().StringVar(
		&verbosity,
		"verbosity",
		"1",
		"the verbosity of the kube-openapi logger")

	rootCommand.SilenceUsage = true
}

func main() {
	if err := rootCommand.Execute(); err != nil {
		fmt.Print(err)
		os.Exit(1)
	}
}

// execute gathers the relevant transitive directories from the golang imports from the import directory
// then executes the openapi-gen binary using the gathered input directories to generate OpenAPI go-code
// to the path given by `output-path`
func execute() error {
	if len(outputDirectory) == 0 {
		return fmt.Errorf("an output path has to be specified")
	}

	if len(rootPackage) == 0 {
		return fmt.Errorf("a package has to be specified")
	}

	if len(inputDirectory) == 0 {
		return fmt.Errorf("an import directory has to be specified")
	}

	if len(rootDirectory) == 0 {
		return fmt.Errorf("the project's root directory has to be specified")
	}

	fp, err := filepath.Abs(inputDirectory)
	if err != nil {
		return err
	}
	info, err := os.Stat(fp)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("import path is not a directory")
	}

	filter := sets.NewString(strings.Split(filterPackages, filterPackagesDelimiter)...)
	inputPackages, err := parseImportPackages(inputDirectory, rootDirectory, sets.NewString(rootPackage), filter)
	if err != nil {
		return err
	}

	fmt.Printf("Found %d packages for openapi-gen \n", len(inputPackages))

	// add the root package containing the landscaper component's import types
	inputPackages = append(inputPackages, rootPackage)

	binaryPath, err := exec.LookPath("openapi-gen")
	if err != nil {
		return fmt.Errorf("unable to find the openapi-gen binary. Is it installed?: %v", err)
	}

	// execute the binary and log its outputs
	return executeOpenAPIGen(binaryPath, outputDirectory, licensePath, verbosity, inputPackages)
}

// executeOpenAPIGen executes the `openapi-gen` binary to generate OpenAPI go-code
func executeOpenAPIGen(executablePath string, outputDirectory string, licensePath string, verbosity string, inputPackages []string) error {
	outputPackageName := "openapi"
	reportFilename := fmt.Sprintf("%s/%s/api_violations.report", outputDirectory, outputPackageName)

	// default license path
	if len(licensePath) == 0 {
		licensePath = fmt.Sprintf("%s/hack/LICENSE_BOILERPLATE.txt", rootDirectory)
	}

	args := []string{
		"--v",
		verbosity,
		"--logtostderr",
		fmt.Sprintf("--report-filename=%s", reportFilename),
		fmt.Sprintf("--output-package=%s", outputPackageName),
		fmt.Sprintf("--output-base=%s", outputDirectory),
		fmt.Sprintf("-h"),
		licensePath,
	}

	inputDirs := []string{}
	for _, in := range inputPackages {
		inputDirs = append(inputDirs, fmt.Sprintf("--input-dirs=%s", in))
	}
	args = append(args, inputDirs...)

	cmd := exec.Command(executablePath, args...)

	// the binary logs to STDERR
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	fmt.Printf("Starting OpenAPI generation to directory: %s \n", outputDirectory)
	err = cmd.Start()
	if err != nil {
		return err
	}

	// print the output of the subprocess
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		m := scanner.Text()
		fmt.Printf("%s\n", m)
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("error waiting for openapi-gen: %+v", err)
	}

	fmt.Printf("OpenAPI generation successful. \n")

	return nil
}

// parseImportPackages recursively parses the import packages of a given inputDirectory
// returns the union of all found packages or an error
func parseImportPackages(inputDirectory string, rootDirectory string, alreadyParsedPackages sets.String, filterPackages sets.String) ([]string, error) {
	fset := token.NewFileSet()
	foo, err := parser.ParseDir(fset, inputDirectory, func(info os.FileInfo) bool {
		if info.IsDir() {
			return false
		}

		if filteredFileNames.Has(info.Name()) || strings.HasSuffix(info.Name(), "_test.go") || strings.HasPrefix(info.Name(), "zz_") {
			return false
		}

		return true
	}, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}

	if len(foo) == 0 {
		return nil, nil
	}

	if len(foo) > 1 {
		return nil, fmt.Errorf("found multiple packages in input directory: %s", inputDirectory)
	}

	inputPackages := sets.NewString()
	for _, v := range foo {
		for _, valueFile := range v.Files {
			for _, i := range valueFile.Imports {
				escapedPackage := strings.ReplaceAll(i.Path.Value, "\"", "")
				isK8sPackage := strings.HasPrefix(escapedPackage, "k8s.io")
				isGithubPackage := strings.HasPrefix(escapedPackage, "github.com")

				if !isGithubPackage && !isK8sPackage {
					continue
				}

				if alreadyParsedPackages.Has(escapedPackage) {
					continue
				}

				if filterPackages.Has(escapedPackage) {
					continue
				}

				inputPackages.Insert(escapedPackage)
			}
		}
	}

	for _, inputPackage := range inputPackages.List() {
		// check if the file/directory exists in the vendor directory
		dependentDirectory := fmt.Sprintf("%s/vendor/%s", rootDirectory, inputPackage)
		if !directoryExists(dependentDirectory) {
			// If not, assume the import is in the source path
			// For example:
			//  - Input package: "github.com/gardener/gardener/pkg/scheduler/apis/config/encoding"
			//  - Root directory: "/Users/d060239/go/src/github.com/gardener/gardener"
			// Directory: "/Users/d060239/go/src/github.com/gardener/gardener/pkg/scheduler/apis/config/encoding

			// Assume that the first three elements in the path are packages
			// For example: github.com/gardener/gardener/pkg/scheduler/apis/config/encoding
			split := strings.Split(inputPackage, "/")
			if len(split) <= 2 {
				return nil, fmt.Errorf("unable to parse imports from dependent path %s", inputPackage)
			}

			path := path.Join(split[3:]...)
			dir := fmt.Sprintf("%s/%s", rootDirectory, path)
			if !directoryExists(dir) {
				return nil, fmt.Errorf("dependent path %q does not exist", dir)
			}
			dependentDirectory = dir
		}

		newPackages, err := parseImportPackages(dependentDirectory, rootDirectory, alreadyParsedPackages, filterPackages)
		if err != nil {
			return nil, err
		}

		alreadyParsedPackages.Insert(inputPackage)
		inputPackages.Insert(newPackages...)
	}

	return inputPackages.List(), nil
}

// directoryExists retursn true if the given directory exists
func directoryExists(dir string) bool {
	_, err := os.Stat(dir)
	return err == nil
}
