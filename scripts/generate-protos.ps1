$ErrorActionPreference = "Stop"

Write-Host "Generating Protos..."
# Note: Requires protoc, protoc-gen-go, protoc-gen-go-grpc, and python grpcio-tools installed on host.
# For a production setup, this would be baked into the Dockerfiles.

# 1. Generate Go Protos
Write-Host "Generating Go protos..."
$goOutDir = "..\services\go-worker\internal\proto"
New-Item -ItemType Directory -Force -Path $goOutDir | Out-Null
protoc --proto_path=..\proto --go_out=$goOutDir --go_opt=paths=source_relative --go-grpc_out=$goOutDir --go-grpc_opt=paths=source_relative ..\proto\sync.proto ..\proto\translation.proto

# 2. Generate Python Protos
Write-Host "Generating Python protos..."
$pyOutDir = "..\services\python-translator\proto"
New-Item -ItemType Directory -Force -Path $pyOutDir | Out-Null
python -m grpc_tools.protoc -I..\proto --python_out=$pyOutDir --grpc_python_out=$pyOutDir ..\proto\translation.proto

# 3. Generate Node/NestJS Protos
Write-Host "For NestJS, we will load the .proto file dynamically at runtime using @grpc/proto-loader, so no pre-generation is strictly required."

Write-Host "Done!"
