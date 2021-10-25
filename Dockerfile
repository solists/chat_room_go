FROM golang:1.16.9-alpine
RUN apk update
RUN apk add git
RUN mkdir /app
ADD . /app
WORKDIR /app
RUN go get ./...
RUN go build -o main .
EXPOSE 8080
EXPOSE 8081
EXPOSE 8082
EXPOSE 8083
CMD ["/app/main"]