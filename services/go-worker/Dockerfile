FROM golang:1.22-bookworm

# Install system dependencies (ffmpeg, protobuf compiler, C++ compiler for CGO)
RUN apt-get update && apt-get install -y \
    ffmpeg \
    protobuf-compiler \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Install Go protobuf plugins
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.33.0 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0

WORKDIR /app

# Copy proto definitions
COPY proto /app/proto

# Setup Go module
COPY services/go-worker/go.mod ./
# RUN go mod download (if we had dependencies beyond grpc)
# We will run go mod tidy later

COPY services/go-worker/ /app/services/go-worker/

# Generate Protos
WORKDIR /app/services/go-worker
RUN mkdir -p internal/pb/syncpb internal/pb/translationpb
RUN protoc --proto_path=/app/proto --go_out=internal/pb/syncpb --go_opt=paths=source_relative --go-grpc_out=internal/pb/syncpb --go-grpc_opt=paths=source_relative /app/proto/sync.proto
RUN protoc --proto_path=/app/proto --go_out=internal/pb/translationpb --go_opt=paths=source_relative --go-grpc_out=internal/pb/translationpb --go-grpc_opt=paths=source_relative /app/proto/translation.proto

RUN go mod tidy

# Build the server
RUN go build -o server ./cmd/server/main.go

CMD ["./server"]
