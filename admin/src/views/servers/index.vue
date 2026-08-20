<template>
  <div class="container">
    <a-card class="general-card" :title="$t('servers.list.title')">
      <!-- 搜索区域 -->
      <a-row style="margin-bottom: 16px">
        <a-col :flex="1">
          <a-form :model="searchForm" layout="inline">
            <a-form-item field="name" style="margin-bottom: 0">
              <a-input
                v-model="searchForm.name"
                :placeholder="$t('servers.search.name.placeholder')"
                allow-clear
                style="width: 200px"
                @press-enter="handleSearch"
              >
                <template #prefix>
                  <icon-search />
                </template>
              </a-input>
            </a-form-item>
            <a-form-item field="tag" style="margin-bottom: 0">
              <a-input
                v-model="searchForm.tag"
                :placeholder="$t('servers.search.tag.placeholder')"
                allow-clear
                style="width: 150px"
                @press-enter="handleSearch"
              />
            </a-form-item>
            <a-form-item field="enabled" style="margin-bottom: 0">
              <a-select
                v-model="searchForm.enabled"
                :placeholder="$t('servers.search.enabled.placeholder')"
                allow-clear
                style="width: 130px"
                @change="handleSearch"
              >
                <a-option :value="true">{{ $t('servers.search.enabled.yes') }}</a-option>
                <a-option :value="false">{{ $t('servers.search.enabled.no') }}</a-option>
              </a-select>
            </a-form-item>
          </a-form>
        </a-col>
        <a-col :flex="'86px'" style="text-align: right">
          <a-button @click="handleReset">
            <template #icon>
              <icon-refresh />
            </template>
            {{ $t('servers.reset') }}
          </a-button>
        </a-col>
      </a-row>

      <!-- 操作按钮 -->
      <a-row style="margin-bottom: 16px">
        <a-col :span="12">
          <a-space>
            <a-button type="primary" @click="handleCreate">
              <template #icon>
                <icon-plus />
              </template>
              {{ $t('servers.create.button') }}
            </a-button>
            <a-button @click="handleRefresh">
              <template #icon>
                <icon-refresh />
              </template>
              {{ $t('servers.refresh') }}
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
        :scroll="{ x: 1980 }"
        @page-change="onPageChange"
        @page-size-change="onPageSizeChange"
      >
        <template #columns>
          <a-table-column
            :title="$t('servers.column.id')"
            data-index="id"
            :width="80"
          />
          <a-table-column
            :title="$t('servers.column.name')"
            data-index="name"
            :width="150"
          />
          <a-table-column
            :title="$t('servers.column.extranetHost')"
            data-index="extranetHost"
            :width="180"
          >
            <template #cell="{ record }">
              {{ record.extranetHost || record.host }}
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('servers.column.intranetHost')"
            data-index="intranetHost"
            :width="180"
          >
            <template #cell="{ record }">
              <span v-if="record.intranetHost">{{ record.intranetHost }}</span>
              <span v-else style="color: var(--color-text-3)">-</span>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('servers.column.enabled')"
            data-index="enabled"
            :width="100"
          >
            <template #cell="{ record }">
              <a-tag :color="record.enabled ? 'green' : 'red'">
                {{ record.enabled ? $t('servers.enabled.yes') : $t('servers.enabled.no') }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('servers.column.status')"
            data-index="statusSummary"
            :width="100"
          >
            <template #cell="{ record }">
              <a-tag :color="getStatusColor(record.statusSummary)">
                {{ getStatusText(record.statusSummary) }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('servers.column.cpuPercent')"
            data-index="cpuPercent"
            :width="120"
          >
            <template #cell="{ record }">
              <span v-if="record.cpuPercent !== undefined">{{ record.cpuPercent.toFixed(2) }}%</span>
              <span v-else style="color: var(--color-text-3)">-</span>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('servers.column.memUsage')"
            data-index="memUsage"
            :width="180"
          >
            <template #cell="{ record }">
              <span v-if="record.memUsed !== undefined && record.memTotal !== undefined">
                {{ formatBytes(record.memUsed) }} / {{ formatBytes(record.memTotal) }}
                ({{ ((record.memUsed / record.memTotal) * 100).toFixed(2) }}%)
              </span>
              <span v-else style="color: var(--color-text-3)">-</span>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('servers.column.load')"
            data-index="load"
            :width="140"
          >
            <template #cell="{ record }">
              <span v-if="record.load1 !== undefined">
                {{ record.load1.toFixed(2) }} / {{ record.load5?.toFixed(2) }} / {{ record.load15?.toFixed(2) }}
              </span>
              <span v-else style="color: var(--color-text-3)">-</span>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('servers.column.tags')"
            data-index="tags"
            :width="150"
            :ellipsis="true"
            :tooltip="true"
          >
            <template #cell="{ record }">
              <a-space wrap v-if="record.tags && Object.keys(record.tags).length > 0">
                <a-tag v-for="(value, key) in record.tags" :key="key" size="small" color="arcoblue">
                  {{ key }}:{{ value }}
                </a-tag>
              </a-space>
              <span v-else style="color: var(--color-text-3)">-</span>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('servers.column.comment')"
            data-index="comment"
            :width="180"
            :ellipsis="true"
            :tooltip="true"
          >
            <template #cell="{ record }">
              <span v-if="record.comment">{{ record.comment }}</span>
              <span v-else style="color: var(--color-text-3)">-</span>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('servers.column.createdAt')"
            data-index="createdAt"
            :width="180"
          >
            <template #cell="{ record }">
              {{ formatDateTime(record.createdAt) }}
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('servers.column.actions')"
            :width="300"
            fixed="right"
          >
            <template #cell="{ record }">
              <a-space>
                <a-button type="text" size="small" @click="handleEdit(record)">
                  {{ $t('servers.action.edit') }}
                </a-button>
                <a-divider direction="vertical" />
                <a-button type="text" size="small" @click="handleSSH(record)">
                  {{ $t('servers.action.ssh') }}
                </a-button>
                <a-divider direction="vertical" />
                <a-button type="text" size="small" @click="handleStatus(record)">
                  {{ $t('servers.action.status') }}
                </a-button>
                <a-divider direction="vertical" />
                <a-popconfirm
                  :content="$t('servers.confirm.delete.content')"
                  @ok="handleDelete(record)"
                >
                  <a-button type="text" status="danger" size="small">
                    {{ $t('servers.action.delete') }}
                  </a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-card>

    <!-- 服务器表单抽屉 -->
    <ServerFormDrawer
      v-model:visible="formDrawerVisible"
      :server="currentServer"
      @submit="handleFormSubmit"
    />

    <!-- SSH 凭证抽屉 -->
    <ServerSshCredentialDrawer
      v-model:visible="sshDrawerVisible"
      :server-id="currentServerId"
      @success="handleSSHSuccess"
    />

    <!-- 状态详情抽屉 -->
    <ServerStatusDrawer
      v-model:visible="statusDrawerVisible"
      :status-info="currentStatusInfo"
    />
  </div>
</template>

<script lang="ts" setup>
import { ref, reactive, onMounted } from 'vue';
import { Message } from '@arco-design/web-vue';
import { useI18n } from 'vue-i18n';
import {
  listServers,
  createServer,
  updateServer,
  deleteServer,
  listServerStatus,
  type ServerDTO,
  type ServerStatusInfo,
  type CreateServerRequest,
  type UpdateServerRequest,
} from '@/api/servers';
import ServerFormDrawer from './components/ServerFormDrawer.vue';
import ServerSshCredentialDrawer from './components/ServerSshCredentialDrawer.vue';
import ServerStatusDrawer from './components/ServerStatusDrawer.vue';

const { t } = useI18n();

// 搜索表单
const searchForm = reactive({
  name: '',
  tag: '',
  enabled: undefined as boolean | undefined,
});

// 表格数据
const loading = ref(false);
const tableData = ref<(ServerDTO & Partial<ServerStatusInfo>)[]>([]);

// 分页
const pagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
  showTotal: true,
  showPageSize: true,
});

// 状态缓存
const statusCache = ref<Map<number, ServerStatusInfo>>(new Map());
const statusFetchedAt = ref<number>(0);
const STATUS_TTL = 30000; // 30秒

// 表单抽屉
const formDrawerVisible = ref(false);
const currentServer = ref<ServerDTO | null>(null);

// SSH 抽屉
const sshDrawerVisible = ref(false);
const currentServerId = ref<number>();

// 状态详情抽屉
const statusDrawerVisible = ref(false);
const currentStatusInfo = ref<ServerStatusInfo | null>(null);

// 加载服务器状态
const loadServerStatus = async (force = false) => {
  const now = Date.now();
  // 如果缓存未过期且非强制刷新，直接返回
  if (!force && statusFetchedAt.value && now - statusFetchedAt.value < STATUS_TTL) {
    return;
  }

  try {
    const res = await listServerStatus();
    const statusList = res.data;
    statusCache.value.clear();
    statusList.forEach((status) => {
      statusCache.value.set(status.id, status);
    });
    statusFetchedAt.value = now;
  } catch (err) {
    console.error('Failed to load server status:', err);
    // 不显示错误消息，避免干扰用户
  }
};

// 合并服务器数据与状态
const mergeServerWithStatus = (servers: ServerDTO[]): (ServerDTO & Partial<ServerStatusInfo>)[] => {
  return servers.map((server) => {
    const status = statusCache.value.get(server.id);
    if (status) {
      return {
        ...server,
        statusSummary: status.statusSummary,
        cpuPercent: status.cpuPercent,
        memUsed: status.memUsed,
        memTotal: status.memTotal,
        load1: status.load1,
        load5: status.load5,
        load15: status.load15,
        reportedAt: status.reportedAt,
      };
    }
    return {
      ...server,
      statusSummary: 'unknown',
    };
  });
};

// 加载数据
const fetchData = async () => {
  try {
    loading.value = true;
    // 先加载状态（如果缓存过期）
    await loadServerStatus();

    // 加载服务器列表
    const params = {
      name: searchForm.name || undefined,
      tag: searchForm.tag || undefined,
      enabled: searchForm.enabled,
      pageNum: pagination.current,
      size: pagination.pageSize,
    };
    const res = await listServers(params);
    const servers = res.data.content || [];
    pagination.total = res.data.total || 0;

    // 合并状态
    tableData.value = mergeServerWithStatus(servers);
  } catch (err) {
    Message.error(t('servers.message.load.failed'));
    console.error(err);
  } finally {
    loading.value = false;
  }
};

// 搜索
const handleSearch = () => {
  pagination.current = 1;
  fetchData();
};

// 重置搜索
const handleReset = () => {
  searchForm.name = '';
  searchForm.tag = '';
  searchForm.enabled = undefined;
  handleSearch();
};

// 刷新
const handleRefresh = async () => {
  await loadServerStatus(true); // 强制刷新状态
  await fetchData();
};

// 分页变化
const onPageChange = (current: number) => {
  pagination.current = current;
  fetchData();
};

const onPageSizeChange = (pageSize: number) => {
  pagination.pageSize = pageSize;
  pagination.current = 1;
  fetchData();
};

// 新增
const handleCreate = () => {
  currentServer.value = null;
  formDrawerVisible.value = true;
};

// 编辑
const handleEdit = (record: ServerDTO) => {
  currentServer.value = record;
  formDrawerVisible.value = true;
};

// 表单提交
const handleFormSubmit = async (data: CreateServerRequest | UpdateServerRequest) => {
  try {
    if (currentServer.value) {
      // 编辑
      await updateServer(currentServer.value.id, data as UpdateServerRequest);
      Message.success(t('servers.message.update.success'));
    } else {
      // 新增
      await createServer(data as CreateServerRequest);
      Message.success(t('servers.message.create.success'));
    }
    await loadServerStatus(true); // 刷新状态缓存
    await fetchData();
  } catch (err) {
    const msgKey = currentServer.value
      ? 'servers.message.update.failed'
      : 'servers.message.create.failed';
    Message.error(t(msgKey));
    console.error(err);
  }
};

// SSH 凭证
const handleSSH = (record: ServerDTO) => {
  currentServerId.value = record.id;
  sshDrawerVisible.value = true;
};

// SSH 成功回调
const handleSSHSuccess = () => {
  // SSH 凭证变更不影响列表，无需刷新
};

// 状态详情
const handleStatus = async (record: ServerDTO & Partial<ServerStatusInfo>) => {
  // 优先使用缓存的完整状态
  const status = statusCache.value.get(record.id);
  if (status) {
    currentStatusInfo.value = status;
  } else {
    // 如果缓存没有，尝试刷新一次
    await loadServerStatus(true);
    const refreshedStatus = statusCache.value.get(record.id);
    if (refreshedStatus) {
      currentStatusInfo.value = refreshedStatus;
    } else {
      // 仍然没有，使用行数据（部分字段）
      currentStatusInfo.value = {
        ...record,
        statusSummary: record.statusSummary || 'unknown',
      } as ServerStatusInfo;
    }
  }
  statusDrawerVisible.value = true;
};

// 删除
const handleDelete = async (record: ServerDTO) => {
  try {
    await deleteServer(record.id);
    Message.success(t('servers.message.delete.success'));
    
    // 如果删除的是当前页最后一条，且不是第一页，回退到上一页
    if (tableData.value.length === 1 && pagination.current > 1) {
      pagination.current -= 1;
    }
    
    await loadServerStatus(true); // 刷新状态缓存
    await fetchData();
  } catch (err) {
    Message.error(t('servers.message.delete.failed'));
    console.error(err);
  }
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

// 页面初始化
onMounted(() => {
  fetchData();
});
</script>

<script lang="ts">
export default {
  name: 'ServersManage',
};
</script>

<style scoped lang="less">
.container {
  padding: 0 20px 20px;
}

.general-card {
  margin-top: 16px;
}

::deep(.arco-table-th) {
  &:last-child {
    .arco-table-th-item-title {
      margin-left: 16px;
    }
  }
}
</style>
