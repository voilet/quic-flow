#!/bin/bash
# 流水线用例自动化测试脚本
#
# 用途：自动化测试流水线的完整功能，包括创建、执行和监控
# 使用方法：./test-pipeline.sh <base_url> [project_id]
#
# 示例：./test-pipeline.sh http://localhost:8080

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 配置
BASE_URL="${1:-http://localhost:8080}"
PROJECT_ID="${2:-}"

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  流水线用例自动化测试${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "后端地址: $BASE_URL"

# 检查服务是否可用
check_server() {
    echo -n "检查后端服务... "
    if curl -s -f "$BASE_URL/health" > /dev/null 2>&1; then
        echo -e "${GREEN}✓${NC}"
        return 0
    else
        echo -e "${RED}✗${NC}"
        echo "错误: 无法连接到后端服务 $BASE_URL"
        exit 1
    fi
}

# 创建测试项目
create_test_project() {
    echo ""
    echo -e "${YELLOW}创建测试项目...${NC}"

    response=$(curl -s -X POST "$BASE_URL/api/release/projects" \
        -H "Content-Type: application/json" \
        -d '{
            "name": "流水线测试项目",
            "type": "custom",
            "description": "用于自动化测试的临时项目"
        }')

    PROJECT_ID=$(echo "$response" | jq -r '.id // .data.id // empty')

    if [ -z "$PROJECT_ID" ] || [ "$PROJECT_ID" = "null" ]; then
        echo -e "${RED}创建项目失败${NC}"
        echo "响应: $response"
        exit 1
    fi

    echo -e "${GREEN}✓ 项目创建成功: $PROJECT_ID${NC}"
}

# 创建流水线
create_pipeline() {
    echo ""
    echo -e "${YELLOW}创建流水线...${NC}"

    # 读取 YAML 文件并转换为 JSON（简化版）
    pipeline_json='{
        "name": "标准应用部署流水线",
        "description": "包含健康检查、备份、部署、验证和通知的完整部署流程",
        "type": "deploy",
        "enabled": true,
        "stages": [
            {
                "name": "预发布检查",
                "phase": "pre_release",
                "on_error": "stop",
                "parallel": false,
                "tasks": [
                    {
                        "id": "test-health-check",
                        "name": "测试健康检查",
                        "type": "health_check",
                        "timeout": 30,
                        "retry": 2,
                        "retry_delay": 5,
                        "config": {
                            "hosts": ["localhost:8080"],
                            "ports": [8080],
                            "expected_status": 200
                        }
                    }
                ]
            },
            {
                "name": "部署执行",
                "phase": "release",
                "on_error": "rollback",
                "parallel": false,
                "tasks": [
                    {
                        "id": "test-script",
                        "name": "测试脚本执行",
                        "type": "script",
                        "timeout": 60,
                        "config": {
                            "script": "#!/bin/bash\necho \"测试脚本执行成功\"\nexit 0",
                            "run_on": "local"
                        }
                    }
                ]
            }
        ]
    }'

    response=$(curl -s -X POST "$BASE_URL/api/release/projects/$PROJECT_ID/pipelines" \
        -H "Content-Type: application/json" \
        -d "$pipeline_json")

    PIPELINE_ID=$(echo "$response" | jq -r '.id // .data.id // empty')

    if [ -z "$PIPELINE_ID" ] || [ "$PIPELINE_ID" = "null" ]; then
        echo -e "${RED}创建流水线失败${NC}"
        echo "响应: $response"
        exit 1
    fi

    echo -e "${GREEN}✓ 流水线创建成功: $PIPELINE_ID${NC}"
}

# 获取流水线列表
get_pipelines() {
    echo ""
    echo -e "${YELLOW}获取流水线列表...${NC}"

    response=$(curl -s "$BASE_URL/api/release/projects/$PROJECT_ID/pipelines")

    count=$(echo "$response" | jq '. | length')
    echo -e "${GREEN}✓ 共有 $count 条流水线${NC}"

    # 显示流水线列表
    echo "$response" | jq -r '.[] | "  - \(.name) (\(.id))"'
}

# 获取流水线详情
get_pipeline_detail() {
    echo ""
    echo -e "${YELLOW}获取流水线详情...${NC}"

    response=$(curl -s "$BASE_URL/api/release/pipelines/$PIPELINE_ID")

    name=$(echo "$response" | jq -r '.name')
    stage_count=$(echo "$response" | jq -r '.stages | length')

    echo -e "${GREEN}✓ 流水线名称: $name${NC}"
    echo -e "${GREEN}✓ 阶段数量: $stage_count${NC}"
}

# 执行流水线
execute_pipeline() {
    echo ""
    echo -e "${YELLOW}执行流水线...${NC}"

    response=$(curl -s -X POST "$BASE_URL/api/release/pipelines/$PIPELINE_ID/execute" \
        -H "Content-Type: application/json" \
        -d '{
            "parameters": {
                "app.name": "test-app",
                "deployment.version": "1.0.0-test"
            }
        }')

    EXECUTION_ID=$(echo "$response" | jq -r '.id // .execution_id // .data.id // empty')

    if [ -z "$EXECUTION_ID" ] || [ "$EXECUTION_ID" = "null" ]; then
        echo -e "${RED}执行流水线失败${NC}"
        echo "响应: $response"
        exit 1
    fi

    echo -e "${GREEN}✓ 流水线执行已启动: $EXECUTION_ID${NC}"
}

# 监控执行状态
monitor_execution() {
    echo ""
    echo -e "${YELLOW}监控执行状态...${NC}"
    echo "（每2秒刷新一次，按 Ctrl+C 停止）"
    echo ""

    while true; do
        response=$(curl -s "$BASE_URL/api/release/executions/$EXECUTION_ID")
        status=$(echo "$response" | jq -r '.status // "unknown"')
        phase=$(echo "$response" | jq -r '.current_phase // "-"')

        # 清除当前行
        printf "\r"

        case $status in
            "pending")
                printf "${YELLOW}等待中...${NC} 当前阶段: $phase"
                ;;
            "running")
                printf "${GREEN}执行中...${NC} 当前阶段: $phase"
                ;;
            "success"|"completed")
                printf "\r${GREEN}✓ 执行完成！${NC}\n\n"
                break
                ;;
            "failed"|"error")
                printf "\r${RED}✗ 执行失败！${NC}\n\n"
                break
                ;;
            "cancelled")
                printf "\r${YELLOW}执行已取消${NC}\n\n"
                break
                ;;
            *)
                printf "状态: $status | 阶段: $phase"
                ;;
        esac

        sleep 2
    done

    # 显示执行摘要
    response=$(curl -s "$BASE_URL/api/release/executions/$EXECUTION_ID")
    echo "执行摘要："
    echo "$response" | jq -r '{
        status: .status,
        started_at: .started_at,
        completed_at: .completed_at,
        total_tasks: .total_tasks,
        completed_tasks: .completed_tasks,
        failed_tasks: .failed_tasks
    }'
}

# 获取执行日志
get_execution_logs() {
    echo ""
    echo -e "${YELLOW}获取执行日志...${NC}"

    response=$(curl -s "$BASE_URL/api/release/executions/$EXECUTION_ID/logs")

    # 显示最近的日志
    echo "$response" | jq -r '.[-10:][]? | "[\(.timestamp)] \(.level): \(.message)"' 2>/dev/null || \
    echo "（暂无日志或日志格式不同）"
}

# 清理测试数据
cleanup() {
    echo ""
    read -p "是否删除测试项目？(y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo -e "${YELLOW}删除测试项目...${NC}"
        curl -s -X DELETE "$BASE_URL/api/release/projects/$PROJECT_ID" > /dev/null
        echo -e "${GREEN}✓ 测试项目已删除${NC}"
    fi
}

# 主流程
main() {
    check_server

    # 如果没有提供 project_id，创建一个
    if [ -z "$PROJECT_ID" ]; then
        create_test_project
    else
        echo -e "${GREEN}使用现有项目: $PROJECT_ID${NC}"
    fi

    create_pipeline
    get_pipelines
    get_pipeline_detail
    execute_pipeline
    monitor_execution
    get_execution_logs

    cleanup

    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  测试完成！${NC}"
    echo -e "${GREEN}========================================${NC}"
}

# 运行主流程
main
