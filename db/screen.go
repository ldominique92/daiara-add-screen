package db

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/google/uuid"
	"time"
)

const ScreensCollectionName = "screens"

type Screen struct {
	ID             string `json:"id"`
	RegisteredDate string `json:"registered_date"`
	WalletAddress  string `json:"wallet_address"`
}

func SaveScreen() (*Screen, error) {
	session := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	screen := &Screen{ID: uuid.NewString(), RegisteredDate: time.Now().UTC().Format("2006-01-02 15:04:05")}
	result, err := dynamodbattribute.MarshalMap(screen)
	if err != nil {
		return nil, err
	}

	svc := dynamodb.New(session)
	input := &dynamodb.PutItemInput{
		Item:      result,
		TableName: aws.String(ScreensCollectionName),
	}

	_, err = svc.PutItem(input)
	if err != nil {
		return nil, err
	}

	return screen, nil
}
