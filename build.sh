#!/bin/bash

# 构建不同架构的 Linux 可执行文件
echo "开始构建 Linux 可执行文件... 🚀"

# 检查是否启用 tag 构建并获取 tag 值
if [ "$1" = "-tags" ] && [ -n "$2" ]; then
    BUILD_TAGS="$2"
    echo "启用 tag 构建模式: $BUILD_TAGS"
else
    BUILD_TAGS="sqlite,s3"
    echo "使用默认 tags: $BUILD_TAGS"
fi

# 构建 AMD64 架构版本
echo "正在构建 AMD64 版本... 💻"
GOOS=linux GOARCH=amd64 go build -tags "$BUILD_TAGS" -o ./monibuca_amd64 ./example/default/main.go
echo "AMD64 版本构建完成 ✅"

# 构建 ARM64 架构版本
echo "正在构建 ARM64 版本... 📱"
GOOS=linux GOARCH=arm64 go build -tags "$BUILD_TAGS" -o ./monibuca_arm64 ./example/default/main.go
echo "ARM64 版本构建完成 ✅"

# 检查是否需要构建 Docker 镜像
BUILD_DOCKER=false
if [ "$3" = "--docker" ]; then
    BUILD_DOCKER=true
elif [ "$1" = "--docker" ] || [ "$2" = "--docker" ]; then
    BUILD_DOCKER=true
fi

if [ "$BUILD_DOCKER" = true ]; then
    # 构建 Docker 镜像
    echo "正在构建 Docker 镜像... 🐳"
    docker build -f ./DockerfileLite --build-arg BUILD_TAGS="$BUILD_TAGS" -t jddt/monibuca:v5-slim .
    echo "Docker 镜像构建完成 ✅"
else
    echo "跳过 Docker 镜像构建 🚫"
fi

echo "所有版本构建完成! 🎉"