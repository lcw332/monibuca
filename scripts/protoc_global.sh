#!/bin/bash

cd pb
# Run the protoc command
protoc -I. \
    --go_out=. \
    --go_opt=paths=source_relative \
    --go-grpc_out=. \
    --go-grpc_opt=paths=source_relative \
    --grpc-gateway_out=. \
    --grpc-gateway_opt=paths=source_relative \
    "global.proto"

# Check if the command was successful
if [ $? -eq 0 ]; then
    echo "Proto files for global built successfully"
else
    echo "Error building proto files for global"
    exit 1
fi