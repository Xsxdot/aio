import axios from 'axios';

// 客户端凭证 DTO（不含 secret）
export interface ClientCredentialDTO {
  id: number;
  name: string;
  clientKey: string;
  status: number; // 1=启用，0=禁用
  description?: string;
  ipWhitelist: string[];
  expiresAt?: string; // ISO 8601 时间字符串，null 表示永不过期
  createdAt: string;
  updatedAt: string;
}

// 客户端凭证 DTO（包含 secret，仅创建/轮换时返回）
export interface ClientCredentialWithSecretDTO extends ClientCredentialDTO {
  clientSecret: string; // 仅创建时返回，请妥善保管
}

// 客户端凭证列表响应
export interface ClientCredentialListResponse {
  total: number;
  content: ClientCredentialDTO[];
}

// 创建客户端凭证请求
export interface CreateClientCredentialRequest {
  name: string;
  description?: string;
  ipWhitelist?: string[];
  expiresAt?: string | null; // ISO 8601 时间字符串或 null
}

// 更新客户端凭证请求
export interface UpdateClientCredentialRequest {
  id: number; // 必须包含 id（后端校验）
  name: string;
  description?: string;
  ipWhitelist?: string[];
  expiresAt?: string | null;
}

// 更新客户端状态请求
export interface UpdateClientStatusRequest {
  id: number; // 必须包含 id（后端校验）
  status: number; // 1=启用，0=禁用
}

// 轮换密钥响应
export interface RotateSecretResponse {
  clientSecret: string;
  message: string;
}

/**
 * 查询客户端凭证列表（仅返回启用且未过期的客户端）
 */
export function listClientCredentials() {
  return axios.get<ClientCredentialListResponse>('/admin/client-credentials');
}

/**
 * 根据 ID 查询客户端凭证详情
 */
export function getClientCredentialById(id: number) {
  return axios.get<ClientCredentialDTO>(`/admin/client-credentials/${id}`);
}

/**
 * 创建客户端凭证
 */
export function createClientCredential(data: CreateClientCredentialRequest) {
  return axios.post<ClientCredentialWithSecretDTO>(
    '/admin/client-credentials',
    data
  );
}

/**
 * 更新客户端凭证
 * 注意：body 必须包含 id 字段（后端校验顺序要求）
 */
export function updateClientCredential(
  id: number,
  data: UpdateClientCredentialRequest
) {
  // 确保 body 中包含 id
  const requestData = { ...data, id };
  return axios.put<void>(`/admin/client-credentials/${id}`, requestData);
}

/**
 * 更新客户端状态
 * 注意：body 必须包含 id 字段（后端校验顺序要求）
 */
export function updateClientCredentialStatus(id: number, status: number) {
  // body 必须包含 id 和 status
  const requestData: UpdateClientStatusRequest = { id, status };
  return axios.put<void>(`/admin/client-credentials/${id}/status`, requestData);
}

/**
 * 轮换客户端密钥（重新生成 secret）
 */
export function rotateClientCredentialSecret(id: number) {
  return axios.post<RotateSecretResponse>(
    `/admin/client-credentials/${id}/rotate-secret`
  );
}

/**
 * 删除客户端凭证
 */
export function deleteClientCredential(id: number) {
  return axios.delete<void>(`/admin/client-credentials/${id}`);
}





