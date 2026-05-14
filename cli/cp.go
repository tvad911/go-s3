package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gos3/client"
)

var cpRecursive bool

var cpCmd = &cobra.Command{
	Use:   "cp [source] [destination]",
	Short: "Copy an object",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		src := args[0]
		dst := args[1]
		ctx := context.Background()

		srcIsS3 := strings.HasPrefix(src, "s3://")
		dstIsS3 := strings.HasPrefix(dst, "s3://")

		if srcIsS3 && dstIsS3 {
			// s3 to s3
			srcBucket, srcKey, err := parseS3URI(src)
			if err != nil {
				return err
			}
			dstBucket, dstKey, err := parseS3URI(dst)
			if err != nil {
				return err
			}

			err = s3Client.CopyObject(ctx, srcBucket, srcKey, dstBucket, dstKey)
			if err != nil {
				return err
			}
			if !jsonOut {
				fmt.Printf("Copied %s to %s\n", src, dst)
			}
		} else if srcIsS3 && !dstIsS3 {
			// s3 to local
			bucket, key, err := parseS3URI(src)
			if err != nil {
				return err
			}

			err = s3Client.FGetObject(ctx, bucket, key, dst, client.GetObjectOptions{})
			if err != nil {
				return err
			}
			if !jsonOut {
				fmt.Printf("Downloaded %s to %s\n", src, dst)
			}
		} else if !srcIsS3 && dstIsS3 {
			// local to s3
			bucket, key, err := parseS3URI(dst)
			if err != nil {
				return err
			}

			err = s3Client.PutObjectMultipart(ctx, bucket, key, src, client.PutObjectOptions{})
			if err != nil {
				return err
			}
			if !jsonOut {
				fmt.Printf("Uploaded %s to %s\n", src, dst)
			}
		} else {
			return fmt.Errorf("local to local copy not supported")
		}

		return nil
	},
}

func init() {
	cpCmd.Flags().BoolVarP(&cpRecursive, "recursive", "R", false, "Recursive copy (not fully implemented yet)")
	rootCmd.AddCommand(cpCmd)
}
