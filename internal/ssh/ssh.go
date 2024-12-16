/*
 * SPDX-License-Identifier: Apache-2.0
 * SPDX-FileCopyrightText: © 2024 Dmitry Kireev
 */

package ssh

import (
	"fmt"
	"github.com/automationd/atun/internal/config"
	"github.com/automationd/atun/internal/logger"
	ssh2 "golang.org/x/crypto/ssh"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"syscall"
)

// c

func RunSSH(app *config.Atun, args []string) error {
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

	//// Stream stdout and stderr
	//if app.Config.LogLevel == "debug" {
	//	// Stream output to os.Stdout and os.Stderr in real-time
	//	stdoutPipe, err := c.StdoutPipe()
	//	if err != nil {
	//		return fmt.Errorf("failed to get stdout pipe: %w", err)
	//	}
	//	stderrPipe, err := c.StderrPipe()
	//	if err != nil {
	//		return fmt.Errorf("failed to get stderr pipe: %w", err)
	//	}
	//
	//	go io.Copy(os.Stdout, stdoutPipe)
	//	go io.Copy(os.Stderr, stderrPipe)
	//} else {
	// Only display logs without streaming
	//c.Stdout = os.Stdout
	//c.Stderr = os.Stderr
	//}
	//if app.Config.LogLevel == "debug" {
	//	stdout, err := c.StdoutPipe()
	//	if err != nil {
	//		return fmt.Errorf("failed to get stdout pipe: %w", err)
	//	}
	//
	//	stderr, err := c.StderrPipe()
	//	if err != nil {
	//		return fmt.Errorf("failed to get stderr pipe: %w", err)
	//	}
	//
	//	go io.Copy(os.Stdout, stdout)
	//	go io.Copy(os.Stderr, stderr)
	//}
	// Run the command
	if err := c.Run(); err != nil {
		logger.Debug("SSH command error", "error", err)
		return fmt.Errorf("failed to run SSH process: %w", err)

		// Print stdot and stderr from the command

	}

	logger.Info("SSH process started in the background", "pid", c.Process.Pid)

	// Disown the process if the parent process is terminating
	// This ensures the child process doesn't get terminated when the parent exits
	go func() {
		_ = c.Process.Release() // Detach the process fully
	}()

	return nil
}

// TODO: Refactor GetSSHCommandArgs into separate functions

func GetSSHCommandArgs(app *config.Atun) []string {
	bastionSockFilePath := getBastionSockFilePath(app)

	args := []string{}

	// Check if the bastion socket file exists
	if _, err := os.Stat(bastionSockFilePath); !os.IsNotExist(err) {
		logger.Info("A tunnel socket from has been found", "path", bastionSockFilePath)
		args = []string{"ssh", "-S", bastionSockFilePath, "-O", "check", ""}

	} else {
		logger.Debug("Tunnel socket not found. Creating a new one", "path", bastionSockFilePath)
		args = []string{"-M", "-t", "-S", bastionSockFilePath, "-fN"}

		// Disable strict host key checking
		if !app.Config.SSHStrictHostKeyChecking {
			args = append(args, "-o", "StrictHostKeyChecking=no")
		}

		// TODO: Add ability to support other instance types, not just AWS Linux
		args = append(args, fmt.Sprintf("%s@%s", app.Config.BastionHostUser, app.Config.BastionHostID))
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

// GetPublicKey gets the public key from the private key
func GetPublicKey(path string) (string, error) {
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
	privateKey, err := ssh2.ParsePrivateKey(f)
	if err != nil {
		return "", err
	}

	// Extract the public key from the private key
	publicKey := privateKey.PublicKey()

	// Marshal the public key to the OpenSSH format
	pubKeyBytes := ssh2.MarshalAuthorizedKey(publicKey)

	// Return the public key as a string
	return string(pubKeyBytes), nil
}

func GenerateSSHConfigFile(app *config.Atun) (string, error) {
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

func GetSSMPluginStatus(app *config.Atun) (bool, error) {
	// Check if `session-manager-plugin' is started and process contains Bastion instance ID
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check SSM plugin status: %w", err)
	}

	if !strings.Contains(string(output), app.Config.BastionHostID) {
		return false, nil
	}

	return true, nil
}

func GetTunnelStatus(app *config.Atun) (bool, error) {
	bastionSockFilePath := getBastionSockFilePath(app)

	// If a bastion socket file exists, check the tunnel status
	if _, err := os.Stat(bastionSockFilePath); !os.IsNotExist(err) {
		logger.Info("A tunnel socket from has been found", "path", bastionSockFilePath)

		args := []string{"ssh", "-S", bastionSockFilePath, "-O", "check", ""}

		// Run the SSH command in a blocking way
		cmd := exec.Command("ssh", args...)
		logger.Debug("Running SSH command", "command", cmd.String())
		cmd.Dir = app.Config.AppDir

		if app.Config.LogLevel == "debug" {
			cmd.Args = append(cmd.Args, "-vvv")
		}

		if err := cmd.Run(); err != nil {
			return false, fmt.Errorf("failed to check tunnel status: %w", err)
		}

		return true, nil

	}

	logger.Debug("Tunnel socket not found. Tunnel is not running", "path", bastionSockFilePath)
	return false, nil
}

func StopTunnel(app *config.Atun) (bool, error) {
	bastionSockFilePath := getBastionSockFilePath(app)

	// If a bastion socket file exists, check the tunnel status
	if _, err := os.Stat(bastionSockFilePath); !os.IsNotExist(err) {
		logger.Info("A tunnel socket from has been found", "path", bastionSockFilePath)

		args := []string{"ssh", "-S", bastionSockFilePath, "-O", "exit", ""}

		// Run the SSH command in a blocking way
		cmd := exec.Command("ssh", args...)
		logger.Debug("Running SSH command", "command", cmd.String())
		cmd.Dir = app.Config.AppDir

		if app.Config.LogLevel == "debug" {
			cmd.Args = append(cmd.Args, "-vvv")
		}

		if err := cmd.Run(); err != nil {
			return false, fmt.Errorf("failed to exit tunnel: %w", err)
		}

		return true, nil

	}

	logger.Debug("Tunnel socket not found. Tunnel is not running", "path", bastionSockFilePath)
	return false, nil
}

func getBastionSockFilePath(app *config.Atun) string {

	logger.Debug("Getting bastion socket file path", "tunnelDir", app.Config.TunnelDir, "env", app.Config.Env, "bastionHostID", app.Config.BastionHostID)
	return path.Join(app.Config.TunnelDir, fmt.Sprintf("%s-tunnel.sock", app.Config.BastionHostID))
}
