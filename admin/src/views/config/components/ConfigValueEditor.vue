<template>
  <div class="config-value-editor">
    <a-alert v-if="!configItem" type="warning" style="margin-bottom: 16px">
      {{ $t('config.editor.noConfig') }}
    </a-alert>

    <div v-else>
      <!-- 字段列表 -->
      <div v-if="fieldRows.length === 0" class="empty-fields">
        <a-empty :description="$t('config.editor.noFields')" />
      </div>

      <div v-else class="fields-table">
        <div class="table-header">
          <div class="col-field-name">{{ $t('config.editor.fieldName') }}</div>
          <div class="col-type">{{ $t('config.editor.fieldType') }}</div>
          <div class="col-value">{{ $t('config.editor.fieldValue') }}</div>
          <div class="col-actions">{{ $t('config.editor.actions') }}</div>
        </div>

        <div v-for="(row, index) in fieldRows" :key="index" class="table-row">
          <!-- 字段名 -->
          <div class="col-field-name">
            <a-input
              v-model="row.fieldName"
              :placeholder="$t('config.editor.fieldNamePlaceholder')"
              :status="getFieldNameStatus(row.fieldName, index)"
              allow-clear
            />
          </div>

          <!-- 类型 -->
          <div class="col-type">
            <a-select v-model="row.type" style="width: 100%">
              <a-option value="string">{{
                $t('config.valueType.string')
              }}</a-option>
              <a-option value="encrypted">{{
                $t('config.valueType.encrypted')
              }}</a-option>
              <a-option value="int">{{ $t('config.valueType.int') }}</a-option>
              <a-option value="float">{{
                $t('config.valueType.float')
              }}</a-option>
              <a-option value="bool">{{
                $t('config.valueType.bool')
              }}</a-option>
              <a-option value="ref">{{ $t('config.valueType.ref') }}</a-option>
              <a-option value="object">{{
                $t('config.valueType.object')
              }}</a-option>
              <a-option value="array">{{
                $t('config.valueType.array')
              }}</a-option>
            </a-select>
          </div>

          <!-- 值 -->
          <div class="col-value">
            <!-- String/Encrypted -->
            <a-input
              v-if="row.type === 'string'"
              v-model="row.value"
              :placeholder="$t('config.field.valuePlaceholder')"
              allow-clear
            />
            <a-input-password
              v-else-if="row.type === 'encrypted'"
              v-model="row.value"
              :placeholder="$t('config.editor.encryptedHint')"
              allow-clear
            />

            <!-- Int -->
            <a-input-number
              v-else-if="row.type === 'int'"
              v-model="row.numValue"
              :placeholder="$t('config.field.valuePlaceholder')"
              :precision="0"
              style="width: 100%"
              @change="row.value = String(row.numValue || 0)"
            />

            <!-- Float -->
            <a-input-number
              v-else-if="row.type === 'float'"
              v-model="row.numValue"
              :placeholder="$t('config.field.valuePlaceholder')"
              :precision="2"
              style="width: 100%"
              @change="row.value = String(row.numValue || 0)"
            />

            <!-- Bool -->
            <a-switch
              v-else-if="row.type === 'bool'"
              v-model="row.boolValue"
              type="round"
              @change="row.value = String(row.boolValue)"
            >
              <template #checked>True</template>
              <template #unchecked>False</template>
            </a-switch>

            <!-- Ref -->
            <a-input
              v-else-if="row.type === 'ref'"
              v-model="row.value"
              :placeholder="$t('config.editor.refPlaceholder')"
              allow-clear
            />

            <!-- Object/Array -->
            <div
              v-else-if="row.type === 'object' || row.type === 'array'"
              class="json-field"
            >
              <a-textarea
                v-model="row.value"
                :placeholder="
                  row.type === 'object'
                    ? $t('config.editor.objectPlaceholder')
                    : $t('config.editor.arrayPlaceholder')
                "
                :auto-size="{ minRows: 2, maxRows: 6 }"
                :status="isValidJSON(row.value) ? undefined : 'error'"
                allow-clear
              />
              <div class="json-actions">
                <a-button
                  type="text"
                  size="mini"
                  :disabled="!isValidJSON(row.value)"
                  @click="formatJSON(row)"
                >
                  {{ $t('config.editor.formatJSON') }}
                </a-button>
                <a-tag
                  v-if="row.value"
                  :color="isValidJSON(row.value) ? 'green' : 'red'"
                >
                  {{
                    isValidJSON(row.value)
                      ? $t('config.editor.validJSON')
                      : $t('config.editor.invalidJSON')
                  }}
                </a-tag>
              </div>
            </div>
          </div>

          <!-- 操作 -->
          <div class="col-actions">
            <a-button
              type="text"
              status="danger"
              size="small"
              @click="removeField(index)"
            >
              <template #icon><icon-delete /></template>
            </a-button>
          </div>
        </div>
      </div>

      <!-- 添加字段按钮 -->
      <div class="add-field-section">
        <a-space>
          <a-button type="dashed" @click="addField">
            <template #icon><icon-plus /></template>
            {{ $t('config.editor.addField') }}
          </a-button>
          <a-button
            type="dashed"
            status="success"
            @click="handleShowRefSelector"
          >
            <template #icon><icon-link /></template>
            {{ $t('config.editor.addRefField') }}
          </a-button>
        </a-space>
      </div>

      <!-- 变更说明和保存 -->
      <a-divider />
      <a-form :model="formData" layout="vertical">
        <a-form-item
          field="changeNote"
          :label="$t('config.field.changeNote')"
          :rules="[
            {
              required: true,
              message: $t('config.validation.changeNoteRequired'),
            },
          ]"
        >
          <a-textarea
            v-model="formData.changeNote"
            :placeholder="$t('config.field.changeNotePlaceholder')"
            :auto-size="{ minRows: 2, maxRows: 4 }"
            allow-clear
          />
        </a-form-item>

        <a-form-item>
          <a-space>
            <a-button type="primary" :loading="saving" @click="handleSave">
              <template #icon><icon-save /></template>
              {{ $t('config.save') }}
            </a-button>
            <a-button @click="handleReset">
              {{ $t('config.reset') }}
            </a-button>
            <a-button type="outline" @click="handleViewFinalJSON">
              <template #icon><icon-eye /></template>
              {{ $t('config.editor.viewFinalJSON') }}
            </a-button>
          </a-space>
        </a-form-item>
      </a-form>
    </div>

    <!-- 引用字段选择弹窗 -->
    <ref-field-selector-modal
      v-model:visible="refSelectorVisible"
      @confirm="handleAddRefFields"
    />

    <!-- 最终JSON查看弹窗 -->
    <a-modal
      v-model:visible="jsonViewerVisible"
      :title="$t('config.editor.finalJSONTitle')"
      :width="800"
      :footer="false"
      unmount-on-close
    >
      <a-alert type="info" style="margin-bottom: 16px">
        {{ $t('config.editor.finalJSONHint') }}
      </a-alert>
      
      <a-spin :loading="loadingJSON" style="width: 100%">
        <div v-if="finalJSON" class="json-viewer">
          <div class="json-actions">
            <a-button type="text" size="small" @click="handleCopyJSON">
              {{ $t('config.editor.copyJSON') }}
            </a-button>
          </div>
          <pre class="json-content">{{ finalJSON }}</pre>
        </div>
      </a-spin>
    </a-modal>
  </div>
</template>

<script lang="ts" setup>
  import { ref, reactive, watch } from 'vue';
  import { Message } from '@arco-design/web-vue';
  import {
    IconSave,
    IconDelete,
    IconPlus,
    IconLink,
    IconEye,
  } from '@arco-design/web-vue/es/icon';
  import { updateConfig, getConfigJSON, type ParsedConfigItem } from '@/api/config';
  import RefFieldSelectorModal from './RefFieldSelectorModal.vue';
  import { isValidEnvSuffix } from '../utils';

  interface FieldRow {
    fieldName: string;
    type: string;
    value: string;
    numValue?: number;
    boolValue?: boolean;
  }

  const props = defineProps<{
    configItem: ParsedConfigItem;
  }>();

  const emit = defineEmits<{
    update: [];
  }>();

  const saving = ref(false);
  const fieldRows = ref<FieldRow[]>([]);
  const formData = reactive({
    changeNote: '',
  });
  const refSelectorVisible = ref(false);
  const jsonViewerVisible = ref(false);
  const loadingJSON = ref(false);
  const finalJSON = ref<string>('');

  // 加载数据
  function loadData() {
    if (!props.configItem || !props.configItem.value) {
      fieldRows.value = [];
      return;
    }

    // 将 value (Record<string, ConfigValue>) 转换为 fieldRows
    const rows: FieldRow[] = [];
    Object.entries(props.configItem.value).forEach(
      ([fieldName, configValue]) => {
        const row: FieldRow = {
          fieldName,
          type: configValue.type,
          value: configValue.value,
        };

        // 根据类型设置辅助字段
        if (configValue.type === 'int' || configValue.type === 'float') {
          row.numValue = parseFloat(configValue.value);
        } else if (configValue.type === 'bool') {
          row.boolValue = configValue.value === 'true';
        }

        rows.push(row);
      }
    );

    fieldRows.value = rows;
    formData.changeNote = '';
  }

  // 监听配置项变化
  watch(
    () => props.configItem,
    () => {
      loadData();
    },
    { immediate: true }
  );

  // 添加字段
  function addField() {
    fieldRows.value.push({
      fieldName: '',
      type: 'string',
      value: '',
    });
  }

  // 显示引用字段选择器
  function handleShowRefSelector() {
    refSelectorVisible.value = true;
  }

  // 添加引用字段
  function handleAddRefFields(refs: string[]) {
    let addedCount = 0;
    let skippedCount = 0;
    const skippedNames: string[] = [];

    refs.forEach((refValue) => {
      // 提取字段名：按 . 分割，若最后一段是环境名则取倒数第二段，否则取最后一段
      const parts = refValue.split('.');
      let fieldName = '';

      if (parts.length >= 2) {
        const lastPart = parts[parts.length - 1];
        // 检查最后一段是否是环境名
        if (isValidEnvSuffix(lastPart) || lastPart === 'global') {
          // 取倒数第二段
          fieldName = parts[parts.length - 2];
        } else {
          // 取最后一段
          fieldName = lastPart;
        }
      } else {
        // 只有一段，直接使用
        [fieldName] = parts;
      }

      // 检查字段名是否已存在
      const exists = fieldRows.value.some((row) => row.fieldName === fieldName);
      if (exists) {
        skippedCount += 1;
        skippedNames.push(fieldName);
        return;
      }

      // 添加新字段行
      fieldRows.value.push({
        fieldName,
        type: 'ref',
        value: refValue,
      });
      addedCount += 1;
    });

    // 提示信息
    if (addedCount > 0) {
      Message.success(`成功添加 ${addedCount} 个引用字段`);
    }
    if (skippedCount > 0) {
      const names = skippedNames.slice(0, 3).join(', ');
      const more = skippedCount > 3 ? ` 等 ${skippedCount} 个` : '';
      Message.warning(`已跳过重复字段：${names}${more}`);
    }
  }

  // 删除字段
  function removeField(index: number) {
    fieldRows.value.splice(index, 1);
  }

  // 验证字段名状态
  function getFieldNameStatus(
    fieldName: string,
    currentIndex: number
  ): 'error' | undefined {
    if (!fieldName) return 'error';

    // 检查重复
    const duplicateIndex = fieldRows.value.findIndex(
      (row, index) => index !== currentIndex && row.fieldName === fieldName
    );

    return duplicateIndex >= 0 ? 'error' : undefined;
  }

  // 验证JSON
  function isValidJSON(str: string): boolean {
    if (!str) return true;
    try {
      JSON.parse(str);
      return true;
    } catch {
      return false;
    }
  }

  // 格式化JSON
  function formatJSON(row: FieldRow) {
    if (isValidJSON(row.value)) {
      try {
        const parsed = JSON.parse(row.value);
        row.value = JSON.stringify(parsed, null, 2);
      } catch (e) {
        Message.error('JSON格式化失败');
      }
    }
  }

  // 保存
  async function handleSave() {
    // 验证
    if (!formData.changeNote) {
      Message.error('请输入变更说明');
      return;
    }

    // 验证字段名
    const emptyFieldNames = fieldRows.value.filter((row) => !row.fieldName);
    if (emptyFieldNames.length > 0) {
      Message.error('字段名不能为空');
      return;
    }

    // 验证字段名唯一性
    const fieldNames = fieldRows.value.map((row) => row.fieldName);
    const duplicateField = fieldRows.value.find(
      (row) =>
        fieldNames.indexOf(row.fieldName) !==
        fieldNames.lastIndexOf(row.fieldName)
    );
    if (duplicateField) {
      Message.error(`字段名 "${duplicateField.fieldName}" 重复`);
      return;
    }

    // 验证JSON字段
    const invalidJsonField = fieldRows.value.find(
      (row) =>
        (row.type === 'object' || row.type === 'array') &&
        !isValidJSON(row.value)
    );
    if (invalidJsonField) {
      Message.error(`字段 "${invalidJsonField.fieldName}" 的JSON格式不正确`);
      return;
    }

    try {
      saving.value = true;

      // 构建新的value对象
      const newValue: Record<string, any> = {};
      fieldRows.value.forEach((row) => {
        let valueToSave = row.value;

        // 序列化数值和布尔值为JSON字符串（后端会用json.Unmarshal解析）
        if (row.type === 'int' || row.type === 'float') {
          valueToSave = JSON.stringify(row.numValue || 0);
        } else if (row.type === 'bool') {
          valueToSave = JSON.stringify(row.boolValue || false);
        } else if (row.type === 'object' || row.type === 'array') {
          // 确保是有效的JSON字符串
          try {
            JSON.parse(valueToSave); // 验证
          } catch {
            valueToSave = row.type === 'object' ? '{}' : '[]';
          }
        }

        newValue[row.fieldName] = {
          value: valueToSave,
          type: row.type,
        };
      });

      // 调用更新接口
      await updateConfig(props.configItem.id, {
        value: newValue,
        metadata: props.configItem.metadata,
        description: props.configItem.description,
        changeNote: formData.changeNote,
      });

      Message.success('保存成功');
      formData.changeNote = '';
      emit('update');
    } catch (error: any) {
      Message.error(`保存失败: ${error.message || '未知错误'}`);
    } finally {
      saving.value = false;
    }
  }

  // 重置
  function handleReset() {
    loadData();
  }

  // 查看最终JSON
  async function handleViewFinalJSON() {
    if (!props.configItem?.id) {
      Message.error('配置项不存在');
      return;
    }

    try {
      loadingJSON.value = true;
      jsonViewerVisible.value = true;
      
      const response = await getConfigJSON(props.configItem.id);
      finalJSON.value = JSON.stringify(response.data, null, 2);
    } catch (error: any) {
      Message.error(`获取配置JSON失败: ${error.message || '未知错误'}`);
      jsonViewerVisible.value = false;
    } finally {
      loadingJSON.value = false;
    }
  }

  // 复制JSON
  async function handleCopyJSON() {
    try {
      await navigator.clipboard.writeText(finalJSON.value);
      Message.success('已复制到剪贴板');
    } catch (error) {
      Message.error('复制失败，请手动复制');
    }
  }
</script>

<style scoped lang="less">
  .config-value-editor {
    padding: 16px;
    max-width: 100%;
    overflow: hidden;
  }

  .empty-fields {
    padding: 32px;
    text-align: center;
  }

  .fields-table {
    border: 1px solid var(--color-border-2);
    border-radius: 4px;
    overflow-x: auto;
    overflow-y: visible;
  }

  .table-header,
  .table-row {
    display: grid;
    grid-template-columns: 160px 140px minmax(300px, 1fr) 60px;
    gap: 12px;
    align-items: center;
    padding: 12px;
    min-width: 660px;
  }

  .table-header {
    background-color: var(--color-fill-2);
    font-weight: 600;
    border-bottom: 1px solid var(--color-border-2);
  }

  .table-row {
    border-bottom: 1px solid var(--color-border-1);

    &:last-child {
      border-bottom: none;
    }

    &:hover {
      background-color: var(--color-fill-1);
    }
  }

  .col-field-name,
  .col-type,
  .col-value,
  .col-actions {
    display: flex;
    align-items: center;
    min-width: 0;
  }

  .col-value {
    width: 100%;

    :deep(.arco-input),
    :deep(.arco-input-number),
    :deep(.arco-input-password),
    :deep(.arco-textarea) {
      width: 100%;
    }
  }

  .json-field {
    width: 100%;

    .json-actions {
      margin-top: 8px;
      display: flex;
      align-items: center;
      gap: 8px;
    }
  }

  .add-field-section {
    margin-top: 16px;
  }

  .json-viewer {
    position: relative;

    .json-actions {
      position: absolute;
      top: 8px;
      right: 8px;
      z-index: 1;
    }

    .json-content {
      background-color: var(--color-fill-2);
      border: 1px solid var(--color-border-2);
      border-radius: 4px;
      padding: 16px;
      max-height: 600px;
      overflow: auto;
      font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
      font-size: 13px;
      line-height: 1.6;
      margin: 0;
      white-space: pre-wrap;
      word-wrap: break-word;
    }
  }
</style>
