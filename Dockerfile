# 1. Frontend
FROM node:18-alpine AS web
WORKDIR /web
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ .
ARG VUE_APP_GOOGLE_CLIENT_ID=""
ARG VUE_APP_MICROSOFT_CLIENT_ID=""
ARG VUE_APP_POSTHOG_API_KEY=""
ENV VUE_APP_GOOGLE_CLIENT_ID=$VUE_APP_GOOGLE_CLIENT_ID \
    VUE_APP_MICROSOFT_CLIENT_ID=$VUE_APP_MICROSOFT_CLIENT_ID \
    VUE_APP_POSTHOG_API_KEY=$VUE_APP_POSTHOG_API_KEY
RUN npm run build

# 2. Server
FROM golang:1.25-alpine AS api
WORKDIR /src
RUN apk add --no-cache git
COPY server/ .
RUN go build -buildvcs=false -o /out/server .

# 3. Runtime
FROM alpine:3.23
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata wget && adduser -D -g '' appuser
COPY --from=api /out/server ./server
COPY --from=web /web/dist ./frontend/dist
RUN mkdir -p /app/logs && chown -R appuser:appuser /app
USER appuser
EXPOSE 3002
CMD ["./server", "-release=true"]
