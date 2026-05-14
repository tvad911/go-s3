package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Admin operations (users, info)",
}

var adminUserCmd = &cobra.Command{
	Use:   "user",
	Short: "Manage users",
}

// Helper to make admin requests
func doAdminReq(method, path string, body io.Reader) ([]byte, error) {
	resp, err := s3Client.DoAdminRequest(context.Background(), method, path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("admin request failed: %s", string(b))
	}

	return io.ReadAll(resp.Body)
}

var adminUserAddCmd = &cobra.Command{
	Use:   "add [username]",
	Short: "Add a new user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// POST /_admin/users
		payload := fmt.Sprintf(`{"username":"%s"}`, args[0])
		res, err := doAdminReq("POST", "/users", strings.NewReader(payload))
		if err != nil {
			return err
		}

		if jsonOut {
			fmt.Println(string(res))
		} else {
			var m map[string]interface{}
			json.Unmarshal(res, &m)
			fmt.Printf("User created.\nUsername: %v\nAccessKey: %v\nSecretKey: %v\n",
				m["username"], m["accessKey"], m["secretKey"])
		}
		return nil
	},
}

var adminUserLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List users",
	RunE: func(cmd *cobra.Command, args []string) error {
		res, err := doAdminReq("GET", "/users", nil)
		if err != nil {
			return err
		}

		if jsonOut {
			fmt.Println(string(res))
		} else {
			var users []map[string]interface{}
			json.Unmarshal(res, &users)
			for _, u := range users {
				fmt.Printf("%s\n", u["username"])
			}
		}
		return nil
	},
}

var adminUserRmCmd = &cobra.Command{
	Use:   "rm [username]",
	Short: "Remove a user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := doAdminReq("DELETE", "/users/"+args[0], nil)
		if err != nil {
			return err
		}
		if !jsonOut {
			fmt.Printf("User %s removed.\n", args[0])
		}
		return nil
	},
}

var adminUserRotateCmd = &cobra.Command{
	Use:   "rotate-key [username]",
	Short: "Rotate user keys",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		res, err := doAdminReq("PUT", "/users/"+args[0], nil) // Assuming PUT updates
		if err != nil {
			return err
		}

		if jsonOut {
			fmt.Println(string(res))
		} else {
			var m map[string]interface{}
			json.Unmarshal(res, &m)
			fmt.Printf("Keys rotated.\nUsername: %v\nAccessKey: %v\nSecretKey: %v\n",
				m["username"], m["accessKey"], m["secretKey"])
		}
		return nil
	},
}

var adminInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show server stats",
	RunE: func(cmd *cobra.Command, args []string) error {
		res, err := doAdminReq("GET", "/info", nil)
		if err != nil {
			return err
		}

		if jsonOut {
			fmt.Println(string(res))
		} else {
			var m map[string]interface{}
			json.Unmarshal(res, &m)
			fmt.Printf("Server Version: %v\nUptime: %v seconds\nStorage Total: %v\nStorage Free: %v\n",
				m["version"], m["uptime_seconds"], m["storage_total_bytes"], m["storage_free_bytes"])
		}
		return nil
	},
}

func init() {
	adminCmd.AddCommand(adminUserCmd)
	adminCmd.AddCommand(adminInfoCmd)

	adminUserCmd.AddCommand(adminUserAddCmd)
	adminUserCmd.AddCommand(adminUserLsCmd)
	adminUserCmd.AddCommand(adminUserRmCmd)
	adminUserCmd.AddCommand(adminUserRotateCmd)

	rootCmd.AddCommand(adminCmd)
}
