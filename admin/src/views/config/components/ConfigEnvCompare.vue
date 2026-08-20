<template>
  <div class="env-compare">
    <div class="toolbar">
      <a-space>
        <a-select
          v-model="sourceEnv"
          :placeholder="$t('config.envCompare.sourceEnv')"
          style="width: 120px"
        >
          <a-option v-for="env in availableEnvs" :key="env" :value="env">
            {{ env }}
          </a-option>
        </a-select>
        <icon-swap />
        <a-select
          v-model="targetEnv"
          :placeholder="$t('config.envCompare.targetEnv')"
          style="width: 120px"
        >
          <a-option v-for="env in availableEnvs" :key="env" :value="env">
            {{ env }}
          </a-option>
        </a-select>
        <a-button type="primary" @click="handleCompare">
          <template #icon>
            <icon-code-block />
          </template>
          {{ $t('config.envCompare.startCompare') }}
        </a-button>
      </a-space>
    </div>

    <a-divider />

    <a-empty
      v-if="!hasCompared"
      :description="$t('config.envCompare.selectTip')"
    />

    <div v-else-if="missingOneEnv">
      <a-alert type="warning" :content="$t('config.envCompare.missingEnv')" />
      <div class="env-status">
        <a-row :gutter="16" style="margin-top: 16px">
          <a-col :span="12">
            <div class="env-header">
              {{ sourceEnv }}
              <a-tag :color="sourceConfig ? 'green' : 'red'">
                {{
                  sourceConfig
                    ? $t('config.envCompare.exists')
                    : $t('config.envCompare.notExists')
                }}
              </a-tag>
            </div>
          </a-col>
          <a-col :span="12">
            <div class="env-header">
              {{ targetEnv }}
              <a-tag :color="targetConfig ? 'green' : 'red'">
                {{
                  targetConfig
                    ? $t('config.envCompare.exists')
                    : $t('config.envCompare.notExists')
                }}
              </a-tag>
            </div>
          </a-col>
        </a-row>
      </div>

      <a-space style="margin-top: 16px">
        <a-button
          v-if="sourceConfig && !targetConfig"
          type="primary"
          status="success"
          @click="copyToTarget"
        >
          {{ $t('config.envCompare.copyToTarget') }}
        </a-button>
        <a-button
          v-if="!sourceConfig && targetConfig"
          type="primary"
          status="success"
          @click="copyToSource"
        >
          {{ $t('config.envCompare.copyToSource') }}
        </a-button>
      </a-space>
    </div>

    <div v-else>
      <div class="diff-summary">
        <a-alert
          v-if="isDifferent"
          type="info"
          :content="$t('config.envCompare.hasDifference')"
          style="margin-bottom: 16px"
        />
        <a-alert
          v-else
          type="success"
          :content="$t('config.envCompare.noDifference')"
          style="margin-bottom: 16px"
        />
      </div>

      <div class="comparison-table">
        <a-table
          :data="comparisonData"
          :bordered="true"
          :pagination="false"
          row-key="field"
        >
          <template #columns>
            <a-table-column
              :title="$t('config.envCompare.field')"
              data-index="field"
              :width="140"
            />
            <a-table-column :title="`${sourceEnv} (${$t('config.envCompare.type')})`" :width="120">
              <template #cell="{ record }">
                <a-tag v-if="record.sourceType">{{ record.sourceType }}</a-tag>
                <span v-else class="missing-tag">-</span>
              </template>
            </a-table-column>
            <a-table-column :title="`${sourceEnv} (${$t('config.envCompare.value')})`">
              <template #cell="{ record }">
                <div
                  class="value-cell"
                  :class="{ 'diff-highlight': record.isDifferent }"
                >
                  <span class="value-text">{{ record.sourceValue || '-' }}</span>
                </div>
              </template>
            </a-table-column>
            <a-table-column :title="`${targetEnv} (${$t('config.envCompare.type')})`" :width="120">
              <template #cell="{ record }">
                <a-tag v-if="record.targetType">{{ record.targetType }}</a-tag>
                <span v-else class="missing-tag">-</span>
              </template>
            </a-table-column>
            <a-table-column :title="`${targetEnv} (${$t('config.envCompare.value')})`">
              <template #cell="{ record }">
                <div
                  class="value-cell"
                  :class="{ 'diff-highlight': record.isDifferent }"
                >
                  <span class="value-text">{{ record.targetValue || '-' }}</span>
                </div>
              </template>
            </a-table-column>
          </template>
        </a-table>
      </div>

      <div v-if="isDifferent" class="action-buttons" style="margin-top: 16px">
        <a-space>
          <a-button type="primary" status="success" @click="syncAllToTarget">
            {{ $t('config.envCompare.syncAllToTarget') }}
          </a-button>
          <a-button type="primary" status="warning" @click="syncAllToSource">
            {{ $t('config.envCompare.syncAllToSource') }}
          </a-button>
        </a-space>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
  import { ref, computed, watch } from 'vue';
  import { Message, Modal } from '@arco-design/web-vue';
  import { IconSwap, IconCodeBlock } from '@arco-design/web-vue/es/icon';
  import { getConfigById, updateConfig, createConfig, type ParsedConfigItem } from '@/api/config';
  import { buildConfigKey } from '../utils';

  const props = defineProps<{
    baseKey: string;
    availableEnvs: string[];
    currentEnv?: string;
  }>();

  const emit = defineEmits<{
    refresh: [];
  }>();

  // 状态变量
  const hasCompared = ref(false);
  const sourceEnv = ref(props.currentEnv || props.availableEnvs[0] || 'dev');
  const targetEnv = ref(props.availableEnvs[1] || 'prod');
  const saving = ref(false);

  const sourceConfig = ref<ParsedConfigItem | null>(null);
  const targetConfig = ref<ParsedConfigItem | null>(null);

  // 监听可用环境列表和当前环境的变化，确保 sourceEnv 和 targetEnv 始终有效
  watch(
    () => [props.availableEnvs, props.currentEnv] as const,
    ([newAvailableEnvs, newCurrentEnv]) => {
      if (!newAvailableEnvs || newAvailableEnvs.length === 0) return;

      // 确保 sourceEnv 有效
      if (!newAvailableEnvs.includes(sourceEnv.value)) {
        sourceEnv.value = newCurrentEnv || newAvailableEnvs[0] || 'dev';
      }

      // 确保 targetEnv 有效
      if (!newAvailableEnvs.includes(targetEnv.value)) {
        // 尝试选择不同于 sourceEnv 的环境
        const differentEnv = newAvailableEnvs.find((env) => env !== sourceEnv.value);
        targetEnv.value = differentEnv || newAvailableEnvs[1] || newAvailableEnvs[0] || 'prod';
      }

      // 重置对比状态
      hasCompared.value = false;
    },
    { immediate: false }
  );

  // 是否缺少某个环境的配置
  const missingOneEnv = computed(() => {
    if (!hasCompared.value) return false;
    return !sourceConfig.value || !targetConfig.value;
  });

  // 是否有差异
  const isDifferent = computed(() => {
    if (!sourceConfig.value || !targetConfig.value) return false;
    return (
      JSON.stringify(sourceConfig.value.value) !== JSON.stringify(targetConfig.value.value)
    );
  });

  // 格式化值显示
  function formatValue(value: string, type: string): string {
    if (type === 'object' || type === 'array') {
      try {
        const parsed = JSON.parse(value);
        return (
          JSON.stringify(parsed, null, 0).substring(0, 100) +
          (value.length > 100 ? '...' : '')
        );
      } catch {
        return value.substring(0, 100) + (value.length > 100 ? '...' : '');
      }
    }
    return value.substring(0, 100) + (value.length > 100 ? '...' : '');
  }

  // 对比数据
  const comparisonData = computed(() => {
    if (!sourceConfig.value && !targetConfig.value) return [];

    const sourceFields = sourceConfig.value?.value || {};
    const targetFields = targetConfig.value?.value || {};

    // 收集所有字段名
    const allFields = new Set<string>();
    Object.keys(sourceFields).forEach((key) => allFields.add(key));
    Object.keys(targetFields).forEach((key) => allFields.add(key));

    return Array.from(allFields).map((field) => {
      const sourceValue = sourceFields[field];
      const targetValue = targetFields[field];

      return {
        field,
        sourceType: sourceValue?.type || null,
        sourceValue: sourceValue ? formatValue(sourceValue.value, sourceValue.type) : null,
        targetType: targetValue?.type || null,
        targetValue: targetValue ? formatValue(targetValue.value, targetValue.type) : null,
        isDifferent: !sourceValue || !targetValue || 
          sourceValue.type !== targetValue.type || 
          sourceValue.value !== targetValue.value,
      };
    });
  });

  // 开始对比
  async function handleCompare() {
    if (sourceEnv.value === targetEnv.value) {
      Message.warning('源环境和目标环境不能相同');
      return;
    }

    try {
      hasCompared.value = false;
      sourceConfig.value = null;
      targetConfig.value = null;
      
      // 构建完整的配置key（使用buildConfigKey确保global不加后缀）
      const sourceKey = buildConfigKey(props.baseKey, sourceEnv.value);
      const targetKey = buildConfigKey(props.baseKey, targetEnv.value);
      
      // 从parent获取配置列表（通过refresh事件触发父组件刷新后重新传入）
      // 这里我们需要通过emit向父组件通信，或者直接调用API
      // 为了简化，我们使用queryConfigs API来查找
      const { queryConfigs, parseConfigItem } = await import('@/api/config');
      
      // 查找源环境配置（后端使用LIKE查询，需要精确匹配key）
      try {
        const sourceRes = await queryConfigs({ key: sourceKey, pageSize: 100 });
        if (sourceRes.data.content && sourceRes.data.content.length > 0) {
          // 从结果中精确匹配key，避免LIKE误匹配
          const exactMatch = sourceRes.data.content.find(item => item.key === sourceKey);
          if (exactMatch) {
            sourceConfig.value = parseConfigItem(exactMatch);
          }
        }
      } catch (error) {
        console.warn(`未找到源环境配置: ${sourceKey}`);
      }
      
      // 查找目标环境配置（后端使用LIKE查询，需要精确匹配key）
      try {
        const targetRes = await queryConfigs({ key: targetKey, pageSize: 100 });
        if (targetRes.data.content && targetRes.data.content.length > 0) {
          // 从结果中精确匹配key，避免LIKE误匹配
          const exactMatch = targetRes.data.content.find(item => item.key === targetKey);
          if (exactMatch) {
            targetConfig.value = parseConfigItem(exactMatch);
          }
        }
      } catch (error) {
        console.warn(`未找到目标环境配置: ${targetKey}`);
      }
      
      hasCompared.value = true;
    } catch (error: any) {
      Message.error(`对比失败: ${error.message || '未知错误'}`);
    }
  }

  // 复制到目标环境
  async function copyToTarget() {
    if (!sourceConfig.value) return;

    Modal.confirm({
      title: '确认复制',
      content: `确定要将 ${sourceEnv.value} 环境的配置复制到 ${targetEnv.value} 环境吗？`,
      okText: '确认',
      cancelText: '取消',
      onOk: async () => {
        try {
          saving.value = true;

          // 创建新配置（使用buildConfigKey确保global不加后缀）
          await createConfig({
            key: buildConfigKey(props.baseKey, targetEnv.value),
            value: sourceConfig.value!.value,
            metadata: sourceConfig.value!.metadata,
            description: sourceConfig.value!.description,
            changeNote: `从 ${sourceEnv.value} 复制`,
          });

          Message.success('复制成功');
          emit('refresh');
          hasCompared.value = false;
        } catch (error: any) {
          Message.error(`复制失败: ${error.message || '未知错误'}`);
        } finally {
          saving.value = false;
        }
      },
    });
  }

  // 复制到源环境
  async function copyToSource() {
    if (!targetConfig.value) return;

    Modal.confirm({
      title: '确认复制',
      content: `确定要将 ${targetEnv.value} 环境的配置复制到 ${sourceEnv.value} 环境吗？`,
      okText: '确认',
      cancelText: '取消',
      onOk: async () => {
        try {
          saving.value = true;

          // 创建新配置（使用buildConfigKey确保global不加后缀）
          await createConfig({
            key: buildConfigKey(props.baseKey, sourceEnv.value),
            value: targetConfig.value!.value,
            metadata: targetConfig.value!.metadata,
            description: targetConfig.value!.description,
            changeNote: `从 ${targetEnv.value} 复制`,
          });

          Message.success('复制成功');
          emit('refresh');
          hasCompared.value = false;
        } catch (error: any) {
          Message.error(`复制失败: ${error.message || '未知错误'}`);
        } finally {
          saving.value = false;
        }
      },
    });
  }

  // 全部同步到目标
  function syncAllToTarget() {
    if (!sourceConfig.value || !targetConfig.value) return;

    Modal.confirm({
      title: '确认同步',
      content: `确定要将 ${sourceEnv.value} 环境的所有配置同步到 ${targetEnv.value} 环境吗？这将覆盖目标环境的配置。`,
      okText: '确认',
      cancelText: '取消',
      onOk: async () => {
        try {
          saving.value = true;

          await updateConfig(targetConfig.value!.id, {
            value: sourceConfig.value!.value,
            metadata: sourceConfig.value!.metadata,
            description: sourceConfig.value!.description,
            changeNote: `从 ${sourceEnv.value} 同步`,
          });

          Message.success('同步成功');
          emit('refresh');
          hasCompared.value = false;
        } catch (error: any) {
          Message.error(`同步失败: ${error.message || '未知错误'}`);
        } finally {
          saving.value = false;
        }
      },
    });
  }

  // 全部同步到源
  function syncAllToSource() {
    if (!sourceConfig.value || !targetConfig.value) return;

    Modal.confirm({
      title: '确认同步',
      content: `确定要将 ${targetEnv.value} 环境的所有配置同步到 ${sourceEnv.value} 环境吗？这将覆盖源环境的配置。`,
      okText: '确认',
      cancelText: '取消',
      onOk: async () => {
        try {
          saving.value = true;

          await updateConfig(sourceConfig.value!.id, {
            value: targetConfig.value!.value,
            metadata: targetConfig.value!.metadata,
            description: targetConfig.value!.description,
            changeNote: `从 ${targetEnv.value} 同步`,
          });

          Message.success('同步成功');
          emit('refresh');
          hasCompared.value = false;
        } catch (error: any) {
          Message.error(`同步失败: ${error.message || '未知错误'}`);
        } finally {
          saving.value = false;
        }
      },
    });
  }
</script>

<style scoped lang="less">
  .env-compare {
    padding: 16px;
    max-width: 100%;
    overflow: hidden;

    .toolbar {
      display: flex;
      align-items: center;
    }

    :deep(.arco-table-container) {
      overflow-x: auto;
    }

    .env-status {
      margin-top: 16px;

      .env-header {
        font-weight: bold;
        padding: 8px;
        background: var(--color-fill-1);
        border-radius: 4px;
      }
    }

    .comparison-table {
      .value-cell {
        display: flex;
        align-items: center;
        gap: 8px;

        .value-text {
          flex: 1;
          word-break: break-all;
        }

        &.diff-highlight {
          background-color: var(--color-warning-light-1);
          padding: 4px;
          border-radius: 2px;
        }
      }
    }

    .missing-tag {
      color: var(--color-text-3);
    }
  }
</style>
