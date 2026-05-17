package main

import (
	"context"
	"fmt"
	"gos3/internal/storage/metadata"
	"path/filepath"
)

func main() {
	store, err := metadata.NewBboltStore(filepath.Join("tmp2", "meta.db"))
	if err != nil {
		panic(err)
	}
	defer store.Close()

	lc, err := store.GetBucketLifecycle(context.Background(), "gos3")
	if err != nil {
		fmt.Printf("Lifecycle Error string: %q\n", err.Error())
	} else {
		fmt.Printf("Lifecycle Success: %+v\n", lc)
	}
	
	web, err := store.GetBucketWebsite(context.Background(), "gos3")
	if err != nil {
		fmt.Printf("Website Error string: %q\n", err.Error())
	} else {
		fmt.Printf("Website Success: %+v\n", web)
	}
}
