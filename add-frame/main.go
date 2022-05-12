package main

import (
	"daiara-add-screen/infrastructure"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func handler(request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
	if !infrastructure.IsAuthorized(request.Headers["authorization"]) {
		return events.APIGatewayProxyResponse{
			Body:       "Wrong authorization token provided",
			StatusCode: 403,
		}, nil
	}

	return events.APIGatewayProxyResponse{
		Body:       fmt.Sprintf("Hello, Daiara!"),
		StatusCode: 200,
	}, nil
}

func main() {
	lambda.Start(handler)
}
