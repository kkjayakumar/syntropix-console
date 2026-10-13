# syntax=docker/dockerfile:1

FROM node:22-alpine AS web
WORKDIR /src/web
ARG NEXT_PUBLIC_SPOTTING_PUBLIC_KEY=""
ARG NEXT_PUBLIC_SPOTTING_API_URL="http://localhost:3000"
ARG NEXT_PUBLIC_SPOTTING_DASHBOARD_URL="http://127.0.0.1:3001"
ARG NEXT_PUBLIC_AEGISOPS_ENV="development"
ARG NEXT_PUBLIC_AEGISOPS_ALLOW_DOGFOOD="false"
ENV NEXT_PUBLIC_SPOTTING_PUBLIC_KEY=$NEXT_PUBLIC_SPOTTING_PUBLIC_KEY
ENV NEXT_PUBLIC_SPOTTING_API_URL=$NEXT_PUBLIC_SPOTTING_API_URL
ENV NEXT_PUBLIC_SPOTTING_DASHBOARD_URL=$NEXT_PUBLIC_SPOTTING_DASHBOARD_URL
ENV NEXT_PUBLIC_AEGISOPS_ENV=$NEXT_PUBLIC_AEGISOPS_ENV
ENV NEXT_PUBLIC_AEGISOPS_ALLOW_DOGFOOD=$NEXT_PUBLIC_AEGISOPS_ALLOW_DOGFOOD
COPY web/package.json web/package-lock.json ./
COPY web/ ./
RUN npm ci && npm run build

FROM golang:1.25-alpine AS gobuild
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/embed/out ./embed/out
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/aegisops ./cmd/aegisops && \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/aegisops-mcp ./cmd/aegisops-mcp

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=gobuild /out/aegisops /usr/local/bin/aegisops
COPY --from=gobuild /out/aegisops-mcp /usr/local/bin/aegisops-mcp
COPY migrations /migrations
ENV PORT=5001
ENV MIGRATIONS_DIR=/migrations
EXPOSE 5001
USER nobody
ENTRYPOINT ["/usr/local/bin/aegisops"]
