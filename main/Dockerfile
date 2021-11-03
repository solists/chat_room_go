FROM golang:1.16.9-alpine
RUN apk update
RUN apk add git
RUN cd /usr/local/go/src
RUN mkdir /usr/local/go/src/chat_room_go
ADD . /usr/local/go/src/chat_room_go
WORKDIR /usr/local/go/src/chat_room_go/main
RUN pwd -P
RUN go get ./...
RUN go build -o main .
EXPOSE 8080
EXPOSE 8081
EXPOSE 8082
EXPOSE 8083
CMD ["/usr/local/go/src/chat_room_go/main/main"]