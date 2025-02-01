FROM golang:1.23-alpine

WORKDIR /app

COPY . .

#RUN go mod download
#
#RUN go build -o main .
#
#EXPOSE 8080
#
#CMD ["./main"]

RUN go install github.com/air-verse/air@latest

RUN go mod download

CMD ["air", "-c", ".air.toml"]