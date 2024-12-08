//go:build e2e
// +build e2e

/*
 * SPDX-License-Identifier: Apache-2.0
 * SPDX-FileCopyrightText: © 2024 Dmitry Kireev
 */
package e2e

import (
	"context"
	"github.com/testcontainers/testcontainers-go"
	"log"
	"os/exec"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go/modules/localstack"
)

func TestAtunLifecycle(t *testing.T) {
	ctx := context.Background()

	// 1. Start LocalStack
	localstackContainer, err := localstack.Run(ctx, "localstack/localstack:4.0.3")
	defer func() {
		if err := testcontainers.TerminateContainer(localstackContainer); err != nil {
			log.Printf("failed to terminate container: %s", err)
		}
	}()
	if err != nil {
		log.Printf("failed to start container: %s", err)
		return
	}

	// 2. Setup AWS Config for LocalStack
	endpoint, err := localstackContainer.Endpoint(ctx, "ec2")
	if err != nil {
		t.Fatalf("Failed to fetch LocalStack EC2 endpoint: %v", err)
	}

	awsConfig := &aws.Config{
		Region:           aws.String("us-east-1"),
		Endpoint:         aws.String(endpoint),
		Credentials:      credentials.NewStaticCredentials("test", "test", ""),
		DisableSSL:       aws.Bool(true),
		S3ForcePathStyle: aws.Bool(true),
	}

	sess, err := session.NewSession(awsConfig)
	if err != nil {
		t.Fatalf("Failed to create AWS session: %v", err)
	}
	ec2Client := ec2.New(sess)

	// 3. Run `atun create`
	t.Log("Running `atun create`")
	createCmd := exec.Command("atun", "create", "--aws-region", "us-east-1", "--endpoint", endpoint)
	if err := createCmd.Run(); err != nil {
		t.Fatalf("`atun create` failed: %v", err)
	}

	// 4. Verify EC2 instance with tag exists
	t.Log("Verifying EC2 instance exists with tag atun.io/version: 1")
	instances, err := ec2Client.DescribeInstances(&ec2.DescribeInstancesInput{})
	if err != nil {
		t.Fatalf("Failed to describe instances: %v", err)
	}

	var instanceID string
	for _, r := range instances.Reservations {
		for _, i := range r.Instances {
			for _, tag := range i.Tags {
				if aws.StringValue(tag.Key) == "atun.io/version" && aws.StringValue(tag.Value) == "1" {
					instanceID = aws.StringValue(i.InstanceId)
				}
			}
		}
	}

	assert.NotEmpty(t, instanceID, "EC2 instance with tag atun.io/version:1 should exist")

	// 5. Run `atun remove`
	t.Log("Running `atun remove`")
	removeCmd := exec.Command("atun", "remove", "--aws-region", "us-east-1", "--endpoint", endpoint)
	if err := removeCmd.Run(); err != nil {
		t.Fatalf("`atun remove` failed: %v", err)
	}

	// 6. Verify EC2 instance no longer exists
	t.Log("Verifying EC2 instance is removed")
	instancesAfter, err := ec2Client.DescribeInstances(&ec2.DescribeInstancesInput{})
	if err != nil {
		t.Fatalf("Failed to describe instances after removal: %v", err)
	}

	for _, r := range instancesAfter.Reservations {
		for _, i := range r.Instances {
			assert.NotEqual(t, aws.StringValue(i.InstanceId), instanceID, "EC2 instance should no longer exist")
		}
	}

	t.Log("Test completed successfully")
}
