<template>
  <div class="container">
    <a-card class="general-card" :title="$t('ssl.dnsCredentials.title')">
      <!-- 搜索区域 -->
      <a-row style="margin-bottom: 16px">
        <a-col :flex="1">
          <a-form :model="searchForm" layout="inline" label-align="left">
            <a-form-item field="keyword" style="margin-bottom: 0">
              <a-input
                v-model="searchForm.keyword"
                :placeholder="$t('ssl.dnsCredentials.search.placeholder')"
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
              {{ $t('ssl.dnsCredentials.search') }}
            </a-button>
            <a-button @click="handleReset">
              <template #icon>
                <icon-refresh />
              </template>
              {{ $t('ssl.dnsCredentials.reset') }}
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
              {{ $t('ssl.dnsCredentials.create') }}
            </a-button>
            <a-button @click="fetchData">
              <template #icon>
                <icon-refresh />
              </template>
              {{ $t('ssl.dnsCredentials.refresh') }}
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
          <a-table-column :title="$t('ssl.dnsCredentials.column.id')" data-index="id" :width="80" />
          <a-table-column :title="$t('ssl.dnsCredentials.column.name')" data-index="name" :width="200" />
          <a-table-column :title="$t('ssl.dnsCredentials.column.provider')" data-index="provider" :width="150">
            <template #cell="{ record }">
              <a-tag color="arcoblue">{{ record.provider }}</a-tag>
            </template>
          </a-table-column>
          <a-table-column :title="$t('ssl.dnsCredentials.column.status')" data-index="status" :width="100">
            <template #cell="{ record }">
              <a-tag :color="record.status === 1 ? 'green' : 'red'">
                {{ record.status === 1 ? $t('ssl.dnsCredentials.status.enabled') : $t('ssl.dnsCredentials.status.disabled') }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('ssl.dnsCredentials.column.description')"
            data-index="description"
            :width="200"
            :ellipsis="true"
            :tooltip="true"
          />
          <a-table-column :title="$t('ssl.dnsCredentials.column.createdAt')" data-index="created_at" :width="180">
            <template #cell="{ record }">
              {{ formatDateTime(record.created_at) }}
            </template>
          </a-table-column>
          <a-table-column :title="$t('ssl.dnsCredentials.column.actions')" :width="220" fixed="right">
            <template #cell="{ record }">
              <a-space>
                <a-button type="text" size="small" @click="handleEdit(record)">
                  {{ $t('ssl.dnsCredentials.action.edit') }}
                </a-button>
                <a-divider direction="vertical" />
                <a-popconfirm
                  v-if="record.status === 1"
                  :content="$t('ssl.dnsCredentials.confirm.disable.content')"
                  @ok="handleToggleStatus(record, 0)"
                >
                  <a-button type="text" status="warning" size="small">
                    {{ $t('ssl.dnsCredentials.action.disable') }}
                  </a-button>
                </a-popconfirm>
                <a-popconfirm
                  v-else
                  :content="$t('ssl.dnsCredentials.confirm.enable.content')"
                  @ok="handleToggleStatus(record, 1)"
                >
                  <a-button type="text" status="success" size="small">
                    {{ $t('ssl.dnsCredentials.action.enable') }}
                  </a-button>
                </a-popconfirm>
                <a-divider direction="vertical" />
                <a-popconfirm
                  :content="$t('ssl.dnsCredentials.confirm.delete.content')"
                  @ok="handleDelete(record)"
                >
                  <a-button type="text" status="danger" size="small">
                    {{ $t('ssl.dnsCredentials.action.delete') }}
                  </a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-card>

    <!-- 创建/编辑抽屉 -->
    <DnsCredentialFormDrawer
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
  listDnsCredentials,
  createDnsCredential,
  updateDnsCredential,
  deleteDnsCredential,
  type DnsCredentialDTO,
  type CreateDnsCredentialRequest,
  type UpdateDnsCredentialRequest,
} from '@/api/ssl';
import DnsCredentialFormDrawer from './components/DnsCredentialFormDrawer.vue';

const { t } = useI18n();

// 搜索表单
const searchForm = reactive({
  keyword: '',
});

// 表格数据
const loading = ref(false);
const allData = ref<DnsCredentialDTO[]>([]);
const filteredData = ref<DnsCredentialDTO[]>([]);

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
const currentEditData = ref<DnsCredentialDTO | null>(null);

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
        item.provider.toLowerCase().includes(keyword)
    );
  }
  pagination.total = filteredData.value.length;
  pagination.current = 1;
};

// 加载数据
const fetchData = async () => {
  try {
    loading.value = true;
    const { data } = await listDnsCredentials(1, 1000); // 获取所有数据
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
const handleEdit = (record: DnsCredentialDTO) => {
  currentEditData.value = record;
  formDrawerVisible.value = true;
};

// 表单提交
const handleFormSubmit = async (data: CreateDnsCredentialRequest | UpdateDnsCredentialRequest) => {
  try {
    if (currentEditData.value) {
      // 编辑
      await updateDnsCredential(currentEditData.value.id, data as UpdateDnsCredentialRequest);
      Message.success(t('ssl.dnsCredentials.message.updateSuccess'));
    } else {
      // 创建
      await createDnsCredential(data as CreateDnsCredentialRequest);
      Message.success(t('ssl.dnsCredentials.message.createSuccess'));
    }
    formDrawerVisible.value = false;
    fetchData();
  } catch (error) {
    // Error already handled by interceptor
  }
};

// 切换状态
const handleToggleStatus = async (record: DnsCredentialDTO, status: number) => {
  try {
    await updateDnsCredential(record.id, { status });
    Message.success(t('ssl.dnsCredentials.message.statusUpdateSuccess'));
    fetchData();
  } catch (error) {
    // Error already handled by interceptor
  }
};

// 删除
const handleDelete = async (record: DnsCredentialDTO) => {
  try {
    await deleteDnsCredential(record.id);
    Message.success(t('ssl.dnsCredentials.message.deleteSuccess'));
    fetchData();
  } catch (error) {
    // Error already handled by interceptor
  }
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

