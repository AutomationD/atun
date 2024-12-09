/*
 * SPDX-License-Identifier: Apache-2.0
 * SPDX-FileCopyrightText: © 2024 Dmitry Kireev
 */
package e2e

import (
	"context"
	"fmt"
	"github.com/automationd/atun/internal/logger"
	"github.com/testcontainers/testcontainers-go"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go/modules/localstack"
)

/*
# as per https://docs.localstack.cloud/user-guide/aws/ssm/
docker pull amazonlinux:2023.6.20241121.0
docker tag amazonlinux:2023.6.20241121.0 localstack-ec2/ubuntu-focal-docker-ami:ami-00a001

aws ec2 run-instances \
    --image-id ami-00a001 --count 1

aws ssm send-command --document-name "AWS-RunShellScript" \
    --document-version "1" \
    --instance-ids i-7fc504d873c6c60d0 \
    --parameters "commands='whoami'"

*/

func TestAtunLifecycle(t *testing.T) {
	ctx := context.Background()

	// Create temp AWS profile
	configPath, credentialsPath, err := setupTempAWSProfile()
	if err != nil {
		t.Fatalf("Failed to set up temp AWS profile: %v", err)
	}

	log.Printf("AWS conf %s, %s", configPath, credentialsPath)

	// Set environment variables
	os.Setenv("AWS_PROFILE", "localstack")
	os.Setenv("AWS_CONFIG_FILE", configPath)
	os.Setenv("AWS_SHARED_CREDENTIALS_FILE", credentialsPath)

	// Start LocalStack
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

	// Setup AWS Config for LocalStack
	localstackAuthToken := os.Getenv("LOCALSTACK_AUTH_TOKEN")
	if localstackAuthToken == "" {
		t.Fatalf("LOCALSTACK_AUTH_TOKEN environment variable is not set")
	}

	localstackContainer, err = localstack.Run(ctx, "localstack/localstack:4.0.3",
		testcontainers.WithEnv(map[string]string{
			"LOCALSTACK_API_KEY": localstackAuthToken,
		}))
	if err != nil {
		t.Fatalf("Failed to start LocalStack container: %v", err)
	}
	defer func() {
		if err := testcontainers.TerminateContainer(localstackContainer); err != nil {
			log.Printf("failed to terminate container: %s", err)
		}
	}()

	endpoint, err := localstackContainer.PortEndpoint(ctx, "4566/tcp", "")
	if err != nil {
		t.Fatalf("Failed to fetch LocalStack endpoint: %v", err)
	}

	awsConfig := &aws.Config{
		Region:           aws.String("us-east-1"),
		Endpoint:         aws.String(endpoint),
		Credentials:      credentials.NewSharedCredentials(credentialsPath, "localstack"),
		DisableSSL:       aws.Bool(true),
		S3ForcePathStyle: aws.Bool(true),
	}
	sess, err := session.NewSession(awsConfig)
	if err != nil {
		t.Fatalf("Failed to create AWS session: %v", err)
	}
	ec2Client := ec2.New(sess)

	// Create VPC
	vpcID, err := createMockVPC(ec2Client, "10.0.0.0/16", "mock-vpc-55555")
	if err != nil {
		t.Fatalf("Failed to create VPC: %v", err)
	}
	t.Log("VPC ID:", vpcID)

	// Create Subnet
	subnetID, err := createMockSubnet(ec2Client, vpcID, "mock-subnet-55555")
	if err != nil {
		t.Fatalf("Failed to create subnet: %v", err)
	}
	t.Log("Subnet ID:", subnetID)

	println(subnetID)

	// Prepare working directory with config file
	workDir := prepareWorkDir(t, subnetID)
	if err != nil {
		t.Fatalf("Failed to prepare working directory: %v", err)
	}

	// Run `atun create`
	t.Log("Running `atun create`")
	createCmd := exec.Command("atun", "create")
	createCmd.Dir = workDir
	createCmd.Env = append(
		os.Environ(),
		fmt.Sprintf("AWS_ENDPOINT=%s", endpoint),
		"ATUN_LOG_LEVEL=debug",
	)
	createOutput, err := createCmd.CombinedOutput()
	if err := createCmd.Run(); err != nil {
		logger.Fatal("Failed to run `atun create`", "output", string(createOutput))
	}

	// Verify EC2 instance with tag exists
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

func setupTempAWSProfile() (string, string, error) {
	tmpDir, err := os.MkdirTemp("", "awsconfig")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Write AWS credentials
	credentialsPath := filepath.Join(tmpDir, "credentials")
	credentialsContent := `[localstack]
aws_access_key_id = test
aws_secret_access_key = test
# endpoint_url = http://localhost:4566 # works only with go sdk v2 (not implemented in v1)
`
	if err := os.WriteFile(credentialsPath, []byte(credentialsContent), 0644); err != nil {
		return "", "", fmt.Errorf("failed to write credentials: %w", err)
	}

	// Write AWS config
	configPath := filepath.Join(tmpDir, "config")
	configContent := `[profile localstack]
region = us-east-1
output = json
# endpoint_url = http://localhost:4566
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return "", "", fmt.Errorf("failed to write config: %w", err)
	}

	return configPath, credentialsPath, nil
}

func createMockSubnet(ec2Client *ec2.EC2, vpcID string, mockID string) (string, error) {
	output, err := ec2Client.CreateSubnet(&ec2.CreateSubnetInput{
		VpcId:     aws.String(vpcID),
		CidrBlock: aws.String("10.0.1.0/24"),
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String("subnet"),
				Tags: []*ec2.Tag{
					{Key: aws.String("MockID"), Value: aws.String(mockID)},
				},
			},
		},
	})
	if err != nil {
		return "", err
	}
	return *output.Subnet.SubnetId, nil
}

// createMockVPC creates a VPC with a CIDR block and adds a "MockID" tag.
func createMockVPC(ec2Client *ec2.EC2, cidrBlock string, mockID string) (string, error) {
	// Create the VPC
	output, err := ec2Client.CreateVpc(&ec2.CreateVpcInput{
		CidrBlock: aws.String(cidrBlock),
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String("vpc"),
				Tags: []*ec2.Tag{
					{Key: aws.String("MockID"), Value: aws.String(mockID)},
				},
			},
		},
	})
	if err != nil {
		return "", err
	}

	// Return the generated VPC ID
	return *output.Vpc.VpcId, nil
}

// prepareWorkDir creates a temporary TOML file with the provided content and bastion subnet.
func prepareWorkDir(t *testing.T, bastionSubnet string) string {
	t.Helper()

	// Dynamically generate the file content
	content := fmt.Sprintf(`
aws_profile="localstack"
aws_region="us-east-1"
bastion_subnet="%s"

[[hosts]]
name = "ipconfig.io"
proto = "ssm"
remote = "80"
local = "10080"

[[hosts]]
name = "icanhazip.com"
proto = "ssm"
remote = "80"
local = "10081"
`, bastionSubnet)

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "atun")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create a regular file within the directory
	tmpFilePath := filepath.Join(tmpDir, "atun.toml")
	tmpFile, err := os.Create(tmpFilePath)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Write content to the file
	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write to file: %v", err)
	}
	tmpFile.Close()

	// Schedule cleanup
	t.Cleanup(func() {
		os.Remove(tmpFile.Name())
	})
	log.Printf(tmpFile.Name())
	return tmpDir
}
