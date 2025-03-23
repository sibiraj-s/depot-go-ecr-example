# Depot Go ECR Example

This is an example for cloning a repository, building a Docker image using [Depot] and pushing it to Amazon ECR using [depot-go](https://github.com/depot/depot-go) SDK.

## Prerequisites

- [Depot Account][depot]
- AWS CLI
- AWS ECR Registry
- Golang

### Prepare

Before you start, configure the AWS CLI with the following command:

```bash
aws configure
```

or export the following environment variables to the shell that aws cli uses to authenticate.

```bash
export AWS_ACCESS_KEY_ID=your_access_key_id
export AWS_SECRET_ACCESS_KEY=your_secret_access_key
export AWS_REGION=your_region
```

Then, create a depot account and get the depot token. Refer to [Depot CLI Authentication](https://depot.dev/docs/cli/authentication#user-access-tokens) for more details.

and add the variables to the `.env` file. See `.env.example` for more details.

```bash
DEPOT_TOKEN=your_depot_token
DEPOT_PROJECT_ID=your_depot_project_id
```

### Run

To run the example, you can use the following command:

```bash
go run .
```

This will do the following:

1. Clone the repository to `tmp` directory
2. Bring up a depot machine with the given architecture
3. Build the Docker image in the depot machine
4. Push the Docker image to the provided ECR registry
5. Remove the clone directory

### Credits

- Modified from [depot-go example](https://github.com/depot/depot-go/blob/bd4e352f0bdbc600d80f5e73753f6ca026e8851d/examples/build/main.go)

[depot]: https://depot.dev
