// Copyright (c) 2020 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package cmd

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	common_grpc "github.com/gitpod-io/gitpod/common-go/grpc"
	"github.com/gitpod-io/gitpod/common-go/log"
	"github.com/gitpod-io/gitpod/common-go/tracing"
	"github.com/gitpod-io/gitpod/image-builder/pkg/builder"
)

var (
	// ServiceName is the name we use for tracing/logging
	ServiceName = "image-builder"
	// Version of this service - set during build
	Version = ""
)

var jsonLog bool
var verbose bool
var configFile string

var rootCmd = &cobra.Command{
	Use:   "image-builder",
	Short: "Workspace image-builder service",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		log.Init(ServiceName, Version, jsonLog && !isatty.IsTerminal(os.Stdout.Fd()), verbose)
	},
}

// Execute runs this main command
func Execute() {
	closer := tracing.Init("image-builder")
	if closer != nil {
		defer closer.Close()
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func getConfig() *config {
	ctnt, err := os.ReadFile(configFile)
	if err != nil {
		log.WithError(xerrors.Errorf("cannot read config: %w", err)).Error("cannot read configuration. Maybe missing --config?")
		os.Exit(1)
	}

	var cfg config
	err = json.Unmarshal(ctnt, &cfg)
	if err != nil {
		log.WithError(err).Error("cannot read configuration. Maybe missing --config?")
		os.Exit(1)
	}

	return &cfg
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&jsonLog, "json-log", "j", true, "produce JSON log output on verbose level")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose JSON logging")
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file")
}

type config struct {
	Builder  builder.Configuration `json:"builder"`
	RefCache refcacheConfig        `json:"refCache,omitempty"`
	Service  struct {
		Addr string    `json:"address"`
		TLS  tlsConfig `json:"tls"`
	} `json:"service"`
	Prometheus struct {
		Addr string `json:"address"`
	} `json:"prometheus"`
	PProf struct {
		Addr string `json:"address"`
	} `json:"pprof"`
}

type refcacheConfig struct {
	Interval string   `json:"interval"`
	Refs     []string `json:"refs"`
}

type tlsConfig struct {
	Authority   string `json:"ca"`
	Certificate string `json:"crt"`
	PrivateKey  string `json:"key"`
}

// ServerOption produces the GRPC option that configures a server to use this TLS configuration
func (c *tlsConfig) ServerOption() (grpc.ServerOption, error) {
	if c.Authority == "" || c.Certificate == "" || c.PrivateKey == "" {
		return nil, nil
	}

	tlsConfig, err := common_grpc.ClientAuthTLSConfig(
		c.Authority, c.Certificate, c.PrivateKey,
		common_grpc.WithClientAuth(tls.RequireAndVerifyClientCert),
		common_grpc.WithSetClientCAs(true),
		common_grpc.WithServerName("ws-manager"),
	)
	if err != nil {
		return nil, xerrors.Errorf("cannot load certs: %w", err)
	}

	return grpc.Creds(credentials.NewTLS(tlsConfig)), nil
}
