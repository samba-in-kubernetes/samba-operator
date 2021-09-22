# Build the manager binary
FROM docker.io/golang:1.17 as builder
ARG GIT_VERSION="(unset)"
ARG COMMIT_ID="(unset)"

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY internal/ internal/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on \
    go build -a \
    -ldflags "-X main.Version=${GIT_VERSION} -X main.CommitID=${COMMIT_ID}" \
    -o manager main.go

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

WORKDIR /
COPY --from=builder /workspace/manager .

ENTRYPOINT ["/manager"]
