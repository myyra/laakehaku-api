FROM golang:1.20 as builder

COPY go.sum go.mod /api/

WORKDIR /api

RUN go mod download

COPY . /api/

RUN CGO_ENABLED=0 go build -mod readonly -o /usr/bin/server

FROM scratch

COPY --from=builder /usr/bin/server /usr/bin/server
COPY --from=builder /etc/ssl/certs/ /etc/ssl/certs

ENTRYPOINT [ "/usr/bin/server" ]
