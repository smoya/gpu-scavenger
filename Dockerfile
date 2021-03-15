FROM golang:alpine

RUN apk --no-cache upgrade && apk add --no-cache make

# Add dependencies first to make use of docker cache
COPY go.mod .
COPY go.sum .
# Unset GOPATH env in favour of .mod
ENV GOPATH=""
RUN go mod download

COPY . .
RUN make build
ENTRYPOINT ["./bin/gpu-scavenger"]