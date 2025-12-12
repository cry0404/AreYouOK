#!/bin/bash
# AreYouOK K3s One-Click Deployment Script
# Usage: ./deploy.sh [command]
# Commands:
#   deploy         - Deploy core services (default)
#   deploy-all     - Deploy all services including observability
#   deploy-obs     - Deploy observability stack only
#   deploy-tls     - Deploy TLS/cert-manager configuration
#   deploy-redis-ha - Deploy Redis Sentinel (high availability)
#   delete         - Delete all services
#   delete-obs     - Delete observability stack only
#   status         - Show deployment status
#   logs           - Show logs for all services
#   restart        - Restart application services

set -e

NAMESPACE="areyouok"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"


RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查 kubectl 是否存在
check_kubectl() {
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl not found. Please install kubectl or k3s first."
        echo ""
        echo "Install k3s with:"
        echo "  curl -sfL https://rancher-mirror.rancher.cn/k3s/k3s-install.sh | INSTALL_K3S_MIRROR=cn sh -"
        exit 1
    fi
}

# 检查集群的连接
check_cluster() {
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster."
        echo ""
        echo "For k3s, try:"
        echo "  export KUBECONFIG=/etc/rancher/k3s/k3s.yaml"
        echo "  # or"
        echo "  mkdir -p ~/.kube && sudo cp /etc/rancher/k3s/k3s.yaml ~/.kube/config && sudo chown \$(id -u):\$(id -g) ~/.kube/config"
        exit 1
    fi
}

# 等待 pod 就绪
wait_for_pod() {
    local label=$1
    local timeout=${2:-120}
    log_info "Waiting for pod with label '$label' to be ready (timeout: ${timeout}s)..."
    if kubectl -n $NAMESPACE wait --for=condition=ready pod -l "$label" --timeout="${timeout}s" 2>/dev/null; then
        log_success "Pod '$label' is ready"
        return 0
    else
        log_warn "Pod '$label' not ready within ${timeout}s, continuing..."
        return 1
    fi
}

# 开始 deploy
deploy() {
    log_info "Starting AreYouOK deployment..."
    echo ""

    # Step 1: Create namespace 创建命名空间
    log_info "Step 1/6: Creating namespace..."
    kubectl apply -f "$SCRIPT_DIR/namespace.yaml"
    echo ""

    # Step 2: Create config and secrets 创建 configMap 和 secrets
    log_info "Step 2/6: Creating ConfigMap and Secrets..."
    kubectl apply -f "$SCRIPT_DIR/configmap.yaml"
    kubectl apply -f "$SCRIPT_DIR/secrets.yaml"
    echo ""

    # Step 3:  部署数据库部分
    log_info "Step 3/6: Deploying PostgreSQL..."
    kubectl apply -f "$SCRIPT_DIR/postgres/statefulset.yaml"
    wait_for_pod "app=postgres" 180
    echo ""

    # Step 4: 部署 redis
    log_info "Step 4/6: Deploying Redis..."
    kubectl apply -f "$SCRIPT_DIR/redis/statefulset.yaml"
    wait_for_pod "app=redis" 60
    echo ""

    # Step 5: 部署 rabbitmq
    log_info "Step 5/6: Deploying RabbitMQ..."
    kubectl apply -f "$SCRIPT_DIR/rabbitmq/statefulset.yaml"
    wait_for_pod "app=rabbitmq" 180
    echo ""

    # Step 6: Deploy applications
    log_info "Step 6/6: Deploying applications (API, Worker, Scheduler)..."
    kubectl apply -f "$SCRIPT_DIR/api/deployment.yaml"
    kubectl apply -f "$SCRIPT_DIR/worker/deployment.yaml"
    kubectl apply -f "$SCRIPT_DIR/scheduler/deployment.yaml"
    kubectl apply -f "$SCRIPT_DIR/ingress.yaml"
    echo ""


    log_info "Waiting for applications to start..."
    wait_for_pod "app=api" 60
    wait_for_pod "app=worker" 60
    wait_for_pod "app=scheduler" 60
    echo ""

    log_success "Deployment completed!"
    echo ""
    show_status
}


deploy_kustomize() {
    log_info "Deploying with Kustomize..."
    kubectl apply -k "$SCRIPT_DIR"
    echo ""
    log_info "Waiting for infrastructure..."
    sleep 10
    wait_for_pod "app=postgres" 180
    wait_for_pod "app=redis" 60
    wait_for_pod "app=rabbitmq" 180
    log_info "Waiting for applications..."
    wait_for_pod "app=api" 60
    wait_for_pod "app=worker" 60
    wait_for_pod "app=scheduler" 60
    echo ""
    log_success "Deployment completed!"
    show_status
}

# 部署可观测性组件 (Prometheus, Jaeger, Loki, Grafana, OTEL Collector)
deploy_observability() {
    log_info "Deploying observability stack..."
    echo ""

    # 确保命名空间存在
    kubectl apply -f "$SCRIPT_DIR/namespace.yaml"

    # Step 1: 部署 Jaeger (Tracing)
    log_info "Step 1/5: Deploying Jaeger..."
    kubectl apply -f "$SCRIPT_DIR/observability/jaeger.yaml"
    wait_for_pod "app=jaeger" 60
    echo ""

    # Step 2: 部署 Loki (Logs)
    log_info "Step 2/5: Deploying Loki..."
    kubectl apply -f "$SCRIPT_DIR/observability/loki.yaml"
    wait_for_pod "app=loki" 120
    echo ""

    # Step 3: 部署 Prometheus (Metrics)
    log_info "Step 3/5: Deploying Prometheus..."
    kubectl apply -f "$SCRIPT_DIR/observability/prometheus.yaml"
    wait_for_pod "app=prometheus" 120
    echo ""

    # Step 4: 部署 OTEL Collector
    log_info "Step 4/5: Deploying OpenTelemetry Collector..."
    kubectl apply -f "$SCRIPT_DIR/observability/otel-collector.yaml"
    wait_for_pod "app=otel-collector" 60
    echo ""

    # Step 5: 部署 Grafana (可视化)
    log_info "Step 5/5: Deploying Grafana..."
    kubectl apply -f "$SCRIPT_DIR/observability/grafana.yaml"
    wait_for_pod "app=grafana" 60
    echo ""

    # 部署 Ingress
    log_info "Deploying observability ingress..."
    kubectl apply -f "$SCRIPT_DIR/observability/ingress.yaml"
    echo ""

    log_success "Observability stack deployed!"
    echo ""
    show_observability_status
}

# 部署所有服务（核心 + 可观测性）
deploy_all() {
    log_info "Deploying all services (core + observability)..."
    echo ""
    
    # 先部署可观测性组件（因为应用依赖 OTEL Collector）
    deploy_observability
    echo ""
    
    # 再部署核心服务
    deploy
}

# 删除可观测性组件
delete_observability() {
    log_warn "This will delete all observability components!"
    read -p "Are you sure? (yes/no): " confirm
    if [ "$confirm" != "yes" ]; then
        log_info "Cancelled."
        return 0
    fi

    log_info "Deleting observability stack..."
    
    kubectl delete -f "$SCRIPT_DIR/observability/ingress.yaml" --ignore-not-found
    kubectl delete -f "$SCRIPT_DIR/observability/grafana.yaml" --ignore-not-found
    kubectl delete -f "$SCRIPT_DIR/observability/otel-collector.yaml" --ignore-not-found
    kubectl delete -f "$SCRIPT_DIR/observability/prometheus.yaml" --ignore-not-found
    kubectl delete -f "$SCRIPT_DIR/observability/loki.yaml" --ignore-not-found
    kubectl delete -f "$SCRIPT_DIR/observability/jaeger.yaml" --ignore-not-found

    # 询问是否删除 PVC
    read -p "Delete observability PVCs (prometheus, loki, grafana data will be lost)? (yes/no): " delete_pvc
    if [ "$delete_pvc" == "yes" ]; then
        kubectl -n $NAMESPACE delete pvc prometheus-data-prometheus-0 --ignore-not-found
        kubectl -n $NAMESPACE delete pvc loki-data-loki-0 --ignore-not-found
        kubectl -n $NAMESPACE delete pvc grafana-pvc --ignore-not-found
        log_warn "Observability PVCs deleted."
    fi

    log_success "Observability stack deleted."
}

# 显示可观测性组件状态
show_observability_status() {
    echo ""
    echo "=========================================="
    echo "     Observability Stack Status"
    echo "=========================================="
    echo ""
    
    log_info "Observability Pods:"
    kubectl -n $NAMESPACE get pods -l 'app in (otel-collector,prometheus,jaeger,loki,grafana)' -o wide 2>/dev/null || echo "No observability pods found"
    echo ""
    
    log_info "Observability Services:"
    kubectl -n $NAMESPACE get svc -l 'app in (otel-collector,prometheus,jaeger,loki,grafana)' 2>/dev/null || echo "No observability services found"
    echo ""

    # 获取访问信息
    local grafana_ip=$(kubectl -n $NAMESPACE get svc grafana -o jsonpath='{.spec.clusterIP}' 2>/dev/null)
    local jaeger_ip=$(kubectl -n $NAMESPACE get svc jaeger -o jsonpath='{.spec.clusterIP}' 2>/dev/null)
    local prometheus_ip=$(kubectl -n $NAMESPACE get svc prometheus -o jsonpath='{.spec.clusterIP}' 2>/dev/null)
    
    echo "=========================================="
    echo "       Observability Access Info"
    echo "=========================================="
    if [ -n "$grafana_ip" ]; then
        echo "  Grafana:    http://$grafana_ip:3000 (admin/admin)"
    fi
    if [ -n "$jaeger_ip" ]; then
        echo "  Jaeger:     http://$jaeger_ip:16686"
    fi
    if [ -n "$prometheus_ip" ]; then
        echo "  Prometheus: http://$prometheus_ip:9090"
    fi
    echo ""
    echo "  Port Forward Examples:"
    echo "    kubectl -n $NAMESPACE port-forward svc/grafana 3000:3000"
    echo "    kubectl -n $NAMESPACE port-forward svc/jaeger 16686:16686"
    echo "    kubectl -n $NAMESPACE port-forward svc/prometheus 9090:9090"
    echo "=========================================="
}

# 部署 cert-manager 和 TLS 配置
deploy_tls() {
    log_info "Deploying TLS configuration..."
    echo ""

    # 检查 cert-manager 是否已安装
    if ! kubectl get namespace cert-manager &>/dev/null; then
        log_warn "cert-manager not installed. Installing..."
        echo ""
        log_info "Installing cert-manager v1.14.0..."
        kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.14.0/cert-manager.yaml
        
        log_info "Waiting for cert-manager to be ready..."
        kubectl -n cert-manager wait --for=condition=ready pod -l app.kubernetes.io/instance=cert-manager --timeout=180s
        echo ""
    else
        log_success "cert-manager already installed"
    fi

    # 部署 ClusterIssuer
    log_info "Deploying ClusterIssuer..."
    kubectl apply -f "$SCRIPT_DIR/cert-manager/issuer.yaml"
    echo ""

    # 部署 TLS Ingress
    log_info "Deploying TLS Ingress..."
    kubectl apply -f "$SCRIPT_DIR/ingress-tls.yaml"
    echo ""

    log_success "TLS configuration deployed!"
    echo ""
    log_warn "Remember to:"
    echo "  1. Update email in cert-manager/issuer.yaml"
    echo "  2. Update domain in ingress-tls.yaml"
    echo "  3. Ensure your domain DNS points to this server"
    echo ""
    
    # 显示证书状态
    log_info "Certificate status:"
    kubectl -n $NAMESPACE get certificate 2>/dev/null || echo "No certificates found yet"
}

# 部署 Redis Sentinel 高可用
deploy_redis_ha() {
    log_info "Deploying Redis Sentinel (High Availability)..."
    echo ""

    # 检查是否存在旧的单节点 Redis
    if kubectl -n $NAMESPACE get statefulset redis &>/dev/null; then
        local replicas=$(kubectl -n $NAMESPACE get statefulset redis -o jsonpath='{.spec.replicas}')
        if [ "$replicas" == "1" ]; then
            log_warn "Found existing single-node Redis. It will be replaced."
            read -p "Continue? (yes/no): " confirm
            if [ "$confirm" != "yes" ]; then
                log_info "Cancelled."
                return 0
            fi
            
            log_info "Deleting old single-node Redis..."
            kubectl delete -f "$SCRIPT_DIR/redis/statefulset.yaml" --ignore-not-found
            # 等待删除完成
            sleep 5
        fi
    fi

    # 部署 Redis Sentinel
    log_info "Deploying Redis Sentinel cluster (3 nodes)..."
    kubectl apply -f "$SCRIPT_DIR/redis-ha/sentinel.yaml"
    echo ""

    # 等待所有 Redis Pod 就绪
    log_info "Waiting for Redis pods to be ready..."
    wait_for_pod "app=redis" 180
    
    # 等待所有 3 个 Pod
    local ready_pods=0
    local max_wait=120
    local waited=0
    while [ $ready_pods -lt 3 ] && [ $waited -lt $max_wait ]; do
        ready_pods=$(kubectl -n $NAMESPACE get pods -l app=redis -o jsonpath='{.items[*].status.containerStatuses[0].ready}' | tr ' ' '\n' | grep -c "true" || echo "0")
        log_info "Redis pods ready: $ready_pods/3"
        if [ $ready_pods -lt 3 ]; then
            sleep 10
            waited=$((waited + 10))
        fi
    done

    if [ $ready_pods -ge 3 ]; then
        log_success "Redis Sentinel cluster deployed!"
    else
        log_warn "Some Redis pods may not be ready yet. Check with: kubectl -n $NAMESPACE get pods -l app=redis"
    fi
    
    echo ""
    show_redis_status
}

# 显示 Redis 状态
show_redis_status() {
    echo ""
    echo "=========================================="
    echo "         Redis Cluster Status"
    echo "=========================================="
    echo ""
    
    log_info "Redis Pods:"
    kubectl -n $NAMESPACE get pods -l app=redis -o wide 2>/dev/null || echo "No Redis pods found"
    echo ""
    
    # 检查主从状态
    log_info "Checking Master/Replica status..."
    if kubectl -n $NAMESPACE get pod redis-0 &>/dev/null; then
        echo ""
        echo "Master info (from redis-0):"
        kubectl -n $NAMESPACE exec redis-0 -c redis -- redis-cli info replication 2>/dev/null | grep -E "^role:|^connected_slaves:|^slave[0-9]+:" || echo "Unable to get replication info"
        echo ""
        echo "Sentinel status:"
        kubectl -n $NAMESPACE exec redis-0 -c sentinel -- redis-cli -p 26379 sentinel masters 2>/dev/null | head -20 || echo "Unable to get sentinel info"
    fi
    echo ""
    echo "=========================================="
}

# Delete all services
delete() {
    log_warn "This will delete ALL AreYouOK resources including data!"
    read -p "Are you sure? (yes/no): " confirm
    if [ "$confirm" != "yes" ]; then
        log_info "Cancelled."
        exit 0
    fi

    log_info "Deleting all resources..."
    
    # Delete in reverse order
    kubectl delete -f "$SCRIPT_DIR/ingress.yaml" --ignore-not-found
    kubectl delete -f "$SCRIPT_DIR/scheduler/deployment.yaml" --ignore-not-found
    kubectl delete -f "$SCRIPT_DIR/worker/deployment.yaml" --ignore-not-found
    kubectl delete -f "$SCRIPT_DIR/api/deployment.yaml" --ignore-not-found
    kubectl delete -f "$SCRIPT_DIR/rabbitmq/statefulset.yaml" --ignore-not-found
    kubectl delete -f "$SCRIPT_DIR/redis/statefulset.yaml" --ignore-not-found
    kubectl delete -f "$SCRIPT_DIR/postgres/statefulset.yaml" --ignore-not-found
    kubectl delete -f "$SCRIPT_DIR/secrets.yaml" --ignore-not-found
    kubectl delete -f "$SCRIPT_DIR/configmap.yaml" --ignore-not-found

    # Ask about PVCs
    read -p "Delete PersistentVolumeClaims (data will be lost)? (yes/no): " delete_pvc
    if [ "$delete_pvc" == "yes" ]; then
        kubectl -n $NAMESPACE delete pvc --all --ignore-not-found
        log_warn "PVCs deleted. Data is permanently lost."
    fi

    # Ask about namespace
    read -p "Delete namespace '$NAMESPACE'? (yes/no): " delete_ns
    if [ "$delete_ns" == "yes" ]; then
        kubectl delete -f "$SCRIPT_DIR/namespace.yaml" --ignore-not-found
    fi

    log_success "Deletion completed."
}

# Show status
show_status() {
    echo ""
    echo "=========================================="
    echo "       AreYouOK Deployment Status"
    echo "=========================================="
    echo ""
    
    log_info "Pods:"
    kubectl -n $NAMESPACE get pods -o wide 2>/dev/null || echo "No pods found"
    echo ""
    
    log_info "Services:"
    kubectl -n $NAMESPACE get svc 2>/dev/null || echo "No services found"
    echo ""
    
    log_info "Ingress:"
    kubectl -n $NAMESPACE get ingress 2>/dev/null || echo "No ingress found"
    echo ""
    
    log_info "PersistentVolumeClaims:"
    kubectl -n $NAMESPACE get pvc 2>/dev/null || echo "No PVCs found"
    echo ""

    # Get API service access info
    local api_ip=$(kubectl -n $NAMESPACE get svc api -o jsonpath='{.spec.clusterIP}' 2>/dev/null)
    local ingress_host=$(kubectl -n $NAMESPACE get ingress api-ingress -o jsonpath='{.spec.rules[0].host}' 2>/dev/null)
    
    echo "=========================================="
    echo "              Access Info"
    echo "=========================================="
    if [ -n "$api_ip" ]; then
        echo "  API (ClusterIP): http://$api_ip:80"
    fi
    if [ -n "$ingress_host" ]; then
        echo "  API (Ingress):   http://$ingress_host"
    fi
    echo ""
    echo "  Port Forward:    kubectl -n $NAMESPACE port-forward svc/api 8080:80"
    echo "                   Then access: http://localhost:8080"
    echo "=========================================="
}

# Show logs
show_logs() {
    local service=${1:-"all"}
    
    case $service in
        api)
            kubectl -n $NAMESPACE logs -f deployment/api
            ;;
        worker)
            kubectl -n $NAMESPACE logs -f deployment/worker
            ;;
        scheduler)
            kubectl -n $NAMESPACE logs -f deployment/scheduler
            ;;
        postgres)
            kubectl -n $NAMESPACE logs -f statefulset/postgres
            ;;
        redis)
            kubectl -n $NAMESPACE logs -f statefulset/redis
            ;;
        rabbitmq)
            kubectl -n $NAMESPACE logs -f statefulset/rabbitmq
            ;;
        otel|otel-collector)
            kubectl -n $NAMESPACE logs -f deployment/otel-collector
            ;;
        prometheus)
            kubectl -n $NAMESPACE logs -f statefulset/prometheus
            ;;
        jaeger)
            kubectl -n $NAMESPACE logs -f deployment/jaeger
            ;;
        loki)
            kubectl -n $NAMESPACE logs -f statefulset/loki
            ;;
        grafana)
            kubectl -n $NAMESPACE logs -f deployment/grafana
            ;;
        all|*)
            log_info "Showing logs for all application pods (use Ctrl+C to stop)..."
            kubectl -n $NAMESPACE logs -f -l 'app in (api,worker,scheduler)' --prefix=true --max-log-requests=10
            ;;
    esac
}

# 重启应用
restart() {
    log_info "Restarting application services..."
    kubectl -n $NAMESPACE rollout restart deployment/api
    kubectl -n $NAMESPACE rollout restart deployment/worker
    kubectl -n $NAMESPACE rollout restart deployment/scheduler
    log_success "Restart triggered. Use './deploy.sh status' to check progress."
}

# 初始化 database
init_db() {
    log_info "Initializing database..."
    
    #  检查 postgres 的状态
    if ! kubectl -n $NAMESPACE get pod postgres-0 &>/dev/null; then
        log_error "PostgreSQL pod not found. Deploy first with './deploy.sh deploy'"
        exit 1
    fi


    local schema_file="$SCRIPT_DIR/../schema/schema.sql"
    if [ ! -f "$schema_file" ]; then
        log_error "Schema file not found: $schema_file"
        exit 1
    fi

    log_info "Applying schema to database..."
    kubectl -n $NAMESPACE exec -i postgres-0 -- psql -U postgres -d areyouok < "$schema_file"
    log_success "Database initialized successfully!"
}

# Update images (trigger rolling update)
update() {
    local tag=${1:-"latest"}
    log_info "Updating images to tag: $tag"
    
    kubectl -n $NAMESPACE set image deployment/api api=ghcr.io/cry0404/areyouok-api:$tag
    kubectl -n $NAMESPACE set image deployment/worker worker=ghcr.io/cry0404/areyouok-worker:$tag
    kubectl -n $NAMESPACE set image deployment/scheduler scheduler=ghcr.io/cry0404/areyouok-scheduler:$tag
    
    log_info "Rolling update triggered. Checking status..."
    kubectl -n $NAMESPACE rollout status deployment/api
    kubectl -n $NAMESPACE rollout status deployment/worker
    kubectl -n $NAMESPACE rollout status deployment/scheduler
    
    log_success "Update completed!"
}


show_help() {
    echo "AreYouOK K3s Deployment Script"
    echo ""
    echo "Usage: $0 [command] [options]"
    echo ""
    echo "Core Commands:"
    echo "  deploy          Deploy core services step by step (default)"
    echo "  deploy-all      Deploy all services including observability"
    echo "  kustomize       Deploy using Kustomize (kubectl apply -k)"
    echo "  delete          Delete all core services"
    echo "  status          Show deployment status"
    echo "  restart         Restart application services (API, Worker, Scheduler)"
    echo "  init-db         Initialize database with schema.sql"
    echo "  update [tag]    Update images to specified tag (default: latest)"
    echo ""
    echo "High Availability:"
    echo "  deploy-tls      Deploy TLS/HTTPS with cert-manager (Let's Encrypt)"
    echo "  deploy-redis-ha Deploy Redis Sentinel cluster (1 Master + 2 Replicas)"
    echo "  status-redis    Show Redis cluster status"
    echo ""
    echo "Observability:"
    echo "  deploy-obs      Deploy observability stack (Prometheus, Jaeger, Loki, Grafana)"
    echo "  delete-obs      Delete observability stack"
    echo "  status-obs      Show observability stack status"
    echo ""
    echo "Logs:"
    echo "  logs [svc]      Show logs for service:"
    echo "                  Core: api|worker|scheduler|postgres|redis|rabbitmq|all"
    echo "                  Obs:  otel|prometheus|jaeger|loki|grafana"
    echo ""
    echo "Other:"
    echo "  help            Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 deploy            # Deploy core services only"
    echo "  $0 deploy-all        # Deploy everything (core + observability)"
    echo "  $0 deploy-tls        # Setup HTTPS with Let's Encrypt"
    echo "  $0 deploy-redis-ha   # Deploy Redis high availability"
    echo "  $0 status            # Check core services status"
    echo "  $0 logs api          # View API logs"
    echo "  $0 update v1.0.1     # Update to v1.0.1"
    echo ""
}


main() {
    check_kubectl
    check_cluster

    local command=${1:-"deploy"}

    case $command in
        deploy)
            deploy
            ;;
        deploy-all)
            deploy_all
            ;;
        deploy-obs|deploy-observability)
            deploy_observability
            ;;
        deploy-tls)
            deploy_tls
            ;;
        deploy-redis-ha)
            deploy_redis_ha
            ;;
        kustomize)
            deploy_kustomize
            ;;
        delete)
            delete
            ;;
        delete-obs|delete-observability)
            delete_observability
            ;;
        status)
            show_status
            ;;
        status-obs|status-observability)
            show_observability_status
            ;;
        status-redis)
            show_redis_status
            ;;
        logs)
            show_logs "$2"
            ;;
        restart)
            restart
            ;;
        init-db)
            init_db
            ;;
        update)
            update "$2"
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            log_error "Unknown command: $command"
            show_help
            exit 1
            ;;
    esac
}

main "$@"

