#!/bin/bash
# 构建并导入镜像到 k3s 的脚本

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 项目根目录
PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$PROJECT_ROOT"

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}    AreYouOK Docker Build Script${NC}"
echo -e "${GREEN}========================================${NC}"

# 镜像列表
IMAGES=(
    "areyouok-api:latest:docker/api/Dockerfile"
    "areyouok-worker:latest:docker/worker/Dockerfile"
    "areyouok-scheduler:latest:docker/scheduler/Dockerfile"
    "areyouok-rabbitmq:latest:docker/rabbitmq/Dockerfile"
)

# 构建单个镜像
build_single_image() {
    local target="$1"
    local found=0
    
    for item in "${IMAGES[@]}"; do
        IFS=':' read -r name tag dockerfile <<< "$item"
        image="${name}:${tag}"
        
        # 匹配镜像名称（支持完整名称或部分匹配）
        if [[ "$name" == *"$target"* ]] || [[ "$image" == *"$target"* ]]; then
            found=1
            echo -e "\n${YELLOW}[INFO] Building ${image}...${NC}"
            docker build -t "$image" -f "$dockerfile" .
            
            if [ $? -eq 0 ]; then
                echo -e "${GREEN}[SUCCESS] Built ${image}${NC}"
            else
                echo -e "${RED}[ERROR] Failed to build ${image}${NC}"
                exit 1
            fi
        fi
    done
    
    if [ $found -eq 0 ]; then
        echo -e "${RED}[ERROR] Image '$target' not found${NC}"
        echo -e "${YELLOW}Available images: api, worker, scheduler, rabbitmq${NC}"
        exit 1
    fi
}

# 构建镜像
build_images() {
    echo -e "\n${YELLOW}[INFO] Building images...${NC}"
    
    for item in "${IMAGES[@]}"; do
        IFS=':' read -r name tag dockerfile <<< "$item"
        image="${name}:${tag}"
        
        echo -e "\n${YELLOW}[INFO] Building ${image}...${NC}"
        docker build -t "$image" -f "$dockerfile" .
        
        if [ $? -eq 0 ]; then
            echo -e "${GREEN}[SUCCESS] Built ${image}${NC}"
        else
            echo -e "${RED}[ERROR] Failed to build ${image}${NC}"
            exit 1
        fi
    done
}

# 导入单个镜像到 k3s
import_single_image() {
    local target="$1"
    local found=0
    
    for item in "${IMAGES[@]}"; do
        IFS=':' read -r name tag dockerfile <<< "$item"
        image="${name}:${tag}"
        
        if [[ "$name" == *"$target"* ]] || [[ "$image" == *"$target"* ]]; then
            found=1
            echo -e "${YELLOW}[INFO] Importing ${image} to k3s...${NC}"
            docker save "$image" | sudo k3s ctr images import -
            
            if [ $? -eq 0 ]; then
                echo -e "${GREEN}[SUCCESS] Imported ${image}${NC}"
            else
                echo -e "${RED}[ERROR] Failed to import ${image}${NC}"
                exit 1
            fi
        fi
    done
    
    if [ $found -eq 0 ]; then
        echo -e "${RED}[ERROR] Image '$target' not found${NC}"
        exit 1
    fi
}

# 导入镜像到 k3s
import_to_k3s() {
    echo -e "\n${YELLOW}[INFO] Importing images to k3s...${NC}"
    
    for item in "${IMAGES[@]}"; do
        IFS=':' read -r name tag dockerfile <<< "$item"
        image="${name}:${tag}"
        
        echo -e "${YELLOW}[INFO] Importing ${image} to k3s...${NC}"
        docker save "$image" | sudo k3s ctr images import -
        
        if [ $? -eq 0 ]; then
            echo -e "${GREEN}[SUCCESS] Imported ${image}${NC}"
        else
            echo -e "${RED}[ERROR] Failed to import ${image}${NC}"
            exit 1
        fi
    done
}

# 验证镜像
verify_images() {
    echo -e "\n${YELLOW}[INFO] Verifying images in k3s...${NC}"
    sudo k3s ctr images list | grep -E "areyouok-(api|worker|scheduler)"
}

# 清理 Docker 镜像（可选）
cleanup_docker() {
    echo -e "\n${YELLOW}[INFO] Cleaning up Docker images...${NC}"
    for item in "${IMAGES[@]}"; do
        IFS=':' read -r name tag dockerfile <<< "$item"
        image="${name}:${tag}"
        docker rmi "$image" 2>/dev/null || true
    done
    echo -e "${GREEN}[SUCCESS] Cleanup complete${NC}"
}

# 主函数
main() {
    case "${1:-all}" in
        build)
            if [ -n "$2" ]; then
                build_single_image "$2"
            else
                build_images
            fi
            ;;
        import)
            if [ -n "$2" ]; then
                import_single_image "$2"
            else
                import_to_k3s
            fi
            ;;
        verify)
            verify_images
            ;;
        cleanup)
            cleanup_docker
            ;;
        all)
            build_images
            import_to_k3s
            verify_images
            echo -e "\n${GREEN}========================================${NC}"
            echo -e "${GREEN}    All images built and imported!${NC}"
            echo -e "${GREEN}========================================${NC}"
            echo -e "\n${YELLOW}[TIP] Run 'cd k3s && ./deploy.sh apps' to deploy${NC}"
            ;;
        api|worker|scheduler|rabbitmq)
            # 直接使用镜像名称作为命令
            build_single_image "$1"
            import_single_image "$1"
            echo -e "\n${GREEN}[SUCCESS] Built and imported $1${NC}"
            ;;
        *)
            echo "Usage: $0 {build|import|verify|cleanup|all|api|worker|scheduler|rabbitmq} [image_name]"
            echo ""
            echo "Commands:"
            echo "  build [name]   - Build all Docker images or a specific one"
            echo "  import [name]  - Import all images to k3s or a specific one"
            echo "  verify         - Verify images in k3s"
            echo "  cleanup        - Remove Docker images (save disk space)"
            echo "  all            - Build, import and verify all (default)"
            echo ""
            echo "Shortcuts (build + import):"
            echo "  api            - Build and import API image"
            echo "  worker         - Build and import Worker image"
            echo "  scheduler      - Build and import Scheduler image"
            echo "  rabbitmq       - Build and import RabbitMQ image"
            echo ""
            echo "Examples:"
            echo "  $0 rabbitmq              # Build and import RabbitMQ only"
            echo "  $0 build rabbitmq        # Build RabbitMQ only"
            echo "  $0 import rabbitmq       # Import RabbitMQ only"
            exit 1
            ;;
    esac
}

main "$@"

