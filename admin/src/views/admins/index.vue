<template>
  <div class="container">
    <a-card class="general-card" :title="$t('admins.list.title')">
      <!-- 搜索区域 -->
      <a-row :gutter="16" style="margin-bottom: 16px">
        <a-col :flex="1">
          <a-space wrap>
            <a-input
              v-model="searchForm.keyword"
              :placeholder="$t('admins.search.placeholder')"
              allow-clear
              style="width: 280px"
              @press-enter="handleSearch"
            >
              <template #prefix>
                <icon-search />
              </template>
            </a-input>
            <a-select
              v-model="searchForm.status"
              :placeholder="$t('admins.search.statusPlaceholder')"
              allow-clear
              style="width: 150px"
              @change="handleSearch"
            >
              <a-option :value="1">{{ $t('admins.status.enabled') }}</a-option>
              <a-option :value="0">{{ $t('admins.status.disabled') }}</a-option>
            </a-select>
          </a-space>
        </a-col>
        <a-col :flex="'86px'" style="text-align: right">
          <a-space>
            <a-button @click="handleReset">
              <template #icon>
                <icon-refresh />
              </template>
              {{ $t('admins.reset') }}
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
              {{ $t('admins.create.button') }}
            </a-button>
            <a-button @click="fetchData">
              <template #icon>
                <icon-refresh />
              </template>
              {{ $t('admins.refresh') }}
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
        @page-change="onPageChange"
        @page-size-change="onPageSizeChange"
      >
        <template #columns>
          <a-table-column
            :title="$t('admins.column.id')"
            data-index="id"
            :width="80"
          />
          <a-table-column
            :title="$t('admins.column.account')"
            data-index="account"
            :width="150"
          />
          <a-table-column
            :title="$t('admins.column.isSuper')"
            data-index="isSuper"
            :width="100"
          >
            <template #cell="{ record }">
              <a-tag :color="record.isSuper ? 'red' : 'blue'">
                {{ record.isSuper ? $t('admins.isSuper.yes') : $t('admins.isSuper.no') }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('admins.column.roles')"
            data-index="roles"
            :width="200"
          >
            <template #cell="{ record }">
              <a-space wrap v-if="record.roles && record.roles.length > 0">
                <a-tag v-for="role in record.roles" :key="role" size="small" color="arcoblue">
                  {{ role }}
                </a-tag>
              </a-space>
              <span v-else style="color: var(--color-text-3)">-</span>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('admins.column.status')"
            data-index="status"
            :width="100"
          >
            <template #cell="{ record }">
              <a-tag :color="record.status === 1 ? 'green' : 'red'">
                {{ record.status === 1 ? $t('admins.status.enabled') : $t('admins.status.disabled') }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('admins.column.remark')"
            data-index="remark"
            :width="200"
            :ellipsis="true"
            :tooltip="true"
          />
          <a-table-column
            :title="$t('admins.column.createdAt')"
            data-index="createdAt"
            :width="180"
          >
            <template #cell="{ record }">
              {{ formatDateTime(record.createdAt) }}
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('admins.column.actions')"
            :width="240"
            fixed="right"
          >
            <template #cell="{ record }">
              <a-space>
                <a-button type="text" size="small" @click="handleResetPassword(record)">
                  {{ $t('admins.action.resetPassword') }}
                </a-button>
                <a-divider direction="vertical" />
                <a-popconfirm
                  v-if="record.status === 1"
                  :content="$t('admins.confirm.disable.content')"
                  @ok="handleToggleStatus(record, 0)"
                >
                  <a-button type="text" status="warning" size="small">
                    {{ $t('admins.action.disable') }}
                  </a-button>
                </a-popconfirm>
                <a-popconfirm
                  v-else
                  :content="$t('admins.confirm.enable.content')"
                  @ok="handleToggleStatus(record, 1)"
                >
                  <a-button type="text" status="success" size="small">
                    {{ $t('admins.action.enable') }}
                  </a-button>
                </a-popconfirm>
                <a-divider direction="vertical" />
                <a-popconfirm
                  :content="$t('admins.confirm.delete.content')"
                  @ok="handleDelete(record)"
                >
                  <a-button type="text" status="danger" size="small">
                    {{ $t('admins.action.delete') }}
                  </a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-card>

    <!-- 创建管理员抽屉 -->
    <AdminCreateDrawer
      v-model:visible="createDrawerVisible"
      @submit="handleCreateSubmit"
    />

    <!-- 重置密码抽屉 -->
    <AdminResetPasswordDrawer
      v-model:visible="resetPasswordDrawerVisible"
      :admin-id="currentAdminId"
      :admin-account="currentAdminAccount"
      @submit="handleResetPasswordSubmit"
    />
  </div>
</template>

<script lang="ts" setup>
import { ref, reactive, onMounted } from 'vue';
import { Message } from '@arco-design/web-vue';
import { useI18n } from 'vue-i18n';
import {
  listAdmins,
  createAdmin,
  updateAdminStatus,
  resetAdminPassword,
  deleteAdmin,
  type AdminDTO,
} from '@/api/admins';
import AdminCreateDrawer from './components/AdminCreateDrawer.vue';
import AdminResetPasswordDrawer from './components/AdminResetPasswordDrawer.vue';

const { t } = useI18n();

// 搜索表单
const searchForm = reactive({
  keyword: '',
  status: undefined as number | undefined,
});

// 表格数据
const loading = ref(false);
const tableData = ref<AdminDTO[]>([]);

// 分页
const pagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
  showTotal: true,
  showPageSize: true,
});

// 创建抽屉
const createDrawerVisible = ref(false);

// 重置密码抽屉
const resetPasswordDrawerVisible = ref(false);
const currentAdminId = ref<number>();
const currentAdminAccount = ref<string>('');

// 加载数据
const fetchData = async () => {
  try {
    loading.value = true;
    const params = {
      page: pagination.current,
      pageSize: pagination.pageSize,
      keyword: searchForm.keyword || undefined,
      status: searchForm.status,
    };
    const res = await listAdmins(params);
    tableData.value = res.data.content || [];
    pagination.total = res.data.total || 0;
  } catch (err) {
    Message.error(t('admins.message.load.failed'));
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
  searchForm.keyword = '';
  searchForm.status = undefined;
  handleSearch();
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

// 创建
const handleCreate = () => {
  createDrawerVisible.value = true;
};

// 创建提交
const handleCreateSubmit = async (formData: any) => {
  try {
    await createAdmin(formData);
    Message.success(t('admins.message.create.success'));
    createDrawerVisible.value = false;
    await fetchData();
  } catch (err) {
    Message.error(t('admins.message.create.failed'));
    console.error(err);
  }
};

// 重置密码
const handleResetPassword = (record: AdminDTO) => {
  currentAdminId.value = record.id;
  currentAdminAccount.value = record.account;
  resetPasswordDrawerVisible.value = true;
};

// 重置密码提交
const handleResetPasswordSubmit = async (adminId: number, newPassword: string) => {
  try {
    await resetAdminPassword(adminId, newPassword);
    Message.success(t('admins.message.resetPassword.success'));
    resetPasswordDrawerVisible.value = false;
  } catch (err) {
    Message.error(t('admins.message.resetPassword.failed'));
    console.error(err);
  }
};

// 切换状态（启用/禁用）
const handleToggleStatus = async (record: AdminDTO, newStatus: number) => {
  try {
    await updateAdminStatus(record.id, newStatus);
    const msgKey = newStatus === 1 ? 'admins.message.enable.success' : 'admins.message.disable.success';
    Message.success(t(msgKey));
    await fetchData();
  } catch (err) {
    const msgKey = newStatus === 1 ? 'admins.message.enable.failed' : 'admins.message.disable.failed';
    Message.error(t(msgKey));
    console.error(err);
  }
};

// 删除
const handleDelete = async (record: AdminDTO) => {
  try {
    await deleteAdmin(record.id);
    Message.success(t('admins.message.delete.success'));
    await fetchData();
  } catch (err) {
    Message.error(t('admins.message.delete.failed'));
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
  name: 'Admins',
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


