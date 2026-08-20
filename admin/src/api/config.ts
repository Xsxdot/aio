import axios from 'axios';

// 配置值类型
export type ValueType =
  | 'string'
  | 'int'
  | 'float'
  | 'bool'
  | 'ref'
  | 'object'
  | 'array'
  | 'encrypted';

// 配置值结构
export interface ConfigValue {
  value: string;
  type: ValueType;
}

// 配置项模型（对应后端 ConfigItemModel）
export interface ConfigItem {
  id: number;
  key: string; // 包含环境后缀，如 app.cert.dev
  value: string; // JSON字符串：map[field]ConfigValue
  version: number;
  metadata: string; // JSON字符串：map[string]string
  description?: string;
  createdAt?: string;
  updatedAt?: string;
}

// 解析后的配置项（方便前端使用）
export interface ParsedConfigItem {
  id: number;
  key: string; // 包含环境后缀，如 app.cert.dev
  value: Record<string, ConfigValue>; // 字段名 -> ConfigValue 映射
  version: number;
  metadata: Record<string, string>; // 解析后的map
  description?: string;
  createdAt?: string;
  updatedAt?: string;
}

// 配置历史响应
export interface ConfigHistoryResponse {
  id: number;
  configKey: string;
  version: number;
  value: Record<string, ConfigValue>;
  metadata: Record<string, string>;
  operator: string;
  operatorId: number;
  changeNote: string;
  createdAt: string;
}

// 查询配置请求
export interface QueryConfigRequest {
  key?: string;
  environment?: string;
  page?: number; // 前端使用page
  pageSize?: number; // 前端使用pageSize
}

// 创建配置请求
export interface CreateConfigRequest {
  key: string; // 包含环境后缀，如 app.cert.dev
  value: Record<string, ConfigValue>; // 字段名 -> ConfigValue 映射
  metadata?: Record<string, string>;
  description?: string;
  changeNote?: string;
}

// 更新配置请求
export interface UpdateConfigRequest {
  value: Record<string, ConfigValue>; // 字段名 -> ConfigValue 映射
  metadata?: Record<string, string>;
  description?: string;
  changeNote: string; // 必填
}

// 导出配置请求
export interface ExportConfigRequest {
  keys?: string[];
  environment?: string;
  targetSalt?: string;
}

// 导出的配置项
export interface ExportConfig {
  key: string;
  value: Record<string, ConfigValue>;
  metadata?: Record<string, string>;
  description?: string;
  version?: number;
}

// 导入配置请求
export interface ImportConfigRequest {
  sourceSalt?: string;
  configs: ExportConfig[];
  overWrite: boolean;
}

// 分页响应
export interface PageResponse<T> {
  total: number;
  content: T[];
}

/**
 * 解析配置项（将JSON字符串转换为对象）
 */
export function parseConfigItem(item: ConfigItem): ParsedConfigItem {
  let value: Record<string, ConfigValue> = {};
  let metadata: Record<string, string> = {};

  try {
    if (item.value) {
      value = JSON.parse(item.value);
    }
  } catch (e) {
    console.error('解析配置值失败:', e);
  }

  try {
    if (item.metadata) {
      metadata = JSON.parse(item.metadata);
    }
  } catch (e) {
    console.error('解析元数据失败:', e);
  }

  return {
    ...item,
    value,
    metadata,
  };
}

/**
 * 查询配置列表（分页）
 */
export function queryConfigs(params: QueryConfigRequest) {
  // 映射参数：page -> pageNum, pageSize -> size
  const backendParams = {
    key: params.key,
    environment: params.environment,
    pageNum: params.page || 1,
    size: params.pageSize || 10,
  };

  return axios.get<PageResponse<ConfigItem>>('/admin/configs', {
    params: backendParams,
  });
}

/**
 * 根据ID查询配置详情
 */
export function getConfigById(id: number) {
  return axios.get<ConfigItem>(`/admin/configs/${id}`);
}

/**
 * 创建配置
 */
export function createConfig(data: CreateConfigRequest) {
  return axios.post<void>('/admin/configs', data);
}

/**
 * 更新配置
 */
export function updateConfig(id: number, data: UpdateConfigRequest) {
  return axios.put<void>(`/admin/configs/${id}`, data);
}

/**
 * 删除配置
 */
export function deleteConfig(id: number) {
  return axios.delete<void>(`/admin/configs/${id}`);
}

/**
 * 查询配置历史版本
 */
export function getHistory(id: number) {
  return axios.get<ConfigHistoryResponse[]>(`/admin/configs/${id}/history`);
}

/**
 * 回滚配置到指定版本
 */
export function rollback(id: number, version: number) {
  return axios.post<void>(`/admin/configs/${id}/rollback/${version}`);
}

/**
 * 导出配置（下载JSON文件）
 */
export function exportConfigs(
  data: ExportConfigRequest,
  filename = 'configs_export.json'
) {
  return axios
    .post('/admin/configs/export', data, {
      responseType: 'blob', // 关键：告诉axios这是文件下载
    })
    .then((response) => {
      // 检查response.data的类型
      let blob: Blob;
      
      if (response.data instanceof Blob) {
        // 如果已经是Blob，直接使用
        blob = response.data;
      } else {
        // 如果不是Blob（可能是对象），先转换为JSON字符串，再创建Blob
        const jsonString = JSON.stringify(response.data, null, 2);
        blob = new Blob([jsonString], { type: 'application/json' });
      }
      
      // 创建下载链接
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.setAttribute('download', filename);
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
      return response;
    });
}

/**
 * 导入配置
 */
export function importConfigs(data: ImportConfigRequest) {
  return axios.post<void>('/admin/configs/import', data);
}

/**
 * 查看配置的最终JSON格式（解密后的纯对象）
 */
export function getConfigJSON(id: number) {
  return axios.get<Record<string, any>>(`/admin/configs/${id}/json`);
}

