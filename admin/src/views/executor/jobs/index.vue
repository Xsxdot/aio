<template>
  <div class="container">
    <!-- 统计卡片 -->
    <a-row :gutter="12" style="margin-bottom: 16px">
      <a-col v-for="stat in statCards" :key="stat.key" :flex="1">
        <a-card class="stat-card" :bordered="false">
          <a-statistic
            :title="$t(`executor.stats.${stat.key}`)"
            :value="getStatValue(stat.dataKey)"
            :value-style="{ color: stat.color, fontSize: '24px', fontWeight: 600 }"
          />
        </a-card>
      </a-col>
    </a-row>

    <a-card class="general-card" :title="$t('executor.jobs.title')">
      <!-- 搜索区域 -->
      <a-row style="margin-bottom: 16px">
        <a-col :flex="1">
          <a-form :model="searchForm" layout="inline">
            <a-form-item field="env" style="margin-bottom: 0">
              <a-select
                v-model="searchForm.env"
                :placeholder="$t('executor.search.env.placeholder')"
                style="width: 130px"
                @change="handleSearch"
              >
                <a-option v-for="e in envOptions" :key="e" :value="e">{{ e }}</a-option>
              </a-select>
            </a-form-item>
            <a-form-item field="target_service" style="margin-bottom: 0">
              <a-input
                v-model="searchForm.target_service"
                :placeholder="$t('executor.search.targetService.placeholder')"
                allow-clear
                style="width: 200px"
                @press-enter="handleSearch"
              >
                <template #prefix>
                  <icon-search />
                </template>
              </a-input>
            </a-form-item>
            <a-form-item field="status" style="margin-bottom: 0">
              <a-select
                v-model="searchForm.status"
                :placeholder="$t('executor.search.status.placeholder')"
                allow-clear
                style="width: 150px"
                @change="handleSearch"
              >
                <a-option v-for="s in statusOptions" :key="s.value" :value="s.value">
                  {{ $t(`executor.status.${s.value}`) }}
                </a-option>
              </a-select>
            </a-form-item>
          </a-form>
        </a-col>
        <a-col :flex="'86px'" style="text-align: right">
          <a-button @click="handleReset">
            <template #icon><icon-refresh /></template>
            {{ $t('executor.jobs.reset') }}
          </a-button>
        </a-col>
      </a-row>

      <!-- 操作按钮 -->
      <a-row style="margin-bottom: 16px">
        <a-col :span="24">
          <a-space>
            <a-button type="primary" @click="handleOpenSubmit">
              <template #icon><icon-plus /></template>
              {{ $t('executor.jobs.submit') }}
            </a-button>
            <a-button @click="handleRefresh">
              <template #icon><icon-refresh /></template>
              {{ $t('executor.jobs.refresh') }}
            </a-button>
            <a-button status="danger" @click="handleOpenCleanup">
              <template #icon><icon-delete /></template>
              {{ $t('executor.jobs.cleanup') }}
            </a-button>
          </a-space>
        </a-col>
      </a-row>

      <!-- 表格 -->
      <a-table
        row-key="id"
        :loading="loading"
        :pagination="pagination"
        :data="tableData"
        :bordered="{ cell: true }"
        :scroll="{ x: 1600 }"
        @page-change="onPageChange"
        @page-size-change="onPageSizeChange"
      >
        <template #columns>
          <a-table-column
            :title="$t('executor.column.id')"
            data-index="id"
            :width="80"
            fixed="left"
          />
          <a-table-column
            :title="$t('executor.column.env')"
            data-index="env"
            :width="90"
          >
            <template #cell="{ record }">
              <a-tag :color="record.env === 'prod' ? 'red' : record.env === 'dev' ? 'arcoblue' : 'orange'">
                {{ record.env || '-' }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('executor.column.targetService')"
            data-index="target_service"
            :width="160"
            :ellipsis="true"
            :tooltip="true"
          />
          <a-table-column
            :title="$t('executor.column.method')"
            data-index="method"
            :width="160"
            :ellipsis="true"
            :tooltip="true"
          />
          <a-table-column
            :title="$t('executor.column.status')"
            data-index="status"
            :width="110"
          >
            <template #cell="{ record }">
              <a-tag :color="getStatusColor(record.status)">
                {{ $t(`executor.status.${record.status}`) }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('executor.column.priority')"
            data-index="priority"
            :width="90"
          />
          <a-table-column
            :title="$t('executor.column.attempts')"
            data-index="attempts"
            :width="100"
          >
            <template #cell="{ record }">
              {{ record.attempts }} / {{ record.max_attempts }}
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('executor.column.nextRunAt')"
            data-index="next_run_at"
            :width="170"
          >
            <template #cell="{ record }">
              {{ record.next_run_at ? formatDateTime(record.next_run_at) : '-' }}
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('executor.column.leaseOwner')"
            data-index="lease_owner"
            :width="140"
            :ellipsis="true"
            :tooltip="true"
          >
            <template #cell="{ record }">
              <span v-if="record.lease_owner">{{ record.lease_owner }}</span>
              <span v-else style="color: var(--color-text-3)">-</span>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('executor.column.leaseUntil')"
            data-index="lease_until"
            :width="170"
          >
            <template #cell="{ record }">
              {{ record.lease_until ? formatDateTime(record.lease_until) : '-' }}
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('executor.column.createdAt')"
            data-index="createdAt"
            :width="170"
          >
            <template #cell="{ record }">
              {{ formatDateTime(record.createdAt) }}
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('executor.column.actions')"
            :width="280"
            fixed="right"
          >
            <template #cell="{ record }">
              <a-space wrap>
                <a-button type="text" size="small" @click="handleDetail(record)">
                  {{ $t('executor.action.detail') }}
                </a-button>
                <a-divider direction="vertical" />
                <a-button type="text" size="small" @click="handleAttempts(record)">
                  {{ $t('executor.action.attempts') }}
                </a-button>
                <template v-if="canCancel(record.status)">
                  <a-divider direction="vertical" />
                  <a-popconfirm
                    :content="$t('executor.confirm.cancel')"
                    @ok="handleCancel(record)"
                  >
                    <a-button type="text" size="small" status="warning">
                      {{ $t('executor.action.cancel') }}
                    </a-button>
                  </a-popconfirm>
                </template>
                <template v-if="canRequeue(record.status)">
                  <a-divider direction="vertical" />
                  <a-button type="text" size="small" @click="handleRequeue(record)">
                    {{ $t('executor.action.requeue') }}
                  </a-button>
                </template>
                <template v-if="canUpdateArgs(record.status)">
                  <a-divider direction="vertical" />
                  <a-button type="text" size="small" @click="handleUpdateArgs(record)">
                    {{ $t('executor.action.updateArgs') }}
                  </a-button>
                </template>
              </a-space>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-card>

    <!-- 提交任务抽屉 -->
    <JobSubmitDrawer
      ref="submitDrawerRef"
      v-model:visible="submitDrawerVisible"
      @submit="handleSubmitJob"
    />

    <!-- 详情抽屉 -->
    <JobDetailDrawer
      v-model:visible="detailDrawerVisible"
      :job-id="currentJobId"
    />

    <!-- 执行历史抽屉 -->
    <JobAttemptsDrawer
      v-model:visible="attemptsDrawerVisible"
      :job-id="currentJobId"
    />

    <!-- 修改参数抽屉 -->
    <JobArgsDrawer
      ref="argsDrawerRef"
      v-model:visible="argsDrawerVisible"
      :initial-args-json="currentJob?.args_json"
      @submit="handleUpdateArgsSubmit"
    />

    <!-- 重新入队弹窗 -->
    <JobRequeueModal
      ref="requeueModalRef"
      v-model:visible="requeueModalVisible"
      @submit="handleRequeueSubmit"
    />

    <!-- 清理弹窗 -->
    <JobCleanupModal
      ref="cleanupModalRef"
      v-model:visible="cleanupModalVisible"
      @submit="handleCleanupSubmit"
    />
  </div>
</template>

<script lang="ts" setup>
import { ref, reactive, onMounted } from 'vue';
import { Message } from '@arco-design/web-vue';
import { useI18n } from 'vue-i18n';
import {
  listJobs,
  submitJob,
  cancelJob,
  requeueJob,
  updateJobArgs,
  cleanupJobs,
  getExecutorStats,
  type ExecutorJobDTO,
  type JobStatus,
  type SubmitJobRequest,
  type CleanupJobsRequest,
  type ExecutorStatsDTO,
} from '@/api/executor';
import JobSubmitDrawer from './components/JobSubmitDrawer.vue';
import JobDetailDrawer from './components/JobDetailDrawer.vue';
import JobAttemptsDrawer from './components/JobAttemptsDrawer.vue';
import JobArgsDrawer from './components/JobArgsDrawer.vue';
import JobRequeueModal from './components/JobRequeueModal.vue';
import JobCleanupModal from './components/JobCleanupModal.vue';

const { t } = useI18n();

// ---- Stats ----
const stats = ref<ExecutorStatsDTO | null>(null);

type ExecutorNumericStatKey = {
  [K in Extract<keyof ExecutorStatsDTO, string>]: ExecutorStatsDTO[K] extends number ? K : never;
}[Extract<keyof ExecutorStatsDTO, string>];

const statCards = [
  { key: 'pending', dataKey: 'pending_count', color: 'var(--color-primary-6)' },
  { key: 'running', dataKey: 'running_count', color: 'var(--color-warning-6)' },
  { key: 'succeeded', dataKey: 'succeeded_count', color: 'var(--color-success-6)' },
  { key: 'failed', dataKey: 'failed_count', color: 'var(--color-danger-6)' },
  { key: 'canceled', dataKey: 'canceled_count', color: 'var(--color-text-3)' },
  { key: 'dead', dataKey: 'dead_count', color: '#722ed1' },
  { key: 'due', dataKey: 'due_count', color: 'var(--color-primary-4)' },
] as Array<{ key: string; dataKey: ExecutorNumericStatKey; color: string }>;

const loadStats = async () => {
  try {
    const res = await getExecutorStats({ env: searchForm.env });
    stats.value = (res as any).data;
  } catch {
    Message.error(t('executor.message.loadStats.failed'));
  }
};

// ---- Search ----
const envOptions = ['dev', 'test', 'prod'];

const statusOptions: { value: JobStatus }[] = [
  { value: 'pending' },
  { value: 'running' },
  { value: 'succeeded' },
  { value: 'failed' },
  { value: 'canceled' },
  { value: 'dead' },
];

const searchForm = reactive({
  env: 'dev',
  target_service: '',
  status: undefined as string | undefined,
});

// ---- Table ----
const loading = ref(false);
const tableData = ref<ExecutorJobDTO[]>([]);

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showTotal: true,
  showPageSize: true,
});

const fetchData = async () => {
  try {
    loading.value = true;
    const res = await listJobs({
      env: searchForm.env,
      target_service: searchForm.target_service || undefined,
      status: searchForm.status || undefined,
      page_num: pagination.current,
      page_size: pagination.pageSize,
    });
    const { data } = res as any;
    tableData.value = data.content || [];
    pagination.total = data.total || 0;
  } catch {
    Message.error(t('executor.message.loadList.failed'));
  } finally {
    loading.value = false;
  }
};

const handleSearch = () => {
  pagination.current = 1;
  fetchData();
};

const handleReset = () => {
  searchForm.env = 'dev';
  searchForm.target_service = '';
  searchForm.status = undefined;
  handleSearch();
};

const handleRefresh = async () => {
  await loadStats();
  await fetchData();
};

const onPageChange = (current: number) => {
  pagination.current = current;
  fetchData();
};

const onPageSizeChange = (pageSize: number) => {
  pagination.pageSize = pageSize;
  pagination.current = 1;
  fetchData();
};

// ---- Status helpers ----
const canCancel = (status: JobStatus) => ['pending', 'running'].includes(status);
const canRequeue = (status: JobStatus) => ['failed', 'canceled', 'dead'].includes(status);
const canUpdateArgs = (status: JobStatus) => status !== 'running';

const getStatusColor = (status: JobStatus) => {
  const map: Record<JobStatus, string> = {
    pending: 'blue',
    running: 'orange',
    succeeded: 'green',
    failed: 'red',
    canceled: 'gray',
    dead: 'purple',
  };
  return map[status] || 'gray';
};

// ---- Current job ----
const currentJobId = ref<number | null>(null);
const currentJob = ref<ExecutorJobDTO | null>(null);

// ---- Detail ----
const detailDrawerVisible = ref(false);
const handleDetail = (record: ExecutorJobDTO) => {
  currentJobId.value = record.id;
  detailDrawerVisible.value = true;
};

// ---- Attempts ----
const attemptsDrawerVisible = ref(false);
const handleAttempts = (record: ExecutorJobDTO) => {
  currentJobId.value = record.id;
  attemptsDrawerVisible.value = true;
};

// ---- Cancel ----
const handleCancel = async (record: ExecutorJobDTO) => {
  try {
    await cancelJob(record.id);
    Message.success(t('executor.message.cancel.success'));
    await fetchData();
    await loadStats();
  } catch {
    Message.error(t('executor.message.cancel.failed'));
  }
};

// ---- Submit ----
const submitDrawerVisible = ref(false);
const submitDrawerRef = ref<InstanceType<typeof JobSubmitDrawer>>();

const handleOpenSubmit = () => {
  submitDrawerVisible.value = true;
};

const handleSubmitJob = async (data: SubmitJobRequest) => {
  try {
    submitDrawerRef.value?.setSubmitting(true);
    const res = await submitJob(data);
    const jobId = (res as any).data?.job_id;
    Message.success(t('executor.message.submit.success', { id: jobId }));
    submitDrawerRef.value?.closeDrawer();
    await fetchData();
    await loadStats();
  } catch {
    Message.error(t('executor.message.submit.failed'));
  } finally {
    submitDrawerRef.value?.setSubmitting(false);
  }
};

// ---- Requeue ----
const requeueModalVisible = ref(false);
const requeueModalRef = ref<InstanceType<typeof JobRequeueModal>>();

const handleRequeue = (record: ExecutorJobDTO) => {
  currentJob.value = record;
  requeueModalVisible.value = true;
};

const handleRequeueSubmit = async (runAt?: number) => {
  if (!currentJob.value) return;
  try {
    requeueModalRef.value?.setSubmitting(true);
    await requeueJob(currentJob.value.id, { run_at: runAt });
    Message.success(t('executor.message.requeue.success'));
    requeueModalVisible.value = false;
    await fetchData();
    await loadStats();
  } catch {
    Message.error(t('executor.message.requeue.failed'));
  } finally {
    requeueModalRef.value?.setSubmitting(false);
  }
};

// ---- Update Args ----
const argsDrawerVisible = ref(false);
const argsDrawerRef = ref<InstanceType<typeof JobArgsDrawer>>();

const handleUpdateArgs = (record: ExecutorJobDTO) => {
  currentJob.value = record;
  argsDrawerVisible.value = true;
};

const handleUpdateArgsSubmit = async (argsJson: string) => {
  if (!currentJob.value) return;
  try {
    argsDrawerRef.value?.setSubmitting(true);
    await updateJobArgs(currentJob.value.id, { args_json: argsJson });
    Message.success(t('executor.message.updateArgs.success'));
    argsDrawerRef.value?.closeDrawer();
    await fetchData();
  } catch {
    Message.error(t('executor.message.updateArgs.failed'));
  } finally {
    argsDrawerRef.value?.setSubmitting(false);
  }
};

// ---- Cleanup ----
const cleanupModalVisible = ref(false);
const cleanupModalRef = ref<InstanceType<typeof JobCleanupModal>>();

const handleOpenCleanup = () => {
  cleanupModalVisible.value = true;
};

const handleCleanupSubmit = async (data: CleanupJobsRequest) => {
  try {
    cleanupModalRef.value?.setSubmitting(true);
    const res = await cleanupJobs(data);
    const count = (res as any).data?.deleted ?? 0;
    Message.success(t('executor.message.cleanup.success', { count }));
    cleanupModalVisible.value = false;
    await fetchData();
    await loadStats();
  } catch {
    Message.error(t('executor.message.cleanup.failed'));
  } finally {
    cleanupModalRef.value?.setSubmitting(false);
  }
};

// ---- Stat helper ----
const getStatValue = (dataKey: ExecutorNumericStatKey): number => {
  if (!stats.value) return 0;
  return stats.value[dataKey] ?? 0;
};

// ---- Util ----
const formatDateTime = (dateStr: string) => {
  if (!dateStr) return '';
  const d = new Date(dateStr);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}:${String(d.getSeconds()).padStart(2, '0')}`;
};

onMounted(() => {
  loadStats();
  fetchData();
});
</script>

<script lang="ts">
export default {
  name: 'ExecutorJobs',
};
</script>

<style scoped lang="less">
.container {
  padding: 0 20px 20px;
}

.stat-card {
  margin-top: 16px;
  :deep(.arco-statistic-title) {
    font-size: 13px;
  }
}

.general-card {
  margin-top: 16px;
}

:deep(.arco-table-th) {
  &:last-child {
    .arco-table-th-item-title {
      margin-left: 16px;
    }
  }
}
</style>
