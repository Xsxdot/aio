import axios from 'axios';
import type { RouteRecordNormalized } from 'vue-router';

export interface LoginData {
  username: string;
  password: string;
}

// 匹配后端 AdminController.Login 返回的 admin 对象
export interface AdminInfo {
  id: number;
  account: string;
  isSuper: boolean;
  roles: string[];
  status: number;
  remark: string;
}

// 匹配后端登录成功的返回结构
export interface LoginRes {
  accessToken: string;
  expiresAt?: number;
  tokenType?: string;
  admin: AdminInfo;
}

export function login(data: LoginData) {
  // 将 username 转换为 account 以匹配后端接口
  const loginRequest = {
    account: data.username,
    password: data.password,
  };
  return axios.post<LoginRes>('/admin/login', loginRequest);
}

export function logout() {
  return axios.post<LoginRes>('/api/user/logout');
}

export function getUserInfo() {
  return axios.get<AdminInfo>('/admin/admins/info');
}

export function getMenuList() {
  return axios.post<RouteRecordNormalized[]>('/api/user/menu');
}
