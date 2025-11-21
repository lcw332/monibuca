#!/bin/bash

# 构建不同架构的 Linux 可执行文件
echo "开始构建 Linux 可执行文件... 🚀"

# 检查是否启用 tag 构建并获取 tag 值
if [ "$1" = "-tags" ] && [ -n "$2" ]; then
    BUILD_TAGS="$2"
    echo "启用 tag 构建模式: $BUILD_TAGS"
    shift 2
else
    BUILD_TAGS="sqlite,s3"
    echo "使用默认 tags: $BUILD_TAGS"
fi

# 解析平台参数
PLATFORMS=""
BUILD_DOCKER=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --platform=*)
            PLATFORMS="${1#*=}"
            shift
            ;;
        --platform)
            if [ -n "$2" ]; then
                PLATFORMS="$2"
                shift 2
            else
                echo "错误: --platform 参数需要指定平台"
                exit 1
            fi
            ;;
        --docker)
            BUILD_DOCKER=true
            shift
            ;;
        *)
            echo "未知参数: $1"
            exit 1
            ;;
    esac
done

# 如果没有指定平台，则默认构建所有平台
if [ -z "$PLATFORMS" ]; then
    PLATFORMS="amd64,arm64"
fi

# 构建指定平台的可执行文件
IFS=',' read -ra PLATFORM_ARRAY <<< "$PLATFORMS"
for platform in "${PLATFORM_ARRAY[@]}"; do
    case $platform in
        amd64)
            echo "正在构建 AMD64 版本... 💻"
            GOOS=linux GOARCH=amd64 go build -tags "$BUILD_TAGS" -o ./monibuca_amd64 ./example/default/main.go
            echo "AMD64 版本构建完成 ✅"
            ;;
        arm64)
            echo "正在构建 ARM64 版本... 📱"
            GOOS=linux GOARCH=arm64 go build -tags "$BUILD_TAGS" -o ./monibuca_arm64 ./example/default/main.go
            echo "ARM64 版本构建完成 ✅"
            ;;
        *)
            echo "不支持的平台: $platform"
            ;;
    esac
done

if [ "$BUILD_DOCKER" = true ]; then
    # 构建 Docker 镜像
    echo "正在构建 Docker 镜像... 🐳"
    # 根据不同平台构建不同的 Docker 镜像
    for platform in "${PLATFORM_ARRAY[@]}"; do
        case $platform in
            amd64)
                echo "正在构建 AMD64 Docker 镜像... 🐳"
                docker build -f ./DockerfileLite --build-arg BUILD_TAGS="$BUILD_TAGS" --platform linux/amd64 -t jddt/monibuca:v5-slim .
                echo "AMD64 Docker 镜像构建完成 ✅"
                ;;
            arm64)
                echo "正在构建 ARM64 Docker 镜像... 🐳"
                docker build -f ./DockerfileLite --build-arg BUILD_TAGS="$BUILD_TAGS" --platform linux/arm64 -t jddt/monibuca:v5-arm-slim .
                echo "ARM64 Docker 镜像构建完成 ✅"
                ;;
        esac
    done
else
    echo "跳过 Docker 镜像构建 🚫"
fi

echo "所有版本构建完成! 🎉"