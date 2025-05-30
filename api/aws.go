package api

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
	fiberadapter "github.com/mrusme/overpush/fiberadapter"
)

func (api *API) AWSLambdaHandler(
	ctx context.Context,
	req events.APIGatewayProxyRequest,
) (events.APIGatewayProxyResponse, error) {
	var fiberLambda *fiberadapter.FiberLambda
	fiberLambda = fiberadapter.New(api.app)
	return fiberLambda.ProxyWithContext(ctx, req)
}
