# syntax=docker/dockerfile:1
#
# Build from the repository root, because the Go module lives there:
#
#   docker build -f apis/todo-api/Dockerfile -t todo-api .
#   docker run --rm -p 8080:8080 -e TODO_API_KEYS=change-me todo-api

# ---- build stage ----
FROM golang:1.24-alpine AS build
WORKDIR /src

# The module has no third-party dependencies, so go.mod is all that is needed
# before the sources. Copying it first keeps this layer cached.
COPY go.mod ./
COPY apis/todo-api ./apis/todo-api

# A static, stripped binary (CGO off) can run on a minimal base image.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/todo-api ./apis/todo-api/cmd/server

# ---- runtime stage ----
# distroless/static has no shell or package manager, only the binary and CA
# certificates, and the :nonroot tag runs as an unprivileged user.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/todo-api /todo-api

ENV PORT=8080
EXPOSE 8080

ENTRYPOINT ["/todo-api"]
