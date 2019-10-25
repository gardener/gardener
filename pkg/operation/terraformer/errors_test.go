// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package terraformer

import (
	"regexp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Errors", func() {
	Describe("#retrieveTerraformErrors", func() {
		var (
			errorLog1error1 = `Error waiting to create Router: Error waiting for Creating Router: Quota 'ROUTERS' exceeded.  Limit: 20.0 globally.

  on tf/main.tf line 32, in resource "google_compute_router" "router":
  32: resource "google_compute_router" "router"{`
			errorLog1 = `foo bar
foo
bar
foo

Error: ` + errorLog1error1 + `

`

			errorLog2error1 = `Error creating service account: googleapi: Error 409: Service account shoot--foo--bar already exists within project projects/my-project., alreadyExists

  on tf/main.tf line 11, in resource "google_service_account" "serviceaccount":
  11: resource "google_service_account" "serviceaccount" {`
			errorLog2error2 = `Error creating Network: googleapi: Error 409: The resource 'projects/my-project/global/networks/shoot--foo--bar' already exists, alreadyExists

  on tf/main.tf line 20, in resource "google_compute_network" "network":
  20: resource "google_compute_network" "network" {`
			errorLog2 = `
Initializing the backend...

Initializing provider plugins...

The following providers do not have any version constraints in configuration,
so the latest version was installed.

To prevent automatic upgrades to new major versions that may contain breaking
changes, it is recommended to add version = "..." constraints to the
corresponding provider blocks in configuration, with the constraint strings
suggested below.

* provider.google: version = "~> 2.14"
* provider.null: version = "~> 2.1"

Terraform has been successfully initialized!

You may now begin working with Terraform. Try running "terraform plan" to see
any changes that are required for your infrastructure. All Terraform commands
should now work.

If you ever set or change modules or backend configuration for Terraform,
rerun this command to reinitialize your working directory. If you forget, other
commands will detect it and remind you to do so if necessary.
null_resource.outputs: Refreshing state... [id=1234]
google_service_account.serviceaccount: Creating...
google_compute_network.network: Creating...

Error: ` + errorLog2error1 + `

Error: ` + errorLog2error2 + `

Nothing to do.
			`

			errorLog3error1 = `Error creating IAM Role shoot--foo--bar-bastions: EntityAlreadyExists: Role with name shoot--foo--bar-bastions already exists.
\tstatus code: 409, request id: d9e4221c-d488-4e52-98a9-a2d53a10b0fd

  on tf/main.tf line 280, in resource "aws_iam_role" "bastions":
 280: resource "aws_iam_role" "bastions" {`
			errorLog3error2 = `Error creating IAM Role shoot--foo--bar-nodes: EntityAlreadyExists: Role with name shoot--foo--bar-nodes already exists.
\tstatus code: 409, request id: fb991e24-8a9c-4d92-b613-4ff1c7e7a17c

  on tf/main.tf line 327, in resource "aws_iam_role" "nodes":
 327: resource "aws_iam_role" "nodes" {`
			errorLog3error3 = `Error import KeyPair: InvalidKeyPair.Duplicate: The keypair 'shoot--foo--bar-ssh-publickey' already exists.
\tstatus code: 400, request id: c5df52d5-aca6-459f-8004-1f3dd49a085e

  on tf/main.tf line 393, in resource "aws_key_pair" "kubernetes":
 393: resource "aws_key_pair" "kubernetes" {`
			errorLog3 = `Error: ` + errorLog3error1 + `

Error: ` + errorLog3error2 + `

Error: ` + errorLog3error3 + `
`

			regexUUID         = regexp.MustCompile(`(?i)[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}`)
			regexMultiNewline = regexp.MustCompile(`\n{2,}`)
		)

		It("should parse and format the errors correctly", func() {
			Expect(retrieveTerraformErrors(map[string]string{
				"pod1": errorLog1,
				"pod2": errorLog2,
				"pod3": errorLog3,
			})).To(ConsistOf(
				"-> Pod 'pod1' reported:\n* "+regexUUID.ReplaceAllString(regexMultiNewline.ReplaceAllString(errorLog1error1, "\n"), "<omitted>"),
				"-> Pod 'pod2' reported:\n* "+regexUUID.ReplaceAllString(regexMultiNewline.ReplaceAllString(errorLog2error2, "\n")+"\n* "+regexMultiNewline.ReplaceAllString(errorLog2error1, "\n"), "<omitted>"),
				"-> Pod 'pod3' reported:\n* "+regexUUID.ReplaceAllString(regexMultiNewline.ReplaceAllString(errorLog3error1, "\n")+"\n* "+regexMultiNewline.ReplaceAllString(errorLog3error2, "\n")+"\n* "+regexMultiNewline.ReplaceAllString(errorLog3error3, "\n"), "<omitted>"),
			))
		})
	})
})
