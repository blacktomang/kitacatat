# ───────────────────────── Build stage ─────────────────────────
# Tesseract + Leptonica DEV headers are needed because gosseract links against
# libtesseract via CGO. On Debian these headers land on the default include
# path, so no extra CGO_CPPFLAGS are required.
FROM golang:1.24-bookworm AS build

RUN apt-get update && apt-get install -y --no-install-recommends \
        libtesseract-dev \
        libleptonica-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=1
RUN go build -trimpath -ldflags="-s -w" -o /bot ./cmd/bot

# ──────────────────────── Runtime stage ────────────────────────
# Slim image that STILL needs the Tesseract runtime libs plus the Indonesian
# and English trained data so OCR runs inside the container.
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
        tesseract-ocr \
        tesseract-ocr-ind \
        tesseract-ocr-eng \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Debian bookworm ships Tesseract 5; trained data lives here.
ENV TESSDATA_PREFIX=/usr/share/tesseract-ocr/5/tessdata

COPY --from=build /bot /usr/local/bin/bot

# Run as a non-root user.
RUN useradd --create-home --uid 10001 appuser
USER appuser

ENTRYPOINT ["/usr/local/bin/bot"]
