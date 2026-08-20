<template>
  <a-drawer
    v-model:visible="drawerVisible"
    :title="$t('executor.attempts.title')"
    :width="800"
    :footer="false"
    @cancel="handleClose"
  >
    <a-spin :loading="loading" style="width: 100%">
      <a-empty v-if="!loading && attempts.length === 0" :description="$t('executor.attempts.empty')" />
      <a-table
        v-else
        row-key="id"
        :data="attempts"
        :loading="loading"
        :pagination="false"
        :bordered="{ cell: true }"
        :scroll="{ x: 700 }"
      >
        <template #columns>
          <a-table-column
            :title="$t('executor.attempts.column.attemptNo')"
            data-index="attempt_no"
            :width="90"
          />
          <a-table-column
            :title="$t('executor.attempts.column.workerId')"
            data-index="worker_id"
            :width="160"
            :ellipsis="true"
            :tooltip="true"
          >
            <template #cell="{ record }">
              <span v-if="record.worker_id">{{ record.worker_id }}</span>
              <span v-else style="color: var(--color-text-3)">-</span>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('executor.attempts.column.status')"
            data-index="status"
            :width="100"
          >
            <template #cell="{ record }">
              <a-tag :color="getStatusColor(record.status)">
                {{ $t(`executor.status.${record.status}`) }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('executor.attempts.column.startedAt')"
            data-index="started_at"
            :width="160"
          >
            <template #cell="{ record }">
              {{ record.started_at ? formatDateTime(record.started_at) : '-' }}
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('executor.attempts.column.finishedAt')"
            data-index="finished_at"
            :width="160"
          >
            <template #cell="{ record }">
              {{ record.finished_at ? formatDateTime(record.finished_at) : '-' }}
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('executor.attempts.column.error')"
            data-index="error"
            :ellipsis="true"
            :tooltip="true"
          >
            <template #cell="{ record }">
              <span v-if="record.error" style="color: var(--color-danger-6)">{{ record.error }}</span>
              <span v-else style="color: var(--color-text-3)">-</span>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-spin>
  </a-drawer>
</template>

<script lang="ts" setup>
import { ref, computed, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { Message } from '@arco-design/web-vue';
import { getJobAttempts, type ExecutorJobAttemptDTO, type JobStatus } from '@/api/executor';

const props = defineProps<{
  visible: boolean;
  jobId?: number | null;
}>();

const emit = defineEmits<{
  (e: 'update:visible', visible: boolean): void;
}>();

const { t } = useI18n();
const loading = ref(false);
const attempts = ref<ExecutorJobAttemptDTO[]>([]);

const drawerVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val),
});

const loadAttempts = async (id: number) => {
  try {
    loading.value = true;
    const res = await getJobAttempts(id);
    attempts.value = (res as any).data || [];
  } catch {
    Message.error(t('executor.message.loadAttempts.failed'));
  } finally {
    loading.value = false;
  }
};

watch(
  () => props.visible,
  async (val) => {
    if (val && props.jobId) {
      await loadAttempts(props.jobId);
    } else {
      attempts.value = [];
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
</script>
