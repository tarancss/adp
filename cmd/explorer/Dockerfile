FROM golang:1.13-alpine AS builder
LABEL maintainer="tarancss santiman625@gmail.com"

RUN apk add --update git gcc libc-dev

RUN go get -d -v -u github.com/lib/pq github.com/gorilla/mux
RUN go get -d -v -u github.com/streadway/amqp
RUN go get -d -v -u go.mongodb.org/mongo-driver/mongo go.mongodb.org/mongo-driver/bson
RUN go get -d -v -u github.com/prometheus/client_golang/prometheus/promhttp
RUN go get -d -v -u github.com/tarancss/ethcli

COPY . ./src/github.com/tarancss/adp

WORKDIR $GOPATH/src/github.com/tarancss/adp

RUN go build -o /go/bin/explorer -v cmd/explorer/main.go

FROM alpine:3.10

RUN apk add --update ca-certificates
COPY --from=builder /go/bin/explorer /explorer
COPY cmd/conf.json /conf.json

ENV ADP_MBCONN=amqp://guest:guest@adp_rmq:5672
ENV ADP_DBCONN=mongodb://adp_mongo

EXPOSE 3030

ENTRYPOINT ["/explorer"]
CMD ["-c", "/conf.json"]
