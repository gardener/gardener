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
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/sts"
)

// NewClient creates a new Client for the given AWS credentials <accessKeyID>, <secretAccessKey>, and
// the AWS region <region>.
// It initializes the clients for the various services like EC2, ELB, etc.
func NewClient(accessKeyID, secretAccessKey, region string) ClientInterface {
	var (
		awsConfig = &aws.Config{
			Credentials: credentials.NewStaticCredentials(accessKeyID, secretAccessKey, ""),
		}
		sess   = session.Must(session.NewSession(awsConfig))
		config = &aws.Config{Region: aws.String(region)}
	)

	return &Client{
		EC2: ec2.New(sess, config),
		ELB: elb.New(sess, config),
		STS: sts.New(sess, config),
	}
}

// GetAccountID returns the ID of the AWS account the Client is interacting with.
func (c *Client) GetAccountID() (string, error) {
	getCallerIdentityOutput, err := c.STS.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}
	return *getCallerIdentityOutput.Account, nil
}

// GetInternetGateway returns the ID of the internet gateway attached to the given VPC <vpcID>.
// If there is no internet gateway attached, the returned string will be empty.
func (c *Client) GetInternetGateway(vpcID string) (string, error) {
	describeInternetGatewaysInput := &ec2.DescribeInternetGatewaysInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("attachment.vpc-id"),
				Values: []*string{
					aws.String(vpcID),
				},
			},
		},
	}
	describeInternetGatewaysOutput, err := c.EC2.DescribeInternetGateways(describeInternetGatewaysInput)
	if err != nil {
		return "", err
	}

	if describeInternetGatewaysOutput.InternetGateways != nil {
		if *describeInternetGatewaysOutput.InternetGateways[0].InternetGatewayId == "" {
			return "", fmt.Errorf("no attached internet gateway found for vpc %s", vpcID)
		}
		return *describeInternetGatewaysOutput.InternetGateways[0].InternetGatewayId, nil
	}
	return "", fmt.Errorf("no attached internet gateway found for vpc %s", vpcID)
}

// GetELB returns the AWS LoadBalancer object for the given name <loadBalancerName>.
func (c *Client) GetELB(loadBalancerName string) (*elb.DescribeLoadBalancersOutput, error) {
	describeLoadBalancersInput := &elb.DescribeLoadBalancersInput{
		LoadBalancerNames: []*string{
			aws.String(loadBalancerName),
		},
		PageSize: aws.Int64(1),
	}
	return c.ELB.DescribeLoadBalancers(describeLoadBalancersInput)
}

// UpdateELBHealthCheck updates the AWS LoadBalancer health check target protocol to SSL for a given
// LoadBalancer <loadBalancerName>.
func (c *Client) UpdateELBHealthCheck(loadBalancerName string, targetPort string) error {
	configureHealthCheckInput := &elb.ConfigureHealthCheckInput{
		HealthCheck: &elb.HealthCheck{
			Target:             aws.String("SSL:" + targetPort),
			Timeout:            aws.Int64(5),
			Interval:           aws.Int64(30),
			HealthyThreshold:   aws.Int64(2),
			UnhealthyThreshold: aws.Int64(6),
		},
		LoadBalancerName: aws.String(loadBalancerName),
	}
	_, err := c.ELB.ConfigureHealthCheck(configureHealthCheckInput)
	return err
}
