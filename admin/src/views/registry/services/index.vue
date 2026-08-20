<template>
  <div class="container">
    <a-card class="general-card" :title="$t('registry.services.title')">
      <!-- 搜索区域 -->
      <a-row style="margin-bottom: 16px">
        <a-col :flex="1">
          <a-space>
            <a-input
              v-model="searchForm.project"
              :placeholder="$t('registry.services.search.placeholder.project')"
              allow-clear
              style="width: 200px"
              @press-enter="handleSearch"
            >
              <template #prefix>
                <icon-search />
              </template>
            </a-input>
            <a-input
              v-model="searchForm.keyword"
              :placeholder="$t('registry.services.search.placeholder.keyword')"
              allow-clear
              style="width: 280px"
              @press-enter="handleSearch"
            >
              <template #prefix>
                <icon-search />
              </template>
            </a-input>
          </a-space>
        </a-col>
        <a-col :flex="'86px'" style="text-align: right">
          <a-space>
            <a-button type="primary" @click="handleSearch">
              <template #icon>
                <icon-search />
              </template>
              {{ $t('registry.services.search') }}
            </a-button>
            <a-button @click="handleReset">
              <template #icon>
                <icon-refresh />
              </template>
              {{ $t('registry.services.reset') }}
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
              {{ $t('registry.services.create') }}
            </a-button>
            <a-button @click="fetchData">
              <template #icon>
                <icon-refresh />
              </template>
              {{ $t('registry.services.refresh') }}
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
            :title="$t('registry.services.column.id')"
            data-index="id"
            :width="80"
          />
          <a-table-column
            :title="$t('registry.services.column.project')"
            data-index="project"
            :width="150"
          />
          <a-table-column
            :title="$t('registry.services.column.name')"
            data-index="name"
            :width="180"
          />
          <a-table-column
            :title="$t('registry.services.column.owner')"
            data-index="owner"
            :width="150"
          />
          <a-table-column
            :title="$t('registry.services.column.description')"
            data-index="description"
            :width="200"
            :ellipsis="true"
            :tooltip="true"
          />
          <a-table-column
            :title="$t('registry.services.column.updatedAt')"
            data-index="updatedAt"
            :width="180"
          >
            <template #cell="{ record }">
              {{ formatDateTime(record.updatedAt) }}
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('registry.services.column.actions')"
            :width="300"
            fixed="right"
          >
            <template #cell="{ record }">
              <a-space>
                <a-button type="text" size="small" @click="handleViewInstances(record)">
                  {{ $t('registry.services.action.viewInstances') }}
                </a-button>
                <a-divider direction="vertical" />
                <a-button type="text" size="small" @click="handleViewSpec(record)">
                  {{ $t('registry.services.action.viewSpec') }}
                </a-button>
                <a-divider direction="vertical" />
                <a-button type="text" size="small" @click="handleEdit(record)">
                  {{ $t('registry.services.action.edit') }}
                </a-button>
                <a-divider direction="vertical" />
                <a-popconfirm
                  :content="$t('registry.services.confirm.delete.content')"
                  @ok="handleDelete(record)"
                >
                  <a-button type="text" status="danger" size="small">
                    {{ $t('registry.services.action.delete') }}
                  </a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-card>

    <!-- 创建/编辑抽屉 -->
    <ServiceFormDrawer
      v-model:visible="formDrawerVisible"
      :edit-data="currentEditData"
      @submit="handleFormSubmit"
    />

    <!-- 实例查看抽屉 -->
    <ServiceInstancesDrawer
      v-model:visible="instancesDrawerVisible"
      :service-info="currentViewService"
      @refresh="fetchData"
    />

    <!-- Spec 查看弹窗 -->
    <JsonEditorModal
      v-model:visible="specViewModalVisible"
      :title="$t('registry.services.form.spec.view')"
      :value="specViewJson"
      mode="object"
      :width="800"
    />
  </div>
</template>

<script lang="ts" setup>
import { ref, reactive, computed, onMounted } from 'vue';
import { Message } from '@arco-design/web-vue';
import { useI18n } from 'vue-i18n';
import {
  listServices,
  createService,
  updateService,
  deleteService,
  type ServiceDTO,
  type CreateServiceRequest,
} from '@/api/registry-services';
import JsonEditorModal from '@/components/json-editor/JsonEditorModal.vue';
import ServiceFormDrawer from './components/ServiceFormDrawer.vue';
import ServiceInstancesDrawer from './components/ServiceInstancesDrawer.vue';

const { t } = useI18n();

// 搜索表单
const searchForm = reactive({
  project: '',
  keyword: '',
});

// 表格数据
const loading = ref(false);
const allData = ref<ServiceDTO[]>([]);
const filteredData = ref<ServiceDTO[]>([]);

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
const currentEditData = ref<ServiceDTO | null>(null);

// 实例抽屉
const instancesDrawerVisible = ref(false);
const currentViewService = ref<ServiceDTO | null>(null);

// Spec 查看弹窗
const specViewModalVisible = ref(false);
const specViewJson = ref('{}');

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
    const params: { project?: string } = {};
    
    // 只有非空时才传递给后端
    if (searchForm.project.trim()) {
      params.project = searchForm.project.trim();
    }

    const res = await listServices(params);
    allData.value = res.data.content || [];
    handleSearch(); // 应用前端搜索过滤
  } catch (err) {
    Message.error(t('registry.services.message.load.failed'));
    console.error(err);
  } finally {
    loading.value = false;
  }
};

// 搜索过滤（前端过滤 keyword）
const handleSearch = () => {
  const keyword = searchForm.keyword.toLowerCase().trim();

  if (!keyword) {
    filteredData.value = [...allData.value];
  } else {
    filteredData.value = allData.value.filter((item) => {
      return (
        item.name.toLowerCase().includes(keyword) ||
        (item.owner && item.owner.toLowerCase().includes(keyword)) ||
        (item.description && item.description.toLowerCase().includes(keyword))
      );
    });
  }

  pagination.total = filteredData.value.length;
  pagination.current = 1; // 重置到第一页
};

// 重置搜索
const handleReset = () => {
  searchForm.project = '';
  searchForm.keyword = '';
  fetchData();
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
const handleEdit = (record: ServiceDTO) => {
  currentEditData.value = record;
  formDrawerVisible.value = true;
};

// 查看实例
const handleViewInstances = (record: ServiceDTO) => {
  currentViewService.value = record;
  instancesDrawerVisible.value = true;
};

// 查看 Spec
const handleViewSpec = (record: ServiceDTO) => {
  try {
    specViewJson.value = JSON.stringify(record.spec || {}, null, 2);
    specViewModalVisible.value = true;
  } catch (e) {
    Message.error('Failed to parse spec');
    console.error(e);
  }
};

// 表单提交
const handleFormSubmit = async (formData: any) => {
  try {
    if (currentEditData.value) {
      // 编辑模式
      const updateData = {
        project: formData.project,
        name: formData.name,
        owner: formData.owner || undefined,
        description: formData.description || undefined,
        spec: formData.spec || {},
      };
      await updateService(currentEditData.value.id, updateData);
      Message.success(t('registry.services.message.update.success'));
    } else {
      // 创建模式
      const createData: CreateServiceRequest = {
        project: formData.project,
        name: formData.name,
        owner: formData.owner || undefined,
        description: formData.description || undefined,
        spec: formData.spec || {},
      };
      await createService(createData);
      Message.success(t('registry.services.message.create.success'));
    }

    await fetchData();
  } catch (err) {
    const errorMsg = currentEditData.value
      ? t('registry.services.message.update.failed')
      : t('registry.services.message.create.failed');
    Message.error(errorMsg);
    console.error(err);
    throw err; // 让表单保持打开状态
  }
};

// 删除
const handleDelete = async (record: ServiceDTO) => {
  try {
    await deleteService(record.id);
    Message.success(t('registry.services.message.delete.success'));
    await fetchData();
  } catch (err) {
    Message.error(t('registry.services.message.delete.failed'));
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
  name: 'RegistryServices',
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


