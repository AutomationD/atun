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
	"github.com/automationd/atun/internal/logger"
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
	Env                      string
}

type Host struct {
	Name   string `jsonschema:"-"`
	Proto  string `json:"proto" jsonschema:"proto"`
	Remote int    `json:"remote" jsonschema:"remote"`
	Local  int    `json:"local" jsonschema:"local"`
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

	// Set default log level early
	viper.SetDefault("LOG_LEVEL", "info")

	// Initialize the logger for a bit to provide early logging
	logger.Initialize(viper.GetString("LOG_LEVEL"))

	currentDir, err := os.Getwd()
	if err != nil {
		logger.Fatal("Error getting current directory")
		panic(err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Fatal("Error getting user home directory")
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
			logger.Debug("No config file found. Using defaults and environment variables.")
		}
	} else {
		logger.Debug("Using config file:", viper.ConfigFileUsed())
	}

	// Initialize the logger after config is read (second time)
	logger.Initialize(viper.GetString("LOG_LEVEL"))

	// Use AWS_PROFILE env var as a default for viper AWS_PROFILE
	if viper.GetString("ENV") == "" {
		if len(os.Getenv("ENV")) > 0 {
			viper.SetDefault("ENV", os.Getenv("ENV"))
		}
		// No default intentionally to avoid confusion
	}

	// Use AWS_PROFILE env var as a default for viper AWS_PROFILE
	if viper.GetString("AWS_PROFILE") == "" {
		if len(os.Getenv("AWS_PROFILE")) > 0 {
			viper.SetDefault("AWS_PROFILE", os.Getenv("AWS_PROFILE"))
		}
		// No default intentionally to avoid confusion
	}

	// Use AWS_REGION env var as a default for viper AWS_REGION
	if viper.GetString("AWS_REGION") == "" {
		if len(os.Getenv("AWS_REGION")) > 0 {
			viper.SetDefault("AWS_REGION", os.Getenv("AWS_REGION"))
		}
		// No default intentionally to avoid confusion
	}

	// Set Default Values if none are set
	viper.SetDefault("SSH_KEY_PATH", filepath.Join(homeDir, ".ssh", "id_rsa"))
	viper.SetDefault("SSH_STRICT_HOST_KEY_CHECKING", true)

	if err := viper.Unmarshal(&InitialApp); err != nil {
		log.Fatalf("Unable to decode initial config into a struct: %v", err)
	}

	// TODO?: Move init a separate file with correct imports of config
	App = &Atun{
		Version: "1",
		Config: &Config{
			Env:                      viper.GetString("ENV"),
			SSHKeyPath:               viper.GetString("SSH_KEY_PATH"),
			SSHStrictHostKeyChecking: viper.GetBool("SSH_STRICT_HOST_KEY_CHECKING"),
			AWSProfile:               viper.GetString("AWS_PROFILE"),
			AWSRegion:                viper.GetString("AWS_REGION"),
			BastionVPCID:             viper.GetString("BASTION_VPC_ID"),
			BastionSubnetID:          viper.GetString("BASTION_SUBNET_ID"),
			BastionHostID:            viper.GetString("BASTION_HOST_ID"),
			ConfigFile:               viper.ConfigFileUsed(),
			AppDir:                   appDir,
			LogLevel:                 viper.GetString("LOG_LEVEL"),
		},
		Session: nil,
		Hosts:   []Host{},
	}

	// Create Cfg.AppDir if it doesn't exist
	if _, err := os.Stat(App.Config.AppDir); os.IsNotExist(err) {
		if err := os.Mkdir(App.Config.AppDir, os.FileMode(0755)); err != nil {
			pterm.Error.Printfln("Error creating app directory %s: %s", App.Config.AppDir, err)
			panic(err)
		}
		pterm.Info.Println("Created app directory:", App.Config.AppDir)
	}

	//
	//pterm.Printfln("Config: %v", App.Config)

	// TODO?: Maybe search for bastion host id during config stage?
	return nil
}
