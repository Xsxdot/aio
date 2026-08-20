import axios from 'axios';

// 管理员 DTO
export interface AdminDTO {
  id: number;
  account: string;
  status: number; // 0=禁用，1=启用
  isSuper: boolean;
  roles: string[];
  remark: string;
  createdAt: string;
  updatedAt: string;
}

// 管理员列表响应
export interface AdminListResponse {
  total: number;
  content: AdminDTO[];
}

// 查询管理员列表请求
export interface ListAdminsParams {
  page?: number;
  pageSize?: number;
  keyword?: string;
  status?: number; // 0=禁用，1=启用
}

// 创建管理员请求
export interface CreateAdminRequest {
  account: string;
  password: string;
  remark?: string;
}

// 更新管理员状态请求
export interface UpdateAdminStatusRequest {
  adminId: number;
  status: number; // 0=禁用，1=启用
}

// 重置管理员密码请求
export interface ResetAdminPasswordRequest {
  adminId: number;
  newPassword: string;
}

/**
 * 查询管理员列表（支持分页和过滤）
 */
export function listAdmins(params?: ListAdminsParams) {
  return axios.get<AdminListResponse>('/admin/admins', { params });
}

/**
 * 根据 ID 查询管理员详情
 */
export function getAdminById(id: number) {
  return axios.get<AdminDTO>(`/admin/admins/${id}`);
}

/**
 * 创建管理员
 */
export function createAdmin(data: CreateAdminRequest) {
  return axios.post<AdminDTO>('/admin/admins', data);
}

/**
 * 更新管理员状态
 * 注意：body 必须包含 adminId 和 status 字段
 */
export function updateAdminStatus(id: number, status: number) {
  const requestData: UpdateAdminStatusRequest = { adminId: id, status };
  return axios.put<void>(`/admin/admins/${id}/status`, requestData);
}

/**
 * 重置管理员密码
 * 注意：body 必须包含 adminId 和 newPassword 字段
 */
export function resetAdminPassword(id: number, newPassword: string) {
  const requestData: ResetAdminPasswordRequest = { adminId: id, newPassword };
  return axios.put<void>(`/admin/admins/${id}/password`, requestData);
}

/**
 * 删除管理员
 */
export function deleteAdmin(id: number) {
  return axios.delete<void>(`/admin/admins/${id}`);
}





