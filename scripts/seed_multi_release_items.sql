-- 为现有项目添加多个发布项的测试数据
-- 演示：一个项目包含多个发布项（前端、后端、数据库等）

-- ==================== 1. 为 "Web Application (Script)" 添加更多发布项 ====================

-- 添加前端发布项
INSERT INTO release_items (project_id, name, description, type, sort_order, script_config, created_at, updated_at) VALUES
('7994e4f7-cbac-4e6c-b7c1-2bf271b2e976', '前端资源', 'Vue.js 前端静态资源部署', 'script', 1,
'{
  "work_dir": "/opt/webapp/frontend",
  "interpreter": "/bin/bash",
  "environment": {"NODE_ENV": "production", "APP_NAME": "webapp-frontend"},
  "install_script": "#!/bin/bash\nset -e\necho \"Installing frontend...\"\nmkdir -p /opt/webapp/frontend\necho \"Frontend installed\"",
  "update_script": "#!/bin/bash\nset -e\necho \"Updating frontend...\"\ncp -r ./dist/* /opt/webapp/frontend/\necho \"Frontend updated\"",
  "rollback_script": "#!/bin/bash\necho \"Rollback frontend...\"",
  "uninstall_script": "#!/bin/bash\nrm -rf /opt/webapp/frontend",
  "timeouts": {"install": 300, "update": 180, "rollback": 120, "uninstall": 60}
}'::jsonb, NOW(), NOW());

-- 保存前端发布项 ID
\set fe_ri_id (SELECT id FROM release_items WHERE project_id = '7994e4f7-cbac-4e6c-b7c1-2bf271b2e976' AND name = '前端资源')

-- 添加后端发布项
INSERT INTO release_items (project_id, name, description, type, sort_order, script_config, created_at, updated_at) VALUES
('7994e4f7-cbac-4e6c-b7c1-2bf271b2e976', '后端服务', 'Go 后端 API 服务部署', 'script', 2,
'{
  "work_dir": "/opt/webapp/backend",
  "interpreter": "/bin/bash",
  "environment": {"SERVICE_NAME": "webapp-api", "PORT": "8080"},
  "install_script": "#!/bin/bash\nset -e\necho \"Installing backend...\"\nmkdir -p /opt/webapp/backend/{bin,conf,logs}\necho \"Backend installed\"",
  "update_script": "#!/bin/bash\nset -e\necho \"Updating backend...\"\nsystemctl restart webapp-api\necho \"Backend updated\"",
  "rollback_script": "#!/bin/bash\necho \"Rollback backend...\"",
  "uninstall_script": "#!/bin/bash\nsystemctl stop webapp-api\nrm -rf /opt/webapp/backend",
  "timeouts": {"install": 600, "update": 300, "rollback": 180, "uninstall": 120}
}'::jsonb, NOW(), NOW());

-- 为前端添加版本
INSERT INTO versions (project_id, release_item_id, version, description, status, created_at, updated_at)
SELECT '7994e4f7-cbac-4e6c-b7c1-2bf271b2e976', id, 'v1.0.1', '首页优化', 'active', NOW(), NOW()
FROM release_items WHERE project_id = '7994e4f7-cbac-4e6c-b7c1-2bf271b2e976' AND name = '前端资源';

INSERT INTO versions (project_id, release_item_id, version, description, status, created_at, updated_at)
SELECT '7994e4f7-cbac-4e6c-b7c1-2bf271b2e976', id, 'v1.0.2', '性能优化', 'active', NOW(), NOW()
FROM release_items WHERE project_id = '7994e4f7-cbac-4e6c-b7c1-2bf271b2e976' AND name = '前端资源';

-- 为后端添加版本
INSERT INTO versions (project_id, release_item_id, version, description, status, created_at, updated_at)
SELECT '7994e4f7-cbac-4e6c-b7c1-2bf271b2e976', id, 'v2.0.0', 'API 重构版本', 'active', NOW(), NOW()
FROM release_items WHERE project_id = '7994e4f7-cbac-4e6c-b7c1-2bf271b2e976' AND name = '后端服务';

INSERT INTO versions (project_id, release_item_id, version, description, status, created_at, updated_at)
SELECT '7994e4f7-cbac-4e6c-b7c1-2bf271b2e976', id, 'v2.1.0', '新增用户模块', 'active', NOW(), NOW()
FROM release_items WHERE project_id = '7994e4f7-cbac-4e6c-b7c1-2bf271b2e976' AND name = '后端服务';

-- ==================== 2. 为 "API Gateway (Docker)" 添加更多发布项 ====================

-- 添加网关配置发布项
INSERT INTO release_items (project_id, name, description, type, sort_order, script_config, created_at, updated_at) VALUES
('15946640-446e-4eec-bd08-aeff517fc1ba', '网关配置', 'Nginx 网关配置更新', 'script', 1,
'{
  "work_dir": "/opt/gateway/config",
  "interpreter": "/bin/bash",
  "environment": {"GATEWAY_NAME": "api-gateway"},
  "install_script": "#!/bin/bash\nmkdir -p /opt/gateway/config",
  "update_script": "#!/bin/bash\nset -e\ncp ./nginx.conf /opt/gateway/config/\nnginx -s reload\necho \"Config updated\"",
  "rollback_script": "#!/bin/bash\ncp ./nginx.conf.bak /opt/gateway/config/nginx.conf\nnginx -s reload",
  "uninstall_script": "#!/bin/bash\nrm -rf /opt/gateway/config",
  "timeouts": {"install": 60, "update": 60, "rollback": 60, "uninstall": 30}
}'::jsonb, NOW(), NOW());

-- 添加监控插件发布项
INSERT INTO release_items (project_id, name, description, type, sort_order, container_config, created_at, updated_at) VALUES
('15946640-446e-4eec-bd08-aeff517fc1ba', '监控插件', 'Prometheus 监控导出器', 'container', 2,
'{
  "image": "prom/nginx-prometheus-exporter:latest",
  "container_name": "gateway-exporter",
  "restart_policy": "always",
  "ports": [{"host_port": 9113, "container_port": 9113, "protocol": "tcp"}],
  "memory_limit": "128m",
  "cpu_limit": "0.1"
}'::jsonb, NOW(), NOW());

-- 为网关配置添加版本
INSERT INTO versions (project_id, release_item_id, version, description, status, created_at, updated_at)
SELECT '15946640-446e-4eec-bd08-aeff517fc1ba', id, 'v1.0.0', '初始配置', 'active', NOW(), NOW()
FROM release_items WHERE project_id = '15946640-446e-4eec-bd08-aeff517fc1ba' AND name = '网关配置';

INSERT INTO versions (project_id, release_item_id, version, description, status, created_at, updated_at)
SELECT '15946640-446e-4eec-bd08-aeff517fc1ba', id, 'v1.1.0', '添加限流配置', 'active', NOW(), NOW()
FROM release_items WHERE project_id = '15946640-446e-4eec-bd08-aeff517fc1ba' AND name = '网关配置';

-- ==================== 3. 为 "Frontend App (Git Pull)" 添加更多发布项 ====================

-- 添加文档站点发布项
INSERT INTO release_items (project_id, name, description, type, repo_url, sort_order, git_pull_config, created_at, updated_at) VALUES
('75687c21-2c4d-446c-9950-1f42b1e441b9', '文档站点', '产品文档站点', 'gitpull',
 'https://github.com/example/docs.git', 1,
'{
  "repo_url": "https://github.com/example/docs.git",
  "branch": "main",
  "work_dir": "/opt/frontend/docs",
  "clean_before": true,
  "backup_before": true,
  "backup_dir": "/data/backup/docs",
  "backup_keep": 3,
  "post_script": "#!/bin/bash\ncd /opt/frontend/docs\nnpm run build",
  "clone_timeout": 120,
  "script_timeout": 300
}'::jsonb, NOW(), NOW());

-- 添加移动端 H5 发布项
INSERT INTO release_items (project_id, name, description, type, repo_url, sort_order, git_pull_config, created_at, updated_at) VALUES
('75687c21-2c4d-446c-9950-1f42b1e441b9', '移动端 H5', '移动端 H5 应用', 'gitpull',
 'https://github.com/example/h5-app.git', 2,
'{
  "repo_url": "https://github.com/example/h5-app.git",
  "branch": "main",
  "work_dir": "/opt/frontend/h5",
  "clean_before": true,
  "post_script": "#!/bin/bash\ncd /opt/frontend/h5\nnpm install && npm run build",
  "clone_timeout": 180,
  "script_timeout": 600
}'::jsonb, NOW(), NOW());

-- 为文档站点添加版本
INSERT INTO versions (project_id, release_item_id, version, git_ref, git_ref_type, description, status, created_at, updated_at)
SELECT '75687c21-2c4d-446c-9950-1f42b1e441b9', id, 'v1.0.0', 'v1.0.0', 'tag', '初始版本', 'active', NOW(), NOW()
FROM release_items WHERE project_id = '75687c21-2c4d-446c-9950-1f42b1e441b9' AND name = '文档站点';

-- ==================== 4. 为 "Microservice API (Kubernetes)" 添加更多发布项 ====================

-- 添加用户服务发布项
INSERT INTO release_items (project_id, name, description, type, repo_url, sort_order, kubernetes_config, created_at, updated_at) VALUES
('2b34600b-ef04-454d-a062-8b3a2d859e16', '用户服务', '用户管理微服务', 'kubernetes',
 'https://github.com/example/user-service.git', 1,
'{
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
  "service_ports": [{"name": "grpc", "port": 9090, "target_port": 9090}],
  "rollout_timeout": 300
}'::jsonb, NOW(), NOW());

-- 添加订单服务发布项
INSERT INTO release_items (project_id, name, description, type, repo_url, sort_order, kubernetes_config, created_at, updated_at) VALUES
('2b34600b-ef04-454d-a062-8b3a2d859e16', '订单服务', '订单处理微服务', 'kubernetes',
 'https://github.com/example/order-service.git', 2,
'{
  "namespace": "production",
  "resource_type": "deployment",
  "resource_name": "order-service",
  "container_name": "order-service",
  "image": "registry.example.com/order-service:latest",
  "replicas": 2,
  "update_strategy": "RollingUpdate",
  "max_unavailable": "50%",
  "max_surge": "25%",
  "cpu_request": "200m",
  "cpu_limit": "1000m",
  "memory_request": "256Mi",
  "memory_limit": "1Gi",
  "environment": {"TZ": "Asia/Shanghai"},
  "service_type": "ClusterIP",
  "service_ports": [{"name": "grpc", "port": 9091, "target_port": 9091}],
  "rollout_timeout": 300
}'::jsonb, NOW(), NOW());

-- 添加支付服务发布项
INSERT INTO release_items (project_id, name, description, type, repo_url, sort_order, kubernetes_config, created_at, updated_at) VALUES
('2b34600b-ef04-454d-a062-8b3a2d859e16', '支付服务', '支付处理微服务', 'kubernetes',
 'https://github.com/example/payment-service.git', 3,
'{
  "namespace": "production",
  "resource_type": "deployment",
  "resource_name": "payment-service",
  "container_name": "payment-service",
  "image": "registry.example.com/payment-service:latest",
  "replicas": 2,
  "cpu_request": "100m",
  "cpu_limit": "500m",
  "memory_request": "128Mi",
  "memory_limit": "512Mi",
  "service_type": "ClusterIP",
  "service_ports": [{"name": "http", "port": 8080, "target_port": 8080}],
  "rollout_timeout": 180
}'::jsonb, NOW(), NOW());

-- 为用户服务添加版本
INSERT INTO versions (project_id, release_item_id, version, container_image, replicas, description, status, created_at, updated_at)
SELECT '2b34600b-ef04-454d-a062-8b3a2d859e16', id, 'v1.0.0', 'registry.example.com/user-service:v1.0.0', 3, '初始版本', 'active', NOW(), NOW()
FROM release_items WHERE project_id = '2b34600b-ef04-454d-a062-8b3a2d859e16' AND name = '用户服务';

INSERT INTO versions (project_id, release_item_id, version, container_image, replicas, description, status, created_at, updated_at)
SELECT '2b34600b-ef04-454d-a062-8b3a2d859e16', id, 'v1.1.0', 'registry.example.com/user-service:v1.1.0', 3, '新增权限管理', 'active', NOW(), NOW()
FROM release_items WHERE project_id = '2b34600b-ef04-454d-a062-8b3a2d859e16' AND name = '用户服务';

-- 为订单服务添加版本
INSERT INTO versions (project_id, release_item_id, version, container_image, replicas, description, status, created_at, updated_at)
SELECT '2b34600b-ef04-454d-a062-8b3a2d859e16', id, 'v1.0.0', 'registry.example.com/order-service:v1.0.0', 2, '初始版本', 'active', NOW(), NOW()
FROM release_items WHERE project_id = '2b34600b-ef04-454d-a062-8b3a2d859e16' AND name = '订单服务';

INSERT INTO versions (project_id, release_item_id, version, container_image, replicas, description, status, created_at, updated_at)
SELECT '2b34600b-ef04-454d-a062-8b3a2d859e16', id, 'v1.2.0', 'registry.example.com/order-service:v1.2.0', 3, '支持分布式事务', 'active', NOW(), NOW()
FROM release_items WHERE project_id = '2b34600b-ef04-454d-a062-8b3a2d859e16' AND name = '订单服务';

-- 为支付服务添加版本
INSERT INTO versions (project_id, release_item_id, version, container_image, replicas, description, status, created_at, updated_at)
SELECT '2b34600b-ef04-454d-a062-8b3a2d859e16', id, 'v1.0.0', 'registry.example.com/payment-service:v1.0.0', 2, '初始版本', 'active', NOW(), NOW()
FROM release_items WHERE project_id = '2b34600b-ef04-454d-a062-8b3a2d859e16' AND name = '支付服务';

-- ==================== 验证数据 ====================
SELECT '=== 项目与发布项统计 ===' AS info;

SELECT
  p.name AS project_name,
  COUNT(DISTINCT ri.id) AS release_items_count,
  COUNT(DISTINCT v.id) AS versions_count
FROM projects p
LEFT JOIN release_items ri ON ri.project_id = p.id
LEFT JOIN versions v ON v.release_item_id = ri.id
GROUP BY p.id, p.name
ORDER BY versions_count DESC, p.name;

SELECT '=== 发布项详情 ===' AS info;

SELECT
  p.name AS project_name,
  ri.name AS release_item_name,
  ri.type,
  (SELECT COUNT(*) FROM versions v WHERE v.release_item_id = ri.id) AS versions_count
FROM release_items ri
JOIN projects p ON ri.project_id = p.id
ORDER BY p.name, ri.sort_order;

SELECT '测试数据添加完成！' AS message;
