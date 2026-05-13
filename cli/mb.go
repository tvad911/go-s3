package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var mbCmd = &cobra.Command{
	Use:   "mb [s3://bucket-name]",
	Short: "Make a new bucket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		bucket, key, err := parseS3URI(args[0])
		if err != nil {
			return err
		}
		if key != "" {
			return fmt.Errorf("mb requires a bucket name only, got key %s", key)
		}

		err = s3Client.MakeBucket(context.Background(), bucket)
		if err != nil {
			return err
		}

		if !jsonOut {
			fmt.Printf("Bucket '%s' created successfully.\n", bucket)
		} else {
			fmt.Printf(`{"status":"success","bucket":"%s","action":"create"}`+"\n", bucket)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(mbCmd)
}
