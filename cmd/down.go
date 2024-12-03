/*
 * SPDX-License-Identifier: Apache-2.0
 * SPDX-FileCopyrightText: © 2024 Dmitry Kireev
 */

package cmd

import (
	//"github.com/automationd/atun/internal/infra"
	"github.com/pterm/pterm"

	"github.com/spf13/cobra"
)

// downCmd represents the down command
var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Bring the tunnel down",
	Long:  `Bring the existing tunnel down.`,
	Run: func(cmd *cobra.Command, args []string) {

		if len(args) == 0 {
			pterm.Info.Printf("Down command called")
		} else {
			if args[0] == "bastion" {

			}
		}
	},
}

func init() {
	//logger.Debug("Down command initialized")
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// downCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// downCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
