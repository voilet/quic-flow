-- 发布系统测试数据脚本
-- 使用方法: psql -h localhost -U postgres -d quic_flow -f seed_release_data.sql

-- 清理现有测试数据（可选，取消注释以启用）
-- DELETE FROM deploy_logs WHERE project_id IN (SELECT id FROM projects WHERE name LIKE '测试%');
-- DELETE FROM deploy_tasks WHERE project_id IN (SELECT id FROM projects WHERE name LIKE '测试%');
-- DELETE FROM versions WHERE project_id IN (SELECT id FROM projects WHERE name LIKE '测试%');
-- DELETE FROM projects WHERE name LIKE '测试%';

-- ==================== 1. 脚本部署项目 ====================
INSERT INTO projects (id, name, description, type, created_at, updated_at) VALUES
('test-script-001', '测试-Web应用部署', '基于脚本的传统Web应用部署', 'script', NOW(), NOW()),
('test-script-002', '测试-微服务部署', 'Go微服务应用脚本部署', 'script', NOW(), NOW());

-- 脚本部署项目配置
UPDATE projects SET script_config = '{
  "work_dir": "/opt/apps/webapp",
  "interpreter": "/bin/bash",
  "environment": {
    "APP_NAME": "webapp",
    "NODE_ENV": "production"
  },
  "install_script": "#!/bin/bash\nset -e\necho \"Installing webapp...\"\nmkdir -p /opt/apps/webapp/{bin,conf,logs}\necho \"Install completed\"",
  "update_script": "#!/bin/bash\nset -e\necho \"Updating webapp...\"\nsystemctl restart webapp\necho \"Update completed\"",
  "rollback_script": "#!/bin/bash\nset -e\necho \"Rolling back webapp...\"\necho \"Rollback completed\"",
  "uninstall_script": "#!/bin/bash\nset -e\necho \"Uninstalling webapp...\"\nrm -rf /opt/apps/webapp\necho \"Uninstall completed\"",
  "timeouts": {
    "install": 600,
    "update": 300,
    "rollback": 180,
    "uninstall": 120
  }
}'::jsonb WHERE id = 'test-script-001';

UPDATE projects SET script_config = '{
  "work_dir": "/opt/apps/microservice",
  "interpreter": "/bin/bash",
  "environment": {
    "SERVICE_NAME": "user-service",
    "GOPROXY": "https://goproxy.cn"
  },
  "install_script": "#!/bin/bash\nset -e\necho \"Installing microservice...\"\nmkdir -p /opt/apps/microservice\necho \"Install completed\"",
  "update_script": "#!/bin/bash\nset -e\necho \"Updating microservice...\"\nsystemctl restart user-service\necho \"Update completed\"",
  "rollback_script": "#!/bin/bash\necho \"Rolling back...\"",
  "uninstall_script": "#!/bin/bash\necho \"Uninstalling...\"",
  "timeouts": {"install": 300, "update": 180, "rollback": 120, "uninstall": 60}
}'::jsonb WHERE id = 'test-script-002';

-- ==================== 2. 容器部署项目 ====================
INSERT INTO projects (id, name, description, type, created_at, updated_at) VALUES
('test-container-001', '测试-Nginx服务', 'Nginx容器部署', 'container', NOW(), NOW()),
('test-container-002', '测试-Redis服务', 'Redis缓存服务容器部署', 'container', NOW(), NOW()),
('test-container-003', '测试-API服务', 'API后端服务容器部署', 'container', NOW(), NOW());

-- 容器部署项目配置
UPDATE projects SET container_config = '{
  "image": "nginx:latest",
  "container_name": "nginx-web",
  "restart_policy": "always",
  "ports": [
    {"host_port": 80, "container_port": 80, "protocol": "tcp"},
    {"host_port": 443, "container_port": 443, "protocol": "tcp"}
  ],
  "volumes": [
    {"host_path": "/data/nginx/html", "container_path": "/usr/share/nginx/html", "read_only": false},
    {"host_path": "/data/nginx/conf", "container_path": "/etc/nginx/conf.d", "read_only": false}
  ],
  "memory_limit": "256m",
  "cpu_limit": "0.5",
  "environment": {"TZ": "Asia/Shanghai"}
}'::jsonb WHERE id = 'test-container-001';

UPDATE projects SET container_config = '{
  "image": "redis:7-alpine",
  "container_name": "redis-cache",
  "restart_policy": "always",
  "ports": [{"host_port": 6379, "container_port": 6379, "protocol": "tcp"}],
  "volumes": [{"host_path": "/data/redis", "container_path": "/data", "read_only": false}],
  "memory_limit": "512m",
  "command": ["redis-server", "--appendonly", "yes"]
}'::jsonb WHERE id = 'test-container-002';

UPDATE projects SET container_config = '{
  "image": "registry.example.com/api-service:latest",
  "container_name": "api-backend",
  "restart_policy": "unless-stopped",
  "ports": [{"host_port": 8080, "container_port": 8080, "protocol": "tcp"}],
  "volumes": [{"host_path": "/data/api/logs", "container_path": "/app/logs", "read_only": false}],
  "memory_limit": "1g",
  "cpu_limit": "1",
  "environment": {"TZ": "Asia/Shanghai", "LOG_LEVEL": "info"},
  "health_check": {
    "command": ["CMD", "curl", "-f", "http://localhost:8080/health"],
    "interval": 30,
    "timeout": 10,
    "retries": 3,
    "start_period": 60
  }
}'::jsonb WHERE id = 'test-container-003';

-- ==================== 3. Kubernetes 部署项目 ====================
INSERT INTO projects (id, name, description, type, created_at, updated_at) VALUES
('test-k8s-001', '测试-K8s微服务', 'Kubernetes Deployment 部署', 'kubernetes', NOW(), NOW()),
('test-k8s-002', '测试-K8s有状态服务', 'Kubernetes StatefulSet 部署', 'kubernetes', NOW(), NOW());

-- K8s 部署项目配置
UPDATE projects SET kubernetes_config = '{
  "namespace": "production",
  "resource_type": "deployment",
  "resource_name": "user-service",
  "container_name": "user-service",
  "image": "registry.example.com/user-service:latest",
  "replicas": 3,
  "update_strategy": "RollingUpdate",
  "max_unavailable": "25%",
  "max_surge": "25%",
  "cpu_request": "100m",
  "cpu_limit": "500m",
  "memory_request": "128Mi",
  "memory_limit": "512Mi",
  "environment": {"TZ": "Asia/Shanghai", "LOG_LEVEL": "info"},
  "service_type": "ClusterIP",
  "service_ports": [{"name": "http", "port": 8080, "target_port": 8080, "protocol": "TCP"}],
  "rollout_timeout": 300
}'::jsonb WHERE id = 'test-k8s-001';

UPDATE projects SET kubernetes_config = '{
  "namespace": "production",
  "resource_type": "statefulset",
  "resource_name": "database",
  "container_name": "postgresql",
  "image": "postgres:15-alpine",
  "replicas": 1,
  "cpu_request": "500m",
  "cpu_limit": "2",
  "memory_request": "512Mi",
  "memory_limit": "2Gi",
  "environment": {"POSTGRES_DB": "appdb", "POSTGRES_USER": "appuser"},
  "service_type": "Headless",
  "service_ports": [{"name": "postgresql", "port": 5432, "target_port": 5432, "protocol": "TCP"}]
}'::jsonb WHERE id = 'test-k8s-002';

-- ==================== 4. Git 拉取部署项目 ====================
INSERT INTO projects (id, name, description, type, created_at, updated_at) VALUES
('test-gitpull-001', '测试-Git前端项目', '从Git仓库拉取部署前端', 'gitpull', NOW(), NOW()),
('test-gitpull-002', '测试-Git文档站点', 'Git仓库拉取部署文档', 'gitpull', NOW(), NOW());

-- Git 拉取项目配置
UPDATE projects SET gitpull_config = '{
  "repo_url": "https://github.com/example/frontend-app.git",
  "branch": "main",
  "auth_type": "none",
  "work_dir": "/opt/apps/frontend",
  "clean_before": true,
  "backup_before": true,
  "backup_dir": "/data/backup/frontend",
  "backup_keep": 5,
  "pre_script": "#!/bin/bash\necho \"Pulling frontend code...\"\nnpm install",
  "post_script": "#!/bin/bash\necho \"Building frontend...\"\nnpm run build\nsystemctl restart nginx",
  "interpreter": "/bin/bash",
  "clone_timeout": 300,
  "script_timeout": 600
}'::jsonb WHERE id = 'test-gitpull-001';

UPDATE projects SET gitpull_config = '{
  "repo_url": "https://github.com/example/docs-site.git",
  "branch": "main",
  "auth_type": "token",
  "work_dir": "/opt/apps/docs",
  "clean_before": false,
  "backup_before": true,
  "pre_script": "#!/bin/bash\necho \"Pulling docs...\"",
  "post_script": "#!/bin/bash\necho \"Building docs...\"\nmkdocs build",
  "clone_timeout": 180,
  "script_timeout": 300
}'::jsonb WHERE id = 'test-gitpull-002';

-- ==================== 5. 版本数据 ====================

-- 脚本项目版本
INSERT INTO versions (id, project_id, version, description, work_dir, install_script, update_script, rollback_script, uninstall_script, status, deploy_count, created_at, updated_at) VALUES
('ver-script-001-v1', 'test-script-001', 'v1.0.0', 'Web应用初始版本', '/opt/apps/webapp',
'#!/bin/bash
set -e
echo "Installing webapp v1.0.0..."
mkdir -p /opt/apps/webapp/{bin,conf,logs,data}
echo "APP_VERSION=v1.0.0" > /opt/apps/webapp/conf/version.conf
echo "Install completed successfully"',
'#!/bin/bash
set -e
echo "Updating webapp to v1.0.0..."
systemctl stop webapp || true
echo "APP_VERSION=v1.0.0" > /opt/apps/webapp/conf/version.conf
systemctl start webapp
echo "Update completed"',
'#!/bin/bash
echo "Rolling back webapp..."',
'#!/bin/bash
echo "Uninstalling webapp..."
systemctl stop webapp || true
rm -rf /opt/apps/webapp',
'active', 5, NOW(), NOW()),

('ver-script-001-v2', 'test-script-001', 'v1.1.0', 'Web应用功能更新', '/opt/apps/webapp',
'#!/bin/bash
echo "Installing webapp v1.1.0..."',
'#!/bin/bash
set -e
echo "Updating webapp to v1.1.0..."
systemctl stop webapp || true
echo "APP_VERSION=v1.1.0" > /opt/apps/webapp/conf/version.conf
systemctl start webapp
echo "Update completed"',
'#!/bin/bash
echo "Rolling back..."',
'#!/bin/bash
echo "Uninstalling..."',
'active', 3, NOW(), NOW()),

('ver-script-002-v1', 'test-script-002', 'v2.0.0', '微服务初始版本', '/opt/apps/microservice',
'#!/bin/bash
echo "Installing user-service v2.0.0..."',
'#!/bin/bash
echo "Updating user-service..."',
'#!/bin/bash
echo "Rolling back..."',
'#!/bin/bash
echo "Uninstalling..."',
'active', 2, NOW(), NOW());

-- 容器项目版本
INSERT INTO versions (id, project_id, version, description, container_image, container_env, status, deploy_count, created_at, updated_at) VALUES
('ver-container-001-v1', 'test-container-001', 'v1.25.0', 'Nginx 1.25.0 稳定版', 'nginx:1.25.0', 'TZ=Asia/Shanghai\nNGINX_HOST=localhost', 'active', 10, NOW(), NOW()),
('ver-container-001-v2', 'test-container-001', 'v1.26.0', 'Nginx 1.26.0 最新稳定版', 'nginx:1.26.0', 'TZ=Asia/Shanghai\nNGINX_HOST=localhost', 'active', 3, NOW(), NOW()),
('ver-container-002-v1', 'test-container-002', 'v7.2.0', 'Redis 7.2.0', 'redis:7.2-alpine', 'TZ=Asia/Shanghai', 'active', 5, NOW(), NOW()),
('ver-container-003-v1', 'test-container-003', 'v1.0.0', 'API服务初始版本', 'registry.example.com/api-service:v1.0.0', 'TZ=Asia/Shanghai\nLOG_LEVEL=info\nPORT=8080', 'active', 8, NOW(), NOW()),
('ver-container-003-v2', 'test-container-003', 'v1.1.0', 'API服务新增功能', 'registry.example.com/api-service:v1.1.0', 'TZ=Asia/Shanghai\nLOG_LEVEL=debug\nPORT=8080', 'active', 2, NOW(), NOW());

-- 容器版本部署配置（新版配置格式）
UPDATE versions SET deploy_config = '{
  "image": "nginx:1.25.0",
  "environment": {"TZ": "Asia/Shanghai", "NGINX_HOST": "localhost"},
  "resources": {"cpu_limit": "500m", "memory_limit": "256Mi"},
  "pre_script": "#!/bin/bash\necho \"Preparing nginx deployment...\"",
  "post_script": "#!/bin/bash\necho \"Nginx deployment completed\""
}'::jsonb WHERE id = 'ver-container-001-v1';

UPDATE versions SET deploy_config = '{
  "image": "registry.example.com/api-service:v1.1.0",
  "environment": {"TZ": "Asia/Shanghai", "LOG_LEVEL": "debug", "PORT": "8080"},
  "resources": {"cpu_limit": "1", "memory_limit": "1Gi"},
  "health_check": {
    "command": ["CMD", "curl", "-f", "http://localhost:8080/health"],
    "interval": 30,
    "timeout": 10,
    "retries": 3,
    "start_period": 60
  }
}'::jsonb WHERE id = 'ver-container-003-v2';

-- K8s 项目版本
INSERT INTO versions (id, project_id, version, description, container_image, replicas, status, deploy_count, created_at, updated_at) VALUES
('ver-k8s-001-v1', 'test-k8s-001', 'v1.0.0', '微服务初始版本', 'registry.example.com/user-service:v1.0.0', 3, 'active', 5, NOW(), NOW()),
('ver-k8s-001-v2', 'test-k8s-001', 'v1.1.0', '微服务性能优化', 'registry.example.com/user-service:v1.1.0', 3, 'active', 2, NOW(), NOW()),
('ver-k8s-002-v1', 'test-k8s-002', 'v15.0', 'PostgreSQL 15', 'postgres:15-alpine', 1, 'active', 1, NOW(), NOW());

-- K8s 版本部署配置
UPDATE versions SET deploy_config = '{
  "image": "registry.example.com/user-service:v1.1.0",
  "replicas": 3,
  "resources": {
    "cpu_request": "100m",
    "cpu_limit": "500m",
    "memory_request": "128Mi",
    "memory_limit": "512Mi"
  }
}'::jsonb WHERE id = 'ver-k8s-001-v2';

-- Git 项目版本
INSERT INTO versions (id, project_id, version, description, git_ref, git_ref_type, status, deploy_count, created_at, updated_at) VALUES
('ver-git-001-v1', 'test-gitpull-001', 'v1.0.0', '前端初始版本', 'v1.0.0', 'tag', 'active', 3, NOW(), NOW()),
('ver-git-001-v2', 'test-gitpull-001', 'v1.1.0', '前端UI优化', 'v1.1.0', 'tag', 'active', 1, NOW(), NOW()),
('ver-git-001-main', 'test-gitpull-001', 'main-latest', '主分支最新版本', 'main', 'branch', 'active', 0, NOW(), NOW()),
('ver-git-002-v1', 'test-gitpull-002', 'v1.0.0', '文档初始版本', 'v1.0.0', 'tag', 'active', 2, NOW(), NOW());

-- Git 版本脚本配置
UPDATE versions SET
  work_dir = '/opt/apps/frontend',
  pre_script = '#!/bin/bash
set -e
echo "=== Pre-deploy script for v1.1.0 ==="
echo "Installing dependencies..."
npm install --production',
  post_script = '#!/bin/bash
set -e
echo "=== Post-deploy script ==="
echo "Building application..."
npm run build
echo "Restarting nginx..."
systemctl reload nginx
echo "Deployment completed!"'
WHERE id = 'ver-git-001-v2';

-- ==================== 6. 部署任务数据 ====================

-- 已完成的部署任务
INSERT INTO deploy_tasks (id, project_id, version_id, version, operation, client_ids, schedule_type, canary_enabled, status, total_count, success_count, failed_count, pending_count, created_by, created_at, started_at, finished_at, updated_at) VALUES
('task-001', 'test-script-001', 'ver-script-001-v1', 'v1.0.0', 'deploy', '["client-001", "client-002", "client-003"]'::jsonb, 'immediate', false, 'completed', 3, 3, 0, 0, 'admin', NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days' + INTERVAL '5 minutes', NOW() - INTERVAL '2 days'),

('task-002', 'test-container-001', 'ver-container-001-v1', 'v1.25.0', 'deploy', '["client-001", "client-002"]'::jsonb, 'immediate', false, 'completed', 2, 2, 0, 0, 'admin', NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day' + INTERVAL '2 minutes', NOW() - INTERVAL '1 day'),

('task-003', 'test-container-003', 'ver-container-003-v1', 'v1.0.0', 'deploy', '["client-001", "client-002", "client-003", "client-004"]'::jsonb, 'immediate', true, 'completed', 4, 4, 0, 0, 'admin', NOW() - INTERVAL '12 hours', NOW() - INTERVAL '12 hours', NOW() - INTERVAL '11 hours', NOW() - INTERVAL '11 hours');

-- 进行中的部署任务
INSERT INTO deploy_tasks (id, project_id, version_id, version, operation, client_ids, schedule_type, canary_enabled, canary_percent, status, total_count, success_count, failed_count, pending_count, created_by, created_at, started_at, updated_at) VALUES
('task-004', 'test-k8s-001', 'ver-k8s-001-v2', 'v1.1.0', 'deploy', '["k8s-cluster-01", "k8s-cluster-02"]'::jsonb, 'immediate', true, 20, 'canary', 2, 1, 0, 1, 'admin', NOW() - INTERVAL '30 minutes', NOW() - INTERVAL '30 minutes', NOW()),

('task-005', 'test-container-002', 'ver-container-002-v1', 'v7.2.0', 'deploy', '["client-001"]'::jsonb, 'immediate', false, 'running', 1, 0, 0, 1, 'admin', NOW() - INTERVAL '2 minutes', NOW() - INTERVAL '2 minutes', NOW());

-- 待执行的部署任务
INSERT INTO deploy_tasks (id, project_id, version_id, version, operation, client_ids, schedule_type, status, total_count, success_count, failed_count, pending_count, created_by, created_at, updated_at) VALUES
('task-006', 'test-gitpull-001', 'ver-git-001-v2', 'v1.1.0', 'deploy', '["client-001", "client-002"]'::jsonb, 'immediate', 'pending', 2, 0, 0, 2, 'admin', NOW(), NOW());

-- ==================== 7. 部署日志数据 ====================

-- 成功的部署日志
INSERT INTO deploy_logs (id, task_id, project_id, version_id, version, client_id, operation, is_canary, status, exit_code, output, error, started_at, finished_at, duration, created_by, created_at) VALUES
('log-001', 'task-001', 'test-script-001', 'ver-script-001-v1', 'v1.0.0', 'client-001', 'deploy', false, 'success', 0, '#!/bin/bash
set -e
echo "Installing webapp v1.0.0..."
mkdir -p /opt/apps/webapp/{bin,conf,logs,data}
echo "APP_VERSION=v1.0.0" > /opt/apps/webapp/conf/version.conf
echo "Install completed successfully"

Deployment completed in 45 seconds', '', NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days' + INTERVAL '45 seconds', 45, 'admin', NOW() - INTERVAL '2 days'),

('log-002', 'task-001', 'test-script-001', 'ver-script-001-v1', 'v1.0.0', 'client-002', 'deploy', false, 'success', 0, 'Deployment completed successfully', '', NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days' + INTERVAL '38 seconds', 38, 'admin', NOW() - INTERVAL '2 days'),

('log-003', 'task-001', 'test-script-001', 'ver-script-001-v1', 'v1.0.0', 'client-003', 'deploy', false, 'success', 0, 'Deployment completed successfully', '', NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days' + INTERVAL '52 seconds', 52, 'admin', NOW() - INTERVAL '2 days'),

('log-004', 'task-002', 'test-container-001', 'ver-container-001-v1', 'v1.25.0', 'client-001', 'deploy', false, 'success', 0, 'Pulling nginx:1.25.0...
Stopping old container...
Starting new container nginx-web...
Health check passed
Deployment completed', '', NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day' + INTERVAL '90 seconds', 90, 'admin', NOW() - INTERVAL '1 day'),

('log-005', 'task-003', 'test-container-003', 'ver-container-003-v1', 'v1.0.0', 'client-001', 'deploy', true, 'success', 0, 'Canary deployment (20%) completed', '', NOW() - INTERVAL '12 hours', NOW() - INTERVAL '12 hours' + INTERVAL '120 seconds', 120, 'admin', NOW() - INTERVAL '12 hours');

-- 失败的部署日志（示例）
INSERT INTO deploy_logs (id, task_id, project_id, version_id, version, client_id, operation, is_canary, status, exit_code, output, error, started_at, finished_at, duration, created_by, created_at) VALUES
('log-fail-001', 'task-fake-001', 'test-container-001', 'ver-container-001-v2', 'v1.26.0', 'client-003', 'deploy', false, 'failed', 1, 'Pulling nginx:1.26.0...
Starting container...', 'Error: Port 80 already in use\nContainer failed to start', NOW() - INTERVAL '6 hours', NOW() - INTERVAL '6 hours' + INTERVAL '30 seconds', 30, 'admin', NOW() - INTERVAL '6 hours');

-- ==================== 8. 任务执行结果（JSON字段） ====================

UPDATE deploy_tasks SET results = '[
  {"client_id": "client-001", "status": "success", "is_canary": false, "started_at": "' || (NOW() - INTERVAL '2 days')::text || '", "finished_at": "' || (NOW() - INTERVAL '2 days' + INTERVAL '45 seconds')::text || '", "duration": 45},
  {"client_id": "client-002", "status": "success", "is_canary": false, "started_at": "' || (NOW() - INTERVAL '2 days')::text || '", "finished_at": "' || (NOW() - INTERVAL '2 days' + INTERVAL '38 seconds')::text || '", "duration": 38},
  {"client_id": "client-003", "status": "success", "is_canary": false, "started_at": "' || (NOW() - INTERVAL '2 days')::text || '", "finished_at": "' || (NOW() - INTERVAL '2 days' + INTERVAL '52 seconds')::text || '", "duration": 52}
]'::jsonb WHERE id = 'task-001';

UPDATE deploy_tasks SET results = '[
  {"client_id": "k8s-cluster-01", "status": "success", "is_canary": true, "started_at": "' || (NOW() - INTERVAL '30 minutes')::text || '", "duration": 180},
  {"client_id": "k8s-cluster-02", "status": "pending", "is_canary": false, "started_at": null, "duration": 0}
]'::jsonb WHERE id = 'task-004';

-- ==================== 9. 容器命名配置 ====================
UPDATE projects SET container_naming = '{
  "prefix": "webapp",
  "separator": "-",
  "include_env": true,
  "include_ver": true,
  "max_length": 63,
  "template": "${PREFIX}${SEPARATOR}${ENV}${SEPARATOR}${VERSION}"
}'::jsonb WHERE id = 'test-container-001';

UPDATE projects SET container_naming = '{
  "prefix": "api",
  "separator": "-",
  "include_env": true,
  "include_ver": false,
  "max_length": 63
}'::jsonb WHERE id = 'test-container-003';

-- ==================== 输出结果 ====================
SELECT '===== 测试数据已成功创建 =====' AS message;

SELECT '项目统计' AS category, type, COUNT(*) AS count
FROM projects
GROUP BY type
ORDER BY type;

SELECT '版本统计' AS category, p.type, COUNT(v.id) AS version_count
FROM projects p
LEFT JOIN versions v ON p.id = v.project_id
GROUP BY p.type
ORDER BY p.type;

SELECT '任务统计' AS category, status, COUNT(*) AS count
FROM deploy_tasks
GROUP BY status
ORDER BY status;
