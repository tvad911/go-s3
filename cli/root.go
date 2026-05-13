package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gos3/client"
)

var (
	endpoint  string
	accessKey string
	secretKey string
	region    string
	noSSL     bool
	cfgFile   string
	jsonOut   bool
)

// s3Client is initialized before executing commands
var s3Client *client.Client

var rootCmd = &cobra.Command{
	Use:   "gos3c",
	Short: "gos3c is a CLI for GoS3 server",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize the S3 client using flags and env vars
		// Env vars fallback can be added via viper if needed, but for now we'll do simple env checking
		
		ep := endpoint
		if ep == "" {
			ep = os.Getenv("GOS3C_ENDPOINT")
		}
		
		ak := accessKey
		if ak == "" {
			ak = os.Getenv("GOS3C_ACCESS_KEY")
		}
		
		sk := secretKey
		if sk == "" {
			sk = os.Getenv("GOS3C_SECRET_KEY")
		}
		
		rg := region
		if rg == "" {
			rg = os.Getenv("GOS3C_REGION")
		}
		if rg == "" {
			rg = "us-east-1"
		}

		if ep == "" {
			return fmt.Errorf("endpoint is required (use --endpoint or GOS3C_ENDPOINT)")
		}

		c, err := client.New(client.Config{
			Endpoint:        ep,
			AccessKeyID:     ak,
			SecretAccessKey: sk,
			Region:          rg,
			UseSSL:          !noSSL,
			PathStyle:       true,
		})
		if err != nil {
			return err
		}
		s3Client = c
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&endpoint, "endpoint", "e", "", "S3 endpoint URL (e.g. http://localhost:9000)")
	rootCmd.PersistentFlags().StringVarP(&accessKey, "access-key", "a", "", "Access Key ID")
	rootCmd.PersistentFlags().StringVarP(&secretKey, "secret-key", "s", "", "Secret Access Key")
	rootCmd.PersistentFlags().StringVarP(&region, "region", "r", "us-east-1", "Region name")
	rootCmd.PersistentFlags().BoolVar(&noSSL, "no-ssl", false, "Disable TLS verification or prefer HTTP")
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.gos3c.yaml)")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Output in JSON format")
}
