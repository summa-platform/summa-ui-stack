FROM node:8 as frontend-prebuild
RUN npm install -g aurelia-cli yarn
WORKDIR /src
COPY frontend/package.json /src/package.json
COPY frontend/yarn.lock /src/yarn.lock
RUN yarn


FROM frontend-prebuild as frontend-build
WORKDIR /src
COPY frontend/ /src
RUN mkdir /deploy
RUN au deploy --env stage --out /deploy


# FROM nginx:latest as frontend-nginx
# COPY nginx.conf /etc/nginx/conf.d/default.conf
# COPY --from=frontend-build /deploy /webroot


FROM golang:alpine as backend-toolchain

RUN apk add --no-cache git openssl
RUN go get -u github.com/golang/dep/cmd/dep


FROM backend-toolchain as backend-dependencies

WORKDIR /go/src/summa-ui-stack-backend

COPY backend/Gopkg.toml /go/src/summa-ui-stack-backend
COPY backend/Gopkg.lock /go/src/summa-ui-stack-backend

RUN dep ensure --vendor-only

COPY backend/genkeys.sh /go/src/summa-ui-stack-backend
RUN /go/src/summa-ui-stack-backend/genkeys.sh


FROM backend-dependencies as backend-build

WORKDIR /go/src/summa-ui-stack-backend

COPY backend/ /go/src/summa-ui-stack-backend

RUN go build


# final output image

FROM alpine

WORKDIR /app
COPY backend/static/login /app/static/login
COPY --from=backend-build /go/src/summa-ui-stack-backend/keys /app
COPY --from=frontend-build /deploy /app/static
COPY --from=backend-build /go/src/summa-ui-stack-backend/summa-ui-stack-backend /app
COPY config.yaml /app

CMD ["/app/summa-ui-stack-backend"]
