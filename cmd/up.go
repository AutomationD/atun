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
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/hazelops/atun/internal/aws"
	"github.com/hazelops/atun/internal/config"
	"github.com/hazelops/atun/internal/infra"
	"github.com/hazelops/ize/pkg/term"
	"os/exec"
	"path"

	//"github.com/hazelops/atun/internal/aws"
	"github.com/hazelops/atun/internal/constraints"
	//"github.com/hazelops/atun/internal/infra"
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
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Use GO Method received on `atun`
		// TODO: Implement VPC, subnet picker (1. get list of VPCs, 2. Get a list of Subnets in it, 3. Ask user if it's not provided)

		// Check if an 'up' command is being invoked (not a subcommand)
		if len(args) == 0 {
			pterm.Info.Printfln("Up command called. bastion: %s, aws profile: %s", config.App.Config.BastionHostID, config.App.Config.AWSProfile)

			var err error
			var bastionHost string

			if err := constraints.CheckConstraints(constraints.WithSSMPlugin()); err != nil {
				return err
			}

			bastionHost = cmd.Flag("bastion").Value.String()

			// TODO?: Add logic if host not found offer create it, add --auto-create-bastion
			if bastionHost == "" {
				config.App.Config.BastionHostID, err = getBastionHostID()
				if err != nil {
					pterm.Error.Printfln("Error getting bastion host: %v", err)
				}
			} else {
				config.App.Config.BastionHostID = bastionHost
			}

			pterm.Printfln("Using atun bastion host: %s", config.App.Config.BastionHostID)

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

			for _, v := range config.App.Hosts {
				pterm.Info.Printf("Name: %s, Proto: %s, Remote: %s, Local: %s\n", v.Name, v.Proto, v.Remote, v.Local)
			}

			// Generate SSH config file
			config.App.Config.SSHConfigFile, err = writeSSHConfig(config.App)
			//atun.Config.SSHConfigFile, err = writeSSHConfig(atun)
			if err != nil {
				pterm.Error.Printfln("Error writing SSH config to %s: %v", config.App.Config.SSHConfigFile, err)
			}
			//if err != nil {
			//	pterm.Error.Printfln("Error writing SSH config to %s: %v", atun.Config.SSHConfigFile, err)
			//}

			pterm.Info.Printfln("SSH config file written to %s", config.App.Config.SSHConfigFile)

			// TODO: Check & Install SSM Agent

			//logrus.Debugf("public key path: %s", publicKeyPath)
			pterm.Info.Printfln("private key path: %s", config.App.Config.SSHKeyPath)

			//err := o.checkOsVersion()
			//if err != nil {
			//	return err
			//}

			// Read private key from HOME/id_rsa.pub
			publicKey, err := getPublicKey(config.App.Config.SSHKeyPath)
			if err != nil {
				fmt.Errorf("can't get public key: %s", err)
			}

			pterm.Info.Printfln("public key:\n%s", publicKey)

			// Send the public key to the bastion instance
			err = aws.SendSSHPublicKey(config.App.Config.BastionHostID, publicKey)
			if err != nil {
				fmt.Errorf("can't run tunnel: %s", err)
			}
			pterm.Info.Printfln("Public key sent to bastion host %s", bastionHost)

			// TODO: Refactor naming of forwardConfig
			forwardConfig, err := upTunnel(config.App)
			if err != nil {
				pterm.Error.Printfln("Error running tunnel: %v", err)
			}

			// TODO: Check if Instance has forwarding working (check ipv4.forwarding sysctl)

			pterm.Success.Println("Tunnel is up! Forwarded ports:")
			pterm.Println(forwardConfig)

		} else {
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
	os.Setenv("AWS_PROFILE", app.Config.AWSRegion)

	pterm.Info.Printfln("Command Executed: ssh ", strings.Join(args, " "))

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

// Gets bastion host ID
func getBastionHostID() (string, error) {
	pterm.Info.Println("Bastion host is required.462567" +
		"462567" +
		"462567 Looking fo atun routers.")
	tagKey := "atun.io/version"
	tagValue := "*"

	instances, err := aws.ListInstancesWithTag(tagKey, tagValue)
	if err != nil {
		pterm.Error.Printfln("Error listing instances with tag %s=%s: %v", tagKey, tagValue, err)
	}

	if len(instances) == 0 {
		pterm.Error.Printfln("No instances found with tag %s=%s", tagKey, tagValue)
		return "", err
	}

	for _, instance := range instances {
		pterm.Info.Printfln("Found Instance IDs: %s\n", *instance.InstanceId)

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
		pterm.Error.Println(err)
	}

	atun := config.Atun{}
	for k, v := range tags {
		// Iterate over the tags and use only atun.io tags
		if strings.HasPrefix(k, "atun.io") {
			if k == "atun.io/version" {
				atun.Version = v
			} else if strings.HasPrefix(k, "atun.io/host/") {
				hostName := strings.TrimPrefix(k, "atun.io/host/")

				var hosts []config.Host
				err := json.Unmarshal([]byte(v), &hosts)
				if err != nil {
					pterm.Error.Println(err)
					continue
				}

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
	pterm.Info.Printfln("Ssh config path: %s", app.Config.SSHConfigFile)

	if err := setAWSCredentials(app.Session); err != nil {
		return "", fmt.Errorf("can't run tunnel: %w", err)
	}

	//
	args := getSSHCommandArgs(app)

	err := runSSH(app, args)
	if err != nil {
		return "", err
	}

	var forwardConfig string

	for _, v := range config.App.Hosts {
		pterm.Info.Printf("Name: %s, Proto: %s, Remote: %s, Local: %s\n", v.Name, v.Proto, v.Remote, v.Local)
		forwardConfig += fmt.Sprintf("%s:%s ➡ 127.0.0.1:%s\n", v.Name, v.Remote, v.Local)
	}

	return forwardConfig, nil
}

func writeSSHConfig(app *config.Atun) (string, error) {
	sshConfigContent := `# SSH over AWS Session Manager (generated by atun.io)
host i-* mi-*
ServerAliveInterval 180
ProxyCommand sh -c "aws ssm start-session --target %h --document-name AWS-StartSSHSession --parameters 'portNumber=%p'"
`

	for _, v := range app.Hosts {
		pterm.Info.Printfln("Host: %s", v.Name)
		sshConfigContent += fmt.Sprintf("LocalForward %s %s:%s\n", v.Local, v.Name, v.Remote)
	}

	sshConfigFile, err := os.CreateTemp(os.TempDir(), "atun-ssh.config")
	if err != nil {
		fmt.Printf("Error creating ssh tunnel config file: %v\n", err)
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

	upCmd.Flags().StringP("bastion", "b", "", "Specify bastion instance id")

	if err := viper.BindPFlags(upCmd.Flags()); err != nil {
		pterm.Error.Println("Error while binding flags")
	}

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	upCmd.PersistentFlags().String("bastion-vpc-id", "", "A help for foo")
	upCmd.PersistentFlags().String("bastion-subnet-id", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// upCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
