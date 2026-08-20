import axios from 'axios';

// ========== 短域名相关类型 ==========
export interface ShortDomainDTO {
  id: number;
  domain: string;
  enabled: boolean;
  isDefault: boolean;
  comment: string;
  createdAt: string;
  updatedAt: string;
}

export interface CreateShortDomainRequest {
  domain: string;
  isDefault?: boolean;
  comment?: string;
}

export interface UpdateShortDomainRequest {
  domain?: string;
  isDefault?: boolean;
  comment?: string;
}

export interface ListShortDomainsResponse {
  total: number;
  content: ShortDomainDTO[];
}

// ========== 短链接相关类型 ==========
export interface ShortLinkDTO {
  id: number;
  domainId: number;
  domain: string;
  code: string;
  shortUrl: string;
  targetType: string;
  targetConfig: Record<string, any>;
  expiresAt?: string;
  maxVisits?: number;
  visitCount: number;
  successCount: number;
  hasPassword: boolean;
  enabled: boolean;
  comment: string;
  createdAt: string;
  updatedAt: string;
}

export interface CreateShortLinkRequest {
  domainId: number;
  targetType: string;
  targetConfig: Record<string, any>;
  expiresAt?: string;
  password?: string;
  maxVisits?: number;
  codeLength?: number;
  customCode?: string;
  comment?: string;
}

export interface UpdateShortLinkRequest {
  targetConfig?: Record<string, any>;
  expiresAt?: number; // unix timestamp in seconds
  maxVisits?: number;
  comment?: string;
}

export interface ListShortLinksRequest {
  domainId: number;
  page?: number;
  size?: number;
}

export interface ListShortLinksResponse {
  total: number;
  content: ShortLinkDTO[];
}

// ========== 短链接统计相关类型 ==========
export interface DailyStatDTO {
  date: string;
  visitCount: number;
  successCount: number;
}

export interface VisitRecordDTO {
  id: number;
  ip: string;
  userAgent: string;
  referer: string;
  visitedAt: string;
}

export interface SuccessEventRecordDTO {
  id: number;
  eventId: string;
  attrs: Record<string, any>;
  createdAt: string;
}

export interface ShortLinkStatsDTO {
  totalVisits: number;
  totalSuccess: number;
  dailyStats: DailyStatDTO[];
  recentVisits: VisitRecordDTO[];
  recentSuccess: SuccessEventRecordDTO[];
}

// ========== 短域名 API ==========

/**
 * 查询短域名列表
 */
export function listShortDomains() {
  return axios.get<ListShortDomainsResponse>('/admin/short-domains');
}

/**
 * 根据ID获取短域名详情
 */
export function getShortDomain(id: number) {
  return axios.get<ShortDomainDTO>(`/admin/short-domains/${id}`);
}

/**
 * 创建短域名
 */
export function createShortDomain(data: CreateShortDomainRequest) {
  return axios.post<ShortDomainDTO>('/admin/short-domains', data);
}

/**
 * 更新短域名
 */
export function updateShortDomain(id: number, data: UpdateShortDomainRequest) {
  return axios.put<void>(`/admin/short-domains/${id}`, data);
}

/**
 * 更新短域名状态
 */
export function updateShortDomainStatus(id: number, enabled: boolean) {
  return axios.put<void>(`/admin/short-domains/${id}/status`, { enabled });
}

/**
 * 删除短域名
 */
export function deleteShortDomain(id: number) {
  return axios.delete<void>(`/admin/short-domains/${id}`);
}

// ========== 短链接 API ==========

/**
 * 查询短链接列表（必须传domainId）
 */
export function listShortLinks(params: ListShortLinksRequest) {
  return axios.get<ListShortLinksResponse>('/admin/short-links', {
    params: {
      domainId: params.domainId,
      page: params.page || 1,
      size: params.size || 20,
    },
  });
}

/**
 * 根据ID获取短链接详情
 */
export function getShortLink(id: number) {
  return axios.get<ShortLinkDTO>(`/admin/short-links/${id}`);
}

/**
 * 创建短链接
 */
export function createShortLink(data: CreateShortLinkRequest) {
  return axios.post<ShortLinkDTO>('/admin/short-links', data);
}

/**
 * 更新短链接
 */
export function updateShortLink(id: number, data: UpdateShortLinkRequest) {
  return axios.put<void>(`/admin/short-links/${id}`, data);
}

/**
 * 更新短链接状态
 */
export function updateShortLinkStatus(id: number, enabled: boolean) {
  return axios.put<void>(`/admin/short-links/${id}/status`, { enabled });
}

/**
 * 删除短链接
 */
export function deleteShortLink(id: number) {
  return axios.delete<void>(`/admin/short-links/${id}`);
}

/**
 * 获取短链接统计
 */
export function getShortLinkStats(id: number, days = 30) {
  return axios.get<ShortLinkStatsDTO>(`/admin/short-links/${id}/stats`, {
    params: { days },
  });
}
