package main

import (
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

const (
	// Dynamo
	dynamoSessionCollectionName = "sessions"
	screensCollectionName       = "screens"
	dateTimeLayout              = "2006-01-02 15:04:05"

	// Alchemy
	alchemyGetNFTSAPIURL = "https://eth-mainnet.alchemyapi.io/nft/v2/lSHAClf-5A5mj37BJ82gVdRMHiXbX7LC/getNFTs"

	// Encryption
	secretKey = "MZI4MTGZNDKWNZA2"
)

type dynamoDBSessionTableRow struct {
	ScreenID string `json:"screen_id"`
	Token    string `json:"session_token"`
	Start    string `json:"start_time"`
	End      string `json:"end_time"`
}

type dynamoDBScreenTableRow struct {
	ID             string `json:"id"`
	RegisteredDate string `json:"registered_date"`
	WalletAddress  string `json:"wallet_address"`
}

type functionRequestBody struct {
	ScreenID      string `json:"screen_id"`
	SessionToken  string `json:"session_token"`
	WalletAddress string `json:"wallet_address"`
}

func getActiveSession(screenID string) (*dynamoDBSessionTableRow, error) {
	dbSession := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	svc := dynamodb.New(dbSession)
	result, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(dynamoSessionCollectionName),
		Key: map[string]*dynamodb.AttributeValue{
			"screen_id": {
				S: aws.String(screenID),
			},
		},
	})

	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, fmt.Errorf("there's no active session for screen %s", screenID)
	}

	_session := dynamoDBSessionTableRow{}
	err = dynamodbattribute.UnmarshalMap(result.Item, &_session)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(dateTimeLayout)

	if _session.Start <= now && _session.End >= now {
		return &_session, nil
	}

	return nil, nil
}

func updateScreen(screen *dynamoDBScreenTableRow) error {
	dbSession := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	svc := dynamodb.New(dbSession)
	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":w": {
				S: aws.String(screen.WalletAddress),
			},
		},
		TableName: aws.String(screensCollectionName),
		Key: map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(screen.ID),
			},
		},
		ReturnValues:     aws.String("UPDATED_NEW"),
		UpdateExpression: aws.String("set wallet_address = :w"),
	}

	_, err := svc.UpdateItem(input)
	if err != nil {
		return err
	}

	return nil
}

func isValidWalletAddress(walletAddress string) (bool, error) {
	url := fmt.Sprintf("%s?owner=%s", alchemyGetNFTSAPIURL, walletAddress)

	resp, err := http.Get(url)
	if err != nil {
		return false, err
	}

	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	return true, nil
}

func handler(request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
	var requestBody functionRequestBody
	err := json.Unmarshal([]byte(request.Body), &requestBody)
	if err != nil {
		log.Printf("Failed to read request body (%s): %s", request.Body, fmt.Errorf("%w", err))
		return events.APIGatewayProxyResponse{
			Body:       "Invalid request",
			StatusCode: 400,
		}, nil
	}

	if requestBody.ScreenID == "" {
		return events.APIGatewayProxyResponse{
			Body:       "Invalid screen ID",
			StatusCode: 400,
		}, nil
	}

	if requestBody.WalletAddress == "" {
		return events.APIGatewayProxyResponse{
			Body:       "Invalid wallet address",
			StatusCode: 400,
		}, nil
	}

	if requestBody.SessionToken == "" {
		return events.APIGatewayProxyResponse{
			Body:       "Invalid session code",
			StatusCode: 400,
		}, nil
	}

	activeSession, err := getActiveSession(requestBody.ScreenID)
	if err != nil {
		log.Printf("Failed to retrieve active session for screen %s: %s", requestBody.ScreenID, fmt.Errorf("%w", err))
		return events.APIGatewayProxyResponse{
			Body:       "Unexpected error",
			StatusCode: 500,
		}, nil
	}

	if activeSession == nil {
		log.Printf("There's no active session for screen %s", requestBody.ScreenID)
		return events.APIGatewayProxyResponse{
			Body:       "There's no active session for screen",
			StatusCode: 403,
		}, nil
	}

	expectedSessionToken, err := generateJWT(requestBody.ScreenID, activeSession.Token)
	if err != nil {
		log.Printf("Failed to encrpyt session token screenID(%s) token(%s): %s", requestBody.ScreenID, activeSession.Token, fmt.Errorf("%w", err))
		return events.APIGatewayProxyResponse{
			Body:       "Unexpected error",
			StatusCode: 500,
		}, nil
	}

	if expectedSessionToken != requestBody.SessionToken {
		log.Printf("Wrong session token provided for screenID(%s). Expected token: %s", requestBody.ScreenID, expectedSessionToken)
		return events.APIGatewayProxyResponse{
			Body:       "Session token code is invalid or expired",
			StatusCode: 403,
		}, nil
	}

	isValidWallet, err := isValidWalletAddress(requestBody.WalletAddress)
	if err != nil {
		log.Printf("Failed to validate wallet address %s for screen %s: %s", requestBody.WalletAddress, requestBody.ScreenID, fmt.Errorf("%w", err))
		return events.APIGatewayProxyResponse{
			Body:       "Unexpected error",
			StatusCode: 500,
		}, nil
	}

	if !isValidWallet {
		return events.APIGatewayProxyResponse{
			Body:       "Invalid wallet address",
			StatusCode: 400,
		}, nil
	}

	screen := dynamoDBScreenTableRow{
		ID:            requestBody.ScreenID,
		WalletAddress: requestBody.WalletAddress,
	}

	err = updateScreen(&screen)
	if err != nil {
		log.Printf("Failed to update wallet address %s for screen %s: %s", requestBody.WalletAddress, requestBody.ScreenID, fmt.Errorf("%w", err))
		return events.APIGatewayProxyResponse{
			Body:       "Unexpected error",
			StatusCode: 500,
		}, nil
	}

	return events.APIGatewayProxyResponse{
		Headers:    map[string]string{"Content-Type": "application/json"},
		StatusCode: 200,
	}, nil
}

func generateJWT(screenID, sessionToken string) (string, error) {
	var mySigningKey = []byte(secretKey)
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)

	claims["authorized"] = true
	claims["screen_id"] = screenID
	claims["session_token"] = sessionToken

	tokenString, err := token.SignedString(mySigningKey)

	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func main() {
	lambda.Start(handler)
}
