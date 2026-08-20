<template>
  <a-drawer
    :width="900"
    :visible="visible"
    :title="$t('shorturl.links.stats.title')"
    :mask-closable="true"
    @cancel="handleClose"
  >
    <a-spin :loading="loading" style="width: 100%">
      <div v-if="stats" class="stats-container">
        <!-- 概览卡片 -->
        <div class="stats-overview">
          <h3>{{ $t('shorturl.links.stats.overview') }}</h3>
          <a-row :gutter="16">
            <a-col :span="12">
              <a-statistic
                :title="$t('shorturl.links.stats.totalVisits')"
                :value="stats.totalVisits"
                :value-style="{ color: '#0fbf60' }"
              >
                <template #prefix>
                  <icon-eye />
                </template>
              </a-statistic>
            </a-col>
            <a-col :span="12">
              <a-statistic
                :title="$t('shorturl.links.stats.totalSuccess')"
                :value="stats.totalSuccess"
                :value-style="{ color: '#165dff' }"
              >
                <template #prefix>
                  <icon-check-circle />
                </template>
              </a-statistic>
            </a-col>
          </a-row>
        </div>

        <!-- 日趋势图表 -->
        <div class="stats-chart">
          <h3>{{ $t('shorturl.links.stats.dailyTrend') }}</h3>
          <a-spin :loading="chartLoading">
            <div ref="chartRef" style="width: 100%; height: 300px"></div>
          </a-spin>
        </div>

        <!-- 最近访问 -->
        <div class="stats-table">
          <h3>{{ $t('shorturl.links.stats.recentVisits') }}</h3>
          <a-table
            :data="stats.recentVisits"
            :pagination="false"
            :bordered="{ cell: true }"
            :scroll="{ x: 800 }"
          >
            <template #columns>
              <a-table-column
                :title="$t('shorturl.links.stats.ip')"
                data-index="ip"
                :width="150"
              />
              <a-table-column
                :title="$t('shorturl.links.stats.userAgent')"
                data-index="userAgent"
                :width="300"
                :ellipsis="true"
                :tooltip="true"
              />
              <a-table-column
                :title="$t('shorturl.links.stats.referer')"
                data-index="referer"
                :width="200"
                :ellipsis="true"
                :tooltip="true"
              >
                <template #cell="{ record }">
                  <span v-if="record.referer">{{ record.referer }}</span>
                  <span v-else style="color: var(--color-text-3)">-</span>
                </template>
              </a-table-column>
              <a-table-column
                :title="$t('shorturl.links.stats.visitedAt')"
                data-index="visitedAt"
                :width="180"
              >
                <template #cell="{ record }">
                  {{ formatDateTime(record.visitedAt) }}
                </template>
              </a-table-column>
            </template>
          </a-table>
        </div>

        <!-- 最近成功事件 -->
        <div class="stats-table">
          <h3>{{ $t('shorturl.links.stats.recentSuccess') }}</h3>
          <a-table
            :data="stats.recentSuccess"
            :pagination="false"
            :bordered="{ cell: true }"
            :scroll="{ x: 600 }"
          >
            <template #columns>
              <a-table-column
                :title="$t('shorturl.links.stats.eventId')"
                data-index="eventId"
                :width="200"
              >
                <template #cell="{ record }">
                  <span v-if="record.eventId">{{ record.eventId }}</span>
                  <span v-else style="color: var(--color-text-3)">-</span>
                </template>
              </a-table-column>
              <a-table-column
                :title="$t('shorturl.links.stats.attrs')"
                data-index="attrs"
                :width="250"
                :ellipsis="true"
                :tooltip="true"
              >
                <template #cell="{ record }">
                  <span v-if="record.attrs && Object.keys(record.attrs).length > 0">
                    {{ JSON.stringify(record.attrs) }}
                  </span>
                  <span v-else style="color: var(--color-text-3)">-</span>
                </template>
              </a-table-column>
              <a-table-column
                :title="$t('shorturl.links.stats.createdAt')"
                data-index="createdAt"
                :width="180"
              >
                <template #cell="{ record }">
                  {{ formatDateTime(record.createdAt) }}
                </template>
              </a-table-column>
            </template>
          </a-table>
        </div>
      </div>
    </a-spin>
  </a-drawer>
</template>

<script lang="ts" setup>
import { ref, watch, nextTick } from 'vue';
import { Message } from '@arco-design/web-vue';
import { useI18n } from 'vue-i18n';
import * as echarts from 'echarts';
import { getShortLinkStats, type ShortLinkStatsDTO } from '@/api/shorturl';

const { t } = useI18n();

interface Props {
  visible: boolean;
  linkId: number | null;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  (e: 'update:visible', visible: boolean): void;
}>();

const loading = ref(false);
const chartLoading = ref(false);
const stats = ref<ShortLinkStatsDTO | null>(null);
const chartRef = ref<HTMLDivElement>();
let chartInstance: echarts.ECharts | null = null;

// 监听 visible 和 linkId 变化
watch(
  () => [props.visible, props.linkId],
  ([visible, linkId]) => {
    if (visible && linkId) {
      fetchStats(linkId as number);
    }
  },
  { immediate: true }
);

// 加载统计数据
const fetchStats = async (linkId: number) => {
  try {
    loading.value = true;
    const res = await getShortLinkStats(linkId, 30);
    stats.value = res.data;
    
    // 等待 DOM 更新后渲染图表
    await nextTick();
    renderChart();
  } catch (err) {
    Message.error(t('shorturl.links.stats.load.failed'));
    console.error(err);
  } finally {
    loading.value = false;
  }
};

// 渲染图表
const renderChart = () => {
  if (!chartRef.value || !stats.value) return;

  chartLoading.value = true;
  
  try {
    // 销毁旧实例
    if (chartInstance) {
      chartInstance.dispose();
    }

    // 创建新实例
    chartInstance = echarts.init(chartRef.value);

    const dates = stats.value.dailyStats.map(d => d.date);
    const visitCounts = stats.value.dailyStats.map(d => d.visitCount);
    const successCounts = stats.value.dailyStats.map(d => d.successCount);

    const option: echarts.EChartsOption = {
      tooltip: {
        trigger: 'axis',
        axisPointer: {
          type: 'cross',
        },
      },
      legend: {
        data: [t('shorturl.links.stats.visitCount'), t('shorturl.links.stats.successCount')],
      },
      grid: {
        left: '3%',
        right: '4%',
        bottom: '3%',
        containLabel: true,
      },
      xAxis: {
        type: 'category',
        boundaryGap: false,
        data: dates,
      },
      yAxis: {
        type: 'value',
      },
      series: [
        {
          name: t('shorturl.links.stats.visitCount'),
          type: 'line',
          smooth: true,
          data: visitCounts,
          itemStyle: {
            color: '#0fbf60',
          },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: 'rgba(15, 191, 96, 0.3)' },
              { offset: 1, color: 'rgba(15, 191, 96, 0.05)' },
            ]),
          },
        },
        {
          name: t('shorturl.links.stats.successCount'),
          type: 'line',
          smooth: true,
          data: successCounts,
          itemStyle: {
            color: '#165dff',
          },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: 'rgba(22, 93, 255, 0.3)' },
              { offset: 1, color: 'rgba(22, 93, 255, 0.05)' },
            ]),
          },
        },
      ],
    };

    chartInstance.setOption(option);
  } finally {
    chartLoading.value = false;
  }
};

// 关闭抽屉
const handleClose = () => {
  if (chartInstance) {
    chartInstance.dispose();
    chartInstance = null;
  }
  emit('update:visible', false);
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

<script lang="ts">
export default {
  name: 'ShortLinkStatsDrawer',
};
</script>

<style scoped lang="less">
.stats-container {
  padding: 16px 0;
}

.stats-overview,
.stats-chart,
.stats-table {
  margin-bottom: 32px;
  
  h3 {
    margin-bottom: 16px;
    font-size: 16px;
    font-weight: 600;
  }
}

.stats-overview {
  padding: 16px;
  background: var(--color-bg-2);
  border-radius: 4px;
}
</style>
