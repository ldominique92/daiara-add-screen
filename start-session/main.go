package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"log"
	"strings"
	"time"
)

const (
	// Dynamo
	dynamoSessionCollectionName = "sessions"
	dynamoScreensCollectionName = "screens"
	dateTimeLayout              = "2006-01-02 15:04:05"

	// Encryption
	secretKey = "MZI4MTGZNDKWNZA2"

	// Authorization
	authToken = "c2VtcmFib25hb3ZhbGVuYWRh="
)

type dynamoDBSessionTableRow struct {
	ScreenID string `json:"screen_id"`
	Token    string `json:"session_token"`
	Start    string `json:"start"`
	End      string `json:"end"`
}

type functionResponseBody struct {
	SessionToken string `json:"session_token"`
}

func isAuthorized(authorizationHeader string) bool {
	token := strings.TrimSpace(strings.Replace(authorizationHeader, "Bearer", "", 1))
	return token == authToken
}

func screenExists(screenID string) (bool, error) {
	dbSession := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	svc := dynamodb.New(dbSession)
	result, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(dynamoScreensCollectionName),
		Key: map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(screenID),
			},
		},
	})

	if err != nil {
		return false, err
	}

	return result.Item != nil, nil
}

func refreshSession(screenID string, token string) error {
	dbSession := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	svc := dynamodb.New(dbSession)
	getItemRow, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(dynamoSessionCollectionName),
		Key: map[string]*dynamodb.AttributeValue{
			"screen_id": {
				S: aws.String(screenID),
			},
		},
	})

	if err != nil {
		return err
	}

	_session := dynamoDBSessionTableRow{
		ScreenID: screenID,
		Token:    token,
		Start:    time.Now().UTC().Format(dateTimeLayout),
		End:      time.Now().Add(time.Minute * 5).UTC().Format(dateTimeLayout),
	}

	if getItemRow.Item == nil {
		insertItemRow, err := dynamodbattribute.MarshalMap(_session)
		if err != nil {
			return err
		}
		input := &dynamodb.PutItemInput{
			Item:      insertItemRow,
			TableName: aws.String(dynamoSessionCollectionName),
		}

		_, err = svc.PutItem(input)
		return err
	} else {
		input := &dynamodb.UpdateItemInput{
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":t": {
					S: aws.String(_session.Token),
				},
				":s": {
					S: aws.String(_session.Start),
				},
				":e": {
					S: aws.String(_session.End),
				},
			},
			TableName: aws.String(dynamoSessionCollectionName),
			Key: map[string]*dynamodb.AttributeValue{
				"screen_id": {
					S: aws.String(screenID),
				},
			},
			ReturnValues:     aws.String("UPDATED_NEW"),
			UpdateExpression: aws.String("set session_token = :t, start = :s, end = :e"),
		}

		_, err := svc.UpdateItem(input)
		if err != nil {
			return err
		}
	}

	return nil
}

func generateSessionToken(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func handler(request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
	if !isAuthorized(request.Headers["authorization"]) {
		return events.APIGatewayProxyResponse{
			Body:       "Wrong authorization token provided",
			StatusCode: 403,
		}, nil
	}

	screenID := request.QueryStringParameters["screen_id"]
	if screenID == "" {
		return events.APIGatewayProxyResponse{
			Body:       "Invalid screen ID",
			StatusCode: 400,
		}, nil
	}

	screenIsRegistered, err := screenExists(screenID)
	if err != nil {
		log.Printf("Failed to retrieve screen %s: %s", screenID, fmt.Errorf("%w", err))
		return events.APIGatewayProxyResponse{
			Body:       "Failed to retrieve screen",
			StatusCode: 500,
		}, nil
	}

	if !screenIsRegistered {
		log.Printf("Screen is not registered %s: %s", screenID, fmt.Errorf("%w", err))
		return events.APIGatewayProxyResponse{
			Body:       "Invalid device code",
			StatusCode: 404,
		}, nil
	}

	token, err := generateSessionToken(uuid.NewString())
	if err != nil {
		log.Printf("Failed to generate token for screen %s: %s", screenID, fmt.Errorf("%w", err))
		return events.APIGatewayProxyResponse{
			Body:       "Unexpected error",
			StatusCode: 500,
		}, nil
	}

	err = refreshSession(screenID, token)
	if err != nil {
		log.Printf("Failed to refresh session for screen %s: %s", screenID, fmt.Errorf("%w", err))
		return events.APIGatewayProxyResponse{
			Body:       "Failed to refresh session for screen",
			StatusCode: 500,
		}, nil
	}

	encryptedSessionToken, err := generateJWT(screenID, token)
	if err != nil {
		log.Printf("Failed to encrpyt session token screenID(%s) token(%s): %s", screenID, token, fmt.Errorf("%w", err))
		return events.APIGatewayProxyResponse{
			Body:       "Unexpected error",
			StatusCode: 500,
		}, nil
	}

	responseBody := functionResponseBody{SessionToken: encryptedSessionToken}
	responseBodyJson, err := json.Marshal(responseBody)
	if err != nil {
		log.Printf("Failed to marshal response body (%s): %s", responseBody, fmt.Errorf("%w", err))
		return events.APIGatewayProxyResponse{
			Body:       "Unexpected error",
			StatusCode: 500,
		}, nil
	}

	return events.APIGatewayProxyResponse{
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(responseBodyJson),
		StatusCode: 201,
	}, nil
}

func generateJWT(email, role string) (string, error) {
	var mySigningKey = []byte(secretKey)
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)

	claims["authorized"] = true
	claims["screen_id"] = email
	claims["session_token"] = role
	claims["exp"] = time.Now().Add(time.Minute * 5).Unix()

	tokenString, err := token.SignedString(mySigningKey)

	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func main() {
	lambda.Start(handler)
}
