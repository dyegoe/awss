package search

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

func getConfig(profile, region string) (aws.Config, error) {
	return config.LoadDefaultConfig(context.TODO(), config.WithSharedConfigProfile(profile), config.WithRegion(region))
}

func split(input string) []string {
	return strings.Split(input, ",")
}
