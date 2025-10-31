#!/bin/bash

echo "🔧 Docker 网络连接问题修复脚本"
echo "=================================="

# 检查 Docker 是否运行
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker 未运行，请启动 Docker Desktop"
    exit 1
fi

echo "✅ Docker 正在运行"

# 停止现有容器
echo "🛑 停止现有容器..."
docker-compose down --remove-orphans

# 清理未使用的镜像和缓存
echo "🧹 清理 Docker 缓存..."
docker system prune -f

# 重新构建
echo "🔨 重新构建应用（使用国内镜像源）..."
docker-compose build --no-cache

echo ""
echo "🎉 修复完成！"
echo ""
echo "📝 后续使用："
echo "   启动服务: docker-compose up -d"
echo "   查看日志: docker-compose logs -f"
echo "   停止服务: docker-compose down"
echo ""
echo "🌐 访问地址："
echo "   应用: http://localhost:8080"
echo "   API文档: http://localhost:8080/swagger/index.html"