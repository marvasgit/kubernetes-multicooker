FROM golang:1.19-bullseye AS builder

COPY . /work
WORKDIR /work
RUN useradd multicooker
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o app/multicooker cmd/main.go

FROM  alpine

COPY  --from=builder /work/app/multicooker /usr/sbin/multicooker
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/
RUN chmod 777 /tmp/
USER multicooker

ENTRYPOINT ["/usr/sbin/multicooker", "-logtostderr"]
