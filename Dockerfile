FROM golang as builder
WORKDIR /src/service
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags "-linkmode external -extldflags '-static' -X main.Version=`git tag --sort=-version:refname | head -n 1`" -o rms-notes rms-notes.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
RUN mkdir /app
WORKDIR /app
COPY --from=builder /src/service/rms-notes .
COPY --from=builder /src/service/configs/rms-notes.json /etc/rms/
CMD ["./rms-notes"]