package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"gos3/client"
)

var lsCmd = &cobra.Command{
	Use:   "ls [s3://bucket/prefix]",
	Short: "List buckets or objects",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		if len(args) == 0 {
			// List buckets
			buckets, err := s3Client.ListBuckets(ctx)
			if err != nil {
				return err
			}

			if jsonOut {
				b, _ := json.Marshal(buckets)
				fmt.Println(string(b))
				return nil
			}

			for _, b := range buckets {
				fmt.Printf("%s\t%s\n", b.CreationDate.Format("2006-01-02 15:04:05"), b.Name)
			}
			return nil
		}

		// List objects
		bucket, prefix, err := parseS3URI(args[0])
		if err != nil {
			return err
		}

		opts := client.ListObjectsOptions{
			Prefix: prefix,
		}

		ch := s3Client.ListObjectsV2(ctx, bucket, opts)

		var objects []client.Object
		for obj := range ch {
			if jsonOut {
				objects = append(objects, obj)
				continue
			}
			
			if obj.Size == 0 && obj.LastModified.IsZero() {
				// Directory (CommonPrefix)
				fmt.Printf("                           PRE %s\n", obj.Key)
			} else {
				fmt.Printf("%s %10d %s\n", obj.LastModified.Format("2006-01-02 15:04:05"), obj.Size, obj.Key)
			}
		}

		if jsonOut {
			b, _ := json.Marshal(objects)
			fmt.Println(string(b))
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(lsCmd)
}
