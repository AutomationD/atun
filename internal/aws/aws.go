/*
 * SPDX-License-Identifier: Apache-2.0
 * SPDX-FileCopyrightText: © 2024 Dmitry Kireev
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package aws

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/hazelops/atun/internal/config"
	"github.com/pterm/pterm"
	"strings"
)

func NewEC2Client() (*ec2.EC2, error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region: aws.String(config.App.Config.AWSRegion),
		},
		Profile: config.App.Config.AWSProfile,
	})
	if err != nil {
		return nil, err
	}

	ec2Client := ec2.New(sess)
	pterm.Info.Printf("Created EC2 client with profile %s and region %s\n", config.App.Config.AWSProfile, config.App.Config.AWSRegion)
	return ec2Client, nil
}

func ListInstancesWithTag(tagKey, tagValue string) ([]*ec2.Instance, error) {
	ec2Client, err := NewEC2Client()
	if err != nil {
		return nil, err
	}

	input := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String(fmt.Sprintf("tag:%s", tagKey)),
				Values: []*string{aws.String(tagValue)},
			},
		},
	}

	// TODO: Add pagination
	result, err := ec2Client.DescribeInstances(input)
	if err != nil {
		return nil, err
	}

	var instances []*ec2.Instance
	for _, reservation := range result.Reservations {
		instances = append(instances, reservation.Instances...)
	}

	return instances, nil
}

func GetInstanceTags(instanceID string) (map[string]string, error) {
	ec2Client, err := NewEC2Client()
	if err != nil {
		pterm.Error.Println(err)
		return nil, err
	}

	input := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{aws.String(instanceID)},
	}

	result, err := ec2Client.DescribeInstances(input)
	if err != nil {
		pterm.Error.Println(err)
		return nil, err
	}

	tags := make(map[string]string)
	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			for _, tag := range instance.Tags {
				tags[*tag.Key] = *tag.Value
			}
		}
	}

	if len(tags) == 0 {
		pterm.Error.Println("No tags found for instance", instanceID)
		return nil, fmt.Errorf("no tags found for instance %s", instanceID)
	}

	return tags, nil
}

func SendSSHPublicKey(instanceID string, publicKey string) error {
	// This command is executed in the bastion host and it checks if our public publicKey is present. If it's not it uploads it to the authorized_keys file.
	command := fmt.Sprintf(
		`grep -qR "%s" /home/ubuntu/.ssh/authorized_keys || echo "%s" >> /home/ubuntu/.ssh/authorized_keys`,
		strings.TrimSpace(publicKey), strings.TrimSpace(publicKey),
	)

	pterm.Info.Printfln("Sending command: \n%s", command)

	_, err := ssm.New(config.App.Session).SendCommand(&ssm.SendCommandInput{
		InstanceIds:  []*string{&instanceID},
		DocumentName: aws.String("AWS-RunShellScript"),
		Comment:      aws.String("Add an SSH public publicKey to authorized_keys"),
		Parameters: map[string][]*string{
			"commands": {&command},
		},
	})
	if err != nil {
		return fmt.Errorf("can't send SSH public publicKey: %w", err)
	}

	return nil
}
