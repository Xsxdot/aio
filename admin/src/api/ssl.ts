import axios from 'axios';

// ========== Type Definitions ==========

// DNS Provider enum
export type DnsProvider = 'alidns' | 'tencentcloud' | 'dnspod';

// Certificate Status enum
export type CertificateStatus =
  | 'pending'
  | 'issuing'
  | 'active'
  | 'renewing'
  | 'expired'
  | 'failed';

// Deploy Target Type enum
export type DeployTargetType = 'local' | 'ssh' | 'aliyun_cas';

// Deploy Status enum
export type DeployStatus =
  | 'pending'
  | 'deploying'
  | 'success'
  | 'failed'
  | 'partial';

// ========== DNS Credentials ==========

export interface DnsCredentialDTO {
  id: number;
  name: string;
  provider: DnsProvider;
  access_key: string;
  secret_key: string;
  extra_config?: string;
  status: number; // 1=enabled, 0=disabled
  description?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateDnsCredentialRequest {
  name: string;
  provider: DnsProvider;
  access_key: string;
  secret_key: string;
  extra_config?: string;
  description?: string;
}

export interface UpdateDnsCredentialRequest {
  name?: string;
  access_key?: string;
  secret_key?: string;
  extra_config?: string;
  status?: number;
  description?: string;
}

export interface DnsCredentialListResponse {
  total: number;
  content: DnsCredentialDTO[];
}

// ========== Certificates ==========

export interface CertificateDTO {
  id: number;
  name: string;
  domain: string; // Single domain, supports wildcard like *.a.com
  email: string;
  dns_credential_id: number;
  status: CertificateStatus;
  expires_at?: string;
  issued_at?: string;
  last_renew_at?: string;
  renew_before_days: number;
  fullchain_pem?: string;
  privkey_pem?: string;
  acme_account_url?: string;
  acme_account_key?: string;
  cert_url?: string;
  auto_renew: number; // 1=yes, 0=no
  auto_deploy: number; // 1=yes, 0=no
  last_error?: string;
  description?: string;
  created_at: string;
  updated_at: string;
}

export interface IssueCertificateRequest {
  name?: string; // Optional: backend can auto-generate
  domain: string; // Single domain, supports wildcard like *.a.com
  email: string;
  dns_credential_id: number;
  renew_before_days?: number;
  auto_renew?: boolean;
  auto_deploy?: boolean; // If true, automatically matches and deploys to targets by domain
  description?: string;
  use_staging?: boolean;
}

export interface CertificateListResponse {
  total: number;
  content: CertificateDTO[];
}

export interface DeployHistoryDTO {
  id: number;
  certificate_id: number;
  deploy_target_id: number;
  status: DeployStatus;
  start_time: string;
  end_time?: string;
  error_message?: string;
  result_data?: string; // JSON string
  trigger_type: string; // manual/auto_renew/auto_issue
  created_at: string;
  updated_at: string;
}

export interface DeployCertificateRequest {
  target_ids: number[];
}

// ========== Deploy Targets ==========

export interface DeployTargetDTO {
  id: number;
  name: string;
  domain: string; // Bound domain, supports wildcard like *.a.com
  type: DeployTargetType;
  config: string; // JSON string
  status: number; // 1=enabled, 0=disabled
  description?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateDeployTargetRequest {
  name: string;
  domain: string; // Required: bound domain (supports *.a.com or b.a.com)
  type: DeployTargetType;
  config: string; // JSON string
  description?: string;
}

export interface UpdateDeployTargetRequest {
  name?: string;
  domain?: string; // Optional: update bound domain
  config?: string;
  status?: number;
  description?: string;
}

export interface DeployTargetListResponse {
  total: number;
  content: DeployTargetDTO[];
}

// Deploy config types for frontend forms
export interface LocalDeployConfig {
  base_path: string;
  fullchain_name?: string;
  privkey_name?: string;
  file_mode?: string;
  reload_command?: string;
}

export interface SSHDeployConfig {
  server_id: number;
  remote_path: string;
  fullchain_name?: string;
  privkey_name?: string;
  file_mode?: string;
  reload_command?: string;
}

export interface AliyunCASDeployConfig {
  dns_credential_id: number;
  region?: string;
  auto_deploy?: boolean;
}

// ========== API Functions ==========

// DNS Credentials
export function createDnsCredential(data: CreateDnsCredentialRequest) {
  return axios.post<DnsCredentialDTO>('/admin/ssl/dns-credentials', data);
}

export function listDnsCredentials(page = 1, pageSize = 20) {
  return axios.get<DnsCredentialListResponse>('/admin/ssl/dns-credentials', {
    params: { page, page_size: pageSize },
  });
}

export function getDnsCredential(id: number) {
  return axios.get<DnsCredentialDTO>(`/admin/ssl/dns-credentials/${id}`);
}

export function updateDnsCredential(
  id: number,
  data: UpdateDnsCredentialRequest
) {
  return axios.put<DnsCredentialDTO>(`/admin/ssl/dns-credentials/${id}`, data);
}

export function deleteDnsCredential(id: number) {
  return axios.delete<void>(`/admin/ssl/dns-credentials/${id}`);
}

// Certificates
export function issueCertificate(data: IssueCertificateRequest) {
  return axios.post<CertificateDTO>('/admin/ssl/certificates', data);
}

export function listCertificates(page = 1, pageSize = 20) {
  return axios.get<CertificateListResponse>('/admin/ssl/certificates', {
    params: { page, page_size: pageSize },
  });
}

export function getCertificate(id: number) {
  return axios.get<CertificateDTO>(`/admin/ssl/certificates/${id}`);
}

export function renewCertificate(id: number) {
  return axios.post<{ message: string }>(`/admin/ssl/certificates/${id}/renew`);
}

export function deployCertificate(id: number, data: DeployCertificateRequest) {
  return axios.post<{ message: string }>(
    `/admin/ssl/certificates/${id}/deploy`,
    data
  );
}

export function getCertificateDeployHistory(id: number, limit = 20) {
  return axios.get<DeployHistoryDTO[]>(
    `/admin/ssl/certificates/${id}/deploy-history`,
    {
      params: { limit },
    }
  );
}

export function deleteCertificate(id: number) {
  return axios.delete<void>(`/admin/ssl/certificates/${id}`);
}

// Deploy Targets
export function createDeployTarget(data: CreateDeployTargetRequest) {
  return axios.post<DeployTargetDTO>('/admin/ssl/deploy-targets', data);
}

export function listDeployTargets(page = 1, pageSize = 20) {
  return axios.get<DeployTargetListResponse>('/admin/ssl/deploy-targets', {
    params: { page, page_size: pageSize },
  });
}

export function getDeployTarget(id: number) {
  return axios.get<DeployTargetDTO>(`/admin/ssl/deploy-targets/${id}`);
}

export function updateDeployTarget(
  id: number,
  data: UpdateDeployTargetRequest
) {
  return axios.put<DeployTargetDTO>(`/admin/ssl/deploy-targets/${id}`, data);
}

export function deleteDeployTarget(id: number) {
  return axios.delete<void>(`/admin/ssl/deploy-targets/${id}`);
}
