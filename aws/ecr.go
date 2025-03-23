package aws

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecrpublic"
)

const (
	proxyEndpointScheme = "https://"
	programName         = "depot-go-ecr-example"
	ecrPublicName       = "public.ecr.aws"
	ecrPublicEndpoint   = proxyEndpointScheme + ecrPublicName
)

// Ref: https://github.com/awslabs/amazon-ecr-credential-helper/blob/abf9177720e144f3f8401f4df3dd2d2e9c2eaf5c/ecr-login/api/client.go#L40
var ecrPattern = regexp.MustCompile(`^(\d{12})\.dkr\.ecr(\-fips)?\.([a-zA-Z0-9][a-zA-Z0-9-_]*)\.(amazonaws\.com(\.cn)?|sc2s\.sgov\.gov|c2s\.ic\.gov|cloud\.adc-e\.uk|csp\.hci\.ic\.gov)$`)

type EcrCreds struct {
	Username string
	Password string
}

type ECRRegistry struct {
	URL  string
	Host string
	Path string
}

func ImageTag(registry *ECRRegistry, tag string) string {
	return fmt.Sprintf("%s/%s:%s", registry.Host, registry.Path, tag)
}

func ExtractEcrRegisty(input string) (*ECRRegistry, error) {
	input = strings.TrimPrefix(input, proxyEndpointScheme)

	serverURL, err := url.Parse(proxyEndpointScheme + input)
	if err != nil {
		return nil, err
	}

	path := strings.Trim(serverURL.Path, "/")
	pathParts := strings.Split(path, "/")
	if len(pathParts) < 2 {
		return nil, fmt.Errorf("invalid registry URL")
	}

	host := serverURL.Hostname()

	if host == ecrPublicName {
		return &ECRRegistry{
			URL:  ecrPublicEndpoint,
			Host: ecrPublicName,
			Path: path,
		}, nil
	}

	matches := ecrPattern.FindStringSubmatch(host)
	if len(matches) == 0 {
		return nil, fmt.Errorf(programName + " can only be used with Amazon Elastic Container Registry")
	} else if len(matches) < 3 {
		return nil, fmt.Errorf("%q is not a valid repository URI for Amazon Elastic Container Registry", input)
	}

	return &ECRRegistry{
		URL:  proxyEndpointScheme + matches[0],
		Host: host,
		Path: path,
	}, nil
}

func getEcrToken(ctx context.Context, sdkConfig *aws.Config, registry *ECRRegistry) string {
	if registry.Host == ecrPublicName {
		svc := ecrpublic.NewFromConfig(*sdkConfig)
		response, err := svc.GetAuthorizationToken(ctx, &ecrpublic.GetAuthorizationTokenInput{})
		if err != nil {
			log.Fatal(err)
		}

		return *response.AuthorizationData.AuthorizationToken
	}

	svc := ecr.NewFromConfig(*sdkConfig)
	response, err := svc.GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		log.Fatal(err)
	}

	return *response.AuthorizationData[0].AuthorizationToken
}

func GetEcrCreds(ctx context.Context, region string, registry *ECRRegistry) EcrCreds {
	sdkConfig, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(region),
	)
	if err != nil {
		log.Fatalf("Couldn't load default configuration. Have you set up your AWS account? %v", err)
	}

	token := getEcrToken(ctx, &sdkConfig, registry)
	data, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		log.Fatal("Failed to decode token:", err)
	}

	creds := strings.SplitN(string(data), ":", 2)

	return EcrCreds{
		Username: creds[0],
		Password: creds[1],
	}
}
