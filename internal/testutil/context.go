// Copyright 2018 The zap-encoder Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testutil

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/jwt"
)

const (
	envProjectID  = "GCLOUD_TESTS_GOLANG_PROJECT_ID"
	envPrivateKey = "GCLOUD_TESTS_GOLANG_KEY"
)

// ProjectID returns the project ID to use in integration tests, or the empty
// string if none is configured.
func ProjectID() string {
	return os.Getenv(envProjectID)
}

// TokenSource returns the OAuth2 token source to use in integration tests,
// or nil if none is configured. It uses the standard environment variable
// for tests in this repo.
func TokenSource(ctx context.Context, scopes ...string) oauth2.TokenSource {
	return TokenSourceEnv(ctx, envPrivateKey, scopes...)
}

// TokenSourceEnv returns the OAuth2 token source to use in integration tests. or nil
// if none is configured. It tries to get credentials from the filename in the
// environment variable envVar. If the environment variable is unset, TokenSourceEnv
// will try to find 'Application Default Credentials'. Else, TokenSourceEnv will
// return nil. TokenSourceEnv will log.Fatal if the token source is specified but
// missing or invalid.
func TokenSourceEnv(ctx context.Context, envVar string, scopes ...string) oauth2.TokenSource {
	key := os.Getenv(envVar)
	if key == "" { // Try for application default credentials.
		ts, err := google.DefaultTokenSource(ctx, scopes...)
		if err != nil {
			log.Println("No 'Application Default Credentials' found.")
			return nil
		}
		return ts
	}
	conf, err := jwtConfigFromFile(key, scopes)
	if err != nil {
		log.Fatal(err)
	}
	return conf.TokenSource(ctx)
}

// JWTConfig reads the JSON private key file whose name is in the default
// environment variable, and returns the jwt.Config it contains. It ignores
// scopes.
// If the environment variable is empty, it returns (nil, nil).
func JWTConfig() (*jwt.Config, error) {
	return jwtConfigFromFile(os.Getenv(envPrivateKey), nil)
}

// jwtConfigFromFile reads the given JSON private key file, and returns the
// jwt.Config it contains.
// If the filename is empty, it returns (nil, nil).
func jwtConfigFromFile(filename string, scopes []string) (*jwt.Config, error) {
	if filename == "" {
		return nil, nil
	}
	jsonKey, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("Cannot read the JSON key file, err: %v", err)
	}
	conf, err := google.JWTConfigFromJSON(jsonKey, scopes...)
	if err != nil {
		return nil, fmt.Errorf("google.JWTConfigFromJSON: %v", err)
	}
	return conf, nil
}

// CanReplay reports whether an integration test can be run in replay mode.
// The replay file must exist, and the GCLOUD_TESTS_GOLANG_ENABLE_REPLAY
// environment variable must be non-empty.
func CanReplay(replayFilename string) bool {
	if os.Getenv("GCLOUD_TESTS_GOLANG_ENABLE_REPLAY") == "" {
		return false
	}
	_, err := os.Stat(replayFilename)
	return err == nil
}
