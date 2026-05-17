package main

import (
	"context"
	"fmt"

	"gos3/internal/storage/metadata"
)

func main() {
	metaStore, err := metadata.NewBboltStore("./data/meta.db")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer metaStore.Close()

	sa, err := metaStore.GetServiceAccountByAccessKey(context.Background(), "minioadmin")
	if err == nil {
		fmt.Printf("SA: %s, SecretKey: %s\n", sa.AccessKeyID, sa.SecretKey)
	}

	sa2, err := metaStore.GetServiceAccountByAccessKey(context.Background(), "b7cc72dfc70c59800e05")
	if err == nil {
		fmt.Printf("SA2: %s, SecretKey: %s\n", sa2.AccessKeyID, sa2.SecretKey)
		parentUser, err := metaStore.GetUserByUsername(context.Background(), sa2.ParentUser)
		if err != nil {
			fmt.Println("Error parentUser:", err)
		} else {
			fmt.Println("ParentUser Disabled:", parentUser.Disabled)
		}
	}
}
