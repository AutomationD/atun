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

package version

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Masterminds/semver"
	"github.com/pterm/pterm"
	"log"
	"net/http"
	"runtime"
	"runtime/debug"
	"time"
)

var (
	GitCommit string
	Version   = "development"
)

func GetVersion() (ret string) {
	if b, ok := debug.ReadBuildInfo(); ok && len(b.Main.Version) > 0 {
		ret = b.Main.Version
	} else {
		ret = "unknown"
	}
	return
}

func FullVersionNumber() string {
	var versionString bytes.Buffer

	if Version == "development" {
		return fmt.Sprintf("development %s", time.Now().Format("2006-01-02T15:04:05"))
	}

	fmt.Fprintf(&versionString, "%s", Version)
	if GitCommit != "" {
		fmt.Fprintf(&versionString, " (%s)", GitCommit)
	}

	return versionString.String()
}

func CheckLatestRelease() {
	_, err := semver.NewVersion(Version)
	if err != nil {
		return
	}

	resp, err := http.Get("https://api.github.com/repos/hazelops/ize/releases/latest")
	if err != nil {
		log.Fatalln(err)
	}

	var gr gitResponse

	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		log.Fatal(err)
	}

	var versionChangeAction = "upgrading"
	if Version > gr.Version {
		versionChangeAction = "downgrading"
	}
	if Version != gr.Version {
		pterm.Warning.Printfln("The newest stable version is %s, but your version is %s. Consider %s.", gr.Version, Version, versionChangeAction)
		ShowUpgradeCommand()
	}
}

type gitResponse struct {
	Version string `json:"tag_name"`
}

func ShowUpgradeCommand() error {
	switch goos := runtime.GOOS; goos {
	case "darwin":
		pterm.Warning.Println("Use the command to update\n:\tbrew upgrade ize")
	//case "linux":
	//	distroName, err := requirements.ReadOSRelease("/etc/os-release")
	//	if err != nil {
	//		return err
	//	}
	//	switch distroName["ID"] {
	//	case "ubuntu":
	//		pterm.Warning.Println("Use the command to update:\n\tapt update && apt install ize")
	//	default:
	//		pterm.Warning.Println("See https://github.com/hazelops/ize/blob/main/DOCS.md#installation")
	//	}
	default:
		pterm.Warning.Println("See https://github.com/hazelops/ize/blob/main/DOCS.md#installation")
	}

	return nil
}
