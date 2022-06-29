package ec2

import (
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
)

var client *imds.Client

func init() {
	client = imds.New(imds.Options{})
}

func IsRunningOn() bool {
	_, err := client.GetMetadata(context.TODO(), &imds.GetMetadataInput{
		Path: "instance-id",
	})

	return err == nil
}

func Hostname() (string, error) {

	// Using 'internal-hostname' as the 'public-hostname' may not exist!
	ih, err := client.GetMetadata(context.TODO(),
		&imds.GetMetadataInput{
			Path: "internal-hostname"})

	if err != nil {
		return "", err
	}

	b, err := io.ReadAll(ih.Content)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
