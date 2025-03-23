package main

import (
	"context"

	"github.com/docker/docker/api/types/registry"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sibiraj-s/depot-go-ecr-example/aws"
)

type buildkitAuthServer struct {
	ecrRegistry *aws.ECRRegistry
	username    string
	password    string
}

func NewBuildkitAuthProvider(creds aws.EcrCreds, ecrRegistry *aws.ECRRegistry) session.Attachable {
	return &buildkitAuthServer{
		ecrRegistry: ecrRegistry,
		username:    creds.Username,
		password:    creds.Password,
	}
}

func (as *buildkitAuthServer) Register(server *grpc.Server) {
	auth.RegisterAuthServer(server, as)
}

func (as *buildkitAuthServer) FetchToken(ctx context.Context, req *auth.FetchTokenRequest) (*auth.FetchTokenResponse, error) {
	return nil, status.Errorf(codes.Unavailable, "client side tokens disabled")
}

func (as *buildkitAuthServer) GetTokenAuthority(ctx context.Context, req *auth.GetTokenAuthorityRequest) (*auth.GetTokenAuthorityResponse, error) {
	return nil, status.Errorf(codes.Unavailable, "client side tokens disabled")
}

func (as *buildkitAuthServer) VerifyTokenAuthority(ctx context.Context, req *auth.VerifyTokenAuthorityRequest) (*auth.VerifyTokenAuthorityResponse, error) {
	return nil, status.Errorf(codes.Unavailable, "client side tokens disabled")
}

func registryAuthConfig(ecrRegistry *aws.ECRRegistry, username, password string) registry.AuthConfig {
	return registry.AuthConfig{
		Username:      username,
		Password:      password,
		ServerAddress: ecrRegistry.URL,
	}
}

func authConfigs(ecrRegistry *aws.ECRRegistry, username, password string) map[string]registry.AuthConfig {
	return map[string]registry.AuthConfig{
		ecrRegistry.Host: registryAuthConfig(ecrRegistry, username, password),
	}
}

func (as *buildkitAuthServer) Credentials(ctx context.Context, req *auth.CredentialsRequest) (*auth.CredentialsResponse, error) {
	auths := authConfigs(as.ecrRegistry, as.username, as.password)
	res := &auth.CredentialsResponse{}

	if a, ok := auths[req.Host]; ok {
		res.Username = a.Username
		res.Secret = a.Password
	}

	return res, nil
}
