/*
 * SPDX-License-Identifier: Apache-2.0
 * SPDX-FileCopyrightText: © 2024 Dmitry Kireev
 */

package cmd

import (
	"github.com/automationd/atun/internal/config"
	"github.com/automationd/atun/internal/constraints"
	"github.com/automationd/atun/internal/ux"
	"github.com/automationd/atun/internal/version"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Version",
	Long:  `Print version`,
	Run: func(cmd *cobra.Command, args []string) {

		pterm.Printfln("Version: %s\n", version.FullVersionNumber())
		//version.CheckLatestRelease()

		// Detect if current terminal is capable of displaying ASCII art
		// If not, disable it

		if !config.App.Config.LogPlainText && constraints.IsInteractiveTerminal() && constraints.SupportsANSIEscapeCodes() {
			//stopChan := make(chan struct{})
			//go func() {

			ux.RenderAsciiArt()

			//close(stopChan)
			//}()

			//go func() {
			//	if err := keyboard.Open(); err != nil {
			//		panic(err)
			//	}
			//	defer keyboard.Close()
			//
			//	for {
			//		char, key, err := keyboard.GetKey()
			//		if err == nil && (char == 'q' || key == keyboard.KeyEsc || key == keyboard.KeyEnter) {
			//			stopChan <- struct{}{}
			//			break
			//		}
			//	}
			//}()
			//
			//<-stopChan
		}

	},
}

func init() {

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// versionCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// versionCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
