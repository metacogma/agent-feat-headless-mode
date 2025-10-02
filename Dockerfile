# Stage 1: Build Go Binary
FROM golang:1.23.6-alpine3.21 AS build-stage
WORKDIR /local-agent
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY config ./config
COPY errors ./errors
COPY http ./http
COPY initialization ./initialization
COPY logger ./logger
COPY models ./models
COPY services ./services
COPY utils ./utils
RUN CGO_ENABLED=0 GOOS=linux go build -o ./agent ./cmd/agent

# Stage 2: Install Playwright
FROM mcr.microsoft.com/playwright:v1.47.2-noble AS playwright-stage
WORKDIR /local-agent/
COPY executions/package.json executions/package-lock.json ./
RUN npm ci

# Final Stage: Minimal Runtime Image
FROM mcr.microsoft.com/playwright:v1.47.2-noble
WORKDIR /local-agent

# Copy only essential artifacts
COPY --from=build-stage /local-agent/agent ./agent
COPY --from=playwright-stage /local-agent ./executions

# Reduce layer count and set permissions
RUN chmod +x ./agent
RUN cd ./executions && mkdir -p tests
COPY ./executions/tests/fixture.ts ./executions/tests/fixture.ts
COPY ./executions/tests/edc.ts ./executions/tests/edc.ts
COPY ./executions/playwright.config.js ./executions/playwright.config.js


EXPOSE 5656

# Set the entrypoint to the Go application
ENTRYPOINT ["./agent", "start",  "--background"]
