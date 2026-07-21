FROM node:25-alpine AS web-build
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
COPY --from=web-build /src/web/dist/ ./internal/app/web/dist/
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/health-diary ./cmd/server
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/health-diary-migrate ./cmd/migrate

FROM alpine:3.22
RUN addgroup -S app && adduser -S -G app app
COPY --from=build /out/health-diary /app/health-diary
COPY --from=build /out/health-diary-migrate /app/health-diary-migrate
USER app
EXPOSE 8080 9090
ENTRYPOINT ["/app/health-diary"]
