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
	"github.com/hazelops/atun/internal/aws"
	"github.com/hazelops/atun/internal/config"
	//"github.com/hazelops/atun/internal/config"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "atun",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// TODO: Use Method receiver. Create atun (config) here
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {

	// TODO: Use Method Receiver (pass atun all the way to the command)
	rootCmd.AddCommand(
		upCmd,
		statusCmd,
		versionCmd)

	//cobra.OnInitialize(config.LoadConfig)
	cobra.OnInitialize(initializeAtun)
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.atun.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

}

func initializeAtun() {
	// Load config into a global struct
	err := config.LoadConfig()
	if err != nil {
		panic(err)
	}

	// Init AWS Session (probably should be moved to a separate function)
	sess, err := aws.GetSession(&aws.SessionConfig{
		Region:      config.App.Config.AWSRegion,
		Profile:     config.App.Config.AWSProfile,
		EndpointUrl: config.App.Config.EndpointUrl,
	})
	if err != nil {
		panic(err)
	}

	config.App.Session = sess

	////Initialize Atun struct with configuration
	//atun, err = NewAtun(cfg)
	//if err != nil {
	//	log.Fatalf("failed to initialize atun: %v", err)
	//}
}

////// NewAtun initializes a new Atun instance with a given configuration
////func NewAtun(cfg *config.Config) (*Atun, error) {
////	sess, err := session.NewSessionWithOptions(session.Options{
////		Config:  aws.Config{Region: &cfg.AWSRegion},
////		Profile: cfg.AWSProfile,
////	})
////	if err != nil {
////		return nil, err
////	}
////
////	return &Atun{
////		Config:  cfg,
////		Session: sess,
////	}, nil
//}
