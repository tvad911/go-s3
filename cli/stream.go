package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"gos3/client"
)

var catCmd = &cobra.Command{
	Use:   "cat [s3://bucket/key]",
	Short: "Stream object to stdout",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		bucket, key, err := parseS3URI(args[0])
		if err != nil {
			return err
		}

		ctx := context.Background()
		reader, _, err := s3Client.GetObject(ctx, bucket, key, client.GetObjectOptions{})
		if err != nil {
			return err
		}
		defer reader.Close()

		_, err = io.Copy(os.Stdout, reader)
		return err
	},
}

var pipeCmd = &cobra.Command{
	Use:   "pipe [s3://bucket/key]",
	Short: "Stream stdin to object",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		bucket, key, err := parseS3URI(args[0])
		if err != nil {
			return err
		}

		ctx := context.Background()
		err = s3Client.PutObject(ctx, bucket, key, os.Stdin, -1, client.PutObjectOptions{})
		if err != nil {
			return err
		}

		if !jsonOut {
			fmt.Printf("Piped stdin to %s\n", args[0])
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(catCmd)
	rootCmd.AddCommand(pipeCmd)
}
