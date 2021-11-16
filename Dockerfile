FROM golang:1.17-alpine as builder

RUN apk --no-cache --update add git ca-certificates tzdata
RUN update-ca-certificates
RUN adduser -D -g '' app

COPY . /src

WORKDIR /src

ENV GO111MODULE=on

RUN go get -v -d

RUN CGO_ENABLED=0 GOOS=linux go build main.go
RUN mv main secret-transformer

FROM scratch

COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /src/secret-transformer /bin/secret-transformer

USER app

ENTRYPOINT ["/bin/secret-transformer"]
