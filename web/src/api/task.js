import { request } from './index.js'

// 任务管理 API
export const taskApi = {
  // 获取任务列表
  listTasks(params) {
    return request.get('/tasks', { params })
  },

  // 获取任务详情
  getTask(id) {
    return request.get(`/tasks/${id}`)
  },

  // 创建任务
  createTask(data) {
    return request.post('/tasks', data)
  },

  // 更新任务
  updateTask(id, data) {
    return request.put(`/tasks/${id}`, data)
  },

  // 删除任务
  deleteTask(id) {
    return request.delete(`/tasks/${id}`)
  },

  // 启用任务
  enableTask(id) {
    return request.post(`/tasks/${id}/enable`)
  },

  // 禁用任务
  disableTask(id) {
    return request.post(`/tasks/${id}/disable`)
  },

  // 手动触发任务
  triggerTask(id) {
    return request.post(`/tasks/${id}/trigger`)
  },

  // 获取下次执行时间
  getNextRunTime(id) {
    return request.get(`/tasks/${id}/next-run`)
  },

  // 获取任务统计信息
  getTaskStats(id) {
    return request.get(`/tasks/${id}/stats`)
  },

  // 重置任务统计信息
  resetTaskStats(id) {
    return request.post(`/tasks/${id}/reset-stats`)
  }
}

// 执行记录 API
export const executionApi = {
  // 获取执行记录列表
  listExecutions(params) {
    return request.get('/executions', { params })
  },

  // 获取执行记录详情
  getExecution(id) {
    return request.get(`/executions/${id}`)
  },

  // 获取执行日志
  getExecutionLogs(id) {
    return request.get(`/executions/${id}/logs`)
  },

  // 获取执行统计
  getExecutionStats(params) {
    return request.get('/executions/stats', { params })
  }
}

// 分组管理 API
export const groupApi = {
  // 获取分组列表
  listGroups() {
    return request.get('/groups')
  },

  // 获取分组详情
  getGroup(id) {
    return request.get(`/groups/${id}`)
  },

  // 创建分组
  createGroup(data) {
    return request.post('/groups', data)
  },

  // 更新分组
  updateGroup(id, data) {
    return request.put(`/groups/${id}`, data)
  },

  // 删除分组
  deleteGroup(id) {
    return request.delete(`/groups/${id}`)
  },

  // 获取分组下的客户端列表
  getGroupClients(id) {
    return request.get(`/groups/${id}/clients`)
  },

  // 添加客户端到分组
  addGroupClients(id, clientIds) {
    return request.post(`/groups/${id}/clients`, { client_ids: clientIds })
  },

  // 从分组移除客户端
  removeGroupClient(id, clientId) {
    return request.delete(`/groups/${id}/clients/${clientId}`)
  }
}
