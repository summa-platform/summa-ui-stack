FROM golang:alpine as backend-toolchain

RUN apk add --no-cache git openssl
RUN go get -u github.com/golang/dep/cmd/dep


FROM backend-toolchain as backend-dependencies

WORKDIR /go/src/summa-ui-stack

COPY Gopkg.toml /go/src/summa-ui-stack
COPY Gopkg.lock /go/src/summa-ui-stack

RUN dep ensure --vendor-only


FROM backend-dependencies as backend-build

WORKDIR /go/src/summa-ui-stack

COPY . /go/src/summa-ui-stack

RUN go build
