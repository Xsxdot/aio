<template>
  <a-drawer
    :width="800"
    :visible="visible"
    :title="$t('servers.status.title')"
    @cancel="handleCancel"
  >
    <a-spin :loading="loading" style="width: 100%">
      <div v-if="!props.statusInfo && !loading" style="text-align: center; padding: 40px 0">
        <a-empty :description="$t('servers.status.noData')" />
      </div>

      <div v-else-if="props.statusInfo">
        <!-- 基本信息 -->
        <a-card :title="$t('servers.status.basic')" style="margin-bottom: 16px">
          <a-descriptions :column="2" bordered>
            <a-descriptions-item :label="$t('servers.column.id')">
              {{ props.statusInfo.id }}
            </a-descriptions-item>
            <a-descriptions-item :label="$t('servers.column.name')">
              {{ props.statusInfo.name }}
            </a-descriptions-item>
            <a-descriptions-item :label="$t('servers.column.extranetHost')">
              {{ props.statusInfo.extranetHost || props.statusInfo.host }}
            </a-descriptions-item>
            <a-descriptions-item :label="$t('servers.column.intranetHost')">
              <span v-if="props.statusInfo.intranetHost">{{ props.statusInfo.intranetHost }}</span>
              <span v-else style="color: var(--color-text-3)">-</span>
            </a-descriptions-item>
            <a-descriptions-item :label="$t('servers.column.status')">
              <a-tag :color="getStatusColor(props.statusInfo.statusSummary)">
                {{ getStatusText(props.statusInfo.statusSummary) }}
              </a-tag>
            </a-descriptions-item>
            <a-descriptions-item :label="$t('servers.column.enabled')">
              <a-tag :color="props.statusInfo.enabled ? 'green' : 'red'">
                {{ props.statusInfo.enabled ? $t('servers.enabled.yes') : $t('servers.enabled.no') }}
              </a-tag>
            </a-descriptions-item>
            <a-descriptions-item v-if="props.statusInfo.agentGrpcAddress" :label="$t('servers.column.agentGrpcAddress')">
              {{ props.statusInfo.agentGrpcAddress }}
            </a-descriptions-item>
            <a-descriptions-item v-if="props.statusInfo.tags" :label="$t('servers.column.tags')" :span="2">
              <a-space wrap>
                <a-tag v-for="(value, key) in props.statusInfo.tags" :key="key" size="small" color="arcoblue">
                  {{ key }}: {{ value }}
                </a-tag>
              </a-space>
            </a-descriptions-item>
            <a-descriptions-item v-if="props.statusInfo.comment" :label="$t('servers.column.comment')" :span="2">
              {{ props.statusInfo.comment }}
            </a-descriptions-item>
          </a-descriptions>
        </a-card>

        <!-- 资源使用情况 -->
        <a-card
          v-if="props.statusInfo.statusSummary !== 'unknown'"
          :title="$t('servers.status.resource')"
          style="margin-bottom: 16px"
        >
          <a-descriptions :column="2" bordered>
            <a-descriptions-item v-if="props.statusInfo.cpuPercent !== undefined" :label="$t('servers.status.cpuPercent')">
              <a-progress :percent="props.statusInfo.cpuPercent / 100" :show-text="false" />
              <span style="margin-left: 8px">{{ props.statusInfo.cpuPercent.toFixed(2) }}%</span>
            </a-descriptions-item>
            <a-descriptions-item v-if="props.statusInfo.memUsed !== undefined && props.statusInfo.memTotal !== undefined" :label="$t('servers.column.memUsage')">
              <a-progress
                :percent="props.statusInfo.memUsed / props.statusInfo.memTotal"
                :show-text="false"
              />
              <span style="margin-left: 8px">
                {{ formatBytes(props.statusInfo.memUsed) }} / {{ formatBytes(props.statusInfo.memTotal) }}
                ({{ ((props.statusInfo.memUsed / props.statusInfo.memTotal) * 100).toFixed(2) }}%)
              </span>
            </a-descriptions-item>
            <a-descriptions-item v-if="props.statusInfo.load1 !== undefined" :label="$t('servers.status.load1')">
              {{ props.statusInfo.load1.toFixed(2) }}
            </a-descriptions-item>
            <a-descriptions-item v-if="props.statusInfo.load5 !== undefined" :label="$t('servers.status.load5')">
              {{ props.statusInfo.load5.toFixed(2) }}
            </a-descriptions-item>
            <a-descriptions-item v-if="props.statusInfo.load15 !== undefined" :label="$t('servers.status.load15')">
              {{ props.statusInfo.load15.toFixed(2) }}
            </a-descriptions-item>
            <a-descriptions-item v-if="props.statusInfo.collectedAt" :label="$t('servers.status.collectedAt')">
              {{ formatDateTime(props.statusInfo.collectedAt) }}
            </a-descriptions-item>
            <a-descriptions-item v-if="props.statusInfo.reportedAt" :label="$t('servers.status.reportedAt')">
              {{ formatDateTime(props.statusInfo.reportedAt) }}
            </a-descriptions-item>
          </a-descriptions>
        </a-card>

        <!-- 磁盘信息 -->
        <a-card
          v-if="props.statusInfo.diskItems && props.statusInfo.diskItems.length > 0"
          :title="$t('servers.status.disk')"
          style="margin-bottom: 16px"
        >
          <a-table
            :data="props.statusInfo.diskItems"
            :pagination="false"
            :bordered="{ cell: true }"
          >
            <template #columns>
              <a-table-column
                :title="$t('servers.status.disk.mountPoint')"
                data-index="mountPoint"
              />
              <a-table-column
                :title="$t('servers.status.disk.used')"
                data-index="used"
              >
                <template #cell="{ record }">
                  {{ formatBytes(record.used) }}
                </template>
              </a-table-column>
              <a-table-column
                :title="$t('servers.status.disk.total')"
                data-index="total"
              >
                <template #cell="{ record }">
                  {{ formatBytes(record.total) }}
                </template>
              </a-table-column>
              <a-table-column
                :title="$t('servers.status.disk.percent')"
                data-index="percent"
              >
                <template #cell="{ record }">
                  <a-progress
                    :percent="record.percent / 100"
                    :show-text="false"
                    style="width: 100px; display: inline-block"
                  />
                  <span style="margin-left: 8px">{{ record.percent.toFixed(2) }}%</span>
                </template>
              </a-table-column>
            </template>
          </a-table>
        </a-card>

        <!-- 错误信息 -->
        <a-card
          v-if="props.statusInfo.errorMessage"
          :title="$t('servers.status.error')"
        >
          <a-alert type="error" :show-icon="true">
            {{ props.statusInfo.errorMessage }}
          </a-alert>
        </a-card>
      </div>
    </a-spin>

    <template #footer>
      <a-button @click="handleCancel">
        {{ $t('servers.form.cancel') }}
      </a-button>
    </template>
  </a-drawer>
</template>

<script lang="ts" setup>
import { ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import type { ServerStatusInfo } from '@/api/servers';

const { t } = useI18n();

interface Props {
  visible: boolean;
  statusInfo?: ServerStatusInfo | null;
}

const props = withDefaults(defineProps<Props>(), {
  visible: false,
  statusInfo: null,
});

const emit = defineEmits<{
  (e: 'update:visible', visible: boolean): void;
}>();

const loading = ref(false);

// 取消
const handleCancel = () => {
  emit('update:visible', false);
};

// 获取状态颜色
const getStatusColor = (status: string) => {
  switch (status) {
    case 'online':
      return 'green';
    case 'offline':
      return 'red';
    case 'error':
      return 'orange';
    default:
      return 'gray';
  }
};

// 获取状态文本
const getStatusText = (status: string) => {
  switch (status) {
    case 'online':
      return t('servers.status.online');
    case 'offline':
      return t('servers.status.offline');
    case 'error':
      return t('servers.status.error');
    default:
      return t('servers.status.unknown');
  }
};

// 格式化字节
const formatBytes = (bytes: number) => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / k ** i).toFixed(2)} ${sizes[i]}`;
};

// 格式化日期时间
const formatDateTime = (dateString: string) => {
  if (!dateString) return '';
  const date = new Date(dateString);
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  const hours = String(date.getHours()).padStart(2, '0');
  const minutes = String(date.getMinutes()).padStart(2, '0');
  const seconds = String(date.getSeconds()).padStart(2, '0');
  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
};
</script>

<script lang="ts">
export default {
  name: 'ServerStatusDrawer',
};
</script>
