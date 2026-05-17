package main
import ("fmt"; "gos3/internal/s3")
func main() { 
	fmt.Printf("Lifecycle: %q\n", s3.ErrNoSuchLifecycleConfiguration.Error()) 
	fmt.Printf("Website: %q\n", s3.ErrNoSuchWebsiteConfiguration.Error()) 
	fmt.Printf("CORS: %q\n", s3.ErrNoSuchCORSConfiguration.Error()) 
	fmt.Printf("Policy: %q\n", s3.ErrNoSuchBucketPolicy.Error()) 
}
