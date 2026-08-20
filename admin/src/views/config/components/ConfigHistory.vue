<template>
  <div class="config-history">
    <div class="toolbar">
      <a-space>
        <a-button @click="fetchHistory">
          <template #icon>
            <icon-refresh />
          </template>
          {{ $t('config.refresh') }}
        </a-button>
      </a-space>
    </div>

    <a-spin :loading="loading">
      <a-empty
        v-if="historyList.length === 0"
        :description="$t('config.history.empty')"
      />
      <a-table
        v-else
        row-key="id"
        :data="historyList"
        :pagination="{ pageSize: 10 }"
        :bordered="true"
        stripe
      >
        <template #columns>
          <a-table-column
            :title="$t('config.history.version')"
            data-index="version"
            :width="100"
          >
            <template #cell="{ record }">
              <a-tag>v{{ record.version }}</a-tag>
            </template>
          </a-table-column>

          <a-table-column
            :title="$t('config.history.operator')"
            data-index="operator"
            :width="120"
          />

          <a-table-column
            :title="$t('config.history.changeNote')"
            data-index="changeNote"
          >
            <template #cell="{ record }">
              {{ record.changeNote || '-' }}
            </template>
          </a-table-column>

          <a-table-column
            :title="$t('config.history.createdAt')"
            data-index="createdAt"
            :width="180"
          >
            <template #cell="{ record }">
              {{
                record.createdAt
                  ? new Date(record.createdAt).toLocaleString()
                  : '-'
              }}
            </template>
          </a-table-column>

          <a-table-column
            :title="$t('config.history.actions')"
            fixed="right"
            :width="150"
          >
            <template #cell="{ record }">
              <a-space>
                <a-button size="mini" @click="viewVersion(record)">
                  {{ $t('config.view') }}
                </a-button>
                <a-button
                  size="mini"
                  status="warning"
                  @click="rollbackToVersion(record)"
                >
                  {{ $t('config.history.rollback') }}
                </a-button>
              </a-space>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-spin>

    <!-- 版本详情对话框 -->
    <a-modal
      v-model:visible="detailVisible"
      :title="$t('config.history.versionDetail')"
      :footer="false"
      :mask-closable="true"
      width="800px"
    >
      <a-descriptions
        v-if="selectedVersion"
        :column="2"
        :label-style="{ 'font-weight': 'bold' }"
        bordered
        size="large"
      >
        <a-descriptions-item :label="$t('config.history.version')">
          v{{ selectedVersion.version }}
        </a-descriptions-item>
        <a-descriptions-item :label="$t('config.history.operator')">
          {{ selectedVersion.operator }}
        </a-descriptions-item>
        <a-descriptions-item :label="$t('config.history.createdAt')" :span="2">
          {{
            selectedVersion.createdAt
              ? new Date(selectedVersion.createdAt).toLocaleString()
              : '-'
          }}
        </a-descriptions-item>
        <a-descriptions-item :label="$t('config.history.changeNote')" :span="2">
          {{ selectedVersion.changeNote || '-' }}
        </a-descriptions-item>
      </a-descriptions>

      <template v-if="selectedVersion">
        <a-divider>{{ $t('config.history.configValues') }}</a-divider>
        <a-card
          v-for="(value, key) in selectedVersion.value"
          :key="key"
          class="version-value-card"
          size="small"
        >
          <template #title>{{ key }}</template>
          <template #extra>
            <a-tag>{{ value.type }}</a-tag>
          </template>
          <div class="value-content">
            <pre v-if="value.type === 'object' || value.type === 'array'">{{
              formatJSON(value.value)
            }}</pre>
            <span v-else>{{ value.value }}</span>
          </div>
        </a-card>
      </template>
    </a-modal>
  </div>
</template>

<script lang="ts" setup>
  import { ref, watch } from 'vue';
  import { Message, Modal } from '@arco-design/web-vue';
  import { IconRefresh } from '@arco-design/web-vue/es/icon';
  import {
    getHistory,
    rollback,
    type ConfigHistoryResponse,
  } from '@/api/config';

  const props = defineProps<{
    configId: number;
  }>();

  const emit = defineEmits<{
    refresh: [];
  }>();

  // 状态变量
  const loading = ref(false);
  const historyList = ref<ConfigHistoryResponse[]>([]);
  const selectedVersion = ref<ConfigHistoryResponse | null>(null);
  const detailVisible = ref(false);

  // 获取历史记录
  async function fetchHistory() {
    try {
      loading.value = true;
      const res = await getHistory(props.configId);
      historyList.value = res.data || [];
    } catch (error) {
      Message.error('获取历史记录失败');
      historyList.value = [];
    } finally {
      loading.value = false;
    }
  }

  watch(
    () => props.configId,
    () => {
      // 切换配置时重置状态，避免仍显示旧配置的弹窗/数据
      selectedVersion.value = null;
      detailVisible.value = false;
      historyList.value = [];
      fetchHistory();
    },
    { immediate: true }
  );

  // 查看版本详情
  function viewVersion(record: ConfigHistoryResponse) {
    selectedVersion.value = record;
    detailVisible.value = true;
  }

  // 回滚到指定版本
  function rollbackToVersion(record: ConfigHistoryResponse) {
    Modal.confirm({
      title: '确认回滚',
      content: `确定要回滚到版本 v${record.version} 吗？`,
      okText: '确认',
      cancelText: '取消',
      onOk: async () => {
        try {
          await rollback(props.configId, record.version);
          Message.success('回滚成功');
          fetchHistory();
          emit('refresh');
        } catch (error: any) {
          Message.error(`回滚失败: ${error.message || '未知错误'}`);
        }
      },
    });
  }

  // 格式化JSON
  function formatJSON(value: string): string {
    try {
      const parsed = JSON.parse(value);
      return JSON.stringify(parsed, null, 2);
    } catch {
      return value;
    }
  }
</script>

<style scoped lang="less">
  .config-history {
    padding: 16px;
    max-width: 100%;
    overflow: hidden;

    .toolbar {
      margin-bottom: 16px;
    }

    :deep(.arco-table-container) {
      overflow-x: auto;
    }

    .version-value-card {
      margin-bottom: 12px;

      .value-content {
        max-height: 300px;
        overflow-y: auto;

        pre {
          margin: 0;
          white-space: pre-wrap;
          word-break: break-all;
        }
      }
    }
  }
</style>
