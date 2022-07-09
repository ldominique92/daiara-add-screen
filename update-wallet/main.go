package main

import (
	"crypto/aes"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

	// Alchemy
	alchemyGetNFTSAPIURL = "https://eth-mainnet.alchemyapi.io/nft/v2/lSHAClf-5A5mj37BJ82gVdRMHiXbX7LC/getNFTs"

	// Encryption
	encryptionKey = "Daiara4K"
)

type dynamoDBSessionTableRow struct {
	ScreenID string `json:"screen_id"`
	Token    string `json:"session_token"`
	Start    string `json:"start"`
	End      string `json:"end"`
}

type dynamoDBScreenTableRow struct {
	ID             string `json:"id"`
	RegisteredDate string `json:"registered_date"`
	WalletAddress  string `json:"wallet_address"`
}

type functionRequestBody struct {
	SessionToken  string `json:"session_token"`
	WalletAddress string `json:"wallet_address"`
}

type SessionToken struct {
	ScreenID            string `json:"screen_id"`
	AuthenticationToken string `json:"authentication_token"`
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
		return nil, nil
	}

	_session := dynamoDBSessionTableRow{}
	err = dynamodbattribute.UnmarshalMap(result.Item, &_session)
	if err != nil {
		return nil, err
	}

	now := time.Now().String()

	if _session.Start <= now && _session.Start >= now {
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

func decryptSessionToken(sessionToken string) (string, error) {
	key := []byte(encryptionKey)
	ciphertext, _ := hex.DecodeString(sessionToken)

	c, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	pt := make([]byte, len(ciphertext))
	c.Decrypt(pt, ciphertext)

	return string(pt[:]), nil
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

	sessionTokenJson, err := decryptSessionToken(requestBody.SessionToken)
	if err != nil {
		log.Printf("Invalid session token %s: %s", request.Body, fmt.Errorf("%w", err))
		return events.APIGatewayProxyResponse{
			Body:       "Invalid request",
			StatusCode: 400,
		}, nil
	}

	var sessionToken SessionToken
	err = json.Unmarshal([]byte(sessionTokenJson), &sessionToken)
	if err != nil {
		log.Printf("Failed to unmarshal request body (%s): %s", request.Body, fmt.Errorf("%w", err))
		return events.APIGatewayProxyResponse{
			Body:       "Invalid request",
			StatusCode: 400,
		}, nil
	}

	if sessionToken.ScreenID == "" {
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

	if sessionToken.AuthenticationToken == "" {
		return events.APIGatewayProxyResponse{
			Body:       "Invalid two factor code",
			StatusCode: 400,
		}, nil
	}

	activeSession, err := getActiveSession(sessionToken.ScreenID)
	if err != nil {
		log.Printf("Failed to retrieve active session for screen %s: %s", sessionToken.ScreenID, fmt.Errorf("%w", err))
		return events.APIGatewayProxyResponse{
			Body:       "Failed to retrieve active session for screen",
			StatusCode: 500,
		}, nil
	}

	if activeSession == nil {
		log.Printf("There's no active session for screen %s", sessionToken.ScreenID)
		return events.APIGatewayProxyResponse{
			Body:       "There's no active session for screen",
			StatusCode: 403,
		}, nil
	}

	if activeSession.Token != sessionToken.AuthenticationToken {
		log.Printf("Two factor authentication code does not match for screen %s", sessionToken.ScreenID)
		return events.APIGatewayProxyResponse{
			Body:       "Two factor authentication code does not match",
			StatusCode: 403,
		}, nil
	}

	isValidWallet, err := isValidWalletAddress(requestBody.WalletAddress)
	if err != nil {
		log.Printf("Failed to validate wallet address %s for screen %s: %s", requestBody.WalletAddress, sessionToken.ScreenID, fmt.Errorf("%w", err))
		return events.APIGatewayProxyResponse{
			Body:       "Failed to retrieve active session for screen",
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
		ID:            sessionToken.ScreenID,
		WalletAddress: requestBody.WalletAddress,
	}

	err = updateScreen(&screen)

	return events.APIGatewayProxyResponse{
		Headers:    map[string]string{"Content-Type": "application/json"},
		StatusCode: 200,
	}, nil
}

func main() {
	lambda.Start(handler)
}
