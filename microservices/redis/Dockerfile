FROM golang:1.16.9-alpine
RUN apk update
RUN apk add git
RUN cd /usr/local/go/src
RUN mkdir /usr/local/go/src/chat_room_go
ADD . /usr/local/go/src/chat_room_go
WORKDIR /usr/local/go/src/chat_room_go/microservices/redis
RUN go get ./...
RUN go build -o main .
EXPOSE 8083
CMD ["/usr/local/go/src/chat_room_go/microservices/redis/main"]