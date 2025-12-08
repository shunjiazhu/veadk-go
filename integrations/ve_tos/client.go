// Copyright (c) 2025 Beijing Volcano Engine Technology Co., Ltd. and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ve_tos

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/volcengine/ve-tos-golang-sdk/v2/tos"
	"github.com/volcengine/ve-tos-golang-sdk/v2/tos/enum"
)

var bucketRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`)
var (
	TosBucketInvalidErr = errors.New("tos bucket invalid, bucket names must be 3-63 characters long, contain only lowercase letters, numbers , and hyphens (-), start and end with a letter or number")
	TosClientInvalidErr = errors.New("TOS client is not initialized")
)

type Config struct {
	AK           string
	SK           string
	SessionToken string
	Region       string
	Endpoint     string
	Bucket       string
}

type Client struct {
	config *Config
	client *tos.ClientV2
}

func New(cfg *Config) (*Client, error) {
	if cfg.Endpoint == "" {
		cfg.Endpoint = fmt.Sprintf("https://tos-%s.volces.com", cfg.Region)
	}

	cred := tos.NewStaticCredentials(cfg.AK, cfg.SK)
	if cfg.SessionToken != "" {
		cred.WithSecurityToken(cfg.SessionToken)
	}

	client, err := tos.NewClientV2(cfg.Endpoint,
		tos.WithRegion(cfg.Region),
		tos.WithCredentials(cred))

	if err != nil {
		return nil, nil
	}

	return &Client{
		config: cfg,
		client: client,
	}, nil
}

func (c *Client) preCheckBucket(bucket string) error {
	if !bucketRe.MatchString(bucket) {
		return TosBucketInvalidErr
	}
	return nil
}

func (c *Client) CreateBucket(ctx context.Context, bucket string) error {
	if err := c.preCheckBucket(bucket); err != nil {
		return err
	}
	_, err := c.client.CreateBucketV2(ctx, &tos.CreateBucketV2Input{
		Bucket:       bucket,
		ACL:          enum.ACLPublicRead,
		StorageClass: enum.StorageClassStandard,
	})
	if err != nil {
		return fmt.Errorf("CreateBucket error: %v", err)
	}
	//Set CORS rules
	_, err = c.client.PutBucketCORS(ctx, &tos.PutBucketCORSInput{
		Bucket: bucket,
		CORSRules: []tos.CorsRule{
			tos.CorsRule{
				AllowedOrigin: []string{"*"},
				AllowedMethod: []string{"GET", "HEAD"},
				AllowedHeader: []string{"*"},
				MaxAgeSeconds: 1000,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("PutBucketCORS error: %v", err)
	}
	return nil
}

func (c *Client) BucketExist(ctx context.Context, bucket string) (bool, error) {
	if err := c.preCheckBucket(bucket); err != nil {
		return false, err
	}
	_, err := c.client.HeadBucket(ctx, &tos.HeadBucketInput{
		Bucket: bucket,
	})
	if err != nil {
		var serverErr *tos.TosServerError
		if errors.As(err, &serverErr) {
			if serverErr.StatusCode == http.StatusNotFound {
				return false, nil
			}
		}
		return false, err
	}
	return true, nil
}

func (c *Client) DeleteBucket(ctx context.Context, bucket string) error {
	if err := c.preCheckBucket(bucket); err != nil {
		return err
	}
	_, err := c.client.DeleteBucket(ctx, &tos.DeleteBucketInput{
		Bucket: bucket,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) BuildObjectKeyForFile(dataPath string) string {
	u, _ := url.Parse(dataPath)
	if u != nil && (u.Scheme == "http" || u.Scheme == "https" || u.Scheme == "ftp" || u.Scheme == "ftps") {
		objectKey := strings.TrimPrefix(u.Host+u.Path, "/")
		return objectKey
	}
	absPath, _ := filepath.Abs(dataPath)
	cwd, _ := os.Getwd()
	relPath, err := filepath.Rel(cwd, absPath)
	var objectKey string
	if err == nil &&
		!strings.HasPrefix(relPath, "../") &&
		!strings.HasPrefix(relPath, `..\`) &&
		!strings.HasPrefix(relPath, "./") &&
		!strings.HasPrefix(relPath, `.\`) {
		objectKey = relPath
	} else {
		objectKey = filepath.Base(dataPath)
	}
	if strings.HasPrefix(objectKey, "/") {
		objectKey = objectKey[1:]
	}
	if objectKey == "" ||
		strings.Contains(objectKey, "../") ||
		strings.Contains(objectKey, `..\`) ||
		strings.Contains(objectKey, "./") ||
		strings.Contains(objectKey, `.\`) {
		objectKey = filepath.Base(dataPath)
	}
	return objectKey
}

func (c *Client) BuildObjectKeyForText() string {
	return time.Now().Format("20060102150405") + ".txt"
}

func (c *Client) BuildObjectKeyForBytes() string {
	return time.Now().Format("20060102150405")
}

func (c *Client) BuildTOSURL(objectKey string) string {
	return fmt.Sprintf("%s/%s", c.config.Bucket, objectKey)
}

func (c *Client) buildTOSSignedURL(objectKey string, bucketName string) (string, error) {
	if err := c.preCheckBucket(bucketName); err != nil {
		return "", err
	}
	out, err := c.client.PreSignedURL(&tos.PreSignedURLInput{
		HTTPMethod: http.MethodGet,
		Bucket:     bucketName,
		Key:        objectKey,
		Expires:    604800,
	})
	if err != nil {
		return "", err
	}
	return out.SignedUrl, nil
}

func (c *Client) ensureClientAndBucket(bucketName string) error {
	// todo refreshClient
	if c.client == nil {
		return TosClientInvalidErr
	}
	if exist, err := c.BucketExist(context.Background(), bucketName); err != nil || !exist {
		if err := c.CreateBucket(context.Background(), bucketName); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) UploadText(text string, bucketName string, objectKey string, metadata map[string]string) error {
	if err := c.preCheckBucket(bucketName); err != nil {
		return err
	}
	if objectKey == "" {
		objectKey = c.BuildObjectKeyForText()
	}
	if err := c.ensureClientAndBucket(bucketName); err != nil {
		return err
	}
	if _, err := c.client.PutObjectV2(context.Background(), &tos.PutObjectV2Input{
		PutObjectBasicInput: tos.PutObjectBasicInput{
			Bucket: bucketName,
			Key:    objectKey,
			Meta:   metadata,
		},
		Content: strings.NewReader(text),
	}); err != nil {
		return err
	}
	return nil
}

func (c *Client) AsyncUploadText(text string, bucketName string, objectKey string, metadata map[string]string) <-chan error {
	ch := make(chan error, 1)
	go func() {
		ch <- c.UploadText(text, bucketName, objectKey, metadata)
		close(ch)
	}()
	return ch
}

func (c *Client) UploadBytes(data []byte, bucketName string, objectKey string, metadata map[string]string) error {
	if err := c.preCheckBucket(bucketName); err != nil {
		return err
	}
	if objectKey == "" {
		objectKey = c.BuildObjectKeyForBytes()
	}
	if err := c.ensureClientAndBucket(bucketName); err != nil {
		return err
	}
	if _, err := c.client.PutObjectV2(context.Background(), &tos.PutObjectV2Input{
		PutObjectBasicInput: tos.PutObjectBasicInput{
			Bucket: bucketName,
			Key:    objectKey,
			Meta:   metadata,
		},
		Content: strings.NewReader(string(data)),
	}); err != nil {
		return err
	}
	return nil
}

func (c *Client) AsyncUploadBytes(data []byte, bucketName string, objectKey string, metadata map[string]string) <-chan error {
	ch := make(chan error, 1)
	go func() {
		ch <- c.UploadBytes(data, bucketName, objectKey, metadata)
		close(ch)
	}()
	return ch
}

func (c *Client) UploadFile(filePath string, bucketName string, objectKey string, metadata map[string]string) error {
	if err := c.preCheckBucket(bucketName); err != nil {
		return err
	}
	if objectKey == "" {
		objectKey = c.BuildObjectKeyForFile(filePath)
	}
	if err := c.ensureClientAndBucket(bucketName); err != nil {
		return err
	}
	if _, err := c.client.PutObjectFromFile(context.Background(), &tos.PutObjectFromFileInput{
		PutObjectBasicInput: tos.PutObjectBasicInput{
			Bucket: bucketName,
			Key:    objectKey,
			Meta:   metadata,
		},
		FilePath: filePath,
	}); err != nil {
		return err
	}
	return nil
}

func (c *Client) AsyncUploadFile(filePath string, bucketName string, objectKey string, metadata map[string]string) <-chan error {
	ch := make(chan error, 1)
	go func() {
		ch <- c.UploadFile(filePath, bucketName, objectKey, metadata)
		close(ch)
	}()
	return ch
}

func (c *Client) UploadFiles(filePaths []string, bucketName string, objectKeys []string, metadata map[string]string) error {
	if err := c.preCheckBucket(bucketName); err != nil {
		return err
	}
	if objectKeys == nil {
		objectKeys = make([]string, 0, len(filePaths))
		for _, fp := range filePaths {
			objectKeys = append(objectKeys, c.BuildObjectKeyForFile(fp))
		}
	}
	if len(objectKeys) != len(filePaths) {
		return fmt.Errorf("objectKeys and filePaths lengths mismatch")
	}
	for i, fp := range filePaths {
		if _, err := c.client.PutObjectFromFile(context.Background(), &tos.PutObjectFromFileInput{
			PutObjectBasicInput: tos.PutObjectBasicInput{
				Bucket: bucketName,
				Key:    objectKeys[i],
				Meta:   metadata,
			},
			FilePath: fp,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) AsyncUploadFiles(filePaths []string, bucketName string, objectKeys []string, metadata map[string]string) <-chan error {
	ch := make(chan error, 1)
	go func() {
		ch <- c.UploadFiles(filePaths, bucketName, objectKeys, metadata)
		close(ch)
	}()
	return ch
}

func (c *Client) UploadDirectory(directoryPath string, bucketName string, metadata map[string]string) error {
	if err := c.preCheckBucket(bucketName); err != nil {
		return err
	}
	if err := filepath.Walk(directoryPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		objectKey, err2 := filepath.Rel(directoryPath, path)
		if err2 != nil {
			return err2
		}
		if _, err = c.client.PutObjectFromFile(context.Background(), &tos.PutObjectFromFileInput{
			PutObjectBasicInput: tos.PutObjectBasicInput{
				Bucket: bucketName,
				Key:    objectKey,
				Meta:   metadata,
			},
			FilePath: path,
		}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (c *Client) AsyncUploadDirectory(directoryPath string, bucketName string, metadata map[string]string) <-chan error {
	ch := make(chan error, 1)
	go func() {
		ch <- c.UploadDirectory(directoryPath, bucketName, metadata)
		close(ch)
	}()
	return ch
}

// Download https://www.volcengine.com/docs/6349/93471?lang=zh
func (c *Client) Download(bucketName string, objectKey string, savePath string) error {
	if err := c.preCheckBucket(bucketName); err != nil {
		return err
	}
	if objectKey == "" || savePath == "" {
		return fmt.Errorf("objectKey or savePath is empty")

	}
	if err := c.ensureClientAndBucket(bucketName); err != nil {
		return err
	}
	rc, err := c.client.GetObjectV2(context.Background(), &tos.GetObjectV2Input{
		Bucket: bucketName,
		Key:    objectKey,
	})
	if err != nil {
		return err
	}
	defer rc.Content.Close()

	if dir := filepath.Dir(savePath); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	f, err := os.Create(savePath)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = io.Copy(f, rc.Content); err != nil {
		return err
	}
	return nil
}

func (c *Client) Close() {
	if c.client != nil {
		c.client.Close()
	}
}
