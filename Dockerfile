FROM golang:1.24

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /img.imreally.gay

EXPOSE 3000


CMD ["/img.imreally.gay"]
