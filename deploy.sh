#!/bin/bash

# AIO 应用部署脚本
# 支持滚动更新、健康检查、自动回滚等功能
# 使用方法: ./deploy.sh [deploy|web-only|rollback]

set -e

# 配置常量
APP_NAME="aio"
SERVERS=("172.17.59.164" "172.17.59.165" "172.17.59.166")
REMOTE_USER="root"
REMOTE_PATH="/opt/aio"
SERVICE_NAME="aio"
HTTP_PORT="9999"
GRPC_PORT="6666"
BUILD_DIR="build"
BACKUP_DIR="backup"
MAX_HEALTH_CHECK_ATTEMPTS=30
HEALTH_CHECK_INTERVAL=2

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查依赖
check_dependencies() {
    log_info "检查依赖..."
    
    if ! command -v go &> /dev/null; then
        log_error "Go 未安装"
        exit 1
    fi
    
    if ! command -v ssh &> /dev/null; then
        log_error "SSH 未安装"
        exit 1
    fi
    
    if ! command -v rsync &> /dev/null; then
        log_error "rsync 未安装，将使用 scp 作为备选"
    fi
    
    log_success "依赖检查通过"
}

# 构建应用
build_app() {
    log_info "开始构建应用..."
    
    # 清理之前的构建
    rm -rf $BUILD_DIR
    mkdir -p $BUILD_DIR
    
    # 设置环境变量
    export CGO_ENABLED=0
    export GOOS=linux
    export GOARCH=amd64
    
    # 构建应用
    if ! go build -ldflags="-s -w" -o $BUILD_DIR/$APP_NAME ./cmd/server/; then
        log_error "构建失败"
        exit 1
    fi
    
    log_success "构建完成"
}

# 准备部署包
prepare_package() {
    local web_only=$1
    
    log_info "准备部署包..."
    
    if [ "$web_only" != "true" ]; then
        # 复制二进制文件
        if [ ! -f "$BUILD_DIR/$APP_NAME" ]; then
            log_error "二进制文件不存在，请先构建"
            exit 1
        fi
    fi
    
    # 复制配置文件
    cp -r conf $BUILD_DIR/
    
    # 复制web文件
    cp -r web $BUILD_DIR/
    
    # 创建systemd服务文件
    cat > $BUILD_DIR/$SERVICE_NAME.service << EOF
[Unit]
Description=AIO Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=$REMOTE_PATH
ExecStart=$REMOTE_PATH/$APP_NAME
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF
    
    log_success "部署包准备完成"
}

# 检查服务器连接
check_server_connection() {
    local server=$1
    
    if ssh -o ConnectTimeout=5 -o BatchMode=yes $REMOTE_USER@$server "echo 'Connection test'" &>/dev/null; then
        return 0
    else
        return 1
    fi
}

# 健康检查
health_check() {
    local server=$1
    local attempts=0
    
    log_info "对服务器 $server 进行健康检查..."
    
    while [ $attempts -lt $MAX_HEALTH_CHECK_ATTEMPTS ]; do
        if ssh $REMOTE_USER@$server "curl -f -s http://localhost:$HTTP_PORT/health || nc -z localhost $HTTP_PORT" &>/dev/null; then
            log_success "服务器 $server 健康检查通过"
            return 0
        fi
        
        attempts=$((attempts + 1))
        log_info "健康检查尝试 $attempts/$MAX_HEALTH_CHECK_ATTEMPTS"
        sleep $HEALTH_CHECK_INTERVAL
    done
    
    log_error "服务器 $server 健康检查失败"
    return 1
}

# 备份当前版本
backup_current_version() {
    local server=$1
    
    log_info "备份服务器 $server 当前版本..."
    
    ssh $REMOTE_USER@$server "
        if [ -d $REMOTE_PATH ]; then
            mkdir -p $REMOTE_PATH/$BACKUP_DIR
            timestamp=\$(date +%Y%m%d_%H%M%S)
            if [ -f $REMOTE_PATH/$APP_NAME ]; then
                cp $REMOTE_PATH/$APP_NAME $REMOTE_PATH/$BACKUP_DIR/${APP_NAME}_\$timestamp
            fi
            if [ -d $REMOTE_PATH/web ]; then
                cp -r $REMOTE_PATH/web $REMOTE_PATH/$BACKUP_DIR/web_\$timestamp
            fi
            if [ -d $REMOTE_PATH/conf ]; then
                cp -r $REMOTE_PATH/conf $REMOTE_PATH/$BACKUP_DIR/conf_\$timestamp
            fi
            # 只保留最近5个备份
            cd $REMOTE_PATH/$BACKUP_DIR && ls -t ${APP_NAME}_* 2>/dev/null | tail -n +6 | xargs rm -f
            cd $REMOTE_PATH/$BACKUP_DIR && ls -dt web_* 2>/dev/null | tail -n +6 | xargs rm -rf
            cd $REMOTE_PATH/$BACKUP_DIR && ls -dt conf_* 2>/dev/null | tail -n +6 | xargs rm -rf
        fi
    "
    
    log_success "服务器 $server 备份完成"
}

# 部署到单台服务器
deploy_to_server() {
    local server=$1
    local web_only=$2
    
    log_info "开始部署到服务器 $server..."
    
    # 检查连接
    if ! check_server_connection $server; then
        log_error "无法连接到服务器 $server"
        return 1
    fi
    
    # 备份当前版本
    backup_current_version $server
    
    # 创建目录结构
    ssh $REMOTE_USER@$server "
        mkdir -p $REMOTE_PATH/logs
        mkdir -p $REMOTE_PATH/$BACKUP_DIR
    "
    
    # 停止服务
    log_info "停止服务器 $server 上的服务..."
    ssh $REMOTE_USER@$server "
        systemctl stop $SERVICE_NAME 2>/dev/null || true
        pkill -f $APP_NAME || true
    "
    
    # 传输文件
    log_info "传输文件到服务器 $server..."
    
    if command -v rsync &> /dev/null; then
        if [ "$web_only" = "true" ]; then
            rsync -avz --delete $BUILD_DIR/web/ $REMOTE_USER@$server:$REMOTE_PATH/web/
        else
            rsync -avz --exclude='*.pid' --exclude='logs/*' $BUILD_DIR/ $REMOTE_USER@$server:$REMOTE_PATH/
        fi
    else
        if [ "$web_only" = "true" ]; then
            scp -r $BUILD_DIR/web/* $REMOTE_USER@$server:$REMOTE_PATH/web/
        else
            scp -r $BUILD_DIR/* $REMOTE_USER@$server:$REMOTE_PATH/
        fi
    fi
    
    # 设置权限
    ssh $REMOTE_USER@$server "
        chmod +x $REMOTE_PATH/$APP_NAME 2>/dev/null || true
    "
    
    # 安装systemd服务
    if [ "$web_only" != "true" ]; then
        ssh $REMOTE_USER@$server "
            cp $REMOTE_PATH/$SERVICE_NAME.service /etc/systemd/system/
            systemctl daemon-reload
            systemctl enable $SERVICE_NAME
        "
    fi
    
    # 启动服务
    log_info "启动服务器 $server 上的服务..."
    ssh $REMOTE_USER@$server "systemctl start $SERVICE_NAME"
    
    # 健康检查
    if ! health_check $server; then
        log_error "服务器 $server 部署失败，正在回滚..."
        rollback_server $server
        return 1
    fi
    
    log_success "服务器 $server 部署成功"
    return 0
}

# 回滚单台服务器
rollback_server() {
    local server=$1
    
    log_warning "回滚服务器 $server..."
    
    ssh $REMOTE_USER@$server "
        systemctl stop $SERVICE_NAME 2>/dev/null || true
        pkill -f $APP_NAME || true
        
        # 恢复最新备份
        cd $REMOTE_PATH/$BACKUP_DIR
        if [ -f \$(ls -t ${APP_NAME}_* 2>/dev/null | head -n 1) ]; then
            cp \$(ls -t ${APP_NAME}_* | head -n 1) $REMOTE_PATH/$APP_NAME
        fi
        if [ -d \$(ls -dt web_* 2>/dev/null | head -n 1) ]; then
            rm -rf $REMOTE_PATH/web
            cp -r \$(ls -dt web_* | head -n 1) $REMOTE_PATH/web
        fi
        if [ -d \$(ls -dt conf_* 2>/dev/null | head -n 1) ]; then
            rm -rf $REMOTE_PATH/conf
            cp -r \$(ls -dt conf_* | head -n 1) $REMOTE_PATH/conf
        fi
        
        chmod +x $REMOTE_PATH/$APP_NAME 2>/dev/null || true
        systemctl start $SERVICE_NAME
    "
    
    log_success "服务器 $server 回滚完成"
}

# 完整部署
deploy_all() {
    local web_only=$1
    
    if [ "$web_only" != "true" ]; then
        build_app
    fi
    
    prepare_package $web_only
    
    log_info "开始滚动部署到 ${#SERVERS[@]} 台服务器..."
    
    local failed_servers=()
    
    for server in "${SERVERS[@]}"; do
        log_info "正在部署服务器 $server (剩余: $((${#SERVERS[@]} - ${#failed_servers[@]} - 1)) 台)..."
        
        if ! deploy_to_server $server $web_only; then
            failed_servers+=($server)
            log_error "服务器 $server 部署失败，停止后续部署"
            break
        fi
        
        log_success "服务器 $server 部署成功"
        
        # 如果不是最后一台服务器，等待一段时间
        if [ "$server" != "${SERVERS[${#SERVERS[@]}-1]}" ]; then
            log_info "等待 5 秒后继续下一台服务器..."
            sleep 5
        fi
    done
    
    if [ ${#failed_servers[@]} -eq 0 ]; then
        log_success "所有服务器部署成功！"
        cleanup
    else
        log_error "部署失败的服务器: ${failed_servers[*]}"
        exit 1
    fi
}

# 清理临时文件
cleanup() {
    log_info "清理临时文件..."
    rm -rf $BUILD_DIR
    log_success "清理完成"
}

# 显示帮助
show_help() {
    echo "AIO 应用部署脚本"
    echo ""
    echo "使用方法:"
    echo "  $0 deploy          完整部署（构建+部署二进制+配置+web文件）"
    echo "  $0 web-only        仅更新web文件"
    echo "  $0 rollback        回滚所有服务器到上一个版本"
    echo "  $0 status          检查所有服务器状态"
    echo "  $0 help            显示帮助信息"
    echo ""
    echo "目标服务器: ${SERVERS[*]}"
}

# 检查服务状态
check_status() {
    log_info "检查所有服务器状态..."
    
    for server in "${SERVERS[@]}"; do
        log_info "检查服务器 $server..."
        
        if ! check_server_connection $server; then
            log_error "无法连接到服务器 $server"
            continue
        fi
        
        # 检查systemd服务状态
        service_status=$(ssh $REMOTE_USER@$server "systemctl is-active $SERVICE_NAME 2>/dev/null || echo 'inactive'")
        
        # 检查端口
        http_port_status="关闭"
        grpc_port_status="关闭"
        
        if ssh $REMOTE_USER@$server "nc -z localhost $HTTP_PORT" &>/dev/null; then
            http_port_status="开启"
        fi
        
        if ssh $REMOTE_USER@$server "nc -z localhost $GRPC_PORT" &>/dev/null; then
            grpc_port_status="开启"
        fi
        
        echo "  服务状态: $service_status"
        echo "  HTTP端口($HTTP_PORT): $http_port_status"
        echo "  GRPC端口($GRPC_PORT): $grpc_port_status"
        echo ""
    done
}

# 回滚所有服务器
rollback_all() {
    log_warning "开始回滚所有服务器..."
    
    for server in "${SERVERS[@]}"; do
        if check_server_connection $server; then
            rollback_server $server
        else
            log_error "无法连接到服务器 $server，跳过回滚"
        fi
    done
    
    log_success "回滚完成"
}

# 主函数
main() {
    local action=${1:-help}
    
    case $action in
        deploy)
            check_dependencies
            deploy_all false
            ;;
        web-only)
            check_dependencies
            deploy_all true
            ;;
        rollback)
            check_dependencies
            rollback_all
            ;;
        status)
            check_dependencies
            check_status
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            log_error "未知操作: $action"
            show_help
            exit 1
            ;;
    esac
}

# 捕获中断信号
trap cleanup EXIT INT TERM

# 执行主函数
main "$@" 