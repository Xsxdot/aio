import axios from 'axios';

export interface ServerDTO {
  id: number;
  name: string;
  host: string;
  intranetHost?: string;
  extranetHost?: string;
  agentGrpcAddress?: string;
  enabled: boolean;
  tags?: Record<string, string>;
  comment?: string;
  createdAt: string;
  updatedAt: string;
}

export interface QueryServerRequest {
  name?: string;
  tag?: string;
  enabled?: boolean;
  pageNum?: number;
  size?: number;
}

export interface ListServersResponse {
  total: number;
  content: ServerDTO[];
}

export interface CreateServerRequest {
  name: string;
  host: string;
  intranetHost?: string;
  extranetHost?: string;
  agentGrpcAddress?: string;
  enabled: boolean;
  tags?: Record<string, string>;
  comment?: string;
}

export interface UpdateServerRequest {
  name?: string;
  host?: string;
  intranetHost?: string;
  extranetHost?: string;
  agentGrpcAddress?: string;
  enabled?: boolean;
  tags?: Record<string, string>;
  comment?: string;
}

export interface DiskItemDTO {
  mountPoint: string;
  used: number;
  total: number;
  percent: number;
}

export interface ServerStatusInfo {
  // 服务器基本信息
  id: number;
  name: string;
  host: string;
  intranetHost?: string;
  extranetHost?: string;
  agentGrpcAddress: string;
  enabled: boolean;
  tags?: Record<string, string>;
  comment: string;
  // 状态信息
  cpuPercent?: number;
  memUsed?: number;
  memTotal?: number;
  load1?: number;
  load5?: number;
  load15?: number;
  diskItems?: DiskItemDTO[];
  collectedAt?: string;
  reportedAt?: string;
  errorMessage?: string;
  statusSummary: string; // online/offline/unknown/error
}

export interface ServerSSHCredentialResponse {
  serverId: number;
  port: number;
  username: string;
  authMethod: string; // password/privatekey
  hasPassword: boolean;
  hasPrivateKey: boolean;
  comment: string;
}

export interface UpsertServerSSHCredentialRequest {
  port: number;
  username: string;
  authMethod: string; // password/privatekey
  password?: string;
  privateKey?: string;
  comment?: string;
}

/**
 * 查询服务器列表
 */
export function listServers(params?: QueryServerRequest) {
  return axios.get<ListServersResponse>('/admin/servers', {
    params: {
      ...params,
      pageNum: params?.pageNum || 1,
      size: params?.size || 100,
    },
  });
}

/**
 * 根据ID获取服务器详情
 */
export function getServerById(id: number) {
  return axios.get<ServerDTO>(`/admin/servers/${id}`);
}

/**
 * 创建服务器
 */
export function createServer(data: CreateServerRequest) {
  return axios.post<ServerDTO>('/admin/servers', data);
}

/**
 * 更新服务器
 */
export function updateServer(id: number, data: UpdateServerRequest) {
  return axios.put<void>(`/admin/servers/${id}`, data);
}

/**
 * 删除服务器
 */
export function deleteServer(id: number) {
  return axios.delete<void>(`/admin/servers/${id}`);
}

/**
 * 获取所有服务器状态（主页/聚合查询）
 */
export function listServerStatus() {
  return axios.get<ServerStatusInfo[]>('/admin/servers/status');
}

/**
 * 获取服务器 SSH 凭证（脱敏）
 */
export function getServerSSHCredential(serverId: number) {
  return axios.get<ServerSSHCredentialResponse | null>(`/admin/servers/${serverId}/ssh`);
}

/**
 * 更新或插入服务器 SSH 凭证
 */
export function upsertServerSSHCredential(serverId: number, data: UpsertServerSSHCredentialRequest) {
  return axios.put<void>(`/admin/servers/${serverId}/ssh`, data);
}

/**
 * 删除服务器 SSH 凭证
 */
export function deleteServerSSHCredential(serverId: number) {
  return axios.delete<void>(`/admin/servers/${serverId}/ssh`);
}
