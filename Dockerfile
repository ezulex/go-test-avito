FROM golang:1.16

WORKDIR /app

COPY . .

RUN go get -d -v ./...

RUN go build -o avito .

EXPOSE 8000

CMD ["./avito"]