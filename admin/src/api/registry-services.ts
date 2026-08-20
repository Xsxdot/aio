import axios from 'axios';

// 服务定义 DTO（已移除 env）
export interface ServiceDTO {
  id: number;
  project: string;
  name: string;
  owner: string;
  description: string;
  spec: Record<string, any>;
  createdAt: string;
  updatedAt: string;
}

// 实例 DTO
export interface InstanceDTO {
  id: number;
  serviceId: number;
  instanceKey: string;
  env: string;
  host: string;
  endpoint: string;
  meta: Record<string, any>;
  ttlSeconds: number;
  lastHeartbeatAt: string;
  createdAt: string;
  updatedAt: string;
}

// 创建服务请求（已移除 env）
export interface CreateServiceRequest {
  project: string;
  name: string;
  owner?: string;
  description?: string;
  spec?: Record<string, any>;
}

// 更新服务请求（已移除 env）
export interface UpdateServiceRequest {
  project?: string;
  name?: string;
  owner?: string;
  description?: string;
  spec?: Record<string, any>;
}

// 列表响应
export interface ListServicesResponse {
  total: number;
  content: ServiceDTO[];
}

// 实例列表响应
export interface ListInstancesResponse {
  total: number;
  content: InstanceDTO[];
}

/**
 * 查询服务列表
 */
export function listServices(params?: { project?: string }) {
  return axios.get<ListServicesResponse>('/admin/registry/services', {
    params,
  });
}

/**
 * 查询服务实例列表
 */
export function listServiceInstances(
  serviceId: number,
  params?: { env?: string; aliveOnly?: boolean }
) {
  return axios.get<ListInstancesResponse>(
    `/admin/registry/services/${serviceId}/instances`,
    {
      params: {
        env: params?.env,
        aliveOnly: params?.aliveOnly ?? true,
      },
    }
  );
}

/**
 * 根据ID获取服务详情
 */
export function getServiceById(id: number) {
  return axios.get<ServiceDTO>(`/admin/registry/services/${id}`);
}

/**
 * 创建服务
 */
export function createService(data: CreateServiceRequest) {
  return axios.post<ServiceDTO>('/admin/registry/services', data);
}

/**
 * 更新服务
 */
export function updateService(id: number, data: UpdateServiceRequest) {
  return axios.put<ServiceDTO>(`/admin/registry/services/${id}`, data);
}

/**
 * 删除服务
 */
export function deleteService(id: number) {
  return axios.delete<{ message: string }>(`/admin/registry/services/${id}`);
}

/**
 * 强制下线实例
 */
export function offlineInstance(serviceId: number, instanceKey: string) {
  return axios.post<{ message: string }>(
    `/admin/registry/services/${serviceId}/instances/${instanceKey}/offline`
  );
}

/**
 * 删除实例
 */
export function deleteInstance(serviceId: number, instanceKey: string) {
  return axios.delete<{ message: string }>(
    `/admin/registry/services/${serviceId}/instances/${instanceKey}`
  );
}





