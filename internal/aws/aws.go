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
	"github.com/automationd/atun/internal/config"
	"github.com/automationd/atun/internal/logger"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/pterm/pterm"
	"strings"
)

func NewEC2Client() (*ec2.EC2, error) {
	logger.Debug("Creating EC2 client.", "profile", config.App.Config.AWSProfile, "awsRegion", config.App.Config.AWSRegion)

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

	return ec2Client, nil
}

func NewSTSClient() (*sts.STS, error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region: aws.String(config.App.Config.AWSRegion),
		},
		Profile: config.App.Config.AWSProfile,
	})
	if err != nil {
		return nil, err
	}

	stsClient := sts.New(sess)

	logger.Debug("Created STS client with profile %s and region %s", config.App.Config.AWSProfile, config.App.Config.AWSRegion)
	return stsClient, nil
}

// ListInstancesWithTag returns a list of EC2 instances with a specific tag
func ListInstancesWithTags(tags map[string]string) ([]*ec2.Instance, error) {
	ec2Client, err := NewEC2Client()
	if err != nil {
		logger.Error("Failed to create EC2 client", "error", err)
		return nil, err
	}

	if len(tags) == 0 {
		return nil, fmt.Errorf("no tags provided for filtering")
	}

	var filters []*ec2.Filter
	for key, value := range tags {
		filters = append(filters, &ec2.Filter{
			Name:   aws.String(fmt.Sprintf("tag:%s", key)),
			Values: []*string{aws.String(value)},
		})
	}

	input := &ec2.DescribeInstancesInput{
		Filters: filters,
	}

	var instances []*ec2.Instance
	err = ec2Client.DescribeInstancesPages(input, func(page *ec2.DescribeInstancesOutput, lastPage bool) bool {
		for _, reservation := range page.Reservations {
			instances = append(instances, reservation.Instances...)
		}
		return !lastPage
	})
	if err != nil {
		logger.Error("Failed to describe instances", "error", err)
		return nil, err
	}

	logger.Debug(fmt.Sprintf("Found %d instances matching tags", len(instances)))
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

func GetAccountId() string {
	stsClient, err := NewSTSClient()
	if err != nil {
		pterm.Error.Println("Error creating STS client:", err)
		return ""
	}

	result, err := stsClient.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		pterm.Error.Println("Error getting caller identity:", err)
		return ""
	}

	return *result.Account
}

func SendSSHPublicKey(instanceID string, publicKey string) error {
	// This command is executed in the bastion host and it checks if our public publicKey is present. If it's not it uploads it to the authorized_keys file.
	command := fmt.Sprintf(
		`grep -qR "%s" /home/ubuntu/.ssh/authorized_keys || echo "%s" >> /home/ubuntu/.ssh/authorized_keys`,
		strings.TrimSpace(publicKey), strings.TrimSpace(publicKey),
	)

	logger.Debug("Sending command", "command", command)

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
