/*
 * SPDX-License-Identifier: Apache-2.0
 * SPDX-FileCopyrightText: © 2024 Dmitry Kireev
 */
package cmd

import (
	"github.com/automationd/atun/internal/aws"
	"github.com/automationd/atun/internal/config"
	"github.com/automationd/atun/internal/constraints"
	"github.com/automationd/atun/internal/infra"
	"github.com/automationd/atun/internal/logger"
	"github.com/automationd/atun/internal/ux"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

// deleteCmd represents the del command
var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Deletes an ad-hoc router host",
	Long:  `Deletes an ad-hoc router host created by atun. Performed via CDKTF/Terraform: doesn't affect other resources`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Add check for --force flag

		// TODO: Add survey to check if the user is sure to destroy the stack

		var deleteRouterInstanceSpinner *pterm.SpinnerPrinter
		showSpinner := config.App.Config.LogLevel != "debug" && config.App.Config.LogLevel != "info" && constraints.IsInteractiveTerminal() && constraints.SupportsANSIEscapeCodes()

		if showSpinner {
			deleteRouterInstanceSpinner = ux.StartCustomSpinner("Deleting Ad-Hoc EC2 Router Instance...")
		} else {
			logger.Debug("Not showing spinner", "logLevel", config.App.Config.LogLevel)
			logger.Info("Deleting Ad-Hoc EC2 Router Instance...")
		}

		aws.InitAWSClients(config.App)

		err := infra.DestroyCDKTF(config.App.Config)
		if err != nil {
			if showSpinner {
				deleteRouterInstanceSpinner.Fail("Failed to delete Router Ad-Hoc Instance")
			} else {
				logger.Error("Failed to delete Router Ad-Hoc Instance")
			}
			logger.Error("Error running CDKTF", "error", err)
			return err
		}

		if showSpinner {
			deleteRouterInstanceSpinner.Success("Router Ad-Hoc Instance deleted successfully")
		} else {
			logger.Info("Router Ad-Hoc Instance deleted successfully")
		}
		logger.Info("CDKTF stack destroyed successfully")
		return nil
	},
}

func init() {
	logger.Debug("Init delete command")
}
