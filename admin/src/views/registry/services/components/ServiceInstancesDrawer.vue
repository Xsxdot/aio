<template>
  <a-drawer
    v-model:visible="drawerVisible"
    :title="$t('registry.instances.title')"
    :width="900"
    @cancel="handleClose"
  >
    <!-- 服务信息卡片 -->
    <a-card class="service-info-card" :bordered="false">
      <a-descriptions :column="2" size="small">
        <a-descriptions-item :label="$t('registry.services.column.project')">
          {{ serviceInfo?.project }}
        </a-descriptions-item>
        <a-descriptions-item :label="$t('registry.services.column.name')">
          {{ serviceInfo?.name }}
        </a-descriptions-item>
        <a-descriptions-item :label="$t('registry.services.column.owner')">
          {{ serviceInfo?.owner || '-' }}
        </a-descriptions-item>
        <a-descriptions-item :label="$t('registry.instances.stats.total')">
          <a-tag color="arcoblue">{{ filteredInstances.length }}</a-tag>
        </a-descriptions-item>
      </a-descriptions>
    </a-card>

    <!-- 筛选区 -->
    <a-card class="filter-card" :bordered="false">
      <a-space wrap>
        <a-select
          v-model="filterEnv"
          :placeholder="$t('registry.instances.filter.env')"
          style="width: 180px"
          allow-clear
          @change="handleEnvChange"
        >
          <a-option value="">{{ $t('registry.instances.filter.env.all') }}</a-option>
          <a-option value="__global__">{{ $t('registry.instances.filter.env.global') }}</a-option>
          <a-option
            v-for="env in uniqueEnvs"
            :key="env"
            :value="env"
          >
            {{ env }}
          </a-option>
        </a-select>

        <a-input
          v-model="filterKeyword"
          :placeholder="$t('registry.instances.filter.keyword')"
          allow-clear
          style="width: 260px"
        >
          <template #prefix>
            <icon-search />
          </template>
        </a-input>

        <a-radio-group v-model="aliveOnly" type="button" @change="fetchInstances">
          <a-radio :value="true">{{ $t('registry.instances.filter.aliveOnly') }}</a-radio>
          <a-radio :value="false">{{ $t('registry.instances.filter.all') }}</a-radio>
        </a-radio-group>

        <a-button @click="fetchInstances">
          <template #icon>
            <icon-refresh />
          </template>
          {{ $t('registry.services.refresh') }}
        </a-button>
      </a-space>
    </a-card>

    <!-- 实例表格 -->
    <a-table
      :loading="loading"
      :data="displayInstances"
      :pagination="false"
      :bordered="{ cell: true }"
      row-key="id"
      class="instances-table"
    >
      <template #columns>
        <a-table-column
          :title="$t('registry.instances.column.env')"
          data-index="env"
          :width="120"
        >
          <template #cell="{ record }">
            <a-tag v-if="isGlobalEnv(record.env)" color="purple">
              {{ $t('registry.instances.filter.env.global') }}
            </a-tag>
            <a-tag v-else color="arcoblue">
              {{ record.env || '-' }}
            </a-tag>
          </template>
        </a-table-column>

        <a-table-column
          :title="$t('registry.instances.column.instanceKey')"
          data-index="instanceKey"
          :width="200"
          :ellipsis="true"
          :tooltip="true"
        >
          <template #cell="{ record }">
            <a-space>
              <span>{{ record.instanceKey }}</span>
              <a-button
                type="text"
                size="mini"
                @click="copyToClipboard(record.instanceKey)"
              >
                <icon-copy />
              </a-button>
            </a-space>
          </template>
        </a-table-column>

        <a-table-column
          :title="$t('registry.instances.column.host')"
          data-index="host"
          :width="150"
        />

        <a-table-column
          :title="$t('registry.instances.column.endpoint')"
          data-index="endpoint"
          :width="200"
          :ellipsis="true"
          :tooltip="true"
        >
          <template #cell="{ record }">
            <a-space>
              <span>{{ record.endpoint }}</span>
              <a-button
                type="text"
                size="mini"
                @click="copyToClipboard(record.endpoint)"
              >
                <icon-copy />
              </a-button>
            </a-space>
          </template>
        </a-table-column>

        <a-table-column
          :title="$t('registry.instances.column.ttl')"
          data-index="ttlSeconds"
          :width="80"
        >
          <template #cell="{ record }">
            {{ record.ttlSeconds }}s
          </template>
        </a-table-column>

        <a-table-column
          :title="$t('registry.instances.column.lastHeartbeat')"
          data-index="lastHeartbeatAt"
          :width="180"
        >
          <template #cell="{ record }">
            {{ formatDateTime(record.lastHeartbeatAt) }}
          </template>
        </a-table-column>

        <a-table-column
          :title="$t('registry.instances.column.meta')"
          :width="80"
        >
          <template #cell="{ record }">
            <a-button type="text" size="small" @click="viewMeta(record)">
              {{ $t('registry.instances.action.viewMeta') }}
            </a-button>
          </template>
        </a-table-column>

        <a-table-column
          :title="$t('registry.instances.column.actions')"
          :width="180"
          fixed="right"
        >
          <template #cell="{ record }">
            <a-space>
              <a-popconfirm
                :content="$t('registry.instances.confirm.offline.content')"
                @ok="handleOffline(record)"
              >
                <a-button type="text" status="warning" size="small">
                  {{ $t('registry.instances.action.offline') }}
                </a-button>
              </a-popconfirm>
              <a-divider direction="vertical" />
              <a-popconfirm
                :content="$t('registry.instances.confirm.delete.content')"
                @ok="handleDelete(record)"
              >
                <a-button type="text" status="danger" size="small">
                  {{ $t('registry.instances.action.delete') }}
                </a-button>
              </a-popconfirm>
            </a-space>
          </template>
        </a-table-column>
      </template>
    </a-table>

    <!-- Meta 查看弹窗 -->
    <JsonEditorModal
      v-model:visible="metaViewModalVisible"
      :title="$t('registry.instances.meta.view')"
      :value="metaViewJson"
      mode="object"
      :width="700"
    />
  </a-drawer>
</template>

<script lang="ts" setup>
import { ref, computed, watch } from 'vue';
import { Message } from '@arco-design/web-vue';
import { useI18n } from 'vue-i18n';
import {
  listServiceInstances,
  offlineInstance,
  deleteInstance,
  type ServiceDTO,
  type InstanceDTO,
} from '@/api/registry-services';
import JsonEditorModal from '@/components/json-editor/JsonEditorModal.vue';

interface Props {
  visible: boolean;
  serviceInfo: ServiceDTO | null;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  (e: 'update:visible', visible: boolean): void;
  (e: 'refresh'): void;
}>();

const { t } = useI18n();

const drawerVisible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value),
});

// 数据状态
const loading = ref(false);
const allInstances = ref<InstanceDTO[]>([]);
const filterEnv = ref('');
const filterKeyword = ref('');
const aliveOnly = ref(true);

// Meta 查看
const metaViewModalVisible = ref(false);
const metaViewJson = ref('{}');

// 判断是否为 global 环境
const isGlobalEnv = (env: string) => {
  return env === '' || env.toLowerCase() === 'global';
};

// 获取所有唯一的非 global 环境
const uniqueEnvs = computed(() => {
  const envs = new Set<string>();
  allInstances.value.forEach((inst) => {
    if (!isGlobalEnv(inst.env)) {
      envs.add(inst.env);
    }
  });
  return Array.from(envs).sort();
});

// 按环境过滤
const filteredByEnv = computed(() => {
  if (!filterEnv.value) {
    return allInstances.value;
  }
  if (filterEnv.value === '__global__') {
    return allInstances.value.filter((inst) => isGlobalEnv(inst.env));
  }
  return allInstances.value.filter((inst) => inst.env === filterEnv.value);
});

// 按关键字过滤
const filteredInstances = computed(() => {
  const keyword = filterKeyword.value.toLowerCase().trim();
  if (!keyword) {
    return filteredByEnv.value;
  }
  return filteredByEnv.value.filter(
    (inst) =>
      inst.instanceKey.toLowerCase().includes(keyword) ||
      inst.host.toLowerCase().includes(keyword) ||
      inst.endpoint.toLowerCase().includes(keyword)
  );
});

// 显示数据
const displayInstances = computed(() => filteredInstances.value);

// 加载实例列表
const fetchInstances = async () => {
  if (!props.serviceInfo) return;

  try {
    loading.value = true;
    const res = await listServiceInstances(props.serviceInfo.id, {
      aliveOnly: aliveOnly.value,
    });
    allInstances.value = res.data.content || [];
  } catch (err) {
    Message.error(t('registry.instances.message.load.failed'));
    console.error(err);
  } finally {
    loading.value = false;
  }
};

// 监听抽屉打开
watch(
  () => props.visible,
  (visible) => {
    if (visible && props.serviceInfo) {
      filterEnv.value = '';
      filterKeyword.value = '';
      fetchInstances();
    }
  }
);

// 环境筛选变化
const handleEnvChange = () => {
  // 触发重新计算
};

// 关闭抽屉
const handleClose = () => {
  drawerVisible.value = false;
  allInstances.value = [];
};

// 查看 Meta
const viewMeta = (record: InstanceDTO) => {
  try {
    metaViewJson.value = JSON.stringify(record.meta || {}, null, 2);
    metaViewModalVisible.value = true;
  } catch (e) {
    Message.error('Failed to parse meta');
    console.error(e);
  }
};

// 复制到剪贴板
const copyToClipboard = async (text: string) => {
  try {
    await navigator.clipboard.writeText(text);
    Message.success(t('registry.instances.message.copy.success'));
  } catch (err) {
    Message.error(t('registry.instances.message.copy.failed'));
    console.error(err);
  }
};

// 强制下线
const handleOffline = async (record: InstanceDTO) => {
  if (!props.serviceInfo) return;

  try {
    await offlineInstance(props.serviceInfo.id, record.instanceKey);
    Message.success(t('registry.instances.message.offline.success'));
    await fetchInstances();
    emit('refresh');
  } catch (err) {
    Message.error(t('registry.instances.message.offline.failed'));
    console.error(err);
  }
};

// 删除实例
const handleDelete = async (record: InstanceDTO) => {
  if (!props.serviceInfo) return;

  try {
    await deleteInstance(props.serviceInfo.id, record.instanceKey);
    Message.success(t('registry.instances.message.delete.success'));
    await fetchInstances();
    emit('refresh');
  } catch (err) {
    Message.error(t('registry.instances.message.delete.failed'));
    console.error(err);
  }
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

<style scoped lang="less">
.service-info-card {
  margin-bottom: 16px;
  background: var(--color-fill-1);
}

.filter-card {
  margin-bottom: 16px;
}

.instances-table {
  :deep(.arco-table-th) {
    background-color: var(--color-fill-2);
  }
}
</style>




