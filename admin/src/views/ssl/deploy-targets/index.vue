<template>
  <div class="container">
    <a-card class="general-card" :title="$t('ssl.deployTargets.title')">
      <!-- 搜索区域 -->
      <a-row style="margin-bottom: 16px">
        <a-col :flex="1">
          <a-form :model="searchForm" layout="inline" label-align="left">
            <a-form-item field="keyword" style="margin-bottom: 0">
              <a-input
                v-model="searchForm.keyword"
                :placeholder="$t('ssl.deployTargets.search.placeholder')"
                allow-clear
                style="width: 280px"
                @press-enter="handleSearch"
              >
                <template #prefix>
                  <icon-search />
                </template>
              </a-input>
            </a-form-item>
          </a-form>
        </a-col>
        <a-col :flex="'auto'" style="text-align: right">
          <a-space>
            <a-button type="primary" @click="handleSearch">
              <template #icon>
                <icon-search />
              </template>
              {{ $t('ssl.deployTargets.search') }}
            </a-button>
            <a-button @click="handleReset">
              <template #icon>
                <icon-refresh />
              </template>
              {{ $t('ssl.deployTargets.reset') }}
            </a-button>
          </a-space>
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
              {{ $t('ssl.deployTargets.create') }}
            </a-button>
            <a-button @click="fetchData">
              <template #icon>
                <icon-refresh />
              </template>
              {{ $t('ssl.deployTargets.refresh') }}
            </a-button>
          </a-space>
        </a-col>
      </a-row>

      <!-- 表格 -->
      <a-table
        row-key="id"
        :loading="loading"
        :pagination="pagination"
        :data="displayData"
        :bordered="{ cell: true }"
        @page-change="onPageChange"
        @page-size-change="onPageSizeChange"
      >
        <template #columns>
          <a-table-column :title="$t('ssl.deployTargets.column.id')" data-index="id" :width="80" />
          <a-table-column :title="$t('ssl.deployTargets.column.name')" data-index="name" :width="180" />
          <a-table-column :title="$t('ssl.deployTargets.column.domain')" data-index="domain" :width="180">
            <template #cell="{ record }">
              <a-tag size="small" color="arcoblue">{{ record.domain }}</a-tag>
            </template>
          </a-table-column>
          <a-table-column :title="$t('ssl.deployTargets.column.type')" data-index="type" :width="120">
            <template #cell="{ record }">
              <a-tag :color="getTypeColor(record.type)">
                {{ getTypeLabel(record.type) }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column :title="$t('ssl.deployTargets.column.status')" data-index="status" :width="100">
            <template #cell="{ record }">
              <a-tag :color="record.status === 1 ? 'green' : 'red'">
                {{ record.status === 1 ? $t('ssl.deployTargets.status.enabled') : $t('ssl.deployTargets.status.disabled') }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('ssl.deployTargets.column.description')"
            data-index="description"
            :width="200"
            :ellipsis="true"
            :tooltip="true"
          />
          <a-table-column :title="$t('ssl.deployTargets.column.createdAt')" data-index="created_at" :width="180">
            <template #cell="{ record }">
              {{ formatDateTime(record.created_at) }}
            </template>
          </a-table-column>
          <a-table-column :title="$t('ssl.deployTargets.column.actions')" :width="220" fixed="right">
            <template #cell="{ record }">
              <a-space>
                <a-button type="text" size="small" @click="handleEdit(record)">
                  {{ $t('ssl.deployTargets.action.edit') }}
                </a-button>
                <a-divider direction="vertical" />
                <a-popconfirm
                  v-if="record.status === 1"
                  :content="$t('ssl.deployTargets.confirm.disable.content')"
                  @ok="handleToggleStatus(record, 0)"
                >
                  <a-button type="text" status="warning" size="small">
                    {{ $t('ssl.deployTargets.action.disable') }}
                  </a-button>
                </a-popconfirm>
                <a-popconfirm
                  v-else
                  :content="$t('ssl.deployTargets.confirm.enable.content')"
                  @ok="handleToggleStatus(record, 1)"
                >
                  <a-button type="text" status="success" size="small">
                    {{ $t('ssl.deployTargets.action.enable') }}
                  </a-button>
                </a-popconfirm>
                <a-divider direction="vertical" />
                <a-popconfirm
                  :content="$t('ssl.deployTargets.confirm.delete.content')"
                  @ok="handleDelete(record)"
                >
                  <a-button type="text" status="danger" size="small">
                    {{ $t('ssl.deployTargets.action.delete') }}
                  </a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-card>

    <!-- 创建/编辑抽屉 -->
    <DeployTargetFormDrawer
      v-model:visible="formDrawerVisible"
      :edit-data="currentEditData"
      @submit="handleFormSubmit"
    />
  </div>
</template>

<script lang="ts" setup>
import { ref, reactive, computed, onMounted } from 'vue';
import { Message } from '@arco-design/web-vue';
import { useI18n } from 'vue-i18n';
import dayjs from 'dayjs';
import {
  listDeployTargets,
  createDeployTarget,
  updateDeployTarget,
  deleteDeployTarget,
  type DeployTargetDTO,
  type CreateDeployTargetRequest,
  type UpdateDeployTargetRequest,
} from '@/api/ssl';
import DeployTargetFormDrawer from './components/DeployTargetFormDrawer.vue';

const { t } = useI18n();

// 搜索表单
const searchForm = reactive({
  keyword: '',
});

// 表格数据
const loading = ref(false);
const allData = ref<DeployTargetDTO[]>([]);
const filteredData = ref<DeployTargetDTO[]>([]);

// 分页
const pagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
  showTotal: true,
  showPageSize: true,
});

// 表单抽屉
const formDrawerVisible = ref(false);
const currentEditData = ref<DeployTargetDTO | null>(null);

// 客户端过滤后的数据（用于分页）
const displayData = computed(() => {
  const start = (pagination.current - 1) * pagination.pageSize;
  const end = start + pagination.pageSize;
  return filteredData.value.slice(start, end);
});

// 应用搜索过滤
const applyFilter = () => {
  const keyword = searchForm.keyword.toLowerCase().trim();
  if (!keyword) {
    filteredData.value = allData.value;
  } else {
    filteredData.value = allData.value.filter(
      (item) =>
        item.name.toLowerCase().includes(keyword) ||
        item.type.toLowerCase().includes(keyword) ||
        item.domain.toLowerCase().includes(keyword)
    );
  }
  pagination.total = filteredData.value.length;
  pagination.current = 1;
};

// 加载数据
const fetchData = async () => {
  try {
    loading.value = true;
    const { data } = await listDeployTargets(1, 1000); // 获取所有数据
    allData.value = data.content || [];
    applyFilter();
  } catch (error) {
    Message.error(t('Failed to fetch data'));
  } finally {
    loading.value = false;
  }
};

// 搜索
const handleSearch = () => {
  applyFilter();
};

// 重置
const handleReset = () => {
  searchForm.keyword = '';
  applyFilter();
};

// 分页变化
const onPageChange = (current: number) => {
  pagination.current = current;
};

const onPageSizeChange = (pageSize: number) => {
  pagination.pageSize = pageSize;
  pagination.current = 1;
};

// 创建
const handleCreate = () => {
  currentEditData.value = null;
  formDrawerVisible.value = true;
};

// 编辑
const handleEdit = (record: DeployTargetDTO) => {
  currentEditData.value = record;
  formDrawerVisible.value = true;
};

// 表单提交
const handleFormSubmit = async (data: CreateDeployTargetRequest | UpdateDeployTargetRequest) => {
  try {
    if (currentEditData.value) {
      // 编辑
      await updateDeployTarget(currentEditData.value.id, data as UpdateDeployTargetRequest);
      Message.success(t('ssl.deployTargets.message.updateSuccess'));
    } else {
      // 创建
      await createDeployTarget(data as CreateDeployTargetRequest);
      Message.success(t('ssl.deployTargets.message.createSuccess'));
    }
    formDrawerVisible.value = false;
    fetchData();
  } catch (error) {
    // Error already handled by interceptor
  }
};

// 切换状态
const handleToggleStatus = async (record: DeployTargetDTO, status: number) => {
  try {
    await updateDeployTarget(record.id, { status });
    Message.success(t('ssl.deployTargets.message.statusUpdateSuccess'));
    fetchData();
  } catch (error) {
    // Error already handled by interceptor
  }
};

// 删除
const handleDelete = async (record: DeployTargetDTO) => {
  try {
    await deleteDeployTarget(record.id);
    Message.success(t('ssl.deployTargets.message.deleteSuccess'));
    fetchData();
  } catch (error) {
    // Error already handled by interceptor
  }
};

// 获取类型颜色
const getTypeColor = (type: string) => {
  const colors: Record<string, string> = {
    local: 'blue',
    ssh: 'green',
    aliyun_cas: 'orange',
  };
  return colors[type] || 'gray';
};

// 获取类型标签
const getTypeLabel = (type: string) => {
  const labels: Record<string, string> = {
    local: t('ssl.deployTargets.type.local'),
    ssh: t('ssl.deployTargets.type.ssh'),
    aliyun_cas: t('ssl.deployTargets.type.aliyunCas'),
  };
  return labels[type] || type;
};

// 格式化日期时间
const formatDateTime = (dateStr?: string) => {
  if (!dateStr) return '-';
  return dayjs(dateStr).format('YYYY-MM-DD HH:mm:ss');
};

onMounted(() => {
  fetchData();
});
</script>

<style scoped lang="less">
.container {
  padding: 0 20px 20px 20px;
}
</style>

