/*
 * SPDX-License-Identifier: Apache-2.0
 * SPDX-FileCopyrightText: © 2024 Dmitry Kireev
 */

package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/automationd/atun/internal/aws"
	"github.com/automationd/atun/internal/config"
	"github.com/automationd/atun/internal/constraints"
	"github.com/automationd/atun/internal/logger"
	"github.com/aws/aws-sdk-go/aws/session"
	"os/exec"
	"path"
	"syscall"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
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
		config.App.Config.Hosts = bastionHostConfig.Config.Hosts

		if err != nil {
			pterm.Error.Printfln("Error getting bastion host config: %v", err)
		}
		//config.App.Config = atun.Config
		//config.App.Hosts = atun.Hosts

		for _, host := range config.App.Config.Hosts {
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
		logger.Debug("Private key path", "path", config.App.Config.SSHKeyPath)

		//err := o.checkOsVersion()
		//if err != nil {
		//	return err
		//}

		// Read private key from HOME/id_rsa.pub
		publicKey, err := getPublicKey(config.App.Config.SSHKeyPath)
		if err != nil {
			logger.Error("Error getting public key", "error", err)
		}

		logger.Debug("Public key", "key", publicKey)

		// Send the public key to the bastion instance
		err = aws.SendSSHPublicKey(config.App.Config.BastionHostID, publicKey)
		if err != nil {
			logger.Error("Can't run tunnel", "error", err)
		}

		logger.Debug("Public key sent to bastion host", "bastion", config.App.Config.BastionHostID)

		// TODO: Refactor naming of connectionInfo
		connectionInfo, err := upTunnel(config.App)
		if err != nil {
			logger.Fatal("Error running tunnel", "error", err)
		}

		// TODO: Check if Instance has forwarding working (check ipv4.forwarding sysctl)

		logger.Info("Tunnel is up! Forwarded ports:", "connectionInfo", connectionInfo)

		return nil
	},
}

func runSSH(app *config.Atun, args []string) error {
	logger.Debug("SSH", "args", args)
	c := exec.Command("ssh", args...)
	logger.Debug("SSH command", "command", c.String())

	c.Dir = app.Config.AppDir
	os.Setenv("AWS_REGION", app.Config.AWSRegion)
	os.Setenv("AWS_PROFILE", app.Config.AWSProfile)

	// Detach the process (platform-dependent)
	c.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Detach process from the parent group
	}

	// Start the process
	if err := c.Start(); err != nil {
		return fmt.Errorf("failed to start SSH process: %w", err)
	}

	logger.Info("SSH process started in the background", "pid", c.Process.Pid)

	// Optionally disown the process if the parent process is terminating
	// This ensures the child process doesn't get terminated when the parent exits
	go func() {
		_ = c.Process.Release() // Detach the process fully
	}()

	return nil
}

func getSSHCommandArgs(app *config.Atun) []string {
	bastionSockFilePath := path.Join(app.Config.AppDir, fmt.Sprintf("%s-%s-tunnel.sock", app.Config.Env, app.Config.BastionHostID))

	args := []string{}

	// Check if the bastion socket file exists
	if _, err := os.Stat(bastionSockFilePath); !os.IsNotExist(err) {
		logger.Info("A tunnel socket from has been found", "path", bastionSockFilePath)
		args = []string{"ssh", "-S", bastionSockFilePath, "-O", "check", ""}

	} else {
		logger.Debug("Tunnel socket not found. Creating a new one", "path", bastionSockFilePath)
		args = []string{"-M", "-t", "-S", bastionSockFilePath, "-fN"}
		if !app.Config.SSHStrictHostKeyChecking {
			args = append(args, "-o", "StrictHostKeyChecking=no")
		}

		// TODO: Add ability to support other instance types, not just ubuntu
		args = append(args, fmt.Sprintf("ubuntu@%s", app.Config.BastionHostID))
		args = append(args, "-F", app.Config.SSHConfigFile)

		if _, err := os.Stat(app.Config.SSHKeyPath); !os.IsNotExist(err) {
			args = append(args, "-i", app.Config.SSHKeyPath)
		}
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
					atun.Config.Hosts = append(atun.Config.Hosts, hosts[i])
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

	var connectionInfo string

	for _, v := range config.App.Config.Hosts {
		logger.Debug("Host", "name", v.Name, "proto", v.Proto, "remote", v.Remote, "local", v.Local)
		connectionInfo += fmt.Sprintf("%s:%d ➡ 127.0.0.1:%d\n", v.Name, v.Remote, v.Local)
	}

	return connectionInfo, nil
}

func generateSSHConfigFile(app *config.Atun) (string, error) {
	sshConfigContent := `# SSH over AWS Session Manager (generated by atun.io)
host i-* mi-*
ServerAliveInterval 180
ProxyCommand sh -c "aws ssm start-session --target %h --document-name AWS-StartSSHSession --parameters 'portNumber=%p'"
`

	for _, host := range app.Config.Hosts {
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

// TODO: Automatic port logic

//func getFreePort() (int, error) {
//	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
//	if err != nil {
//		return 0, err
//	}
//
//	l, err := net.ListenTCP("tcp", addr)
//	if err != nil {
//		return 0, err
//	}
//	defer func(l *net.TCPListener) {
//		err := l.Close()
//		if err != nil {
//			log.Fatal(err)
//		}
//	}(l)
//	return l.Addr().(*net.TCPAddr).Port, nil
//}
//
//func checkPort(port int, dir string) error {
//	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("127.0.0.1:%d", port))
//	if err != nil {
//		return fmt.Errorf("can't check address %s: %w", fmt.Sprintf("127.0.0.1:%d", port), err)
//	}
//
//	l, err := net.ListenTCP("tcp", addr)
//	if err != nil {
//		command := fmt.Sprintf("lsof -i tcp:%d | grep LISTEN | awk '{print $1, $2}'", port)
//		stdout, stderr, code, err := term.New(term.WithStdout(io.Discard), term.WithStderr(io.Discard)).Run(exec.Command("bash", "-c", command))
//		if err != nil {
//			return fmt.Errorf("can't run command '%s': %w", command, err)
//		}
//		if code == 0 {
//			stdout = strings.TrimSpace(stdout)
//			processName := strings.Split(stdout, " ")[0]
//			processPid, err := strconv.Atoi(strings.Split(stdout, " ")[1])
//			if err != nil {
//				return fmt.Errorf("can't get pid: %w", err)
//			}
//			pterm.Info.Printfln("Can't start tunnel on port %d. It seems like it's take by a process '%s'.", port, processName)
//			proc, err := os.FindProcess(processPid)
//			if err != nil {
//				return fmt.Errorf("can't find process: %w", err)
//			}
//
//			_, err = os.Stat(filepath.Join(dir, "bastion.sock"))
//			if processName == "ssh" && os.IsNotExist(err) {
//				return fmt.Errorf("it could be another ize tunnel, but we can't find a socket. Something went wrong. We suggest terminating it and starting it up again")
//			}
//			isContinue := false
//			if terminal.IsTerminal(int(os.Stdout.Fd())) {
//				isContinue, err = pterm.DefaultInteractiveConfirm.WithDefaultText("Would you like to terminate it?").Show()
//				if err != nil {
//					return err
//				}
//			} else {
//				isContinue = true
//			}
//
//			if !isContinue {
//				return fmt.Errorf("destroying was canceled")
//			}
//			err = proc.Kill()
//			if err != nil {
//				return fmt.Errorf("can't kill process: %w", err)
//			}
//
//			pterm.Info.Printfln("Process '%s' (pid %d) was killed", processName, processPid)
//
//			return nil
//		}
//		return fmt.Errorf("error during run command: %s (exit code: %d, stderr: %s)", command, code, stderr)
//	}
//
//	err = l.Close()
//	if err != nil {
//		return err
//	}
//
//	return nil
//}

//func checkTunnel(app *config.Atun) (bool, error) {
//	bastionSocketPath := path.Join(app.Config.AppDir, "bastion.sock")
//
//	// Check if the socket file exists. If it does, check if the tunnel is up
//	if _, err := os.Stat(bastionSocketPath); !os.IsNotExist(err) {
//		logger.Info("A socket file from another tunnel has been found", "path", bastionSocketPath)
//		c := exec.Command(
//			logger.Debug("Checking tunnel in socket", "socket", bastionSocketPath)
//			"ssh", "-S", bastionSocketPath, "-O", "check", "",
//		)
//
//		out := &bytes.Buffer{}
//		c.Stdout = out
//		c.Stderr = out
//		c.Dir = dir
//
//		err := c.Run()
//		if err == nil {
//			sshConfigPath := fmt.Sprintf("%s/ssh.config", dir)
//			sshConfig, err := getSSHConfig(sshConfigPath)
//			if err != nil {
//				return false, fmt.Errorf("can't check tunnel: %w", err)
//			}
//
//			pterm.Success.Println("Tunnel is up. Forwarding config:")
//			hosts := getHosts(sshConfig)
//			var forwardConfig string
//			for _, h := range hosts {
//				forwardConfig += fmt.Sprintf("%s:%s ➡ localhost:%s\n", h[2], h[3], h[1])
//			}
//			pterm.Println(forwardConfig)
//
//			return true, nil
//		} else {
//			pterm.Warning.Println("Tunnel socket file seems to be not useable. We have deleted it")
//			err := os.Remove(bastionSocketPath)
//			if err != nil {
//				return false, err
//			}
//			return false, nil
//		}
//	}
//
//	return false, nil
//}

// TODO: Implement getFreePort - ability to use a random local if port is set to "auto" or "0"

func init() {
	logger.Debug("Initializing up command")
	upCmd.PersistentFlags().StringP("bastion", "b", "", "Bastion instance id to use. If not specified the first running instance with the atun.io tags is used")
	logger.Debug("Up command initialized")
}
