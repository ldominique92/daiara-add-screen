package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/google/uuid"
)

// Authorization
const authToken = "c2VtcmFib25hb3ZhbGVuYWRh="
const ScreensCollectionName = "screens"

type functionRequestBody struct {
	PushNotificationToken string `json:"push_notification_token"`
}

type Screen struct {
	ID                    string `json:"id"`
	RegisteredDate        string `json:"registered_date"`
	WalletAddress         string `json:"wallet_address"`
	PushNotificationToken string `json:"push_notification_token"`
}

func handler(request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
	if !isAuthorized(request.Headers["authorization"]) {
		return events.APIGatewayProxyResponse{
			Body:       "Wrong authorization token provided",
			StatusCode: 403,
		}, nil
	}

	var requestBody functionRequestBody
	err := json.Unmarshal([]byte(request.Body), &requestBody)
	if err != nil {
		log.Printf("Failed to read request body (%s): %s", request.Body, fmt.Errorf("%w", err))
		return events.APIGatewayProxyResponse{
			Body:       "Invalid request",
			StatusCode: 400,
		}, nil
	}

	if requestBody.PushNotificationToken == "" {
		return events.APIGatewayProxyResponse{
			Body:       "Invalid push notification token",
			StatusCode: 400,
		}, nil
	}

	screen, err := saveScreen(requestBody.PushNotificationToken)
	if err != nil {
		log.Printf("Failed to persist screen object %s:", fmt.Errorf("%w", err))
		return events.APIGatewayProxyResponse{
			Body:       "Failed to persist screen object",
			StatusCode: 500,
		}, nil
	}

	response, err := json.Marshal(screen)
	if err != nil {
		log.Printf("Failed to marshall screen object %s:", fmt.Errorf("%w", err))
		return events.APIGatewayProxyResponse{
			Body:       "Failed to marshall screen object",
			StatusCode: 500,
		}, nil
	}

	return events.APIGatewayProxyResponse{
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(response),
		StatusCode: 200,
	}, nil
}

func main() {
	lambda.Start(handler)
}

func isAuthorized(authorizationHeader string) bool {
	token := strings.TrimSpace(strings.Replace(authorizationHeader, "Bearer", "", 1))
	return token == authToken
}

func saveScreen(pushNotificationToken string) (*Screen, error) {
	session := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	screen := &Screen{ID: uuid.NewString(), RegisteredDate: time.Now().UTC().Format("2006-01-02 15:04:05"), PushNotificationToken: pushNotificationToken}
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
