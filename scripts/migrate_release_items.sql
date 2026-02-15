-- 发布项层级结构迁移脚本
-- 将现有项目结构升级为 Project → ReleaseItem → Version 层级结构

-- 1. 创建 release_items 表
CREATE TABLE IF NOT EXISTS release_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description VARCHAR(500),
    type VARCHAR(20) NOT NULL DEFAULT 'script',
    repo_url VARCHAR(500),
    repo_type VARCHAR(20),
    sort_order INTEGER DEFAULT 0,
    script_config JSONB,
    container_config JSONB,
    kubernetes_config JSONB,
    git_pull_config JSONB,
    container_naming JSONB,
    callback_config JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_release_items_project_id ON release_items(project_id);
CREATE INDEX IF NOT EXISTS idx_release_items_deleted_at ON release_items(deleted_at);

-- 2. 给 versions 表添加 release_item_id 字段
ALTER TABLE versions ADD COLUMN IF NOT EXISTS release_item_id UUID REFERENCES release_items(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_versions_release_item_id ON versions(release_item_id);

-- 3. 给 deploy_tasks 表添加 release_item_id 字段
ALTER TABLE deploy_tasks ADD COLUMN IF NOT EXISTS release_item_id UUID REFERENCES release_items(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_deploy_tasks_release_item_id ON deploy_tasks(release_item_id);

-- 4. 为每个现有项目创建对应的 ReleaseItem（数据迁移）
-- 将项目级别的配置迁移到发布项级别
INSERT INTO release_items (id, project_id, name, description, type, repo_url, sort_order,
    script_config, container_config, kubernetes_config, git_pull_config, created_at, updated_at)
SELECT
    gen_random_uuid() as id,
    p.id as project_id,
    p.name as name,
    COALESCE(p.description, '') as description,
    p.type as type,
    NULL as repo_url,
    0 as sort_order,
    p.script_config,
    p.container_config,
    p.kubernetes_config,
    p.git_pull_config,
    p.created_at,
    p.updated_at
FROM projects p
WHERE NOT EXISTS (
    SELECT 1 FROM release_items ri WHERE ri.project_id = p.id
);

-- 5. 更新 versions 表，关联到对应的 release_item
UPDATE versions v
SET release_item_id = ri.id
FROM release_items ri
WHERE v.project_id = ri.project_id
AND v.release_item_id IS NULL;

-- 6. 更新 deploy_tasks 表，关联到对应的 release_item
UPDATE deploy_tasks dt
SET release_item_id = ri.id
FROM release_items ri
WHERE dt.project_id = ri.project_id
AND dt.release_item_id IS NULL;

-- 验证迁移结果
SELECT '迁移完成！' as status,
       (SELECT COUNT(*) FROM projects) as projects_count,
       (SELECT COUNT(*) FROM release_items) as release_items_count,
       (SELECT COUNT(*) FROM versions WHERE release_item_id IS NOT NULL) as versions_with_item,
       (SELECT COUNT(*) FROM versions WHERE release_item_id IS NULL) as versions_without_item;
