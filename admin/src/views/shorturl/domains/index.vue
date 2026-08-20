<template>
  <div class="container">
    <a-card class="general-card" :title="$t('shorturl.domains.title')">
      <!-- 搜索区域 -->
      <a-row style="margin-bottom: 16px">
        <a-col :flex="1">
          <a-form :model="searchForm" layout="inline">
            <a-form-item field="keyword" style="margin-bottom: 0">
              <a-input
                v-model="searchForm.keyword"
                :placeholder="$t('shorturl.domains.search.placeholder')"
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
              {{ $t('shorturl.domains.search') }}
            </a-button>
            <a-button @click="handleReset">
              <template #icon>
                <icon-refresh />
              </template>
              {{ $t('shorturl.domains.reset') }}
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
              {{ $t('shorturl.domains.create') }}
            </a-button>
            <a-button @click="fetchData">
              <template #icon>
                <icon-refresh />
              </template>
              {{ $t('shorturl.domains.refresh') }}
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
            :title="$t('shorturl.domains.column.id')"
            data-index="id"
            :width="80"
          />
          <a-table-column
            :title="$t('shorturl.domains.column.domain')"
            data-index="domain"
            :width="250"
          />
          <a-table-column
            :title="$t('shorturl.domains.column.enabled')"
            data-index="enabled"
            :width="100"
          >
            <template #cell="{ record }">
              <a-tag :color="record.enabled ? 'green' : 'red'">
                {{ record.enabled ? $t('shorturl.domains.enabled.yes') : $t('shorturl.domains.enabled.no') }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('shorturl.domains.column.isDefault')"
            data-index="isDefault"
            :width="120"
          >
            <template #cell="{ record }">
              <a-tag :color="record.isDefault ? 'blue' : 'gray'">
                {{ record.isDefault ? $t('shorturl.domains.default.yes') : $t('shorturl.domains.default.no') }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('shorturl.domains.column.comment')"
            data-index="comment"
            :width="200"
            :ellipsis="true"
            :tooltip="true"
          >
            <template #cell="{ record }">
              <span v-if="record.comment">{{ record.comment }}</span>
              <span v-else style="color: var(--color-text-3)">-</span>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('shorturl.domains.column.createdAt')"
            data-index="createdAt"
            :width="180"
          >
            <template #cell="{ record }">
              {{ formatDateTime(record.createdAt) }}
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('shorturl.domains.column.actions')"
            :width="360"
            fixed="right"
          >
            <template #cell="{ record }">
              <a-space>
                <a-button type="text" size="small" @click="handleEdit(record)">
                  {{ $t('shorturl.domains.action.edit') }}
                </a-button>
                <a-divider direction="vertical" />
                <a-popconfirm
                  v-if="record.enabled"
                  :content="$t('shorturl.domains.confirm.disable.content')"
                  @ok="handleToggleStatus(record, false)"
                >
                  <a-button type="text" status="warning" size="small">
                    {{ $t('shorturl.domains.action.disable') }}
                  </a-button>
                </a-popconfirm>
                <a-popconfirm
                  v-else
                  :content="$t('shorturl.domains.confirm.enable.content')"
                  @ok="handleToggleStatus(record, true)"
                >
                  <a-button type="text" status="success" size="small">
                    {{ $t('shorturl.domains.action.enable') }}
                  </a-button>
                </a-popconfirm>
                <a-divider direction="vertical" />
                <a-popconfirm
                  v-if="!record.isDefault"
                  :content="$t('shorturl.domains.confirm.setDefault.content')"
                  @ok="handleSetDefault(record)"
                >
                  <a-button type="text" size="small">
                    {{ $t('shorturl.domains.action.setDefault') }}
                  </a-button>
                </a-popconfirm>
                <a-divider direction="vertical" />
                <a-popconfirm
                  :content="$t('shorturl.domains.confirm.delete.content')"
                  @ok="handleDelete(record)"
                >
                  <a-button type="text" status="danger" size="small">
                    {{ $t('shorturl.domains.action.delete') }}
                  </a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-card>

    <!-- 创建/编辑抽屉 -->
    <DomainFormDrawer
      v-model:visible="formDrawerVisible"
      :domain="currentDomain"
      @submit="handleFormSubmit"
    />
  </div>
</template>

<script lang="ts" setup>
import { ref, reactive, computed, onMounted } from 'vue';
import { Message } from '@arco-design/web-vue';
import { useI18n } from 'vue-i18n';
import {
  listShortDomains,
  createShortDomain,
  updateShortDomain,
  updateShortDomainStatus,
  deleteShortDomain,
  type ShortDomainDTO,
  type CreateShortDomainRequest,
  type UpdateShortDomainRequest,
} from '@/api/shorturl';
import DomainFormDrawer from './components/DomainFormDrawer.vue';

const { t } = useI18n();

// 搜索表单
const searchForm = reactive({
  keyword: '',
});

// 表格数据
const loading = ref(false);
const allData = ref<ShortDomainDTO[]>([]);
const filteredData = ref<ShortDomainDTO[]>([]);

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
const currentDomain = ref<ShortDomainDTO | null>(null);

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
    const res = await listShortDomains();
    allData.value = res.data.content || [];
    handleSearch(); // 应用搜索过滤
  } catch (err) {
    Message.error(t('shorturl.domains.message.load.failed'));
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
      return item.domain.toLowerCase().includes(keyword);
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
  currentDomain.value = null;
  formDrawerVisible.value = true;
};

// 编辑
const handleEdit = (record: ShortDomainDTO) => {
  currentDomain.value = record;
  formDrawerVisible.value = true;
};

// 表单提交
const handleFormSubmit = async (data: CreateShortDomainRequest | UpdateShortDomainRequest) => {
  try {
    if (currentDomain.value) {
      // 编辑
      await updateShortDomain(currentDomain.value.id, data as UpdateShortDomainRequest);
      Message.success(t('shorturl.domains.message.update.success'));
    } else {
      // 创建
      await createShortDomain(data as CreateShortDomainRequest);
      Message.success(t('shorturl.domains.message.create.success'));
    }
    await fetchData();
  } catch (err) {
    const msgKey = currentDomain.value
      ? 'shorturl.domains.message.update.failed'
      : 'shorturl.domains.message.create.failed';
    Message.error(t(msgKey));
    console.error(err);
  }
};

// 切换状态
const handleToggleStatus = async (record: ShortDomainDTO, enabled: boolean) => {
  try {
    await updateShortDomainStatus(record.id, enabled);
    Message.success(t('shorturl.domains.message.status.success'));
    await fetchData();
  } catch (err) {
    Message.error(t('shorturl.domains.message.status.failed'));
    console.error(err);
  }
};

// 设为默认
const handleSetDefault = async (record: ShortDomainDTO) => {
  try {
    // 先将目标域名设为默认
    await updateShortDomain(record.id, { isDefault: true });
    
    // 将其他域名的 isDefault 设为 false
    const otherDomains = allData.value.filter(d => d.id !== record.id && d.isDefault);
    await Promise.all(
      otherDomains.map(d => updateShortDomain(d.id, { isDefault: false }))
    );
    
    Message.success(t('shorturl.domains.message.setDefault.success'));
    await fetchData();
  } catch (err) {
    Message.error(t('shorturl.domains.message.setDefault.failed'));
    console.error(err);
  }
};

// 删除
const handleDelete = async (record: ShortDomainDTO) => {
  try {
    await deleteShortDomain(record.id);
    Message.success(t('shorturl.domains.message.delete.success'));
    await fetchData();
  } catch (err) {
    Message.error(t('shorturl.domains.message.delete.failed'));
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
  name: 'ShortUrlDomains',
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
