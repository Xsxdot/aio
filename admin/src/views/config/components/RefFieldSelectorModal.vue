<template>
  <a-modal
    v-model:visible="modalVisible"
    :title="$t('config.refSelector.title')"
    :width="800"
    :mask-closable="false"
    @before-ok="handleConfirm"
    @cancel="handleCancel"
  >
    <div class="ref-selector-container">
      <a-input-search
        v-model="searchKey"
        :placeholder="$t('config.refSelector.searchPlaceholder')"
        allow-clear
        style="margin-bottom: 16px"
      />

      <a-spin :loading="loading" class="full-height">
        <div v-if="!loading && refOptions.length === 0" class="empty-state">
          <a-empty :description="$t('config.refSelector.empty')" />
        </div>

        <div v-else class="ref-list-container">
          <div class="ref-list-header">
            <a-checkbox
              v-model="selectAll"
              :indeterminate="indeterminate"
              @change="handleSelectAllChange"
            >
              {{ $t('config.refSelector.selectAll') }}
              <span class="count-hint">
                ({{ selectedRefs.length }} / {{ filteredOptions.length }})
              </span>
            </a-checkbox>
          </div>

          <a-divider style="margin: 12px 0" />

          <div class="ref-list-scroll">
            <a-checkbox-group v-model="selectedRefs" class="ref-checkbox-group">
              <div
                v-for="option in filteredOptions"
                :key="option.value"
                class="ref-option-item"
              >
                <a-checkbox :value="option.value">
                  <div class="ref-option-content">
                    <span class="ref-value">{{ option.value }}</span>
                    <a-tag
                      v-if="option.configKey"
                      size="small"
                      :color="getEnvColor(option.env)"
                      style="margin-left: 8px"
                    >
                      {{ option.env }}
                    </a-tag>
                    <span v-if="option.fieldName" class="field-hint">
                      → {{ option.fieldName }}
                    </span>
                  </div>
                </a-checkbox>
              </div>
            </a-checkbox-group>
          </div>
        </div>
      </a-spin>
    </div>
  </a-modal>
</template>

<script lang="ts" setup>
  import { ref, computed, watch } from 'vue';
  import { Message } from '@arco-design/web-vue';
  import { queryConfigs, parseConfigItem, type ConfigItem } from '@/api/config';
  import { parseKeyEnv } from '../utils';

  interface RefOption {
    value: string; // 引用字符串，如 "app.cert.dev.privateKey"
    configKey: string; // 配置键，如 "app.cert.dev"
    fieldName: string; // 字段名，如 "privateKey"
    env: string; // 环境，如 "dev"
    label: string; // 显示标签
  }

  interface Props {
    visible: boolean;
  }

  interface Emits {
    (e: 'update:visible', value: boolean): void;
    (e: 'confirm', refs: string[]): void;
  }

  const props = defineProps<Props>();
  const emit = defineEmits<Emits>();

  const modalVisible = computed({
    get: () => props.visible,
    set: (value) => emit('update:visible', value),
  });

  const loading = ref(false);
  const searchKey = ref('');
  const selectedRefs = ref<string[]>([]);
  const refOptions = ref<RefOption[]>([]);

  // 过滤后的选项
  const filteredOptions = computed(() => {
    if (!searchKey.value) return refOptions.value;

    const keyword = searchKey.value.toLowerCase();
    return refOptions.value.filter(
      (opt) =>
        opt.value.toLowerCase().includes(keyword) ||
        opt.label.toLowerCase().includes(keyword)
    );
  });

  // 全选状态
  const selectAll = computed({
    get: () => {
      if (filteredOptions.value.length === 0) return false;
      return selectedRefs.value.length === filteredOptions.value.length;
    },
    set: (value) => {
      if (value) {
        selectedRefs.value = filteredOptions.value.map((opt) => opt.value);
      } else {
        selectedRefs.value = [];
      }
    },
  });

  // 半选状态
  const indeterminate = computed(() => {
    const len = selectedRefs.value.length;
    const total = filteredOptions.value.length;
    return len > 0 && len < total;
  });

  // 取消
  function handleCancel() {
    selectedRefs.value = [];
    searchKey.value = '';
  }

  // 加载所有配置项并解析引用选项
  async function loadRefOptions() {
    try {
      loading.value = true;
      const allConfigs: ConfigItem[] = [];
      let pageNum = 1;
      const pageSize = 200;
      let hasMore = true;

      // 分页拉取所有配置
      while (hasMore) {
        // eslint-disable-next-line no-await-in-loop
        const res = await queryConfigs({
          page: pageNum,
          pageSize,
        });

        const content = res.data.content || [];
        allConfigs.push(...content);

        const totalCount = res.data.total || 0;
        if (allConfigs.length >= totalCount || content.length < pageSize) {
          hasMore = false;
        } else {
          pageNum += 1;
        }
      }

      // 解析配置项，生成引用选项
      const options: RefOption[] = [];

      allConfigs.forEach((config) => {
        const parsed = parseConfigItem(config);
        const { env } = parseKeyEnv(config.key);

        // 1. 添加整项引用（配置键本身）
        options.push({
          value: config.key,
          configKey: config.key,
          fieldName: '',
          env: env || 'global',
          label: config.key,
        });

        // 2. 添加字段级引用
        Object.keys(parsed.value).forEach((fieldName) => {
          const refValue = `${config.key}.${fieldName}`;
          options.push({
            value: refValue,
            configKey: config.key,
            fieldName,
            env: env || 'global',
            label: refValue,
          });
        });
      });

      refOptions.value = options;
    } catch (error: any) {
      Message.error(`加载配置失败: ${error.message || '未知错误'}`);
      refOptions.value = [];
    } finally {
      loading.value = false;
    }
  }

  // 监听 visible 变化，加载数据
  watch(
    () => props.visible,
    (visible) => {
      if (visible && refOptions.value.length === 0) {
        loadRefOptions();
      }
    },
    { immediate: true }
  );

  // 全选/取消全选
  function handleSelectAllChange(
    value: boolean | (string | number | boolean)[]
  ) {
    if (typeof value === 'boolean') {
      selectAll.value = value;
    }
  }

  // 确认
  function handleConfirm() {
    if (selectedRefs.value.length === 0) {
      Message.warning('请至少选择一个引用字段');
      return false;
    }

    emit('confirm', selectedRefs.value);
    handleCancel();
    return true;
  }

  // 获取环境颜色
  function getEnvColor(env: string): string {
    const colorMap: Record<string, string> = {
      dev: 'blue',
      test: 'green',
      staging: 'orange',
      prod: 'red',
      global: 'purple',
    };
    return colorMap[env] || 'gray';
  }
</script>

<style scoped lang="less">
  .ref-selector-container {
    min-height: 400px;
    max-height: 600px;
    display: flex;
    flex-direction: column;
  }

  .full-height {
    flex: 1;
    display: flex;
    flex-direction: column;
  }

  .empty-state {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 300px;
  }

  .ref-list-container {
    display: flex;
    flex-direction: column;
    height: 100%;
  }

  .ref-list-header {
    padding: 8px 12px;
    background-color: var(--color-fill-2);
    border-radius: 4px;

    .count-hint {
      margin-left: 4px;
      color: var(--color-text-3);
      font-size: 12px;
    }
  }

  .ref-list-scroll {
    flex: 1;
    overflow-y: auto;
    max-height: 450px;
    padding: 8px 0;
  }

  .ref-checkbox-group {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .ref-option-item {
    padding: 8px 12px;
    border-radius: 4px;
    transition: background-color 0.2s;

    &:hover {
      background-color: var(--color-fill-1);
    }
  }

  .ref-option-content {
    display: flex;
    align-items: center;
    gap: 8px;

    .ref-value {
      font-family: 'Monaco', 'Menlo', 'Courier New', monospace;
      font-size: 13px;
    }

    .field-hint {
      margin-left: auto;
      color: var(--color-text-3);
      font-size: 12px;
    }
  }
</style>
