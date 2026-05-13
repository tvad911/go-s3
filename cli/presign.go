package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var presignExpires int

var presignCmd = &cobra.Command{
	Use:   "presign [s3://bucket/key]",
	Short: "Generate a presigned URL",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		bucket, key, err := parseS3URI(args[0])
		if err != nil {
			return err
		}

		url, err := s3Client.PresignGetObject(bucket, key, time.Duration(presignExpires)*time.Second)
		if err != nil {
			return err
		}

		if jsonOut {
			fmt.Printf(`{"url":"%s"}`+"\n", url)
		} else {
			fmt.Println(url)
		}
		return nil
	},
}

func init() {
	presignCmd.Flags().IntVar(&presignExpires, "expires", 3600, "Expiration time in seconds")
	rootCmd.AddCommand(presignCmd)
}
