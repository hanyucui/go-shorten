package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/pkg/errors"
)

func init() {
	SupportedStorageTypes["S3"] = new(interface{})
}

type S3 struct {
	Client     *s3.S3
	BucketName string

	storageVersion string
	hashFunc       func(string) string
}

func NewS3(awsSession *session.Session, bucketName string) (*S3, error) {
	if awsSession == nil {
		var err error
		awsSession, err = session.NewSession(&aws.Config{
			Region: aws.String("us-west-2"),
			CredentialsChainVerboseErrors: aws.Bool(true),
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to create AWS session")
		}
	}

	s := &S3{
		Client:     s3.New(awsSession),
		BucketName: bucketName,

		storageVersion: "v2",
		hashFunc: func(s string) string {
			h := sha256.Sum256([]byte(s))
			return hex.EncodeToString(h[:])
		},
	}

	_, err := s.Client.HeadBucket(&s3.HeadBucketInput{
		Bucket: aws.String(s.BucketName),
	})
	if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == "NotFound" {
		_, err = s.Client.CreateBucket(&s3.CreateBucketInput{
			Bucket: aws.String(s.BucketName),
		})
	}

	return s, err
}

func (s *S3) saveKey(short, url string) (err error) {
	hashedShort := s.hashFunc(short)
	s3BucketPrefix := path.Join(s.storageVersion, hashedShort)

	_, err = s.Client.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(s.BucketName),
		Key:         aws.String(path.Join(s3BucketPrefix, "long")),
		Body:        strings.NewReader(url),
		ContentType: aws.String("text/plain"),
	})
	if err != nil {
		return errors.Wrap(err, "failed to save long url to s3")
	}

	_, err = s.Client.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(s.BucketName),
		Key:         aws.String(path.Join(s3BucketPrefix, "short")),
		Body:        strings.NewReader(short),
		ContentType: aws.String("text/plain"),
	})
	if err != nil {
		return errors.Wrap(err, "failed to save short url to s3")
	}

	changeLog, err := json.Marshal(
		struct {
			URL  string
			User string
		}{
			url,
			"TODO",
		},
	)
	if err != nil {
		return errors.Wrap(err, "unable to format change history")
	}

	_, err = s.Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(s.BucketName),
		Key: aws.String(
			path.Join(
				s3BucketPrefix,
				"change_history",
				time.Now().Format(time.RFC3339Nano),
			)),
		Body:        bytes.NewReader(changeLog),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return errors.Wrap(err, "failed to save changelog to s3")
	}

	return nil
}

func (s *S3) Save(url string) (string, error) {
	if _, err := validateURL(url); err != nil {
		return "", err
	}

	for i := 0; i < 10; i++ {
		short := getRandomString(8)
		pathToShort := path.Join(s.storageVersion, s.hashFunc(short))

		_, err := s.Client.HeadObject(&s3.HeadObjectInput{
			Bucket: aws.String(s.BucketName),
			Key:    aws.String(pathToShort),
		})
		if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == "NotFound" {
			return short, s.saveKey(short, url)
		}
	}

	return "", ErrShortExhaustion
}

func (s *S3) SaveName(rawShort string, url string) error {
	short, err := sanitizeShort(rawShort)
	if err != nil {
		return err
	}
	if _, err := validateURL(url); err != nil {
		return err
	}

	return s.saveKey(short, url)
}

func (s *S3) Load(rawShort string) (string, error) {
	short, err := sanitizeShort(rawShort)
	if err != nil {
		return "", err
	}

	resp, err := s.Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s.BucketName),
		Key:    aws.String(path.Join(s.storageVersion, s.hashFunc(short), "long")),
	})
	if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == "NoSuchKey" {
		return "", ErrShortNotSet
	}

	var bb bytes.Buffer
	if _, err := bb.ReadFrom(resp.Body); err != nil {
		return "", errors.Wrap(err, "failed to read long url")
	}

	return bb.String(), err
}
