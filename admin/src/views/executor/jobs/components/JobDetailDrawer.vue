<template>
  <a-drawer
    v-model:visible="drawerVisible"
    :title="$t('executor.detail.title')"
    :width="560"
    :footer="false"
    @cancel="handleClose"
  >
    <a-spin :loading="loading" style="width: 100%">
      <template v-if="detail">
        <a-descriptions :column="1" bordered>
          <a-descriptions-item :label="$t('executor.detail.id')">
            {{ detail.id }}
          </a-descriptions-item>
          <a-descriptions-item :label="$t('executor.detail.env')">
            <a-tag :color="detail.env === 'prod' ? 'red' : detail.env === 'dev' ? 'arcoblue' : 'orange'">
              {{ detail.env || '-' }}
            </a-tag>
          </a-descriptions-item>
          <a-descriptions-item :label="$t('executor.detail.targetService')">
            {{ detail.target_service }}
          </a-descriptions-item>
          <a-descriptions-item :label="$t('executor.detail.method')">
            {{ detail.method }}
          </a-descriptions-item>
          <a-descriptions-item :label="$t('executor.detail.status')">
            <a-tag :color="getStatusColor(detail.status)">
              {{ $t(`executor.status.${detail.status}`) }}
            </a-tag>
          </a-descriptions-item>
          <a-descriptions-item :label="$t('executor.detail.priority')">
            {{ detail.priority }}
          </a-descriptions-item>
          <a-descriptions-item :label="$t('executor.detail.attempts')">
            {{ detail.attempts }} / {{ detail.max_attempts }}
          </a-descriptions-item>
          <a-descriptions-item :label="$t('executor.detail.nextRunAt')">
            {{ detail.next_run_at ? formatDateTime(detail.next_run_at) : $t('executor.detail.none') }}
          </a-descriptions-item>
          <a-descriptions-item :label="$t('executor.detail.leaseOwner')">
            {{ detail.lease_owner || $t('executor.detail.none') }}
          </a-descriptions-item>
          <a-descriptions-item :label="$t('executor.detail.leaseUntil')">
            {{ detail.lease_until ? formatDateTime(detail.lease_until) : $t('executor.detail.none') }}
          </a-descriptions-item>
          <a-descriptions-item :label="$t('executor.detail.dedupKey')">
            {{ detail.dedup_key || $t('executor.detail.none') }}
          </a-descriptions-item>
          <a-descriptions-item :label="$t('executor.detail.createdAt')">
            {{ formatDateTime(detail.createdAt) }}
          </a-descriptions-item>
          <a-descriptions-item :label="$t('executor.detail.updatedAt')">
            {{ formatDateTime(detail.updatedAt) }}
          </a-descriptions-item>
        </a-descriptions>

        <div v-if="detail.args_json" style="margin-top: 16px">
          <div style="font-weight: 600; margin-bottom: 8px">{{ $t('executor.detail.argsJson') }}</div>
          <pre class="json-block">{{ formatJson(detail.args_json) }}</pre>
        </div>

        <div v-if="detail.result_json" style="margin-top: 16px">
          <div style="font-weight: 600; margin-bottom: 8px">{{ $t('executor.detail.resultJson') }}</div>
          <pre class="json-block">{{ formatJson(detail.result_json) }}</pre>
        </div>

        <div v-if="detail.last_error" style="margin-top: 16px">
          <div style="font-weight: 600; margin-bottom: 8px; color: var(--color-danger-6)">
            {{ $t('executor.detail.lastError') }}
          </div>
          <pre class="json-block error-block">{{ detail.last_error }}</pre>
        </div>
      </template>
    </a-spin>
  </a-drawer>
</template>

<script lang="ts" setup>
import { ref, computed, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { Message } from '@arco-design/web-vue';
import { getJob, type ExecutorJobDTO, type JobStatus } from '@/api/executor';

const props = defineProps<{
  visible: boolean;
  jobId?: number | null;
}>();

const emit = defineEmits<{
  (e: 'update:visible', visible: boolean): void;
}>();

const { t } = useI18n();
const loading = ref(false);
const detail = ref<ExecutorJobDTO | null>(null);

const drawerVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val),
});

const loadDetail = async (id: number) => {
  try {
    loading.value = true;
    const res = await getJob(id);
    detail.value = (res as any).data;
  } catch {
    Message.error(t('executor.message.loadDetail.failed'));
  } finally {
    loading.value = false;
  }
};

watch(
  () => props.visible,
  async (val) => {
    if (val && props.jobId) {
      await loadDetail(props.jobId);
    } else {
      detail.value = null;
    }
  }
);

const handleClose = () => {
  drawerVisible.value = false;
};

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

const formatDateTime = (dateStr: string) => {
  if (!dateStr) return '';
  const d = new Date(dateStr);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}:${String(d.getSeconds()).padStart(2, '0')}`;
};

const formatJson = (str: string) => {
  if (!str) return '';
  try { return JSON.stringify(JSON.parse(str), null, 2); } catch { return str; }
};
</script>

<style scoped lang="less">
.json-block {
  background: var(--color-fill-2);
  border: 1px solid var(--color-border);
  border-radius: 4px;
  padding: 12px;
  font-size: 12px;
  line-height: 1.6;
  overflow: auto;
  max-height: 300px;
  white-space: pre-wrap;
  word-break: break-all;
}

.error-block {
  border-color: var(--color-danger-3);
  background: var(--color-danger-1);
  color: var(--color-danger-6);
}
</style>
