package storage

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/google/uuid"
)

const ArtWorkS3BucketName = "daiara-screens-artwork"

func UploadArtwork(filename string, fileContent []byte) (string, error) {
	session := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	uploader := s3manager.NewUploader(session)

	key := fmt.Sprintf("%s_%s", uuid.NewString(), filename)

	_, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(ArtWorkS3BucketName),
		ACL:    aws.String("public-read"),
		Key:    aws.String(key),
		Body:   bytes.NewReader(fileContent),
	})
	if err != nil {
		return "", err
	}

	return key, nil
}
