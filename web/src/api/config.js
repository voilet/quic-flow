import { request } from './index.js'

/**
 * 配置中心 API
 */
export const configApi = {
  // ==================== 配置管理 ====================

  /**
   * 获取配置列表
   * @param {Object} params - 查询参数 {namespace, group, data_id, type, tags, page, page_size}
   */
  listConfigs(params) {
    return request.get('/config', { params })
  },

  /**
   * 获取配置详情
   * @param {string} id - 配置 ID
   */
  getConfig(id) {
    return request.get(`/config/${id}`)
  },

  /**
   * 创建配置
   * @param {Object} data - 配置数据
   */
  createConfig(data) {
    return request.post('/config', data)
  },

  /**
   * 更新配置
   * @param {string} id - 配置 ID
   * @param {Object} data - 配置数据
   */
  updateConfig(id, data) {
    return request.put(`/config/${id}`, data)
  },

  /**
   * 删除配置
   * @param {string} id - 配置 ID
   */
  deleteConfig(id) {
    return request.delete(`/config/${id}`)
  },

  /**
   * 发布配置
   * @param {string} id - 配置 ID
   * @param {Object} data - 发布数据 {gray_rule?, comment?}
   */
  publishConfig(id, data) {
    return request.post(`/config/${id}/publish`, data)
  },

  /**
   * 验证配置内容（YAML/JSON 语法验证）
   * @param {Object} data - {content, type}
   */
  validateConfig(data) {
    return request.post('/config/validate', data)
  },

  /**
   * 配置对比
   * @param {string} id - 配置 ID
   * @param {Object} params - {from_version, to_version}
   */
  compareConfig(id, params) {
    return request.get(`/config/${id}/diff`, { params })
  },

  // ==================== 发布管理 ====================

  /**
   * 获取发布记录列表
   * @param {Object} params - 查询参数
   */
  listReleases(params) {
    return request.get('/config/releases', { params })
  },

  /**
   * 获取发布详情
   * @param {string} releaseId - 发布 ID
   */
  getRelease(releaseId) {
    return request.get(`/config/releases/${releaseId}`)
  },

  /**
   * 获取发布状态
   * @param {string} releaseId - 发布 ID
   */
  getReleaseStatus(releaseId) {
    return request.get(`/config/releases/${releaseId}/status`)
  },

  /**
   * 获取发布事件流 URL（用于 SSE 订阅）
   * @param {string} releaseId - 发布 ID
   */
  getReleaseEventsUrl(releaseId) {
    return `/api/config/releases/${releaseId}/events`
  },

  /**
   * 回滚发布
   * @param {string} id - 配置 ID
   * @param {Object} data - {to_version, comment?}
   */
  rollbackConfig(id, data) {
    return request.post(`/config/${id}/rollback`, data)
  },

  /**
   * 取消发布
   * @param {string} releaseId - 发布 ID
   */
  cancelRelease(releaseId) {
    return request.post(`/config/releases/${releaseId}/cancel`)
  },

  // ==================== 灰度规则 ====================

  /**
   * 获取灰度规则列表
   * @param {string} configId - 配置 ID
   */
  listGrayRules(configId) {
    return request.get(`/config/${configId}/gray-rules`)
  },

  /**
   * 创建灰度规则
   * @param {string} configId - 配置 ID
   * @param {Object} data - 规则数据
   */
  createGrayRule(configId, data) {
    return request.post(`/config/${configId}/gray-rules`, data)
  },

  /**
   * 更新灰度规则
   * @param {string} configId - 配置 ID
   * @param {string} ruleId - 规则 ID
   * @param {Object} data - 规则数据
   */
  updateGrayRule(configId, ruleId, data) {
    return request.put(`/config/${configId}/gray-rules/${ruleId}`, data)
  },

  /**
   * 删除灰度规则
   * @param {string} configId - 配置 ID
   * @param {string} ruleId - 规则 ID
   */
  deleteGrayRule(configId, ruleId) {
    return request.delete(`/config/${configId}/gray-rules/${ruleId}`)
  },

  /**
   * 全量发布灰度配置
   * @param {string} configId - 配置 ID
   * @param {string} ruleId - 规则 ID
   */
  promoteGrayRule(configId, ruleId) {
    return request.post(`/config/${configId}/gray-rules/${ruleId}/promote`)
  },

  // ==================== 配置历史 ====================

  /**
   * 获取配置变更历史
   * @param {string} id - 配置 ID
   * @param {Object} params - 查询参数
   */
  listConfigHistory(id, params) {
    return request.get(`/config/${id}/history`, { params })
  },

  /**
   * 获取历史版本详情
   * @param {string} id - 配置 ID
   * @param {string} version - 版本号
   */
  getHistoryVersion(id, version) {
    return request.get(`/config/${id}/history/${version}`)
  },

  /**
   * 获取配置操作日志
   * @param {Object} params - 查询参数
   */
  listOperationLogs(params) {
    return request.get('/config/logs', { params })
  },

  // ==================== 命名空间与分组 ====================

  /**
   * 获取命名空间列表
   */
  listNamespaces() {
    return request.get('/config/namespaces')
  },

  /**
   * 创建命名空间
   * @param {Object} data - {name, description}
   */
  createNamespace(data) {
    return request.post('/config/namespaces', data)
  },

  /**
   * 删除命名空间
   * @param {string} name - 命名空间名称
   */
  deleteNamespace(name) {
    return request.delete(`/config/namespaces/${name}`)
  },

  /**
   * 获取分组列表
   * @param {string} namespace - 命名空间
   */
  listGroups(namespace) {
    return request.get(`/config/namespaces/${namespace}/groups`)
  },

  /**
   * 创建分组
   * @param {string} namespace - 命名空间
   * @param {Object} data - {name, description}
   */
  createGroup(namespace, data) {
    return request.post(`/config/namespaces/${namespace}/groups`, data)
  },

  /**
   * 删除分组
   * @param {string} namespace - 命名空间
   * @param {string} name - 分组名称
   */
  deleteGroup(namespace, name) {
    return request.delete(`/config/namespaces/${namespace}/groups/${name}`)
  },

  // ==================== 标签管理 ====================

  /**
   * 获取所有标签
   */
  listTags() {
    return request.get('/config/tags')
  },

  /**
   * 批量操作配置
   * @param {Object} data - {action, config_ids, data?}
   */
  batchAction(data) {
    return request.post('/config/batch', data)
  }
}
