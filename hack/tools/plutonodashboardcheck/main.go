// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type Dashboard struct {
	Title string `json:"title"`
	Uid   string `json:"uid"`
	// Add other fields as needed
}

type MultiError []error

func (m MultiError) Error() string {
	var errMsgs []string
	for _, err := range m {
		errMsgs = append(errMsgs, err.Error())
	}
	return strings.Join(errMsgs, "\n")
}

func validateJSONFile(data []byte) (MultiError, error) {
	errs := make(MultiError, 0)

	decoder := json.NewDecoder(bytes.NewReader(data))
	for {
		_, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			if parseErr, ok := err.(*json.SyntaxError); ok {
				line, col := findLineAndColumn(data, int(parseErr.Offset))
				errMsg := fmt.Sprintf("invalid JSON format at line %d, column %d: %v", line, col, err)
				errs = append(errs, fmt.Errorf(errMsg))
				return errs, nil // Return early on syntax error
			} else {
				return nil, fmt.Errorf("failed to parse JSON: %v", err)
			}
		}
	}

	return errs, nil
}

func validateDashboardFile(filePath string) (MultiError, error) {
	errs := make(MultiError, 0)

	jsonData, err := ioutil.ReadFile(filePath)
	if err != nil {
		return errs, fmt.Errorf("failed to read JSON file: %v", err)
	}

	fileErrs, err := validateJSONFile(jsonData)
	if err != nil {
		return errs, err
	}

	errs = append(errs, fileErrs...)

	var dashboard Dashboard
	err = json.Unmarshal(jsonData, &dashboard)
	if err != nil {
		return errs, fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	errs = append(errs, validateNonEmptyTitle(filePath, dashboard.Title)...)
	errs = append(errs, validateUniqueUID(filePath, dashboard.Uid)...)

	return errs, nil
}

func validateUniqueUID(filePath, uid string) []error {
	var errs []error
	dir := filepath.Dir(filePath)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && path != dir {
			return filepath.SkipDir
		}

		if path != filePath && filepath.Ext(path) == ".json" {
			jsonFile, err := ioutil.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read JSON file: %v", err)
			}

			var dashboard Dashboard
			err = json.Unmarshal(jsonFile, &dashboard)
			if err != nil {
				return fmt.Errorf("failed to unmarshal JSON file: %v", err)
			}

			if dashboard.Uid == uid {
				errs = append(errs, fmt.Errorf("duplicate UID found in file: %s", path))
			}
		}
		return nil
	})

	if err != nil {
		errs = append(errs, err)
	}
	return errs
}

func validateNonEmptyTitle(filePath, title string) MultiError {
	errs := make(MultiError, 0)

	if title == "" {
		errs = append(errs, fmt.Errorf("empty title found in file: %s", filePath))
	}

	return errs
}

func validateDashboardJSONFiles(folderPath string) MultiError {
	errs := make(MultiError, 0)

	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("error accessing path: %v", err)
			return nil
		}

		if info.IsDir() && info.Name() == "dashboards" {
			err := filepath.Walk(path, func(filePath string, fileInfo os.FileInfo, err error) error {
				if err != nil {
					log.Printf("error accessing path: %v", err)
					return nil
				}

				if filepath.Ext(filePath) == ".json" {
					fileErrs, err := validateDashboardFile(filePath)
					if err != nil {
						log.Printf("Dashboard validation failed: %v", err)
						errs = append(errs, err)
					}

					errs = append(errs, fileErrs...)
				}

				return nil
			})

			if err != nil {
				log.Fatalf("error walking through dashboards folder: %v", err)
			}

			return filepath.SkipDir // Skip further traversal of subdirectories under "dashboards"
		}

		return nil
	})

	if err != nil {
		log.Fatalf("error walking through folder: %v", err)
	}

	return errs
}

// findLineAndColumn returns the line and column number for a given offset in a byte array.
func findLineAndColumn(data []byte, offset int) (line, col int) {
	line = 1
	col = 1

	for i, char := range data {
		if i >= offset {
			return line, col
		}

		if char == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}

	return line, col
}

func main() {
	//current path is the directory of main.go file
	currentPath, err := os.Getwd()

	if err != nil {
		log.Fatalf("failed to get current working directory: %v", err)
	}

	// Go levels up to the gardener directory
	parentPath := filepath.Dir(filepath.Dir(filepath.Dir(currentPath)))

	errs := validateDashboardJSONFiles(parentPath)

	if len(errs) > 0 {
		log.Println("Validation errors found in dashboards:")
		for _, err := range errs {
			log.Println(err)
		}
	} else {
		log.Println("All dashboards are valid.")
	}
}
