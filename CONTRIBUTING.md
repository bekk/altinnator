# Contributing guides
## Getting started

Install [mise](https://mise.jdx.dev/getting-started.html) and [Docker Desktop](https://www.docker.com/products/docker-desktop/)
(recommended for running the local Kind cluster), then install the repository's pinned development tools:

```bash
mise install
```

The Go version is read from `go.mod`; `mise.toml` pins the other tools. Besides the tools the tasks need (`kubectl`, `kind`,
`kustomize`, `controller-gen`, `chainsaw`, `golangci-lint` and `setup-envtest`), `mise install` also sets up helpful
development tools like `kubectx` and `k9s`. Local development and end-to-end testing use a
Kind cluster named `altinnator` with the Kubernetes context `kind-altinnator`. The commands below create and manage
that cluster. You can override the cluster name with `KIND_CLUSTER_NAME` and the context with `KUBECONTEXT`.

### Use the mise-installed Go in your JetBrains IDE

`mise install` also creates a gitignored `.goroot` symlink in the project root, pointing to the mise-installed Go SDK.
To make GoLand/IntelliJ use the same Go version as the project, do the following once:

1. In the IDE, go to **Settings → Go → GOROOT → Add SDK → Local…** and select the `.goroot` folder in the project root.
2. Select that SDK as the project's GOROOT.

After a Go version bump, run `mise install` again; the symlink is refreshed and the IDE follows it (re-index if prompted).

## Run Altinnator locally

### Run on your host machine

Create the local cluster and install the Altinnator CRDs:

```bash
mise run cluster:local
```

Start the controller from a JetBrains IDE with the **Altinnator** run configuration. It automatically runs the 
**Setup** configuration first, which checks the local context and installs the generated CRDs, and loads environment 
variables from `config/manager/base/.env`. Stop it with the IDE's stop button.

If you want to start the controller from a terminal on your host, run:

```bash
mise run dev:run
```

This command generates the required code and CRDs before starting the controller. It refuses to run if Altinnator is
already deployed in the cluster or if your current Kubernetes context is not the configured local Kind context. Stop
the process with `Ctrl-C`.

### Run in the local cluster

To build and deploy Altinnator into the local Kind cluster instead of running it on your host:

```bash
mise run cluster:local
mise run deploy
```

The deploy task builds the image, loads it into Kind, applies the controller resources, and waits for the deployment
to become ready. To remove the deployment, run `mise run undeploy`. To delete the local cluster and its contents, run
`mise run cluster:clean`.

## Running tests

Unit and integration tests use Go's test runner with [envtest](https://book.kubebuilder.io/reference/envtest) to run
a local Kubernetes API server. Run them with:

```bash
mise run test
```

The first run downloads the envtest binaries for the Kubernetes version used by this project.

End-to-end tests use [Chainsaw](https://kyverno.github.io/chainsaw/). They need the local Kind cluster and the
controller running, either on your host (`mise run dev:run`) or deployed into the cluster (`mise run deploy`). Run
the full suite with:

```bash
mise run chainsaw:test-all
```

Run one test directory with:

```bash
mise run chainsaw:test-single test/chainsaw/altinnresource/dummy-test
```

## Git conventions

### Commit strategy

- Separate features, bug fixes, and refactorings into different commits.
- Each commit should represent one logical change that compiles and passes tests on its own.
- Do not mix refactoring or unrelated changes with feature changes in the same commit.
- Keep changes and their corresponding tests in the same commit.

### Commit messages

- Limit the subject line to 50 characters.
- Capitalize the subject line.
- Use the imperative mood in the subject line.
- Do not end the subject line with a period.
- Separate the subject from the body with a blank line.
- Wrap the body at 72 characters.
- Use the body to explain what and why, not how.

Example:
```
Add secretRef support for Entra ID config

Previously, Entra ID credentials could only be configured via an
inline AzureAdApplication or by referencing an existing one. This
adds a third path: sourcing credentials directly from existing
Kubernetes secrets, mirroring the existing Maskinporten secretRef
behaviour.
```

## Managing dependencies and patching vulnerabilities

### Managing Kubernetes, Istio and CertManager versions
These versions are defined in the top of the `Makefile`, and are used when running locally and when testing in CI/CD.
Versions should match the versions running in your production Kubernetes cluster.

### Managing helper tools
Versions for helper tools like Kustomize, Chainsaw, Kubectl etc. are defined in the top of the `Makefile`. Versions
should be updated either regularly, or as is required when updating other dependencies (such as upgrading Chainsaw when
updating Go version).

### Updating Go version
#### Updating Go version
1. Find a desired stable Go version [on the go.dev website](https://go.dev/dl/#stable)
2. Investigate changes between current and desired version, for instance through [the go release history page](https://go.dev/doc/devel/release),
   or by googling "Go <old_version> to <new_version> migration guide"
3. Ensure there exists a compatible Chainsaw version for the chosen Go version, for instance by checking [the latest Chainsaw releases](https://github.com/kyverno/chainsaw/releases).
   The version of Chainsaw must support the same major and minor version as your chosen Go version. Patch versions may
   typically be updated without updating Chainsaw. If necessary, update `CHAINSAW_VERSION` in the `Makefile`
4. Update Go version in `go.mod`
5. Perform dependency updates for direct dependencies with `go get -u ./...`
6. Revert any changes made to `github.com/nais/liberator`. Liberator must be updated separately.
7. Run `go mod tidy`

#### Updating Golang base image version
1. Find a Golang docker image [on Docker Hub](https://hub.docker.com/_/golang/tags) corresponding to the Go version you
   updated to previously. As of August 2026, this is called "1.26.6-alpine3.24".
2. Copy the "index digest" (top left), and paste into all relevant dockerfiles:
   3. hack/mock_controller/Dockerfile
   4. Dockerfile
   5. Dockerfile.goreleaser

#### Verify linting and tests
1. Run `make lint`. This ensures both that the linting is correct, and that the version of `golangci-lint` supports the
   new Go version. If the version of `golangci-lint` does not support the new version of Go, update
   `GOLANGCI_LINT_VERSION` in the `Makefile`. If there are linting errors after updating the linter version, solve these
   either by running `make lint-fix` or resolving manually
2. Follow guides above to run tests (with `make test`) and e2e-tests (with `make chainsaw-test-all`), and ensure all
   tests pass
