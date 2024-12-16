/*
 * SPDX-License-Identifier: Apache-2.0
 * SPDX-FileCopyrightText: © 2024 Dmitry Kireev
 */

package infra

import (
	"encoding/json"
	"fmt"
	"github.com/automationd/atun/internal/config"
	"github.com/automationd/atun/internal/constraints"
	"github.com/automationd/atun/internal/logger"
	"github.com/aws/jsii-runtime-go"
	awsprovider "github.com/cdktf/cdktf-provider-aws-go/aws/v19/provider"
	"github.com/hashicorp/terraform-cdk-go/cdktf"
	"os"
	"os/exec"
	"path/filepath"
)

// createStack defines the CDKTF stack (generates Terraform).
func createStack(c *config.Config) {
	app := cdktf.NewApp(&cdktf.AppConfig{
		Outdir: jsii.String(filepath.Join(c.TunnelDir)), // Set your desired directory here
	})

	stack := cdktf.NewTerraformStack(app, jsii.String(fmt.Sprintf("%s-%s", c.AWSProfile, c.Env)))

	// Configure the local backend to store state in the tunnel directory
	cdktf.NewLocalBackend(stack, &cdktf.LocalBackendConfig{
		Path: jsii.String(filepath.Join(c.TunnelDir, "terraform.tfstate")), // Specify state file path
	})

	awsprovider.NewAwsProvider(stack, jsii.String("AWS"), &awsprovider.AwsProviderConfig{
		Region:  jsii.String(c.AWSRegion),
		Profile: jsii.String(c.AWSProfile),
	})

	// TODO: get hosts from atun.toml and add it to the tags with a loop

	atun := config.Atun{
		Version: "1",
		Config:  c,
	}

	//hostConfigJSON, err := json.Marshal(Host{
	//	Proto:  "ssm",
	//	Remote: "22",
	//	Local:  "10001",
	//})
	//
	//if err != nil {
	//	pterm.Error.Sprintf("Error marshalling host config: %v", err)
	//}
	//
	//tags := map[string]interface{}{
	//	"atun.io/version": "1",
	//	fmt.Sprintf("atun.io/host/%s", "ip-10-30-25-144.ec2.internal"): string(hostConfigJSON),
	//}

	// Create a final map to hold the JSON structure
	tags := make(map[string]interface{})

	// Add the version directly to the final map
	tags["atun.io/version"] = atun.Version

	// Set Env
	tags["atun.io/env"] = atun.Config.Env

	// TODO: Support multiple port configurations per host
	// Group hosts by their Name and create slices for their configurations
	hostConfigs := make(map[string]map[string]interface{})

	// Process each host and add it to the final map using the Name as the key
	for _, host := range atun.Config.Hosts {
		key := fmt.Sprintf("atun.io/host/%s", host.Name)
		hostConfig := map[string]interface{}{
			"proto":  host.Proto,
			"local":  host.Local,
			"remote": host.Remote,
		}

		hostConfigs[key] = hostConfig

	}

	// Marshal each grouped configuration into a JSON string and store in finalMap
	for key, configs := range hostConfigs {
		configsJSON, _ := json.Marshal(configs)
		tags[key] = string(configsJSON)
	}

	//// Convert struct to JSON
	//jsonData, err := json.Marshal(atun)
	//if err != nil {
	//	fmt.Println("Error marshaling to JSON:", err)
	//	return
	//}

	//if err := json.Unmarshal(jsonData, &tags); err != nil {
	//	fmt.Println("Error unmarshaling JSON to map:", err)
	//	return
	//}

	// TODO: Add ability to use other Terraform modules. Maybe use a map of modules and their Parameters, like "module-name": {"param1": "value1", "param2": "value2"}
	// Add the module

	if err := constraints.CheckConstraints(
		constraints.WithSSMPlugin(),
		constraints.WithAWSProfile(),
		constraints.WithAWSRegion(),
		constraints.WithENV(),
	); err != nil {
		logger.Fatal("Error checking constraints", "error", err)
	}

	logger.Debug("All constraints satisfied")

	// Override the default ami if one is provided to atun
	bastionHostAmi := ""
	if config.App.Config.BastionHostAMI != "" {
		bastionHostAmi = config.App.Config.BastionHostAMI
	}

	// TODO: Add ability to specify other modules
	terraformVariablesModules := map[string]interface{}{
		"env":                 config.App.Config.Env,
		"name":                config.App.Config.BastionInstanceName,
		"ec2_key_pair_name":   config.App.Config.AWSKeyPair,
		"public_subnets":      []string{},
		"private_subnets":     []string{config.App.Config.BastionSubnetID},
		"allowed_cidr_blocks": []string{"0.0.0.0/0"},
		"instance_type":       config.App.Config.AWSInstanceType,
		"instance_ami":        bastionHostAmi,
		"vpc_id":              config.App.Config.BastionVPCID,
		"tags":                tags,
	}

	logger.Debug("Terraform Variables", "variables", terraformVariablesModules)

	cdktf.NewTerraformHclModule(stack, jsii.String("Vpc"), &cdktf.TerraformHclModuleConfig{
		// TODO: Make an abstraction atun-bastion module so anyone can fork and switch configs
		Source:  jsii.String("hazelops/ec2-bastion/aws"),
		Version: jsii.String("~>4.0"),

		Variables: &terraformVariablesModules,
	})

	//cdktf.NewRemoteBackend(stack, &cdktf.RemoteBackendProps{
	//	Hostname:     jsii.String("app.terraform.io"),
	//	Organization: jsii.String("<YOUR_ORG>"),
	//	Workspaces:   cdktf.NewNamedRemoteWorkspace(jsii.String("learn-cdktf")),
	//})

	app.Synth()
}

// ApplyCDKTF performs the 'apply' of theCDKTF stack
func ApplyCDKTF(c *config.Config) error {
	logger.Info("Applying CDKTF stack.", "profile", c.AWSProfile, "region", c.AWSRegion)

	createStack(c)
	// Change to the synthesized directory
	synthDir := filepath.Join(c.TunnelDir, "stacks", fmt.Sprintf("%s-%s", c.AWSProfile, c.Env))
	cmd := exec.Command("terraform", "init")
	cmd.Dir = synthDir
	if c.LogLevel == "info" || c.LogLevel == "debug" {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		return err
	}

	// TODO: Manage Terraform / OpenTofu Install
	cmd = exec.Command("terraform", "apply", "-auto-approve")
	cmd.Dir = synthDir
	if c.LogLevel == "info" || c.LogLevel == "debug" {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	return cmd.Run()
}

func DestroyCDKTF(c *config.Config) error {
	createStack(c)
	// Change to the synthesized directory
	synthDir := filepath.Join(c.TunnelDir, "stacks", fmt.Sprintf("%s-%s", c.AWSProfile, c.Env))
	cmd := exec.Command("terraform", "init")
	cmd.Dir = synthDir
	if c.LogLevel == "info" || c.LogLevel == "debug" {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		return err
	}

	// TODO: Manage Install Terraform
	cmd = exec.Command("terraform", "destroy", "-auto-approve")
	cmd.Dir = synthDir
	if c.LogLevel == "info" || c.LogLevel == "debug" {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}
