package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
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

type handler struct {
	clientCLoudStorageService cloudStorageService
}

func newHandler(clientCloudStorageService *cloudStorageService) handler {
	return handler{clientCLoudStorageService: *clientCloudStorageService}
}

func (h handler) uploadFiles(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	
	bucket := os.Getenv("BUCKET_NAME")

	if err := r.ParseMultipartForm(r.ContentLength); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	filesHeader := r.MultipartForm.File["images"]
	if len(filesHeader) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var mu sync.Mutex
	for _, fileHeader := range filesHeader {
		go func(fileHeader *multipart.FileHeader) {
			file, err := fileHeader.Open()
			if err != nil {
				log.Printf("[%s]: %s", fileHeader.Filename, err.Error())
				return
			}
			defer file.Close()

			bytes, err := io.ReadAll(file)
			if err != nil {
				log.Printf("[%s]: %s", fileHeader.Filename, err.Error())
				return
			}
			
			mu.Lock()
			extention := filepath.Ext(fileHeader.Filename)
			filename := fmt.Sprintf("%v-%s%s", time.Now().Unix(), strings.Split(fileHeader.Filename, extention)[0], extention)

			imageUrl, err := h.clientCLoudStorageService.uploadFile(bucket, filename, bytes)
			if err != nil {
				log.Printf("[%s]: %s", fileHeader.Filename, err.Error())
				return
			}
			log.Printf("File %s sukses diupload., URL: %s\n", fileHeader.Filename, imageUrl)
			mu.Unlock()
		}(fileHeader)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`{"message": "File sedang diproses."}`)))
	return
}

func main() {

	client, err := storage.NewClient(context.Background(), option.WithCredentialsFile(os.Getenv("STORAGE_CREDENTIAL_FILE")))
	if err != nil {
		log.Fatal(err.Error())
	}

	cloudStorageService := newCloudStorageService(client)
	handler := newHandler(&cloudStorageService)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/upload-files", handler.uploadFiles)

	port := os.Getenv("APP_PORT")
	log.Printf("Running server on: %s\n", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), mux); err != nil {
		log.Fatal(err.Error())
	}
}