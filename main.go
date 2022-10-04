package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"cloud.google.com/go/storage"
)

type cloudStorageService struct {
	clientStorage *storage.Client
}

func newCloudStorageService(client *storage.Client) cloudStorageService {
	return cloudStorageService{clientStorage: client}
}

func (c cloudStorageService) uploadFile(bucketName string, fileName string, bytesObject []byte) (
	string,
	error,
) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second * 60)
	defer cancel()

	wc := c.clientStorage.Bucket(bucketName).Object(fileName).NewWriter(ctx)
	wc.ChunkSize = 0

	buff := bytes.NewBuffer(bytesObject)
	if _, err := io.Copy(wc, buff); err != nil {
		return "", err
	}

	if err := wc.Close(); err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/%s/%s", "https://storage.googleapis.com", bucketName, fileName), nil
}