package main

import (
	"daiara-add-screen/db"
	"daiara-add-screen/infrastructure"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"log"
)

func handler(request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
	if !infrastructure.IsAuthorized(request.Headers["authorization"]) {
		return events.APIGatewayProxyResponse{
			Body:       "Wrong authorization token provided",
			StatusCode: 403,
		}, nil
	}

	screen, err := db.SaveScreen()
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
