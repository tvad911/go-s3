package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var statCmd = &cobra.Command{
	Use:   "stat [s3://bucket/key]",
	Short: "Get object metadata",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		bucket, key, err := parseS3URI(args[0])
		if err != nil {
			return err
		}

		info, err := s3Client.StatObject(context.Background(), bucket, key)
		if err != nil {
			return err
		}

		if jsonOut {
			b, _ := json.Marshal(info)
			fmt.Println(string(b))
			return nil
		}

		fmt.Printf("File: %s\n", info.Key)
		fmt.Printf("Size: %d\n", info.Size)
		fmt.Printf("LastModified: %s\n", info.LastModified)
		fmt.Printf("ETag: %s\n", info.ETag)
		fmt.Printf("ContentType: %s\n", info.ContentType)
		for k, v := range info.Metadata {
			fmt.Printf("%s: %s\n", k, v)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statCmd)
}
