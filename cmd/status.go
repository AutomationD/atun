/*
 * SPDX-License-Identifier: Apache-2.0
 * SPDX-FileCopyrightText: © 2024 Dmitry Kireev
 */

package cmd

import (
	"fmt"
	"github.com/automationd/atun/internal/aws"
	"github.com/automationd/atun/internal/config"

	"github.com/pterm/pterm"
	"os"

	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		//// Define symbols for boolean values
		//checkMark := "✔"
		//crossMark := "✘"
		//
		//toggle, err := cmd.Flags().GetBool("toggle")
		//if err != nil {
		//	pterm.Error.Println("Error while getting the toggle value")
		//}
		//
		//toggleValue := crossMark
		//if toggle {
		//	toggleValue = checkMark
		//}

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("can't load options for a command: %w", err)
		}

		dt := pterm.DefaultTable

		pterm.DefaultSection.Println("Status")
		_ = dt.WithData(pterm.TableData{
			{"AWS_ACCOUNT", aws.GetAccountId()},
			{"AWS_PROFILE", config.App.Config.AWSProfile},
			{"AWS_REGION", config.App.Config.AWSRegion},
			{"PWD", cwd},
			{"SSH_KEY_PATH", config.App.Config.SSHKeyPath},
			{"Config File", config.App.Config.ConfigFile},
			{"Bastion Host", config.App.Config.BastionHostID},

			//{"Toggle", toggleValue},
		}).WithLeftAlignment().Render()

		return err
	},
}

func init() {

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// statusCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	//statusCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
