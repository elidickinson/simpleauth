FROM golang:1 AS build
WORKDIR /src
COPY go.* ./
COPY pkg ./pkg/
COPY cmd ./cmd/
RUN go get ./...
RUN CGO_ENABLED=0 GOOS=linux go install ./...

FROM alpine AS runtime
WORKDIR /target
COPY web web
COPY --from=build /go/bin/ .

FROM scratch
COPY --from=runtime /target /
ENTRYPOINT ["/simpleauth"]
