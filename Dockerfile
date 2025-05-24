FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev git

WORKDIR /app

COPY go.mod go.sum ./

RUN go get github.com/google/uuid

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o shallowseek

FROM ubuntu:22.04

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    poppler-utils \
    antiword \
    docx2txt \
    ca-certificates && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/shallowseek /app/
COPY --from=builder /app/static /app/static
COPY --from=builder /app/templates /app/templates
COPY --from=builder /app/data /app/data

RUN mkdir -p /app/data/documents /app/temp_uploads && \
    chmod -R 755 /app/data /app/temp_uploads /app/static /app/templates && \
    chmod +x /app/shallowseek && \
    chown -R 1000:1000 /app/data /app/temp_uploads /app/static /app/templates

USER 1000

EXPOSE 8080
CMD ["/app/shallowseek"]
