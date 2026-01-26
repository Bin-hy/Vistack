package main

import (
	"context"
	"fmt"
	"log"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	endpoint := "localhost:2591"
	accessKeyID := "minioadmin"
	secretAccessKey := "c9jA9ZvNXLwfs6n6fog6EJ0396Q77TbEm6G1XeDQbFG02GYwBsMh5wcTeJFzquD6sYE5saMGsrLnXernC5VaxjNfUuKqZxGRh9wf"
	useSSL := false
	bucketName := "vistack"

	// Initialize minio client object.
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Fatalln(err)
	}

	// Check if bucket exists
	exists, err := minioClient.BucketExists(context.Background(), bucketName)
	if err != nil {
		log.Fatalln(err)
	}
	if !exists {
		log.Printf("Bucket %s does not exist\n", bucketName)
		return
	}

	log.Printf("Bucket %s exists\n", bucketName)

	// Get bucket policy
	policy, err := minioClient.GetBucketPolicy(context.Background(), bucketName)
	if err != nil {
		log.Printf("Failed to get bucket policy: %v\n", err)
	} else {
		log.Printf("Current Bucket Policy:\n%s\n", policy)
	}

	// Try to set policy again to see if it works
	newPolicy := fmt.Sprintf(`{
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Principal": {"AWS": ["*"]},
                "Action": ["s3:GetObject"],
                "Resource": ["arn:aws:s3:::%s/avatars/*","arn:aws:s3:::%s/covers/*"]
            }
        ]
    }`, bucketName, bucketName)

	log.Println("Trying to set policy...")
	err = minioClient.SetBucketPolicy(context.Background(), bucketName, newPolicy)
	if err != nil {
		log.Printf("Failed to set policy: %v\n", err)
	} else {
		log.Println("Policy set successfully.")
	}
}
