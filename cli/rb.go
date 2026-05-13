package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var forceDelete bool

var rbCmd = &cobra.Command{
	Use:   "rb [s3://bucket-name]",
	Short: "Remove a bucket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		bucket, key, err := parseS3URI(args[0])
		if err != nil {
			return err
		}
		if key != "" {
			return fmt.Errorf("rb requires a bucket name only, got key %s", key)
		}

		ctx := context.Background()

		if forceDelete {
			// Not fully implemented recursive delete yet
			// Real implementation would delete all objects first
			fmt.Println("Warning: --force is partially implemented. Objects must be removed manually for now.")
		}

		err = s3Client.RemoveBucket(ctx, bucket)
		if err != nil {
			return err
		}

		if !jsonOut {
			fmt.Printf("Bucket '%s' removed successfully.\n", bucket)
		} else {
			fmt.Printf(`{"status":"success","bucket":"%s","action":"delete"}`+"\n", bucket)
		}
		return nil
	},
}

func init() {
	rbCmd.Flags().BoolVarP(&forceDelete, "force", "f", false, "Force delete bucket and all its objects")
	rootCmd.AddCommand(rbCmd)
}
