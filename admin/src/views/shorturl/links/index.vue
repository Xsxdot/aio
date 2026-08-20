<template>
  <div class="container">
    <a-card class="general-card" :title="$t('shorturl.links.title')">
      <!-- 域名选择区域 -->
      <a-row style="margin-bottom: 16px">
        <a-col :span="24">
          <a-form layout="inline">
            <a-form-item :label="$t('shorturl.links.domain.label')" style="margin-bottom: 0">
              <a-select
                v-model="selectedDomainId"
                :placeholder="$t('shorturl.links.domain.placeholder')"
                style="width: 300px"
                @change="handleDomainChange"
              >
                <a-option
                  v-for="domain in enabledDomains"
                  :key="domain.id"
                  :value="domain.id"
                  :label="domain.domain"
                >
                  {{ domain.domain }}
                  <a-tag v-if="domain.isDefault" color="blue" size="small" style="margin-left: 8px">
                    {{ $t('shorturl.domains.default.yes') }}
                  </a-tag>
                </a-option>
              </a-select>
            </a-form-item>
          </a-form>
        </a-col>
      </a-row>

      <!-- 搜索区域 -->
      <a-row style="margin-bottom: 16px" :gutter="16">
        <a-col :flex="'auto'">
          <a-form :model="searchForm" layout="inline">
            <a-form-item field="keyword" style="margin-bottom: 0">
              <a-input
                v-model="searchForm.keyword"
                :placeholder="$t('shorturl.links.search.placeholder')"
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
        <a-col flex="320px" style="text-align: right">
          <a-space>
            <a-button type="primary" @click="handleSearch">
              <template #icon>
                <icon-search />
              </template>
              {{ $t('shorturl.links.search') }}
            </a-button>
            <a-button @click="handleReset">
              <template #icon>
                <icon-refresh />
              </template>
              {{ $t('shorturl.links.reset') }}
            </a-button>
          </a-space>
        </a-col>
      </a-row>

      <!-- 操作按钮 -->
      <a-row v-if="selectedDomainId" style="margin-bottom: 16px">
        <a-col :span="12">
          <a-space>
            <a-button type="primary" @click="handleCreate">
              <template #icon>
                <icon-plus />
              </template>
              {{ $t('shorturl.links.create') }}
            </a-button>
            <a-button @click="fetchData">
              <template #icon>
                <icon-refresh />
              </template>
              {{ $t('shorturl.links.refresh') }}
            </a-button>
          </a-space>
        </a-col>
      </a-row>

      <!-- 提示：选择域名 -->
      <a-empty v-if="!selectedDomainId" :description="$t('shorturl.links.domain.empty')" />

      <!-- 表格 -->
      <a-table
        v-else
        row-key="id"
        :loading="loading"
        :pagination="pagination"
        :data="displayData"
        :bordered="{ cell: true }"
        :scroll="{ x: 2000 }"
        @page-change="onPageChange"
        @page-size-change="onPageSizeChange"
      >
        <template #columns>
          <a-table-column
            :title="$t('shorturl.links.column.id')"
            data-index="id"
            :width="80"
          />
          <a-table-column
            :title="$t('shorturl.links.column.shortUrl')"
            data-index="shortUrl"
            :width="250"
          >
            <template #cell="{ record }">
              <a-typography-text copyable>
                {{ record.shortUrl }}
              </a-typography-text>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('shorturl.links.column.code')"
            data-index="code"
            :width="120"
          />
          <a-table-column
            :title="$t('shorturl.links.column.targetType')"
            data-index="targetType"
            :width="150"
          >
            <template #cell="{ record }">
              <a-tag color="arcoblue">
                {{ $t(`shorturl.links.targetType.${record.targetType}`) }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('shorturl.links.column.expiresAt')"
            data-index="expiresAt"
            :width="180"
          >
            <template #cell="{ record }">
              <span v-if="record.expiresAt">
                {{ formatDateTime(record.expiresAt) }}
              </span>
              <span v-else style="color: var(--color-text-3)">
                {{ $t('shorturl.links.expiresAt.never') }}
              </span>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('shorturl.links.column.maxVisits')"
            data-index="maxVisits"
            :width="120"
          >
            <template #cell="{ record }">
              <span v-if="record.maxVisits">{{ record.maxVisits }}</span>
              <span v-else style="color: var(--color-text-3)">
                {{ $t('shorturl.links.maxVisits.unlimited') }}
              </span>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('shorturl.links.column.visitCount')"
            data-index="visitCount"
            :width="100"
          />
          <a-table-column
            :title="$t('shorturl.links.column.successCount')"
            data-index="successCount"
            :width="100"
          />
          <a-table-column
            :title="$t('shorturl.links.column.hasPassword')"
            data-index="hasPassword"
            :width="100"
          >
            <template #cell="{ record }">
              <a-tag :color="record.hasPassword ? 'orange' : 'gray'">
                {{ record.hasPassword ? $t('shorturl.links.hasPassword.yes') : $t('shorturl.links.hasPassword.no') }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('shorturl.links.column.enabled')"
            data-index="enabled"
            :width="100"
          >
            <template #cell="{ record }">
              <a-tag :color="record.enabled ? 'green' : 'red'">
                {{ record.enabled ? $t('shorturl.links.enabled.yes') : $t('shorturl.links.enabled.no') }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('shorturl.links.column.comment')"
            data-index="comment"
            :width="150"
            :ellipsis="true"
            :tooltip="true"
          >
            <template #cell="{ record }">
              <span v-if="record.comment">{{ record.comment }}</span>
              <span v-else style="color: var(--color-text-3)">-</span>
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('shorturl.links.column.createdAt')"
            data-index="createdAt"
            :width="180"
          >
            <template #cell="{ record }">
              {{ formatDateTime(record.createdAt) }}
            </template>
          </a-table-column>
          <a-table-column
            :title="$t('shorturl.links.column.actions')"
            :width="320"
            fixed="right"
          >
            <template #cell="{ record }">
              <a-space>
                <a-button type="text" size="small" @click="handleEdit(record)">
                  {{ $t('shorturl.links.action.edit') }}
                </a-button>
                <a-divider direction="vertical" />
                <a-popconfirm
                  v-if="record.enabled"
                  :content="$t('shorturl.links.confirm.disable.content')"
                  @ok="handleToggleStatus(record, false)"
                >
                  <a-button type="text" status="warning" size="small">
                    {{ $t('shorturl.links.action.disable') }}
                  </a-button>
                </a-popconfirm>
                <a-popconfirm
                  v-else
                  :content="$t('shorturl.links.confirm.enable.content')"
                  @ok="handleToggleStatus(record, true)"
                >
                  <a-button type="text" status="success" size="small">
                    {{ $t('shorturl.links.action.enable') }}
                  </a-button>
                </a-popconfirm>
                <a-divider direction="vertical" />
                <a-button type="text" size="small" @click="handleStats(record)">
                  {{ $t('shorturl.links.action.stats') }}
                </a-button>
                <a-divider direction="vertical" />
                <a-popconfirm
                  :content="$t('shorturl.links.confirm.delete.content')"
                  @ok="handleDelete(record)"
                >
                  <a-button type="text" status="danger" size="small">
                    {{ $t('shorturl.links.action.delete') }}
                  </a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-card>

    <!-- 创建/编辑抽屉 -->
    <ShortLinkFormDrawer
      v-model:visible="formDrawerVisible"
      :domain-id="selectedDomainId"
      :link="currentLink"
      @submit="handleFormSubmit"
    />

    <!-- 统计抽屉 -->
    <ShortLinkStatsDrawer
      v-model:visible="statsDrawerVisible"
      :link-id="currentStatsLinkId"
    />
  </div>
</template>

<script lang="ts" setup>
import { ref, reactive, computed, onMounted } from 'vue';
import { Message } from '@arco-design/web-vue';
import { useI18n } from 'vue-i18n';
import {
  listShortDomains,
  listShortLinks,
  createShortLink,
  updateShortLink,
  updateShortLinkStatus,
  deleteShortLink,
  type ShortDomainDTO,
  type ShortLinkDTO,
  type CreateShortLinkRequest,
  type UpdateShortLinkRequest,
} from '@/api/shorturl';
import ShortLinkFormDrawer from './components/ShortLinkFormDrawer.vue';
import ShortLinkStatsDrawer from './components/ShortLinkStatsDrawer.vue';

const { t } = useI18n();

// 域名数据
const enabledDomains = ref<ShortDomainDTO[]>([]);
const selectedDomainId = ref<number>(0);

// 搜索表单
const searchForm = reactive({
  keyword: '',
});

// 表格数据
const loading = ref(false);
const allData = ref<ShortLinkDTO[]>([]);
const filteredData = ref<ShortLinkDTO[]>([]);

// 分页
const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showTotal: true,
  showPageSize: true,
});

// 表单抽屉
const formDrawerVisible = ref(false);
const currentLink = ref<ShortLinkDTO | null>(null);

// 统计抽屉
const statsDrawerVisible = ref(false);
const currentStatsLinkId = ref<number | null>(null);

// 客户端过滤后的显示数据
const displayData = computed(() => {
  const start = (pagination.current - 1) * pagination.pageSize;
  const end = start + pagination.pageSize;
  return filteredData.value.slice(start, end);
});

// 加载域名列表
const loadDomains = async () => {
  try {
    const res = await listShortDomains();
    const domains = res.data.content || [];
    enabledDomains.value = domains.filter(d => d.enabled);
    
    // 默认选择默认域名，否则选择第一个启用域名
    if (enabledDomains.value.length > 0) {
      const defaultDomain = enabledDomains.value.find(d => d.isDefault);
      selectedDomainId.value = defaultDomain ? defaultDomain.id : enabledDomains.value[0].id;
    }
  } catch (err) {
    console.error('Failed to load domains:', err);
  }
};

// 域名变化
const handleDomainChange = () => {
  // 重置搜索和分页
  searchForm.keyword = '';
  pagination.current = 1;
  fetchData();
};

// 加载数据
const fetchData = async () => {
  if (!selectedDomainId.value) return;

  try {
    loading.value = true;
    const res = await listShortLinks({
      domainId: selectedDomainId.value,
      page: 1,
      size: 1000, // 获取所有数据，前端分页
    });
    allData.value = res.data.content || [];
    handleSearch(); // 应用搜索过滤
  } catch (err) {
    Message.error(t('shorturl.links.message.load.failed'));
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
        item.code.toLowerCase().includes(keyword) ||
        (item.comment && item.comment.toLowerCase().includes(keyword))
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
  currentLink.value = null;
  formDrawerVisible.value = true;
};

// 编辑
const handleEdit = (record: ShortLinkDTO) => {
  currentLink.value = record;
  formDrawerVisible.value = true;
};

// 表单提交
const handleFormSubmit = async (data: CreateShortLinkRequest | UpdateShortLinkRequest) => {
  try {
    if (currentLink.value) {
      // 编辑
      await updateShortLink(currentLink.value.id, data as UpdateShortLinkRequest);
      Message.success(t('shorturl.links.message.update.success'));
    } else {
      // 创建
      await createShortLink(data as CreateShortLinkRequest);
      Message.success(t('shorturl.links.message.create.success'));
    }
    await fetchData();
  } catch (err) {
    const msgKey = currentLink.value
      ? 'shorturl.links.message.update.failed'
      : 'shorturl.links.message.create.failed';
    Message.error(t(msgKey));
    console.error(err);
  }
};

// 切换状态
const handleToggleStatus = async (record: ShortLinkDTO, enabled: boolean) => {
  try {
    await updateShortLinkStatus(record.id, enabled);
    Message.success(t('shorturl.links.message.status.success'));
    await fetchData();
  } catch (err) {
    Message.error(t('shorturl.links.message.status.failed'));
    console.error(err);
  }
};

// 统计
const handleStats = (record: ShortLinkDTO) => {
  currentStatsLinkId.value = record.id;
  statsDrawerVisible.value = true;
};

// 删除
const handleDelete = async (record: ShortLinkDTO) => {
  try {
    await deleteShortLink(record.id);
    Message.success(t('shorturl.links.message.delete.success'));
    await fetchData();
  } catch (err) {
    Message.error(t('shorturl.links.message.delete.failed'));
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
onMounted(async () => {
  await loadDomains();
  if (selectedDomainId.value) {
    await fetchData();
  }
});
</script>

<script lang="ts">
export default {
  name: 'ShortUrlLinks',
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
