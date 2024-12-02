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

package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/automationd/atun/internal/aws"
	"github.com/automationd/atun/internal/config"
	"github.com/automationd/atun/internal/infra"
	"github.com/automationd/atun/internal/logger"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/hazelops/ize/pkg/term"
	"os/exec"
	"path"

	//"github.com/automationd/atun/internal/aws"
	"github.com/automationd/atun/internal/constraints"
	//"github.com/automationd/atun/internal/infra"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
	"os"
	"path/filepath"
	"strings"
)

// upCmd represents the up command
var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Starts a tunnel to the bastion host",
	Long: `Starts a tunnel to the bastion host and forwards ports to the local machine.

	If the bastion host is not provided, the first running instance with the atun.io/version tag is used.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Use GO Method received on `atun`
		// TODO: Implement VPC, subnet picker (1. get list of VPCs, 2. Get a list of Subnets in it, 3. Ask user if it's not provided)

		// Check if an 'up' command is being invoked (not a subcommand)
		if len(args) == 0 {

			var err error
			var bastionHost string
			logger.Debug("Up command called", "bastion", bastionHost, "aws profile", config.App.Config.AWSProfile, "env", config.App.Config.Env)

			if err := constraints.CheckConstraints(
				constraints.WithSSMPlugin(),
				constraints.WithAWSProfile(),
				constraints.WithAWSRegion(),
				constraints.WithENV(),
			); err != nil {
				return err
			}

			logger.Debug("All constraints satisfied")

			// Get the bastion host ID from the command line
			bastionHost = cmd.Flag("bastion").Value.String()

			// TODO: Add logic if host not found offer create it, add --auto-create-bastion

			// If bastion host is not provided, get the first running instance based on the discovery tag (atun.io/version)
			if bastionHost == "" {
				config.App.Config.BastionHostID, err = getBastionHostID()
				if err != nil {
					logger.Fatal("Error discovering bastion host", "error", err)
				}
			} else {
				config.App.Config.BastionHostID = bastionHost
			}

			logger.Debug("Bastion host ID", "bastion", config.App.Config.BastionHostID)

			// TODO: refactor as a better functional
			// Read atun:config from the instance as `config`
			bastionHostConfig, err := getBastionHostConfig(config.App.Config.BastionHostID)
			config.App.Version = bastionHostConfig.Version
			config.App.Hosts = bastionHostConfig.Hosts

			if err != nil {
				pterm.Error.Printfln("Error getting bastion host config: %v", err)
			}
			//config.App.Config = atun.Config
			//config.App.Hosts = atun.Hosts

			for _, host := range config.App.Hosts {
				// Review the hosts
				logger.Debug("Host", "name", host.Name, "proto", host.Proto, "remote", host.Remote, "local", host.Local)
			}

			// Generate SSH config file
			config.App.Config.SSHConfigFile, err = generateSSHConfigFile(config.App)
			//atun.Config.SSHConfigFile, err = generateSSHConfigFile(atun)
			if err != nil {
				logger.Error("Error generating SSH config file", "SSHConfigFile", config.App.Config.SSHConfigFile, "err", err)
			}
			//if err != nil {
			//	pterm.Error.Printfln("Error writing SSH config to %s: %v", atun.Config.SSHConfigFile, err)
			//}

			logger.Debug("Saved SSH config file", "path", config.App.Config.SSHConfigFile)

			// TODO: Check & Install SSM Agent

			//logrus.Debugf("public key path: %s", publicKeyPath)
			logger.Debug("private key path", "path", config.App.Config.SSHKeyPath)

			//err := o.checkOsVersion()
			//if err != nil {
			//	return err
			//}

			// Read private key from HOME/id_rsa.pub
			publicKey, err := getPublicKey(config.App.Config.SSHKeyPath)
			if err != nil {
				logger.Error("Error getting public key", "error", err)
			}

			logger.Debug("public key", "key", publicKey)

			// Send the public key to the bastion instance
			err = aws.SendSSHPublicKey(config.App.Config.BastionHostID, publicKey)
			if err != nil {
				logger.Error("Can't run tunnel", "error", err)
			}

			logger.Debug("Public key sent to bastion host", "bastion", config.App.Config.BastionHostID)

			// TODO: Refactor naming of forwardConfig
			forwardConfig, err := upTunnel(config.App)
			if err != nil {
				logger.Fatal("Error running tunnel", "error", err)
			}

			// TODO: Check if Instance has forwarding working (check ipv4.forwarding sysctl)

			logger.Info("Tunnel is up! Forwarded ports:", "forwardConfig", forwardConfig)

		} else {
			// TODO: Possibly refactor to use a separate command like install? atun add bastion / atun del|remove bastion?
			if args[0] == "bastion" {
				err := infra.ApplyCDKTF(config.App.Config)
				if err != nil {
					pterm.Info.Printf("Error running CDKTF: %v\n", err)
					return err
				}
				pterm.Info.Println("CDKTF stack applied successfully")
			}

		}

		return nil
	},
}

func runSSH(app *config.Atun, args []string) error {

	c := exec.Command("ssh", args...)

	c.Dir = app.Config.AppDir
	os.Setenv("AWS_REGION", app.Config.AWSRegion)
	os.Setenv("AWS_PROFILE", app.Config.AWSProfile)

	logger.Debug("Command Executed", "command", c.String())

	runner := term.New(term.WithStdin(os.Stdin))
	_, _, code, err := runner.Run(c)
	if err != nil {
		return err
	}

	if code != 0 {
		return fmt.Errorf("exit status: %d", code)
	}
	return nil
}

func getSSHCommandArgs(app *config.Atun) []string {
	bastionSockFilePath := path.Join(app.Config.AppDir, "bastion.sock")
	args := []string{"-M", "-t", "-S", bastionSockFilePath, "-fN"}
	if !app.Config.SSHStrictHostKeyChecking {
		args = append(args, "-o", "StrictHostKeyChecking=no")
	}

	// TODO: Add ability to support other instance types, not just ubuntu
	args = append(args, fmt.Sprintf("ubuntu@%s", app.Config.BastionHostID))
	args = append(args, "-F", app.Config.SSHConfigFile)

	if _, err := os.Stat(app.Config.SSHKeyPath); !os.IsNotExist(err) {
		args = append(args, "-i", app.Config.SSHKeyPath)
	}

	if app.Config.LogLevel == "debug" {
		args = append(args, "-vvv")
	}

	return args
}

// GetBastionHostID retrieves the Bastion Host ID from AWS tags.
// It takes a session, tag name, and tag value as parameters and returns the instance ID of the Bastion Host.
func getBastionHostID() (string, error) {
	logger.Debug("Getting bastion host ID. Looking for atun routers.")

	// Build a map of tags to filter instances
	tags := map[string]string{
		"atun.io/version": config.App.Version,
		"atun.io/env":     config.App.Config.Env,
	}

	instances, err := aws.ListInstancesWithTags(tags)
	if err != nil {
		logger.Error("Error listing instances with tags", "tags", tags)
		return "", err
	}

	if len(instances) == 0 {
		logger.Fatal("No instances found with required tags", "tags", tags)
		return "", err
	}

	logger.Debug("Found instances", "instances", len(instances))

	for _, instance := range instances {
		logger.Debug("Found instance", "instance_id", *instance.InstanceId, "state", *instance.State.Name)

		// Use the first running instance found
		if *instance.InstanceId != "" && *instance.State.Name == "running" {
			return *instance.InstanceId, err
		}
	}

	return "", err
}

// Gets the public key from the private key
func getPublicKey(path string) (string, error) {
	if !filepath.IsAbs(path) {
		var err error
		path, err = filepath.Abs(path)
		if err != nil {
			return "", err
		}
	}

	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("%s does not exist", path)
	}

	f, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// Parse the private key
	privateKey, err := ssh.ParsePrivateKey(f)
	if err != nil {
		return "", err
	}

	// Extract the public key from the private key
	publicKey := privateKey.PublicKey()

	// Marshal the public key to the OpenSSH format
	pubKeyBytes := ssh.MarshalAuthorizedKey(publicKey)

	// Return the public key as a string
	return string(pubKeyBytes), nil
}

// Gets bastion host tags and unmarshals it into a struct
func getBastionHostConfig(bastionHostID string) (config.Atun, error) {
	// TODO:Implement logic:
	// - Get all tags from the host bastionHostID
	// - filter those that have atun.io
	// - unmarshal the tags into a struct

	// Use AWS SDK to get instance tags
	tags, err := aws.GetInstanceTags(bastionHostID)
	if err != nil {
		logger.Error("Error getting instance tags", "instance_id", bastionHostID)
	}

	logger.Debug("Instance tags", "tags", tags)

	atun := config.Atun{}
	for k, v := range tags {
		// Iterate over the tags and use only atun.io tags
		if strings.HasPrefix(k, "atun.io") {
			if k == "atun.io/version" {
				atun.Version = v
			} else if strings.HasPrefix(k, "atun.io/host/") {
				hostName := strings.TrimPrefix(k, "atun.io/host/")

				var host config.Host
				var hosts []config.Host

				err := json.Unmarshal([]byte(v), &host)
				if err != nil {
					logger.Error("Error unmarshalling host tags", "host", hostName, "error", err)
					continue
				}

				// Append the host to the hosts slice
				hosts = append(hosts, host)

				// Assign the hostName to each Host and append to atun.Hosts
				for i := range hosts {
					hosts[i].Name = hostName
					atun.Hosts = append(atun.Hosts, hosts[i])
				}
			}
		}
	}

	return atun, nil

}

//func sendSSHPublicKeyViaMetadata(bastionID string, key string, sess *session.Session) error {
//	_, err := ec2instanceconnect.New(sess).SendSSHPublicKey(&ec2instanceconnect.SendSSHPublicKeyInput{
//		InstanceId:     aws.String(bastionID),
//		InstanceOSUser: aws.String("ubuntu"),
//		SSHPublicKey:   aws.String(key),
//	})
//	if err != nil {
//		return err
//	}
//
//	return nil
//}

func setAWSCredentials(sess *session.Session) error {
	v, err := sess.Config.Credentials.Get()
	if err != nil {
		return fmt.Errorf("can't set AWS credentials: %w", err)
	}

	err = os.Setenv("AWS_SECRET_ACCESS_KEY", v.SecretAccessKey)
	if err != nil {
		return err
	}
	err = os.Setenv("AWS_ACCESS_KEY_ID", v.AccessKeyID)
	if err != nil {
		return err
	}
	err = os.Setenv("AWS_SESSION_TOKEN", v.SessionToken)
	if err != nil {
		return err
	}

	return nil
}

//type SSMWrapper struct {
//	Api ssmiface.SSMAPI
//}

func upTunnel(app *config.Atun) (string, error) {
	logger.Debug("Starting tunnel", "bastion", app.Config.BastionHostID)
	logger.Debug("SSH key path", "path", app.Config.SSHKeyPath)
	logger.Debug("SSH config file", "path", app.Config.SSHConfigFile)

	if err := setAWSCredentials(app.Session); err != nil {
		return "", fmt.Errorf("can't up tunnel: %w", err)
	}

	args := getSSHCommandArgs(app)

	err := runSSH(app, args)
	if err != nil {
		return "", err
	}

	var forwardConfig string

	for _, v := range config.App.Hosts {
		logger.Debug("Host", "name", v.Name, "proto", v.Proto, "remote", v.Remote, "local", v.Local)
		forwardConfig += fmt.Sprintf("%s:%d ➡ 127.0.0.1:%d\n", v.Name, v.Remote, v.Local)
	}

	return forwardConfig, nil
}

func generateSSHConfigFile(app *config.Atun) (string, error) {
	sshConfigContent := `# SSH over AWS Session Manager (generated by atun.io)
host i-* mi-*
ServerAliveInterval 180
ProxyCommand sh -c "aws ssm start-session --target %h --document-name AWS-StartSSHSession --parameters 'portNumber=%p'"
`

	for _, host := range app.Hosts {
		logger.Debug("Host", "name", host.Name, "proto", host.Proto, "remote", host.Remote, "local", host.Local)
		sshConfigContent += fmt.Sprintf("LocalForward %d %s:%d\n", host.Local, host.Name, host.Remote)
	}

	sshConfigFile, err := os.CreateTemp(os.TempDir(), "atun-ssh.config")
	if err != nil {
		logger.Error("Error creating ssh tunnel config file", "error", err)
		return "", err
	}

	defer sshConfigFile.Close()

	// Write the content to the file
	_, err = sshConfigFile.WriteString(sshConfigContent)
	if err != nil {
		return "", err
	}

	return sshConfigFile.Name(), nil
}

// TODO: Implement getFreePort - ability to use a random local if port is set to "auto" or "0"

func init() {
	upCmd.Flags().StringP("bastion", "b", "", "Bastion instance id to use. If not specified the first running instance with the atun.io tags is used")

	if err := viper.BindPFlags(upCmd.Flags()); err != nil {
		pterm.Error.Println("Error while binding flags")
	}

	// Add flags for VPC and Subnet
	upCmd.PersistentFlags().String("bastion-vpc-id", "", "VPC ID of the bastion host to be created")
	upCmd.PersistentFlags().String("bastion-subnet-id", "", "Subnet ID of the bastion host to be created")
}
