############################
# STEP 1 build executable binary
############################
FROM golang:alpine as builder

# Install git + SSL ca certificates.
# Git is required for fetching the dependencies.
# Ca-certificates is required to call HTTPS endpoints.
RUN apk update && apk add --no-cache git ca-certificates tzdata && update-ca-certificates

# Create appuser
RUN adduser -D -g '' appuser

COPY . $GOPATH/src/gonflux/
WORKDIR $GOPATH/src/gonflux/

# Fetch dependencies.

# Using go get.
RUN go get -d -v

# Using go mod.
# RUN go mod download

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -a -installsuffix cgo -o /go/bin/gonflux

############################
# STEP 2 build a small image
############################
FROM scratch

# Import from builder.
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd

# Copy our static executable
COPY --from=builder /go/bin/gonflux /go/bin/gonflux

# Use an unprivileged user.
USER appuser

# Run the gonflux binary, push the netflow into influxdb over udp
ENTRYPOINT ["/go/bin/gonflux"]

