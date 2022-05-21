package db

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

const ArtworksCollectionName = "artworks"

type ArtWork struct {
	ID          string   `json:"id"`
	ScreenID    string   `json:"screen_id"`
	CreatedDate string   `json:"created_date"`
	S3ID        string   `json:"s3_id"`
	Title       *string  `json:"title"`
	Artist      *string  `json:"artist"`
	Price       *float64 `json:"price"`
	Currency    *string  `json:"currency"`
	Link        *string  `json:"link"`
	ShortText   *string  `json:"short_text"`
}

func SaveArtwork(artWork ArtWork) error {
	session := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	result, err := dynamodbattribute.MarshalMap(artWork)
	if err != nil {
		return err
	}

	svc := dynamodb.New(session)
	input := &dynamodb.PutItemInput{
		Item:      result,
		TableName: aws.String(ArtworksCollectionName),
	}

	_, err = svc.PutItem(input)
	if err != nil {
		return err
	}

	return nil
}
