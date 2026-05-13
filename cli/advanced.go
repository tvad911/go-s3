package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var mvCmd = &cobra.Command{
	Use:   "mv [source] [destination]",
	Short: "Move an object",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Warning: mv is an alias for cp + rm, and is partially implemented.")
		// For a robust mv, we do cp then rm
		err := cpCmd.RunE(cmd, args)
		if err != nil {
			return err
		}
		// Assuming src is S3
		_, _, err = parseS3URI(args[0])
		if err == nil {
			return rmCmd.RunE(cmd, []string{args[0]})
		}
		return nil
	},
}

var syncCmd = &cobra.Command{
	Use:   "sync [source] [destination]",
	Short: "Synchronize local folder and S3 bucket",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("sync is not fully implemented in this phase, use cp --recursive")
	},
}

func init() {
	rootCmd.AddCommand(mvCmd)
	rootCmd.AddCommand(syncCmd)
}
