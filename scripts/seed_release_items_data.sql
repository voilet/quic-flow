-- 发布项层级结构测试数据脚本
-- 演示：一个项目包含多个发布项（前端、后端、数据库等）
-- 使用方法: psql -h localhost -U postgres -d quic_flow -f seed_release_items_data.sql

-- 清理现有测试数据（可选，取消注释以启用）
-- DELETE FROM deploy_logs WHERE task_id IN (SELECT id FROM deploy_tasks WHERE release_item_id LIKE 'item-%');
-- DELETE FROM deploy_tasks WHERE release_item_id LIKE 'item-%';
-- DELETE FROM versions WHERE release_item_id LIKE 'item-%';
-- DELETE FROM release_items WHERE id LIKE 'item-%';
-- DELETE FROM projects WHERE id LIKE 'proj-ecom%';

-- ==================== 项目：电商系统 ====================
INSERT INTO projects (id, name, description, category, type, created_at, updated_at) VALUES
('proj-ecom-001', '电商系统', '完整的电商业务系统，包含前端、后端API、数据库等多个服务组件', 'web', 'container', NOW(), NOW());

-- ==================== 发布项 1：前端 Web ====================
INSERT INTO release_items (
  id, project_id, name, description, type, sort_order,
  container_config, created_at, updated_at
) VALUES (
  'item-ecom-frontend', 'proj-ecom-001', '前端 Web', 'Vue.js 前端应用', 'container', 1,
  '{
    "image": "registry.example.com/ecom-frontend:latest",
    "container_name": "ecom-frontend",
    "restart_policy": "always",
    "ports": [{"host_port": 80, "container_port": 80, "protocol": "tcp"}],
    "volumes": [
      {"host_path": "/data/ecom/frontend/dist", "container_path": "/usr/share/nginx/html", "read_only": false}
    ],
    "memory_limit": "256m",
    "cpu_limit": "0.5",
    "environment": {"TZ": "Asia/Shanghai", "API_BASE_URL": "https://api.example.com"}
  }'::jsonb,
  NOW(), NOW()
);

-- 前端版本
INSERT INTO versions (id, project_id, release_item_id, version, description, container_image, status, created_at, updated_at) VALUES
('ver-fe-v101', 'proj-ecom-001', 'item-ecom-frontend', 'v1.0.1', '首页优化版本', 'registry.example.com/ecom-frontend:v1.0.1', 'active', NOW(), NOW()),
('ver-fe-v102', 'proj-ecom-001', 'item-ecom-frontend', 'v1.0.2', '购物车功能增强', 'registry.example.com/ecom-frontend:v1.0.2', 'active', NOW(), NOW()),
('ver-fe-v110', 'proj-ecom-001', 'item-ecom-frontend', 'v1.1.0', '新增订单追踪功能', 'registry.example.com/ecom-frontend:v1.1.0', 'draft', NOW(), NOW());

-- ==================== 发布项 2：后端 API ====================
INSERT INTO release_items (
  id, project_id, name, description, type, repo_url, sort_order,
  container_config, created_at, updated_at
) VALUES (
  'item-ecom-api', 'proj-ecom-001', '后端 API', 'Go 语言后端 API 服务', 'container',
  'https://github.com/example/ecom-api.git', 2,
  '{
    "image": "registry.example.com/ecom-api:latest",
    "container_name": "ecom-api",
    "restart_policy": "unless-stopped",
    "ports": [{"host_port": 8080, "container_port": 8080, "protocol": "tcp"}],
    "volumes": [
      {"host_path": "/data/ecom/api/logs", "container_path": "/app/logs", "read_only": false},
      {"host_path": "/data/ecom/api/config", "container_path": "/app/config", "read_only": true}
    ],
    "memory_limit": "1g",
    "cpu_limit": "1",
    "environment": {
      "TZ": "Asia/Shanghai",
      "LOG_LEVEL": "info",
      "DB_HOST": "postgres.default.svc.cluster.local",
      "REDIS_HOST": "redis.default.svc.cluster.local"
    },
    "health_check": {
      "command": ["CMD", "curl", "-f", "http://localhost:8080/health"],
      "interval": 30,
      "timeout": 10,
      "retries": 3,
      "start_period": 60
    }
  }'::jsonb,
  NOW(), NOW()
);

-- 后端 API 版本
INSERT INTO versions (id, project_id, release_item_id, version, description, container_image, status, created_at, updated_at) VALUES
('ver-api-v200', 'proj-ecom-001', 'item-ecom-api', 'v2.0.0', 'API 2.0 重构版本', 'registry.example.com/ecom-api:v2.0.0', 'active', NOW(), NOW()),
('ver-api-v201', 'proj-ecom-001', 'item-ecom-api', 'v2.0.1', '性能优化版本', 'registry.example.com/ecom-api:v2.0.1', 'active', NOW(), NOW()),
('ver-api-v210', 'proj-ecom-001', 'item-ecom-api', 'v2.1.0', '新增推荐系统接口', 'registry.example.com/ecom-api:v2.1.0', 'active', NOW(), NOW());

-- ==================== 发布项 3：订单服务（K8s 部署） ====================
INSERT INTO release_items (
  id, project_id, name, description, type, repo_url, sort_order,
  kubernetes_config, created_at, updated_at
) VALUES (
  'item-ecom-order', 'proj-ecom-001', '订单服务', '订单处理微服务（Kubernetes 部署）', 'kubernetes',
  'https://github.com/example/ecom-order.git', 3,
  '{
    "namespace": "production",
    "resource_type": "deployment",
    "resource_name": "order-service",
    "container_name": "order-service",
    "image": "registry.example.com/ecom-order:latest",
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
  }'::jsonb,
  NOW(), NOW()
);

-- 订单服务版本
INSERT INTO versions (id, project_id, release_item_id, version, description, container_image, replicas, status, created_at, updated_at) VALUES
('ver-order-v100', 'proj-ecom-001', 'item-ecom-order', 'v1.0.0', '订单服务初始版本', 'registry.example.com/ecom-order:v1.0.0', 3, 'active', NOW(), NOW()),
('ver-order-v110', 'proj-ecom-001', 'item-ecom-order', 'v1.1.0', '支持分布式事务', 'registry.example.com/ecom-order:v1.1.0', 3, 'active', NOW(), NOW());

-- ==================== 发布项 4：管理后台（脚本部署） ====================
INSERT INTO release_items (
  id, project_id, name, description, type, sort_order,
  script_config, created_at, updated_at
) VALUES (
  'item-ecom-admin', 'proj-ecom-001', '管理后台', 'React 管理后台（脚本部署）', 'script', 4,
  '{
    "work_dir": "/opt/apps/ecom-admin",
    "interpreter": "/bin/bash",
    "environment": {
      "APP_NAME": "ecom-admin",
      "NODE_ENV": "production"
    },
    "install_script": "#!/bin/bash\nset -e\necho \"Installing admin panel...\"\nmkdir -p /opt/apps/ecom-admin/{dist,logs}\necho \"Install completed\"",
    "update_script": "#!/bin/bash\nset -e\necho \"Updating admin panel...\"\ncd /opt/apps/ecom-admin\nsystemctl reload nginx\necho \"Update completed\"",
    "rollback_script": "#!/bin/bash\nset -e\necho \"Rolling back admin panel...\"\necho \"Rollback completed\"",
    "uninstall_script": "#!/bin/bash\nset -e\necho \"Uninstalling admin panel...\"\nrm -rf /opt/apps/ecom-admin\necho \"Uninstall completed\"",
    "timeouts": {"install": 300, "update": 180, "rollback": 120, "uninstall": 60}
  }'::jsonb,
  NOW(), NOW()
);

-- 管理后台版本
INSERT INTO versions (id, project_id, release_item_id, version, description, status, created_at, updated_at) VALUES
('ver-admin-v100', 'proj-ecom-001', 'item-ecom-admin', 'v1.0.0', '管理后台初始版本', 'active', NOW(), NOW());

-- ==================== 项目 2：内容管理系统 ====================
INSERT INTO projects (id, name, description, category, type, created_at, updated_at) VALUES
('proj-cms-001', '内容管理系统', '企业内容管理平台', 'api', 'container', NOW(), NOW());

-- ==================== 发布项：CMS API ====================
INSERT INTO release_items (
  id, project_id, name, description, type, sort_order,
  container_config, created_at, updated_at
) VALUES (
  'item-cms-api', 'proj-cms-001', 'CMS API', '内容管理 API 服务', 'container', 1,
  '{
    "image": "registry.example.com/cms-api:latest",
    "container_name": "cms-api",
    "restart_policy": "always",
    "ports": [{"host_port": 9000, "container_port": 9000, "protocol": "tcp"}],
    "memory_limit": "512m",
    "cpu_limit": "0.5"
  }'::jsonb,
  NOW(), NOW()
);

-- CMS API 版本
INSERT INTO versions (id, project_id, release_item_id, version, description, container_image, status, created_at, updated_at) VALUES
('ver-cms-v100', 'proj-cms-001', 'item-cms-api', 'v1.0.0', 'CMS API 初始版本', 'registry.example.com/cms-api:v1.0.0', 'active', NOW(), NOW()),
('ver-cms-v110', 'proj-cms-001', 'item-cms-api', 'v1.1.0', '新增富文本编辑功能', 'registry.example.com/cms-api:v1.1.0', 'active', NOW(), NOW());

-- ==================== 发布项：CMS Worker（后台任务） ====================
INSERT INTO release_items (
  id, project_id, name, description, type, sort_order,
  container_config, created_at, updated_at
) VALUES (
  'item-cms-worker', 'proj-cms-001', 'CMS Worker', '后台任务处理服务', 'container', 2,
  '{
    "image": "registry.example.com/cms-worker:latest",
    "container_name": "cms-worker",
    "restart_policy": "always",
    "memory_limit": "256m",
    "cpu_limit": "0.25",
    "environment": {"WORKER_TYPE": "image-processor"}
  }'::jsonb,
  NOW(), NOW()
);

-- CMS Worker 版本
INSERT INTO versions (id, project_id, release_item_id, version, description, container_image, status, created_at, updated_at) VALUES
('ver-worker-v100', 'proj-cms-001', 'item-cms-worker', 'v1.0.0', 'Worker 初始版本', 'registry.example.com/cms-worker:v1.0.0', 'active', NOW(), NOW());

-- ==================== 验证数据 ====================
SELECT '=== 项目列表 ===' AS info;
SELECT id, name, description, category FROM projects WHERE id LIKE 'proj-%';

SELECT '=== 发布项列表 ===' AS info;
SELECT ri.id, ri.name, ri.type, p.name AS project_name
FROM release_items ri
JOIN projects p ON ri.project_id = p.id
WHERE ri.id LIKE 'item-%'
ORDER BY p.name, ri.sort_order;

SELECT '=== 版本列表 ===' AS info;
SELECT v.version, v.description, ri.name AS release_item_name, p.name AS project_name
FROM versions v
JOIN release_items ri ON v.release_item_id = ri.id
JOIN projects p ON ri.project_id = p.id
WHERE v.release_item_id LIKE 'item-%'
ORDER BY p.name, ri.name, v.version;

-- 完成提示
SELECT '测试数据已成功创建！' AS message,
       (SELECT COUNT(*) FROM projects WHERE id LIKE 'proj-%') AS projects_count,
       (SELECT COUNT(*) FROM release_items WHERE id LIKE 'item-%') AS release_items_count,
       (SELECT COUNT(*) FROM versions WHERE release_item_id LIKE 'item-%') AS versions_count;
