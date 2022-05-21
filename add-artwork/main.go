package main

import (
	"daiara-add-screen/db"
	"daiara-add-screen/infrastructure"
	"daiara-add-screen/storage"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/google/uuid"
	"github.com/grokify/go-awslambda"
	"io"
	"log"
	"time"
)

type ScreenInformation struct {
	ID           string `json:"screen_id"`
	SessionToken string `json:"session_token"`
}

type RequestBody struct {
	Title     *string  `json:"title"`
	Artist    *string  `json:"artist"`
	Price     *float64 `json:"price"`
	Currency  *string  `json:"currency"`
	Link      *string  `json:"link"`
	ShortText *string  `json:"short_text"`
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	if !infrastructure.IsAuthorized(request.Headers["authorization"]) {
		return events.APIGatewayProxyResponse{
			Body:       "Wrong authorization token provided",
			StatusCode: 403,
		}, nil
	}

	screenSessionToken := request.Headers["screen-session-token"]
	screenInformation, err := extractSessionToken(screenSessionToken)
	if err != nil || screenInformation == nil {
		return events.APIGatewayProxyResponse{
			Body:       "Wrong session token provided",
			StatusCode: 401,
		}, nil
	}

	fileContent, fileName, err := GetFileBytesFromRequest(request)
	if err != nil {
		return events.APIGatewayProxyResponse{
			Body:       "Failed to read file",
			StatusCode: 500,
		}, nil
	}

	s3FileID, err := storage.UploadArtwork(fileName, fileContent)
	if err != nil {
		log.Printf("Failed to upload file to s3: %s", fmt.Errorf("%w", err))
		return events.APIGatewayProxyResponse{
			Body:       "Failed to upload file",
			StatusCode: 500,
		}, nil
	}

	var requestBody *RequestBody
	err = json.Unmarshal([]byte(request.Body), requestBody)
	if err != nil {
		log.Printf("Failed to ready request body (%s): %s", request.Body, fmt.Errorf("%w", err))
		return events.APIGatewayProxyResponse{
			Body:       "Failed to ready request body",
			StatusCode: 500,
		}, nil
	}

	artwork := db.ArtWork{
		ID:          uuid.NewString(),
		ScreenID:    screenInformation.ID,
		CreatedDate: time.Now().UTC().Format("2006-01-02 15:04:05"),
		S3ID:        s3FileID,
		Title:       requestBody.Title,
		Artist:      requestBody.Artist,
		Price:       requestBody.Price,
		Currency:    requestBody.Currency,
		Link:        requestBody.Link,
		ShortText:   requestBody.ShortText,
	}

	err = db.SaveArtwork(artwork)
	if err != nil {
		log.Printf("Failed to persist artwork object: %s", fmt.Errorf("%w", err))
		return events.APIGatewayProxyResponse{
			Body:       "Failed to persist artwork object",
			StatusCode: 500,
		}, nil
	}

	// TODO: send push notification to app

	return events.APIGatewayProxyResponse{
		StatusCode: 201,
	}, nil
}

func GetFileBytesFromRequest(request events.APIGatewayProxyRequest) ([]byte, string, error) {
	reader, err := awslambda.NewReaderMultipart(request)
	if err != nil {
		log.Printf("Failed to instantiate multipart reader from request object: %s", fmt.Errorf("%w", err))
		return nil, "", err
	}

	part, err := reader.NextPart()
	if err != nil {
		log.Printf("Failed to get file from reader: %s", fmt.Errorf("%w", err))
		return nil, "", err
	}

	content, err := io.ReadAll(part)
	if err != nil {
		log.Printf("Failed to read file content: %s", fmt.Errorf("%w", err))
		return nil, "", err
	}

	return content, part.FileName(), nil
}

func extractSessionToken(screenSessionToken string) (*ScreenInformation, error) {
	if screenSessionToken == "" {
		return nil, nil
	}

	decodedScreenSessionToken, err := base64.StdEncoding.DecodeString(screenSessionToken)
	if err != nil {
		log.Printf("Failed to decode session tokent (%s): %s", screenSessionToken, fmt.Errorf("%w", err))
		return nil, err
	}

	var screenInformation ScreenInformation
	err = json.Unmarshal(decodedScreenSessionToken, &screenInformation)
	if err != nil {
		log.Printf("Failed to unmarshal screen information (%s): %s", decodedScreenSessionToken, fmt.Errorf("%w", err))
		return nil, err
	}

	return &screenInformation, nil
}

func main() {
	lambda.Start(handler)
}
