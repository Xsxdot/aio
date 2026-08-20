<template>
  <a-drawer
    v-model:visible="drawerVisible"
    :title="$t('ssl.certificates.history.title')"
    :width="900"
    @cancel="handleCancel"
  >
    <a-alert style="margin-bottom: 20px">
      {{ $t('Certificate') }}: <strong>{{ certificate?.name }}</strong>
    </a-alert>

    <a-table
      row-key="id"
      :loading="loading"
      :data="historyData"
      :pagination="false"
      :bordered="{ cell: true }"
    >
      <template #columns>
        <a-table-column :title="$t('ssl.certificates.history.column.id')" data-index="id" :width="80" />
        <a-table-column :title="$t('ssl.certificates.history.column.targetId')" data-index="deploy_target_id" :width="100" />
        <a-table-column :title="$t('ssl.certificates.history.column.status')" data-index="status" :width="100">
          <template #cell="{ record }">
            <a-tag :color="getDeployStatusColor(record.status)">
              {{ $t(`ssl.certificates.history.deployStatus.${record.status}`) }}
            </a-tag>
          </template>
        </a-table-column>
        <a-table-column :title="$t('ssl.certificates.history.column.triggerType')" data-index="trigger_type" :width="120">
          <template #cell="{ record }">
            <a-tag color="arcoblue">
              {{ getTriggerTypeLabel(record.trigger_type) }}
            </a-tag>
          </template>
        </a-table-column>
        <a-table-column :title="$t('ssl.certificates.history.column.startTime')" data-index="start_time" :width="180">
          <template #cell="{ record }">
            {{ formatDateTime(record.start_time) }}
          </template>
        </a-table-column>
        <a-table-column :title="$t('ssl.certificates.history.column.endTime')" data-index="end_time" :width="180">
          <template #cell="{ record }">
            {{ formatDateTime(record.end_time) }}
          </template>
        </a-table-column>
        <a-table-column :title="$t('ssl.certificates.history.column.errorMessage')" data-index="error_message">
          <template #cell="{ record }">
            <a-tooltip v-if="record.error_message" :content="record.error_message" position="top">
              <template #content>
                <div class="error-tooltip-content">
                  {{ record.error_message }}
                </div>
              </template>
              <a-typography-text type="danger" :ellipsis="{ rows: 2 }">
                {{ record.error_message }}
              </a-typography-text>
            </a-tooltip>
            <span v-else>-</span>
          </template>
        </a-table-column>
      </template>
    </a-table>

    <template #footer>
      <a-button @click="handleCancel">
        {{ $t('Close') }}
      </a-button>
    </template>
  </a-drawer>
</template>

<script lang="ts" setup>
import { ref, computed, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import dayjs from 'dayjs';
import {
  getCertificateDeployHistory,
  type CertificateDTO,
  type DeployHistoryDTO,
} from '@/api/ssl';

const props = defineProps<{
  visible: boolean;
  certificate: CertificateDTO | null;
}>();

const emit = defineEmits<{
  (e: 'update:visible', visible: boolean): void;
}>();

const { t } = useI18n();
const loading = ref(false);
const historyData = ref<DeployHistoryDTO[]>([]);

const drawerVisible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value),
});

// 加载部署历史
const fetchHistory = async () => {
  if (!props.certificate) return;
  
  try {
    loading.value = true;
    const { data } = await getCertificateDeployHistory(props.certificate.id, 50);
    historyData.value = data || [];
  } catch (error) {
    // Error handled by interceptor
  } finally {
    loading.value = false;
  }
};

// 监听抽屉打开，加载数据
watch(
  () => props.visible,
  (visible) => {
    if (visible) {
      fetchHistory();
    }
  }
);

const handleCancel = () => {
  drawerVisible.value = false;
};

// 获取部署状态颜色
const getDeployStatusColor = (status: string) => {
  const colors: Record<string, string> = {
    pending: 'gray',
    deploying: 'blue',
    success: 'green',
    failed: 'red',
    partial: 'orange',
  };
  return colors[status] || 'gray';
};

// 获取触发类型标签
const getTriggerTypeLabel = (triggerType: string) => {
  const labels: Record<string, string> = {
    manual: t('ssl.certificates.history.triggerType.manual'),
    auto_renew: t('ssl.certificates.history.triggerType.autoRenew'),
    auto_issue: t('ssl.certificates.history.triggerType.autoIssue'),
  };
  return labels[triggerType] || triggerType;
};

// 格式化日期时间
const formatDateTime = (dateStr?: string) => {
  if (!dateStr) return '-';
  return dayjs(dateStr).format('YYYY-MM-DD HH:mm:ss');
};
</script>

<style scoped lang="less">
.error-tooltip-content {
  max-width: 500px;
  max-height: 300px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-word;
  line-height: 1.5;
}
</style>
