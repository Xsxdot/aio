<template>
  <a-drawer
    v-model:visible="drawerVisible"
    :title="isEdit ? $t('registry.services.form.edit.title') : $t('registry.services.form.create.title')"
    :width="600"
    @cancel="handleCancel"
    @before-ok="handleSubmit"
  >
    <a-form
      ref="formRef"
      :model="formData"
      :rules="formRules"
      layout="vertical"
    >
      <a-form-item
        field="project"
        :label="$t('registry.services.form.project')"
      >
        <a-input
          v-model="formData.project"
          :placeholder="$t('registry.services.form.project.placeholder')"
          allow-clear
        />
      </a-form-item>

      <a-form-item
        field="name"
        :label="$t('registry.services.form.name')"
      >
        <a-input
          v-model="formData.name"
          :placeholder="$t('registry.services.form.name.placeholder')"
          allow-clear
        />
      </a-form-item>

      <a-form-item
        field="owner"
        :label="$t('registry.services.form.owner')"
      >
        <a-input
          v-model="formData.owner"
          :placeholder="$t('registry.services.form.owner.placeholder')"
          allow-clear
        />
      </a-form-item>

      <a-form-item
        field="description"
        :label="$t('registry.services.form.description')"
      >
        <a-textarea
          v-model="formData.description"
          :placeholder="$t('registry.services.form.description.placeholder')"
          :auto-size="{ minRows: 2, maxRows: 4 }"
          allow-clear
        />
      </a-form-item>

      <a-form-item
        field="spec"
        :label="$t('registry.services.form.spec')"
      >
        <a-space direction="vertical" style="width: 100%">
          <a-button
            type="outline"
            long
            @click="openSpecEditor"
          >
            <template #icon>
              <icon-edit />
            </template>
            {{ $t('registry.services.form.spec.edit') }}
          </a-button>
          <a-textarea
            v-model="specJsonDisplay"
            :placeholder="$t('registry.services.form.spec.placeholder')"
            :auto-size="{ minRows: 3, maxRows: 8 }"
            readonly
            style="font-family: 'Menlo', 'Monaco', 'Courier New', monospace; font-size: 12px;"
          />
        </a-space>
      </a-form-item>
    </a-form>

    <template #footer>
      <a-space>
        <a-button @click="handleCancel">
          {{ $t('registry.services.form.cancel') }}
        </a-button>
        <a-button type="primary" :loading="submitting" @click="handleSubmit">
          {{ $t('registry.services.form.submit') }}
        </a-button>
      </a-space>
    </template>
  </a-drawer>

  <!-- Spec 编辑弹窗 -->
  <JsonTreeEditorModal
    v-model:visible="specEditorVisible"
    :title="$t('registry.services.form.spec')"
    :value="specJson"
    mode="object"
    :width="900"
    @apply="handleSpecApply"
  />
</template>

<script lang="ts" setup>
import { ref, computed, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import type { FormInstance } from '@arco-design/web-vue';
import { ServiceDTO } from '@/api/registry-services';
import JsonTreeEditorModal from '@/components/json-editor/JsonTreeEditorModal.vue';

interface FormData {
  project: string;
  name: string;
  owner: string;
  description: string;
  spec: Record<string, any>;
}

const props = defineProps<{
  visible: boolean;
  editData?: ServiceDTO | null;
}>();

const emit = defineEmits<{
  (e: 'update:visible', visible: boolean): void;
  (e: 'submit', data: FormData): Promise<void>;
}>();

const { t } = useI18n();
const formRef = ref<FormInstance>();
const submitting = ref(false);

const isEdit = computed(() => !!props.editData);

const drawerVisible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value),
});

const formData = ref<FormData>({
  project: '',
  name: '',
  owner: '',
  description: '',
  spec: {},
});

// Spec JSON 字符串（用于编辑器）
const specJson = ref('{}');
const specEditorVisible = ref(false);

// Spec JSON 显示（格式化）
const specJsonDisplay = computed(() => {
  try {
    return JSON.stringify(formData.value.spec, null, 2);
  } catch (e) {
    return '{}';
  }
});

const formRules = {
  project: [
    {
      required: true,
      message: t('registry.services.validation.project.required'),
    },
  ],
  name: [
    {
      required: true,
      message: t('registry.services.validation.name.required'),
    },
  ],
};

const resetForm = () => {
  formData.value = {
    project: '',
    name: '',
    owner: '',
    description: '',
    spec: {},
  };
  specJson.value = '{}';
  formRef.value?.clearValidate();
};

// 监听编辑数据变化，填充表单
watch(
  () => props.editData,
  (data) => {
    if (data) {
      formData.value = {
        project: data.project,
        name: data.name,
        owner: data.owner || '',
        description: data.description || '',
        spec: data.spec || {},
      };
      // 同步 spec 到 JSON 字符串
      try {
        specJson.value = JSON.stringify(data.spec || {}, null, 2);
      } catch (e) {
        specJson.value = '{}';
      }
    } else {
      resetForm();
    }
  },
  { immediate: true }
);

const handleCancel = () => {
  drawerVisible.value = false;
  resetForm();
};

const handleSubmit = async () => {
  const valid = await formRef.value?.validate();
  if (!valid) {
    try {
      submitting.value = true;
      await emit('submit', formData.value);
      drawerVisible.value = false;
      resetForm();
    } finally {
      submitting.value = false;
    }
  }
};

// 打开 Spec 编辑器
const openSpecEditor = () => {
  // 同步当前 spec 到编辑器
  try {
    specJson.value = JSON.stringify(formData.value.spec, null, 2);
  } catch (e) {
    specJson.value = '{}';
  }
  specEditorVisible.value = true;
};

// Spec 编辑器应用
const handleSpecApply = (data: { value: string }) => {
  try {
    const parsed = JSON.parse(data.value || '{}');
    formData.value.spec = parsed;
    specJson.value = data.value;
  } catch (e) {
    console.error('Invalid JSON:', e);
    // 保持原值不变
  }
};
</script>

<style scoped lang="less">
:deep(.arco-form-item) {
  margin-bottom: 20px;
}
</style>

