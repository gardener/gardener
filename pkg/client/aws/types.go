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

package aws

import (
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/sts"
)

// ClientInterface is an interface which must be implemented by AWS clients.
type ClientInterface interface {
	GetAccountID() (string, error)
	GetInternetGateway(string) (string, error)

	// The following functions are only temporary needed due to https://github.com/gardener/gardener/issues/129.
	ListKubernetesELBs(vpcID, clusterName string) ([]string, error)
	ListKubernetesSecurityGroups(vpcID, clusterName string) ([]string, error)
	DeleteELB(name string) error
	DeleteSecurityGroup(id string) error
}

// Client is a struct containing several clients for the different AWS services it needs to interact with.
// * EC2 is the standard client for the EC2 service.
// * ELB is the standard client for the ELB service.
// * STS is the standard client for the STS service.
type Client struct {
	EC2 *ec2.EC2
	ELB *elb.ELB
	STS *sts.STS
}
