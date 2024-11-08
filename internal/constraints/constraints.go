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

package constraints

import (
	"errors"
	"fmt"
	"github.com/Masterminds/semver"
	"github.com/hazelops/atun/internal/config"

	"github.com/pterm/pterm"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"strings"
)

type constraints struct {
	configFile bool
	ssmplugin  bool
	structure  bool
	nvm        bool
}

func CheckConstraints(options ...Option) error {
	r := constraints{}
	for _, opt := range options {
		opt(&r)
	}

	if r.ssmplugin {
		if err := checkSessionManagerPlugin(); err != nil {
			return err
		}
	}

	if len(viper.ConfigFileUsed()) == 0 && r.configFile {
		return fmt.Errorf("this command requires a config file. Please add atun.toml to %s", config.App.Config.AppDir)
	}

	return nil
}

type Option func(*constraints)

func WithIzeStructure() Option {
	return func(r *constraints) {
		r.structure = true
	}
}

func WithConfigFile() Option {
	return func(r *constraints) {
		r.configFile = true
	}
}

func WithSSMPlugin() Option {
	return func(r *constraints) {
		r.ssmplugin = true
	}
}

func WithNVM() Option {
	return func(r *constraints) {
		r.nvm = true
	}
}

func checkNVM() error {
	if len(os.Getenv("NVM_DIR")) == 0 {
		return errors.New("nvm is not installed (visit https://github.com/nvm-sh/nvm)")
	}

	return nil
}

func checkDocker() error {
	exist, _ := CheckCommand("docker", []string{"info"})
	if !exist {
		return errors.New("docker is not running or is not installed (visit https://www.docker.com/get-started)")
	}

	return nil
}

func isStructured() bool {
	var isStructured = false

	cwd, err := os.Getwd()
	if err != nil {
		logrus.Fatalln("can't initialize config: %w", err)
	}

	_, err = os.Stat(filepath.Join(cwd, ".ize"))
	if !os.IsNotExist(err) {
		isStructured = true
	}

	_, err = os.Stat(filepath.Join(cwd, ".infra"))
	if !os.IsNotExist(err) {
		isStructured = true
	}

	return isStructured
}

func checkSessionManagerPlugin() error {
	exist, _ := CheckCommand("session-manager-plugin", []string{})
	if !exist {
		pterm.Warning.Println("SSM Agent plugin is not installed. Trying to install SSM Agent plugin")

		var pyVersion string

		exist, pyVersion := CheckCommand("python3", []string{"--version"})
		if !exist {
			exist, pyVersion = CheckCommand("python", []string{"--version"})
			if !exist {
				return errors.New("python is not installed")
			}

			c, err := semver.NewConstraint("<= 2.6.5")
			if err != nil {
				return err
			}

			v, err := semver.NewVersion(strings.TrimSpace(strings.Split(pyVersion, " ")[1]))
			if err != nil {
				return err
			}

			if c.Check(v) {
				return fmt.Errorf("python version %s below required %s", v.String(), "2.6.5")
			}
			return errors.New("python is not installed")
		}

		c, err := semver.NewConstraint("<= 3.3.0")
		if err != nil {
			return err
		}

		v, err := semver.NewVersion(strings.TrimSpace(strings.Split(pyVersion, " ")[1]))
		if err != nil {
			return err
		}

		if c.Check(v) {
			return fmt.Errorf("python version %s below required %s", v.String(), "3.3.0")
		}

		pterm.DefaultSection.Println("Installing SSM Agent plugin")

		err = downloadSSMAgentPlugin()
		if err != nil {
			return fmt.Errorf("download SSM Agent plugin error: %v (visit https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html)", err)
		}

		pterm.Success.Println("Downloading SSM Agent plugin")

		err = installSSMAgent()
		if err != nil {
			return fmt.Errorf("install SSM Agent plugin error: %v (visit https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html)", err)
		}

		pterm.Success.Println("Installing SSM Agent plugin")

		err = cleanupSSMAgent()
		if err != nil {
			return fmt.Errorf("cleanup SSM Agent plugin error: %v (visit https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html)", err)
		}

		pterm.Success.Println("Cleanup Session Manager plugin installation package")

		exist, _ = CheckCommand("session-manager-plugin", []string{})
		if !exist {
			return fmt.Errorf("check SSM Agent plugin error: %v (visit https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html)", err)
		}
	}

	return nil
}
