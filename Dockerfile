FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /out/local-clipboard .

FROM alpine:3.20
COPY --from=build /out/local-clipboard /usr/local/bin/local-clipboard
ENTRYPOINT ["local-clipboard"]
