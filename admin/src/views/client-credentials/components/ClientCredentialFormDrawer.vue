<template>
  <a-drawer
    v-model:visible="drawerVisible"
    :title="isEdit ? $t('clientCredentials.form.edit.title') : $t('clientCredentials.form.create.title')"
    :width="500"
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
        field="name"
        :label="$t('clientCredentials.form.name')"
      >
        <a-input
          v-model="formData.name"
          :placeholder="$t('clientCredentials.form.name.placeholder')"
          allow-clear
        />
      </a-form-item>

      <a-form-item
        field="description"
        :label="$t('clientCredentials.form.description')"
      >
        <a-textarea
          v-model="formData.description"
          :placeholder="$t('clientCredentials.form.description.placeholder')"
          :auto-size="{ minRows: 2, maxRows: 4 }"
          allow-clear
        />
      </a-form-item>

      <a-form-item
        field="ipWhitelist"
        :label="$t('clientCredentials.form.ipWhitelist')"
      >
        <a-input-tag
          v-model="formData.ipWhitelist"
          :placeholder="$t('clientCredentials.form.ipWhitelist.placeholder')"
          allow-clear
        />
      </a-form-item>

      <a-form-item
        field="expiresAt"
        :label="$t('clientCredentials.form.expiresAt')"
      >
        <a-date-picker
          v-model="formData.expiresAt"
          :placeholder="$t('clientCredentials.form.expiresAt.placeholder')"
          show-time
          format="YYYY-MM-DD HH:mm:ss"
          style="width: 100%"
          allow-clear
        />
      </a-form-item>
    </a-form>

    <template #footer>
      <a-space>
        <a-button @click="handleCancel">
          {{ $t('clientCredentials.form.cancel') }}
        </a-button>
        <a-button type="primary" :loading="submitting" @click="handleSubmit">
          {{ $t('clientCredentials.form.submit') }}
        </a-button>
      </a-space>
    </template>
  </a-drawer>
</template>

<script lang="ts" setup>
import { ref, computed, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import type { FormInstance } from '@arco-design/web-vue';
import { ClientCredentialDTO } from '@/api/client-credentials';

interface FormData {
  name: string;
  description: string;
  ipWhitelist: string[];
  expiresAt: string | undefined;
}

const props = defineProps<{
  visible: boolean;
  editData?: ClientCredentialDTO | null;
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
  name: '',
  description: '',
  ipWhitelist: [],
  expiresAt: undefined,
});

const formRules = {
  name: [
    {
      required: true,
      message: t('clientCredentials.validation.name.required'),
    },
    {
      minLength: 3,
      message: t('clientCredentials.validation.name.minLength'),
    },
    {
      maxLength: 200,
      message: t('clientCredentials.validation.name.maxLength'),
    },
  ],
};

const resetForm = () => {
  formData.value = {
    name: '',
    description: '',
    ipWhitelist: [],
    expiresAt: undefined,
  };
  formRef.value?.clearValidate();
};

// 监听编辑数据变化，填充表单
watch(
  () => props.editData,
  (data) => {
    if (data) {
      formData.value = {
        name: data.name,
        description: data.description || '',
        ipWhitelist: data.ipWhitelist || [],
        expiresAt: data.expiresAt || undefined,
      };
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
</script>

<style scoped lang="less">
:deep(.arco-form-item) {
  margin-bottom: 20px;
}
</style>

