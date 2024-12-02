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

package infra

import (
	"encoding/json"
	"fmt"
	"github.com/automationd/atun/internal/config"
	"github.com/aws/jsii-runtime-go"
	awsprovider "github.com/cdktf/cdktf-provider-aws-go/aws/v19/provider"
	"github.com/hashicorp/terraform-cdk-go/cdktf"

	//"github.com/automationd/atun/internal/config"
	"github.com/pterm/pterm"
	"os"
	"os/exec"
	"path/filepath"
)

// CreateStack defines the CDKTF stack
func CreateStack(c *config.Config) {
	app := cdktf.NewApp(nil)
	stack := cdktf.NewTerraformStack(app, jsii.String("atun-stack"))

	awsprovider.NewAwsProvider(stack, jsii.String("AWS"), &awsprovider.AwsProviderConfig{
		Region:  jsii.String(config.App.Config.AWSRegion),
		Profile: jsii.String(config.App.Config.AWSProfile),
	})

	// TODO: get hosts from atun.toml and add it to the tags with a loop

	atun := config.Atun{
		Version: "1",
		Hosts:   config.InitialApp.Hosts,
		//Hosts: []config.Host{
		//	{
		//		Name:   "ip-10-30-25-144.ec2.internal",
		//		Proto:  "ssm",
		//		Remote: "22",018401
		//		Local:  "10001",
		//	},
		//	{
		//		Name:   "ip-10-30-25-144.ec2.internal",
		//		Proto:  "ssm",
		//		Remote: "443",
		//		Local:  "10002",
		//	},
		//	{
		//		Name:   "ip-10-30-00-00.ec2.internal",
		//		Proto:  "ssm",
		//		Remote: "443",
		//		Local:  "10003",
		//	},
		//	{
		//		Name:   "ip-10-30-00-10.ec2.internal",
		//		Proto:  "ssm",
		//		Remote: "4444",
		//		Local:  "10005",
		//	},
		//},
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

	// Group hosts by their Name and create slices for their configurations
	hostConfigs := make(map[string][]map[string]interface{})

	// Process each host and add it to the final map using the Name as the key
	for _, host := range atun.Hosts {
		key := fmt.Sprintf("atun.io/host/%s", host.Name)
		hostConfig := map[string]interface{}{
			"proto":  host.Proto,
			"local":  host.Local,
			"remote": host.Remote,
		}

		hostConfigs[key] = append(hostConfigs[key], hostConfig)

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
	cdktf.NewTerraformHclModule(stack, jsii.String("Vpc"), &cdktf.TerraformHclModuleConfig{
		// TODO: Parameterize all variables
		Source:  jsii.String("hazelops/ec2-bastion/aws"),
		Version: jsii.String("~>3.0.6"),

		Variables: &map[string]interface{}{
			"env":                 "test",
			"name":                "atun-bastion",
			"ec2_key_pair_name":   "dmitry.kireev",
			"public_subnets":      []string{config.App.Config.BastionSubnetID},
			"private_subnets":     []string{},
			"allowed_cidr_blocks": []string{"0.0.0.0/0"},
			"aws_profile":         config.App.Config.AWSProfile,
			"instance_type":       "t3.nano",
			"vpc_id":              config.App.Config.BastionVPCID,
			"tags":                tags,
		},
	})

	//cdktf.NewRemoteBackend(stack, &cdktf.RemoteBackendProps{
	//	Hostname:     jsii.String("app.terraform.io"),
	//	Organization: jsii.String("<YOUR_ORG>"),
	//	Workspaces:   cdktf.NewNamedRemoteWorkspace(jsii.String("learn-cdktf")),
	//})

	app.Synth()
}

// ApplyCDKTF runs the CDKTF commands
func ApplyCDKTF(c *config.Config) error {
	pterm.Info.Printf("Applying CDKTF stack with profile %s and region %s\n", c.AWSProfile, c.AWSRegion)
	CreateStack(c)
	// Change to the synthesized directory
	synthDir := filepath.Join("cdktf.out", "stacks", "atun-stack")
	cmd := exec.Command("terraform", "init")
	cmd.Dir = synthDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	// TODO: Manage Terraform Install
	cmd = exec.Command("terraform", "apply", "-auto-approve")
	cmd.Dir = synthDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// DestroyCDKTF runs the CDKTF commands
func DestroyCDKTF(config *config.Config) error {
	CreateStack(config)
	// Change to the synthesized directory
	synthDir := filepath.Join("cdktf.out", "stacks", "atun-stack")
	cmd := exec.Command("terraform", "init")
	cmd.Dir = synthDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	// TODO: Manage Install Terraform
	cmd = exec.Command("terraform", "destroy", "-auto-approve")
	cmd.Dir = synthDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
