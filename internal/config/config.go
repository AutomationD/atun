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

package config

import (
	"errors"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/pterm/pterm"
	"github.com/spf13/viper"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type Atun struct {
	Version string `json:"atun.io/version"`
	Config  *Config
	Session *session.Session
	Hosts   []Host
}

type Config struct {
	SSHKeyPath               string
	SSHConfigFile            string
	SSHStrictHostKeyChecking bool
	AWSProfile               string
	AWSRegion                string
	EndpointUrl              string
	ConfigFile               string
	BastionVPCID             string
	BastionSubnetID          string
	BastionHostID            string
	AppDir                   string
	LogLevel                 string
}

type Host struct {
	Name   string `json:"-" jsonschema:"-"`
	Proto  string `json:"proto" jsonschema:"proto"`
	Remote string `json:"remote" jsonschema:"remote"`
	Local  string `json:"local" jsonschema:"local"`
}

var App *Atun
var InitialApp *Atun

func LoadConfig() error {

	viper.SetEnvPrefix("ATUN")

	replacer := strings.NewReplacer(".", "__")

	viper.SetEnvKeyReplacer(replacer)
	viper.AutomaticEnv()

	// Optionally read from a configuration file
	viper.SetConfigName("atun")
	viper.SetConfigType("toml")

	currentDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	appDir := filepath.Join(homeDir, ".atun")

	// Add config paths. Current directory is the priority over home app path
	viper.AddConfigPath(currentDir)
	viper.AddConfigPath(appDir)

	if err := viper.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			// Config file not found; ignore error if desired
			pterm.Info.Println("No config file found. Using defaults and environment variables.")
		}
	} else {
		pterm.Info.Println("Using config file:", viper.ConfigFileUsed())
	}

	// Set Default Values if none are set
	viper.SetDefault("SSH_KEY_PATH", filepath.Join(homeDir, ".ssh", "id_rsa"))
	viper.SetDefault("AWS_PROFILE", "default")
	viper.SetDefault("AWS_REGION", "us-east-1")
	viper.SetDefault("SSH_STRICT_HOST_KEY_CHECKING", true)
	viper.SetDefault("LOG_LEVEL", "info")

	if err := viper.Unmarshal(&InitialApp); err != nil {
		log.Fatalf("Unable to decode initial config into a struct: %v", err)
	}

	// TODO?: Move init a separate file with correct imports of config
	App = &Atun{
		Version: "1",
		Config: &Config{
			SSHKeyPath:               viper.GetString("SSH_KEY_PATH"),
			SSHStrictHostKeyChecking: viper.GetBool("SSH_STRICT_HOST_KEY_CHECKING"),
			AWSProfile:               viper.GetString("AWS_PROFILE"),
			AWSRegion:                viper.GetString("AWS_REGION"),
			BastionVPCID:             viper.GetString("BASTION_VPC_ID"),
			BastionSubnetID:          viper.GetString("BASTION_SUBNET_ID"),
			BastionHostID:            viper.GetString("BASTION_HOST_ID"),
			ConfigFile:               viper.ConfigFileUsed(),
			LogLevel:                 viper.GetString("LOG_LEVEL"),
			AppDir:                   appDir,
		},
		Session: nil,
		Hosts:   []Host{},
	}

	// Create Cfg.AppDir if it doesn't exist
	if _, err := os.Stat(App.Config.AppDir); os.IsNotExist(err) {
		if err := os.Mkdir(App.Config.AppDir, os.FileMode(0755)); err != nil {
			pterm.Error.Println("Error creating app directory:", err)
			panic(err)
		}
		pterm.Info.Println("Created app directory:", App.Config.AppDir)
	}

	//
	pterm.Printfln("Config: %v", App.Config)

	// TODO?: Maybe search for bastion host id during config stage?
	return nil
}
