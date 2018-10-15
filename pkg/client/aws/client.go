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
	"github.com/aws/aws-sdk-go/aws/awserr"
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

// The following functions are only temporary needed due to https://github.com/gardener/gardener/issues/129.

// ListKubernetesELBs returns the list of load balancers in the given <vpcID> tagged with <clusterName>.
func (c *Client) ListKubernetesELBs(vpcID, clusterName string) ([]string, error) {
	output, err := c.ELB.DescribeLoadBalancers(&elb.DescribeLoadBalancersInput{})
	if err != nil {
		return nil, err
	}

	results := []string{}
	for _, lb := range output.LoadBalancerDescriptions {
		if lb.VPCId != nil && *lb.VPCId == vpcID {
			tags, err := c.ELB.DescribeTags(&elb.DescribeTagsInput{
				LoadBalancerNames: []*string{lb.LoadBalancerName},
			})
			if err != nil {
				return nil, err
			}

			for _, description := range tags.TagDescriptions {
				for _, tag := range description.Tags {
					if tag.Key != nil && *tag.Key == fmt.Sprintf("kubernetes.io/cluster/%s", clusterName) && tag.Value != nil && *tag.Value == "owned" {
						results = append(results, *lb.LoadBalancerName)
					}
				}
			}
		}
	}

	return results, nil
}

// DeleteELB deletes the load balancer with the specific <name>. If it does not exist,
// no error is returned.
func (c *Client) DeleteELB(name string) error {
	if _, err := c.ELB.DeleteLoadBalancer(&elb.DeleteLoadBalancerInput{LoadBalancerName: aws.String(name)}); err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == elb.ErrCodeAccessPointNotFoundException {
			return nil
		}
		return err
	}
	return nil
}

// ListKubernetesSecurityGroups returns the list of security groups in the given <vpcID> tagged with <clusterName>.
func (c *Client) ListKubernetesSecurityGroups(vpcID, clusterName string) ([]string, error) {
	groups, err := c.EC2.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(vpcID)},
			},
			{
				Name:   aws.String("tag-key"),
				Values: []*string{aws.String(fmt.Sprintf("kubernetes.io/cluster/%s", clusterName))},
			},
			{
				Name:   aws.String("tag-value"),
				Values: []*string{aws.String("owned")},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	results := []string{}
	for _, group := range groups.SecurityGroups {
		results = append(results, *group.GroupId)
	}

	return results, nil
}

// DeleteSecurityGroup deletes the security group with the specific <id>. If it does not exist,
// no error is returned.
func (c *Client) DeleteSecurityGroup(id string) error {
	if _, err := c.EC2.DeleteSecurityGroup(&ec2.DeleteSecurityGroupInput{GroupId: aws.String(id)}); err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "InvalidGroup.NotFound" {
			return nil
		}
		return err
	}
	return nil
}
