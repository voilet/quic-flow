import { request } from './index'

/**
 * 告警系统 API
 */

// ===== 规则管理 =====

/**
 * 创建告警规则
 * @param {Object} data - 规则数据
 * @returns {Promise}
 */
export function createAlertRule(data) {
  return request.post('/alert/rules', data)
}

/**
 * 更新告警规则
 * @param {string} id - 规则 ID
 * @param {Object} data - 规则数据
 * @returns {Promise}
 */
export function updateAlertRule(id, data) {
  return request.put(`/alert/rules/${id}`, data)
}

/**
 * 删除告警规则
 * @param {string} id - 规则 ID
 * @returns {Promise}
 */
export function deleteAlertRule(id) {
  return request.delete(`/alert/rules/${id}`)
}

/**
 * 获取告警规则详情
 * @param {string} id - 规则 ID
 * @returns {Promise}
 */
export function getAlertRule(id) {
  return request.get(`/alert/rules/${id}`)
}

/**
 * 获取告警规则列表
 * @param {Object} params - 查询参数
 * @returns {Promise}
 */
export function listAlertRules(params) {
  return request.get('/alert/rules', { params })
}

/**
 * 启用/禁用告警规则
 * @param {string} id - 规则 ID
 * @param {boolean} enabled - 是否启用
 * @returns {Promise}
 */
export function toggleAlertRule(id, enabled) {
  return request.put(`/alert/rules/${id}/toggle`, { enabled })
}

/**
 * 测试告警规则
 * @param {Object} data - 规则数据
 * @returns {Promise}
 */
export function testAlertRule(data) {
  return request.post('/alert/rules/test', data)
}

// ===== 告警实例 =====

/**
 * 获取告警列表
 * @param {Object} params - 查询参数
 * @returns {Promise}
 */
export function listAlerts(params) {
  return request.get('/alerts', { params })
}

/**
 * 获取告警详情
 * @param {string} id - 告警 ID
 * @returns {Promise}
 */
export function getAlert(id) {
  return request.get(`/alerts/${id}`)
}

/**
 * 解决告警
 * @param {string} id - 告警 ID
 * @param {Object} data - 解决数据
 * @returns {Promise}
 */
export function resolveAlert(id, data) {
  return request.post(`/alerts/${id}/resolve`, data)
}

/**
 * 抑制告警
 * @param {string} id - 告警 ID
 * @param {Object} data - 抑制数据
 * @returns {Promise}
 */
export function silenceAlert(id, data) {
  return request.post(`/alerts/${id}/silence`, data)
}

/**
 * 批量解决告警
 * @param {Object} data - 批量操作数据
 * @returns {Promise}
 */
export function batchResolveAlerts(data) {
  return request.post('/alerts/batch-resolve', data)
}

/**
 * 批量抑制告警
 * @param {Object} data - 批量操作数据
 * @returns {Promise}
 */
export function batchSilenceAlerts(data) {
  return request.post('/alerts/batch-silence', data)
}

/**
 * 获取告警统计
 * @param {Object} params - 查询参数
 * @returns {Promise}
 */
export function getAlertStats(params) {
  return request.get('/alerts/stats', { params })
}

// ===== 通知渠道 =====

/**
 * 创建通知渠道
 * @param {Object} data - 渠道数据
 * @returns {Promise}
 */
export function createAlertChannel(data) {
  return request.post('/alert/channels', data)
}

/**
 * 更新通知渠道
 * @param {string} id - 渠道 ID
 * @param {Object} data - 渠道数据
 * @returns {Promise}
 */
export function updateAlertChannel(id, data) {
  return request.put(`/alert/channels/${id}`, data)
}

/**
 * 删除通知渠道
 * @param {string} id - 渠道 ID
 * @returns {Promise}
 */
export function deleteAlertChannel(id) {
  return request.delete(`/alert/channels/${id}`)
}

/**
 * 获取通知渠道列表
 * @returns {Promise}
 */
export function listAlertChannels() {
  return request.get('/alert/channels')
}

/**
 * 获取通知渠道详情
 * @param {string} id - 渠道 ID
 * @returns {Promise}
 */
export function getAlertChannel(id) {
  return request.get(`/alert/channels/${id}`)
}

/**
 * 测试通知渠道
 * @param {string} id - 渠道 ID
 * @param {Object} data - 测试数据
 * @returns {Promise}
 */
export function testAlertChannel(id, data) {
  return request.post(`/alert/channels/${id}/test`, data)
}

// ===== 抑制规则 =====

/**
 * 创建抑制规则
 * @param {Object} data - 抑制规则数据
 * @returns {Promise}
 */
export function createSilenceRule(data) {
  return request.post('/alert/silences', data)
}

/**
 * 更新抑制规则
 * @param {string} id - 规则 ID
 * @param {Object} data - 规则数据
 * @returns {Promise}
 */
export function updateSilenceRule(id, data) {
  return request.put(`/alert/silences/${id}`, data)
}

/**
 * 删除抑制规则
 * @param {string} id - 规则 ID
 * @returns {Promise}
 */
export function deleteSilenceRule(id) {
  return request.delete(`/alert/silences/${id}`)
}

/**
 * 获取抑制规则列表
 * @param {Object} params - 查询参数
 * @returns {Promise}
 */
export function listSilenceRules(params) {
  return request.get('/alert/silences', { params })
}

/**
 * 获取抑制规则详情
 * @param {string} id - 规则 ID
 * @returns {Promise}
 */
export function getSilenceRule(id) {
  return request.get(`/alert/silences/${id}`)
}

/**
 * 启用/禁用抑制规则
 * @param {string} id - 规则 ID
 * @param {boolean} enabled - 是否启用
 * @returns {Promise}
 */
export function toggleSilenceRule(id, enabled) {
  return request.put(`/alert/silences/${id}/toggle`, { enabled })
}

// ===== 实时事件 (SSE) =====

/**
 * 订阅告警事件流
 * @param {Function} onAlert - 新增告警回调
 * @param {Function} onUpdate - 告警更新回调
 * @param {Function} onResolve - 告警解决回调
 * @param {Function} onError - 错误回调
 * @returns {Object} - 返回可关闭的连接对象
 */
export function subscribeAlertEvents(onAlert, onUpdate, onResolve, onError) {
  const token = localStorage.getItem('x-token')
  const url = `/api/alert/events?token=${encodeURIComponent(token)}`

  const eventSource = new EventSource(url)

  eventSource.addEventListener('alert', (event) => {
    try {
      const data = JSON.parse(event.data)
      if (onAlert) onAlert(data)
    } catch (e) {
      console.error('Failed to parse alert event:', e)
    }
  })

  eventSource.addEventListener('update', (event) => {
    try {
      const data = JSON.parse(event.data)
      if (onUpdate) onUpdate(data)
    } catch (e) {
      console.error('Failed to parse update event:', e)
    }
  })

  eventSource.addEventListener('resolve', (event) => {
    try {
      const data = JSON.parse(event.data)
      if (onResolve) onResolve(data)
    } catch (e) {
      console.error('Failed to parse resolve event:', e)
    }
  })

  eventSource.onerror = (error) => {
    if (onError) onError(error)
  }

  return {
    close: () => eventSource.close()
  }
}

/**
 * 订阅通知发送结果流
 * @param {Function} onSuccess - 发送成功回调
 * @param {Function} onFailure - 发送失败回调
 * @returns {Object} - 返回可关闭的连接对象
 */
export function subscribeNotificationEvents(onSuccess, onFailure) {
  const token = localStorage.getItem('x-token')
  const url = `/api/alert/notification-events?token=${encodeURIComponent(token)}`

  const eventSource = new EventSource(url)

  eventSource.addEventListener('success', (event) => {
    try {
      const data = JSON.parse(event.data)
      if (onSuccess) onSuccess(data)
    } catch (e) {
      console.error('Failed to parse success event:', e)
    }
  })

  eventSource.addEventListener('failure', (event) => {
    try {
      const data = JSON.parse(event.data)
      if (onFailure) onFailure(data)
    } catch (e) {
      console.error('Failed to parse failure event:', e)
    }
  })

  eventSource.onerror = (error) => {
    console.error('Notification event source error:', error)
  }

  return {
    close: () => eventSource.close()
  }
}

// ===== 通知历史 =====

/**
 * 获取通知历史列表
 * @param {Object} params - 查询参数
 * @returns {Promise}
 */
export function listNotificationHistory(params) {
  return request.get('/alert/notifications', { params })
}

/**
 * 重试失败的通知
 * @param {string} id - 通知记录 ID
 * @returns {Promise}
 */
export function retryNotification(id) {
  return request.post(`/alert/notifications/${id}/retry`)
}

// ===== 告警分组 =====

/**
 * 获取告警分组列表
 * @param {Object} params - 查询参数
 * @returns {Promise}
 */
export function listAlertGroups(params) {
  return request.get('/alerts/groups', { params })
}

/**
 * 获取分组详情
 * @param {string} groupId - 分组 ID
 * @returns {Promise}
 */
export function getAlertGroup(groupId) {
  return request.get(`/alerts/groups/${groupId}`)
}

// 导出默认对象
export default {
  // 规则管理
  createAlertRule,
  updateAlertRule,
  deleteAlertRule,
  getAlertRule,
  listAlertRules,
  toggleAlertRule,
  testAlertRule,

  // 告警实例
  listAlerts,
  getAlert,
  resolveAlert,
  silenceAlert,
  batchResolveAlerts,
  batchSilenceAlerts,
  getAlertStats,

  // 通知渠道
  createAlertChannel,
  updateAlertChannel,
  deleteAlertChannel,
  listAlertChannels,
  getAlertChannel,
  testAlertChannel,

  // 抑制规则
  createSilenceRule,
  updateSilenceRule,
  deleteSilenceRule,
  listSilenceRules,
  getSilenceRule,
  toggleSilenceRule,

  // 实时事件
  subscribeAlertEvents,
  subscribeNotificationEvents,

  // 通知历史
  listNotificationHistory,
  retryNotification,

  // 告警分组
  listAlertGroups,
  getAlertGroup
}
