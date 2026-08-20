import axios from 'axios';

export type JobStatus =
  | 'pending'
  | 'running'
  | 'succeeded'
  | 'failed'
  | 'canceled'
  | 'dead';

export interface ExecutorJobDTO {
  id: number;
  env: string;
  target_service: string;
  method: string;
  args_json: string;
  status: JobStatus;
  priority: number;
  next_run_at: string | null;
  max_attempts: number;
  attempts: number;
  lease_owner: string;
  lease_until: string | null;
  dedup_key: string;
  last_error: string;
  result_json: string;
  createdAt: string;
  updatedAt: string;
}

export interface ExecutorJobAttemptDTO {
  id: number;
  job_id: number;
  attempt_no: number;
  worker_id: string;
  status: JobStatus;
  error: string;
  started_at: string | null;
  finished_at: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface ExecutorStatsDTO {
  queue_length: number;
  pending_count: number;
  running_count: number;
  succeeded_count: number;
  failed_count: number;
  canceled_count: number;
  dead_count: number;
  due_count: number;
  retry_distribution: Record<string, number>;
}

export interface ListJobsRequest {
  env: string;
  target_service?: string;
  status?: string;
  page_num?: number;
  page_size?: number;
}

export interface ListJobsResponse {
  total: number;
  content: ExecutorJobDTO[];
}

export interface SubmitJobRequest {
  env: string;
  target_service: string;
  method: string;
  args_json?: string;
  run_at?: number;
  max_attempts?: number;
  priority?: number;
  dedup_key?: string;
}

export interface RequeueJobRequest {
  run_at?: number;
}

export interface UpdateJobArgsRequest {
  args_json: string;
}

export interface CleanupJobsRequest {
  env: string;
  succeeded_days: number;
  canceled_days: number;
  dead_days: number;
}

export interface GetStatsRequest {
  env: string;
}

export interface CleanupJobsResponse {
  deleted: number;
  message: string;
}

/**
 * 任务列表（分页 + 过滤）
 */
export function listJobs(params?: ListJobsRequest) {
  return axios.get<ListJobsResponse>('/admin/executor/jobs', { params });
}

/**
 * 任务详情
 */
export function getJob(id: number) {
  return axios.get<ExecutorJobDTO>(`/admin/executor/jobs/${id}`);
}

/**
 * 提交新任务
 */
export function submitJob(data: SubmitJobRequest) {
  return axios.post<{ job_id: number }>('/admin/executor/jobs', data);
}

/**
 * 取消任务
 */
export function cancelJob(id: number) {
  return axios.post<string>(`/admin/executor/jobs/${id}/cancel`);
}

/**
 * 重新入队任务
 */
export function requeueJob(id: number, data?: RequeueJobRequest) {
  return axios.post<string>(`/admin/executor/jobs/${id}/requeue`, data || {});
}

/**
 * 更新任务参数 JSON
 */
export function updateJobArgs(id: number, data: UpdateJobArgsRequest) {
  return axios.put<string>(`/admin/executor/jobs/${id}/args`, data);
}

/**
 * 获取任务的执行历史（attempts）
 */
export function getJobAttempts(id: number) {
  return axios.get<ExecutorJobAttemptDTO[]>(`/admin/executor/jobs/${id}/attempts`);
}

/**
 * 获取统计信息
 */
export function getExecutorStats(params: GetStatsRequest) {
  return axios.get<ExecutorStatsDTO>('/admin/executor/stats', { params });
}

/**
 * 清理旧任务
 */
export function cleanupJobs(data: CleanupJobsRequest) {
  return axios.post<CleanupJobsResponse>('/admin/executor/cleanup', data);
}
