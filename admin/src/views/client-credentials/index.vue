<template>
  <div class="container">
    <a-card class="general-card" :title="$t('clientCredentials.list.title')">
      <!-- 搜索区域 -->
      <a-row style="margin-bottom: 16px">
        <a-col :flex="1">
          <a-form
            :model="searchForm"
            layout="inline"
            label-align="left"
          >
            <a-form-item field="keyword" style="margin-bottom: 0">
              <a-input
                v-model="searchForm.keyword"
                :placeholder="$t('clientCredentials.search.placeholder')"
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
              {{ $t('clientCredentials.search') }}
            </a-button>
            <a-button @click="handleReset">
              <template #icon>
                <icon-refresh />
              </template>
              {{ $t('clientCredentials.reset') }}
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
              {{ $t('clientCredentials.create') }}
            </a-button>
            <a-button @click="fetchData">
              <template #icon>
                <icon-refresh />
              </template>
              {{ $t('clientCredentials.refresh') }}
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
          <a-table-column
            :title="$t('clientCredentials.column.id')"
            data-index="id"
            :width="80"
          />
          <a-table-column
            :title="$t('clientCredentials.column.name')"
            data-index="name"
            :width="150"
          />
          <a-table-column
            :title="$t('clientCredentials.column.clientKey')"
            data-index="clientKey"
            :width="200"
          >
            <template #cell="{ record }">
              <a-typography-text copyable>
                {{ record.clientKey }}
              </a-typography-text>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('clientCredentials.column.status')"
            data-index="status"
            :width="100"
          >
            <template #cell="{ record }">
              <a-tag :color="record.status === 1 ? 'green' : 'red'">
                {{ record.status === 1 ? $t('clientCredentials.status.enabled') : $t('clientCredentials.status.disabled') }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('clientCredentials.column.description')"
            data-index="description"
            :width="200"
            :ellipsis="true"
            :tooltip="true"
          />
          <a-table-column
            :title="$t('clientCredentials.column.ipWhitelist')"
            data-index="ipWhitelist"
            :width="150"
          >
            <template #cell="{ record }">
              <a-space wrap v-if="record.ipWhitelist && record.ipWhitelist.length > 0">
                <a-tag v-for="ip in record.ipWhitelist" :key="ip" size="small">
                  {{ ip }}
                </a-tag>
              </a-space>
              <span v-else style="color: var(--color-text-3)">-</span>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('clientCredentials.column.expiresAt')"
            data-index="expiresAt"
            :width="180"
          >
            <template #cell="{ record }">
              <span v-if="record.expiresAt">
                {{ formatDateTime(record.expiresAt) }}
              </span>
              <span v-else style="color: var(--color-text-3)">
                {{ $t('clientCredentials.status.neverExpires') }}
              </span>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('clientCredentials.column.createdAt')"
            data-index="createdAt"
            :width="180"
          >
            <template #cell="{ record }">
              {{ formatDateTime(record.createdAt) }}
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('clientCredentials.column.actions')"
            :width="260"
            fixed="right"
          >
            <template #cell="{ record }">
              <a-space>
                <a-button type="text" size="small" @click="handleEdit(record)">
                  {{ $t('clientCredentials.action.edit') }}
                </a-button>
                <a-divider direction="vertical" />
                <a-popconfirm
                  :content="$t('clientCredentials.confirm.disable.content')"
                  @ok="handleDisable(record)"
                >
                  <a-button type="text" status="warning" size="small">
                    {{ $t('clientCredentials.action.disable') }}
                  </a-button>
                </a-popconfirm>
                <a-divider direction="vertical" />
                <a-popconfirm
                  :content="$t('clientCredentials.confirm.rotateSecret.content')"
                  @ok="handleRotateSecret(record)"
                >
                  <a-button type="text" size="small">
                    {{ $t('clientCredentials.action.rotateSecret') }}
                  </a-button>
                </a-popconfirm>
                <a-divider direction="vertical" />
                <a-popconfirm
                  :content="$t('clientCredentials.confirm.delete.content')"
                  @ok="handleDelete(record)"
                >
                  <a-button type="text" status="danger" size="small">
                    {{ $t('clientCredentials.action.delete') }}
                  </a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-card>

    <!-- 创建/编辑抽屉 -->
    <ClientCredentialFormDrawer
      v-model:visible="formDrawerVisible"
      :edit-data="currentEditData"
      @submit="handleFormSubmit"
    />

    <!-- 密钥展示弹窗 -->
    <SecretModal
      v-model:visible="secretModalVisible"
      :secret-data="secretData"
    />
  </div>
</template>

<script lang="ts" setup>
import { ref, reactive, computed, onMounted } from 'vue';
import { Message } from '@arco-design/web-vue';
import { useI18n } from 'vue-i18n';
import {
  listClientCredentials,
  createClientCredential,
  updateClientCredential,
  updateClientCredentialStatus,
  rotateClientCredentialSecret,
  deleteClientCredential,
  type ClientCredentialDTO,
  type CreateClientCredentialRequest,
} from '@/api/client-credentials';
import ClientCredentialFormDrawer from './components/ClientCredentialFormDrawer.vue';
import SecretModal from './components/SecretModal.vue';

const { t } = useI18n();

// 搜索表单
const searchForm = reactive({
  keyword: '',
});

// 表格数据
const loading = ref(false);
const allData = ref<ClientCredentialDTO[]>([]);
const filteredData = ref<ClientCredentialDTO[]>([]);

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
const currentEditData = ref<ClientCredentialDTO | null>(null);

// 密钥弹窗
const secretModalVisible = ref(false);
const secretData = ref({
  name: '',
  clientKey: '',
  clientSecret: '',
  description: '',
});

// 分页后的显示数据
const displayData = computed(() => {
  const start = (pagination.current - 1) * pagination.pageSize;
  const end = start + pagination.pageSize;
  return filteredData.value.slice(start, end);
});

// 加载数据
const fetchData = async () => {
  try {
    loading.value = true;
    const res = await listClientCredentials();
    allData.value = res.data.content || [];
    handleSearch(); // 应用搜索过滤
  } catch (err) {
    Message.error(t('clientCredentials.message.load.failed'));
    console.error(err);
  } finally {
    loading.value = false;
  }
};

// 搜索过滤
const handleSearch = () => {
  const keyword = searchForm.keyword.toLowerCase().trim();
  
  if (!keyword) {
    filteredData.value = [...allData.value];
  } else {
    filteredData.value = allData.value.filter((item) => {
      return (
        item.clientKey.toLowerCase().includes(keyword) ||
        item.name.toLowerCase().includes(keyword)
      );
    });
  }
  
  pagination.total = filteredData.value.length;
  pagination.current = 1; // 重置到第一页
};

// 重置搜索
const handleReset = () => {
  searchForm.keyword = '';
  handleSearch();
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
const handleEdit = (record: ClientCredentialDTO) => {
  currentEditData.value = record;
  formDrawerVisible.value = true;
};

// 表单提交
const handleFormSubmit = async (formData: any) => {
  try {
    if (currentEditData.value) {
      // 编辑模式
      const updateData = {
        id: currentEditData.value.id,
        name: formData.name,
        description: formData.description || undefined,
        ipWhitelist: formData.ipWhitelist || [],
        expiresAt: formData.expiresAt || null,
      };
      await updateClientCredential(currentEditData.value.id, updateData);
      Message.success(t('clientCredentials.message.update.success'));
    } else {
      // 创建模式
      const createData: CreateClientCredentialRequest = {
        name: formData.name,
        description: formData.description || undefined,
        ipWhitelist: formData.ipWhitelist || [],
        expiresAt: formData.expiresAt || null,
      };
      const res = await createClientCredential(createData);
      
      // 显示密钥信息
      secretData.value = {
        name: res.data.name,
        clientKey: res.data.clientKey,
        clientSecret: res.data.clientSecret,
        description: res.data.description || '',
      };
      secretModalVisible.value = true;
      
      Message.success(t('clientCredentials.message.create.success'));
    }
    
    await fetchData();
  } catch (err) {
    const errorMsg = currentEditData.value
      ? t('clientCredentials.message.update.failed')
      : t('clientCredentials.message.create.failed');
    Message.error(errorMsg);
    console.error(err);
    throw err; // 让表单保持打开状态
  }
};

// 禁用
const handleDisable = async (record: ClientCredentialDTO) => {
  try {
    await updateClientCredentialStatus(record.id, 0);
    Message.success(t('clientCredentials.message.disable.success'));
    await fetchData();
  } catch (err) {
    Message.error(t('clientCredentials.message.disable.failed'));
    console.error(err);
  }
};

// 轮换密钥
const handleRotateSecret = async (record: ClientCredentialDTO) => {
  try {
    const res = await rotateClientCredentialSecret(record.id);
    
    // 显示新密钥
    secretData.value = {
      name: record.name,
      clientKey: record.clientKey,
      clientSecret: res.data.clientSecret,
      description: record.description || '',
    };
    secretModalVisible.value = true;
    
    Message.success(t('clientCredentials.message.rotateSecret.success'));
  } catch (err) {
    Message.error(t('clientCredentials.message.rotateSecret.failed'));
    console.error(err);
  }
};

// 删除
const handleDelete = async (record: ClientCredentialDTO) => {
  try {
    await deleteClientCredential(record.id);
    Message.success(t('clientCredentials.message.delete.success'));
    await fetchData();
  } catch (err) {
    Message.error(t('clientCredentials.message.delete.failed'));
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

// 页面初始化
onMounted(() => {
  fetchData();
});
</script>

<script lang="ts">
export default {
  name: 'ClientCredentials',
};
</script>

<style scoped lang="less">
.container {
  padding: 0 20px 20px;
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






