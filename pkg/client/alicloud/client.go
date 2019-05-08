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

package alicloud

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/vpc"
)

// DefaultInternetChargeType is used for EIP
const DefaultInternetChargeType = "PayByTraffic"

type client struct {
	vpcCli *vpc.Client
}

// NewClient creates a new Client for the given Alicloud credentials <accessKeyID>, <accessKeySecret>, and
// the region <region>.
func NewClient(accessKeyID, accessKeySecret, region string) (ClientInterface, error) {
	var vpcCli *vpc.Client
	var err error
	if accessKeyID != "" && accessKeySecret != "" && region != "" {
		vpcCli, err = vpc.NewClientWithAccessKey(region, accessKeyID, accessKeySecret)
	} else {
		err = errors.New("alicloudAccessKeyID or alicloudAccessKeySecret can't be empty")
	}

	return &client{vpcCli}, err
}

//GetCIDR gets CIDR of the VPC specified by vpcID
func (c *client) GetCIDR(vpcID string) (string, error) {
	req := vpc.CreateDescribeVpcsRequest()
	req.VpcId = vpcID

	resp, err := c.vpcCli.DescribeVpcs(req)
	if err != nil {
		return "", err
	}

	if len(resp.Vpcs.Vpc) != 1 {
		return "", fmt.Errorf("Can't get VPC via vpc id: %s", vpcID)
	}
	vpc := resp.Vpcs.Vpc[0]

	return vpc.CidrBlock, err
}

//GetNatGatewayID gets NatGatewayID and SnatTableID of the VPC specified by vpcID
func (c *client) GetNatGatewayInfo(vpcID string) (string, string, error) {
	req := vpc.CreateDescribeNatGatewaysRequest()
	req.VpcId = vpcID

	resp, err := c.vpcCli.DescribeNatGateways(req)
	if err != nil {
		return "", "", err
	}

	if len(resp.NatGateways.NatGateway) != 1 {
		return "", "", fmt.Errorf("Can't get NAT Gateway via id: %s", vpcID)
	}
	natgw := resp.NatGateways.NatGateway[0]

	return natgw.NatGatewayId, strings.Join(natgw.SnatTableIds.SnatTableId, ","), nil
}

//GetEIPInternetChargeType gets Binded NatGateway, then get binded IP. If found, return the InternetChargeType
// If not, return PayByTraffic
func (c *client) GetEIPInternetChargeType(vpcID string) (string, error) {
	req := vpc.CreateDescribeNatGatewaysRequest()
	req.VpcId = vpcID

	resp, err := c.vpcCli.DescribeNatGateways(req)
	if err != nil {
		return "", err
	}

	if len(resp.NatGateways.NatGateway) != 1 {
		return DefaultInternetChargeType, nil
	}
	natgw := resp.NatGateways.NatGateway[0]
	if len(natgw.IpLists.IpList) == 0 {
		return DefaultInternetChargeType, nil
	}
	iplist := natgw.IpLists.IpList[0]
	eipReq := vpc.CreateDescribeEipAddressesRequest()
	eipReq.AllocationId = iplist.AllocationId
	eipResp, err := c.vpcCli.DescribeEipAddresses(eipReq)
	if err != nil {
		return "", err
	}
	if len(eipResp.EipAddresses.EipAddress) == 0 {
		return DefaultInternetChargeType, nil
	}

	return eipResp.EipAddresses.EipAddress[0].InternetChargeType, nil
}
