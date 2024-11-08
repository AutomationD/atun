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

package aws

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/sirupsen/logrus"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"gopkg.in/ini.v1"
)

const (
	path = "/.aws/credentials-mfa"
)

type SessionConfig struct {
	Region      string
	Profile     string
	EndpointUrl string
}

func GetSession(c *SessionConfig) (*session.Session, error) {
	upd := false

	config := aws.NewConfig().WithRegion(c.Region).WithCredentials(credentials.NewSharedCredentials("", c.Profile)).WithEndpoint(c.EndpointUrl)

	if len(c.EndpointUrl) > 0 {
		logrus.Debug(fmt.Sprintf("Session established. Endpoint: %s", c.EndpointUrl))
	} else {
		logrus.Debug(fmt.Sprintf("Session established with a default endpoint"))
	}

	//
	//if len(c.EndpointUrl) > 0 {
	//	// If EndpointUrl is set to a non-default value specify it
	//
	//} else {
	//	config = aws.NewConfig().WithRegion(c.Region).WithCredentials(credentials.NewSharedCredentials("", c.Profile))
	//}

	sess, err := session.NewSessionWithOptions(session.Options{
		Config: *config,
	})
	if err != nil {
		return nil, err
	}

	devices, err := iam.New(sess).ListMFADevices(&iam.ListMFADevicesInput{})
	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		case "SharedCredsLoad":
			logrus.Error(err)
			return nil, fmt.Errorf("AWS_PROFILE is not set. Please set it via AWS_PROFILE env var, --aws-profile flag or aws_profile config entry in ize.toml")
		default:
			// Error only if it's not a localhost endpoint
			if !(strings.Contains(c.EndpointUrl, "localhost") || strings.Contains(c.EndpointUrl, "127.0.0.1")) {
				// If endpoint is not related to LocalStack then it's an error
				return nil, err
			}

			logrus.Debug("[NO MFA] Using Endpoint: ", c.EndpointUrl)
		}
	}

	if len(devices.MFADevices) == 0 {
		return sess, nil
	}

	home, _ := os.UserHomeDir()
	filePath := home + path

	credFile, err := ini.Load(filePath)
	if err != nil {
		credFile = ini.Empty(ini.LoadOptions{})
		upd = true
	}

	var sect *ini.Section
	var exp *ini.Key

	if !upd {
		sect, err = credFile.GetSection(fmt.Sprintf("%s-mfa", c.Profile))
		if err != nil {
			upd = true
		}
	}

	if !upd {
		if len(sect.KeyStrings()) != 4 {
			upd = true
		}
	}

	if !upd {
		exp, err = sect.GetKey("token_expiration")
		if err != nil {
			upd = true
		}
	}

	if !upd {
		timeExp, err := time.Parse("2006-01-02T15:04:05Z07:00", exp.String())
		if err != nil {
			upd = true
		}

		if timeExp.Before(time.Now().UTC()) {
			upd = true
		}
	}

	if upd {
		cred, err := getNewToken(sess, devices.MFADevices[0].SerialNumber)
		if err != nil {
			return nil, err
		}

		err = writeCredsToFile(cred, credFile, filePath, c.Profile)
		if err != nil {
			return nil, err
		}
	}

	sess, err = session.NewSessionWithOptions(
		session.Options{
			Config:            *aws.NewConfig().WithRegion(c.Region),
			Profile:           fmt.Sprintf("%s-mfa", c.Profile),
			SharedConfigFiles: []string{filePath},
		},
	)
	if err != nil {
		return nil, err
	}

	return sess, nil
}

func GetTestSession(c *SessionConfig) (*session.Session, error) {
	sess, err := session.NewSessionWithOptions(
		session.Options{
			Config:  *aws.NewConfig().WithRegion(c.Region),
			Profile: c.Profile,
		},
	)
	if err != nil {
		return nil, err
	}

	return sess, nil
}

func getNewToken(sess *session.Session, sn *string) (*sts.Credentials, error) {
	stsSvc := sts.New(sess)

	token, err := stscreds.StdinTokenProvider()
	if err != nil {
		return nil, err
	}

	out, err := stsSvc.GetSessionToken(&sts.GetSessionTokenInput{
		SerialNumber: sn,
		TokenCode:    &token,
	})

	if err != nil {
		return nil, err
	}

	return out.Credentials, nil
}

func writeCredsToFile(creds *sts.Credentials, f *ini.File, filepath, profile string) error {
	sect, err := f.NewSection(fmt.Sprintf("%s-mfa", profile))
	if err != nil {
		return err
	}

	_, err = sect.NewKey("aws_access_key_id", *creds.AccessKeyId)
	if err != nil {
		return err
	}
	_, err = sect.NewKey("aws_secret_access_key", *creds.SecretAccessKey)
	if err != nil {
		return err
	}
	_, err = sect.NewKey("aws_session_token", *creds.SessionToken)
	if err != nil {
		return err
	}
	_, err = sect.NewKey("token_expiration", creds.Expiration.Format("2006-01-02T15:04:05Z07:00"))
	if err != nil {
		return err
	}

	err = f.SaveTo(filepath)
	if err != nil {
		return err
	}

	return nil
}
