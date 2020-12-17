// +build tools

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

// This package imports things required by build scripts, to force `go mod` to see them as dependencies
package tools

import (
	_ "github.com/ahmetb/gen-crd-api-reference-docs"
	_ "github.com/golang/mock/mockgen"
	_ "github.com/onsi/ginkgo/ginkgo"
	_ "golang.org/x/lint/golint"
	_ "k8s.io/code-generator"
	_ "k8s.io/code-generator/cmd/go-to-protobuf/protoc-gen-gogo"
	_ "k8s.io/kube-openapi/cmd/openapi-gen"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
