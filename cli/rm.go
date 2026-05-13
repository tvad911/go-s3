package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var rmRecursive bool

var rmCmd = &cobra.Command{
	Use:   "rm [s3://bucket/key]",
	Short: "Remove an object",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		bucket, key, err := parseS3URI(args[0])
		if err != nil {
			return err
		}
		if key == "" && !rmRecursive {
			return fmt.Errorf("key is required. To remove a bucket use 'rb', or to empty a bucket use '--recursive'")
		}

		ctx := context.Background()

		if rmRecursive {
			fmt.Println("Warning: recursive rm not fully implemented yet.")
			// Would list all objects and delete them
		} else {
			err = s3Client.RemoveObject(ctx, bucket, key)
			if err != nil {
				return err
			}
			if !jsonOut {
				fmt.Printf("Object '%s' removed successfully.\n", args[0])
			}
		}

		return nil
	},
}

func init() {
	rmCmd.Flags().BoolVarP(&rmRecursive, "recursive", "r", false, "Remove multiple objects recursively")
	rootCmd.AddCommand(rmCmd)
}
