<template>
  <div class="container">
    <a-card class="general-card" :title="$t('ssl.certificates.title')">
      <!-- 搜索区域 -->
      <a-row style="margin-bottom: 16px">
        <a-col :flex="1">
          <a-form :model="searchForm" layout="inline" label-align="left">
            <a-form-item field="keyword" style="margin-bottom: 0">
              <a-input
                v-model="searchForm.keyword"
                :placeholder="$t('ssl.certificates.search.placeholder')"
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
              {{ $t('ssl.certificates.search') }}
            </a-button>
            <a-button @click="handleReset">
              <template #icon>
                <icon-refresh />
              </template>
              {{ $t('ssl.certificates.reset') }}
            </a-button>
          </a-space>
        </a-col>
      </a-row>

      <!-- 操作按钮 -->
      <a-row style="margin-bottom: 16px">
        <a-col :span="12">
          <a-space>
            <a-button type="primary" @click="handleIssue">
              <template #icon>
                <icon-plus />
              </template>
              {{ $t('ssl.certificates.issue') }}
            </a-button>
            <a-button @click="fetchData">
              <template #icon>
                <icon-refresh />
              </template>
              {{ $t('ssl.certificates.refresh') }}
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
          <a-table-column :title="$t('ssl.certificates.column.id')" data-index="id" :width="80" />
          <a-table-column :title="$t('ssl.certificates.column.name')" data-index="name" :width="150" />
          <a-table-column :title="$t('ssl.certificates.column.domain')" data-index="domain" :width="200">
            <template #cell="{ record }">
              <a-tag size="small">{{ record.domain }}</a-tag>
            </template>
          </a-table-column>
          <a-table-column :title="$t('ssl.certificates.column.status')" data-index="status" :width="100">
            <template #cell="{ record }">
              <a-tag :color="getStatusColor(record.status)">
                {{ $t(`ssl.certificates.status.${record.status}`) }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column :title="$t('ssl.certificates.column.expiresAt')" data-index="expires_at" :width="180">
            <template #cell="{ record }">
              <span v-if="record.expires_at" :style="{ color: getExpiryColor(record.expires_at) }">
                {{ formatDateTime(record.expires_at) }}
              </span>
              <span v-else>-</span>
            </template>
          </a-table-column>
          <a-table-column :title="$t('ssl.certificates.column.autoRenew')" data-index="auto_renew" :width="100">
            <template #cell="{ record }">
              <a-tag :color="record.auto_renew === 1 ? 'green' : 'gray'">
                {{ record.auto_renew === 1 ? '是' : '否' }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column :title="$t('ssl.certificates.column.createdAt')" data-index="created_at" :width="180">
            <template #cell="{ record }">
              {{ formatDateTime(record.created_at) }}
            </template>
          </a-table-column>
          <a-table-column :title="$t('ssl.certificates.column.actions')" :width="300" fixed="right">
            <template #cell="{ record }">
              <a-space>
                <a-popconfirm
                  :content="$t('ssl.certificates.confirm.renew.content')"
                  @ok="handleRenew(record)"
                >
                  <a-button type="text" size="small">
                    {{ $t('ssl.certificates.action.renew') }}
                  </a-button>
                </a-popconfirm>
                <a-divider direction="vertical" />
                <a-button type="text" size="small" @click="handleDeploy(record)">
                  {{ $t('ssl.certificates.action.deploy') }}
                </a-button>
                <a-divider direction="vertical" />
                <a-button type="text" size="small" @click="handleViewHistory(record)">
                  {{ $t('ssl.certificates.action.history') }}
                </a-button>
                <a-divider direction="vertical" />
                <a-popconfirm
                  :content="$t('ssl.certificates.confirm.delete.content')"
                  @ok="handleDelete(record)"
                >
                  <a-button type="text" status="danger" size="small">
                    {{ $t('ssl.certificates.action.delete') }}
                  </a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-card>

    <!-- 申请证书抽屉 -->
    <IssueCertificateDrawer
      v-model:visible="issueDrawerVisible"
      @submit="handleIssueSubmit"
    />

    <!-- 部署证书抽屉 -->
    <DeployCertificateDrawer
      v-model:visible="deployDrawerVisible"
      :certificate="currentCertificate"
      @submit="handleDeploySubmit"
    />

    <!-- 部署历史抽屉 -->
    <DeployHistoryDrawer
      v-model:visible="historyDrawerVisible"
      :certificate="currentCertificate"
    />
  </div>
</template>

<script lang="ts" setup>
import { ref, reactive, computed, onMounted } from 'vue';
import { Message } from '@arco-design/web-vue';
import { useI18n } from 'vue-i18n';
import dayjs from 'dayjs';
import {
  listCertificates,
  renewCertificate,
  deleteCertificate,
  type CertificateDTO,
} from '@/api/ssl';
import IssueCertificateDrawer from './components/IssueCertificateDrawer.vue';
import DeployCertificateDrawer from './components/DeployCertificateDrawer.vue';
import DeployHistoryDrawer from './components/DeployHistoryDrawer.vue';

const { t } = useI18n();

// 搜索表单
const searchForm = reactive({
  keyword: '',
});

// 表格数据
const loading = ref(false);
const allData = ref<CertificateDTO[]>([]);
const filteredData = ref<CertificateDTO[]>([]);

// 分页
const pagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
  showTotal: true,
  showPageSize: true,
});

// 抽屉状态
const issueDrawerVisible = ref(false);
const deployDrawerVisible = ref(false);
const historyDrawerVisible = ref(false);
const currentCertificate = ref<CertificateDTO | null>(null);

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
    const { data } = await listCertificates(1, 1000); // 获取所有数据
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

// 申请证书
const handleIssue = () => {
  issueDrawerVisible.value = true;
};

// 申请证书提交
const handleIssueSubmit = async () => {
  Message.success(t('ssl.certificates.message.issueSuccess'));
  issueDrawerVisible.value = false;
  fetchData();
};

// 续期
const handleRenew = async (record: CertificateDTO) => {
  try {
    await renewCertificate(record.id);
    Message.success(t('ssl.certificates.message.renewSuccess'));
    fetchData();
  } catch (error) {
    // Error already handled by interceptor
  }
};

// 部署
const handleDeploy = (record: CertificateDTO) => {
  currentCertificate.value = record;
  deployDrawerVisible.value = true;
};

// 部署提交
const handleDeploySubmit = async () => {
  Message.success(t('ssl.certificates.message.deploySuccess'));
  deployDrawerVisible.value = false;
};

// 查看部署历史
const handleViewHistory = (record: CertificateDTO) => {
  currentCertificate.value = record;
  historyDrawerVisible.value = true;
};

// 删除
const handleDelete = async (record: CertificateDTO) => {
  try {
    await deleteCertificate(record.id);
    Message.success(t('ssl.certificates.message.deleteSuccess'));
    fetchData();
  } catch (error) {
    // Error already handled by interceptor
  }
};

// 获取状态颜色
const getStatusColor = (status: string) => {
  const colors: Record<string, string> = {
    pending: 'gray',
    issuing: 'blue',
    active: 'green',
    renewing: 'cyan',
    expired: 'red',
    failed: 'red',
  };
  return colors[status] || 'gray';
};

// 获取过期时间颜色
const getExpiryColor = (expiresAt: string) => {
  const now = dayjs();
  const expiry = dayjs(expiresAt);
  const daysLeft = expiry.diff(now, 'day');
  
  if (daysLeft < 0) return 'var(--color-danger-6)';
  if (daysLeft < 7) return 'var(--color-warning-6)';
  if (daysLeft < 30) return 'var(--color-warning-5)';
  return 'var(--color-text-1)';
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

