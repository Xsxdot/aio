<template>
  <a-drawer
    :width="600"
    :visible="visible"
    :title="isEdit ? $t('shorturl.domains.form.title.edit') : $t('shorturl.domains.form.title.create')"
    :mask-closable="false"
    unmount-on-close
    @cancel="handleCancel"
  >
    <a-form
      ref="formRef"
      :model="formData"
      :rules="rules"
      layout="vertical"
      @submit="handleSubmit"
    >
      <a-form-item field="domain" :label="$t('shorturl.domains.form.domain')">
        <a-input
          v-model="formData.domain"
          :placeholder="$t('shorturl.domains.form.domain.placeholder')"
          allow-clear
        />
      </a-form-item>

      <a-form-item field="isDefault" :label="$t('shorturl.domains.form.isDefault')">
        <a-switch v-model="formData.isDefault" />
      </a-form-item>

      <a-form-item field="comment" :label="$t('shorturl.domains.form.comment')">
        <a-textarea
          v-model="formData.comment"
          :placeholder="$t('shorturl.domains.form.comment.placeholder')"
          :max-length="500"
          show-word-limit
          allow-clear
        />
      </a-form-item>
    </a-form>

    <template #footer>
      <a-space>
        <a-button @click="handleCancel">
          {{ $t('shorturl.domains.form.cancel') }}
        </a-button>
        <a-button type="primary" :loading="submitting" @click="handleSubmit">
          {{ $t('shorturl.domains.form.submit') }}
        </a-button>
      </a-space>
    </template>
  </a-drawer>
</template>

<script lang="ts" setup>
import { ref, reactive, watch, computed } from 'vue';
import { useI18n } from 'vue-i18n';
import type { FormInstance } from '@arco-design/web-vue';
import type { ShortDomainDTO, CreateShortDomainRequest, UpdateShortDomainRequest } from '@/api/shorturl';

const { t } = useI18n();

interface Props {
  visible: boolean;
  domain?: ShortDomainDTO | null;
}

const props = withDefaults(defineProps<Props>(), {
  visible: false,
  domain: null,
});

const emit = defineEmits<{
  (e: 'update:visible', visible: boolean): void;
  (e: 'submit', data: CreateShortDomainRequest | UpdateShortDomainRequest): void;
}>();

const formRef = ref<FormInstance>();
const submitting = ref(false);

const isEdit = computed(() => !!props.domain);

const formData = reactive<{
  domain: string;
  isDefault: boolean;
  comment: string;
}>({
  domain: '',
  isDefault: false,
  comment: '',
});

const rules = {
  domain: [
    {
      required: true,
      message: t('shorturl.domains.form.domain.required'),
    },
  ],
};

// 监听 domain 变化，用于编辑模式
watch(
  () => props.domain,
  (domain) => {
    if (domain) {
      formData.domain = domain.domain;
      formData.isDefault = domain.isDefault;
      formData.comment = domain.comment || '';
    }
  },
  { immediate: true }
);

// 重置表单
const resetForm = () => {
  formData.domain = '';
  formData.isDefault = false;
  formData.comment = '';
  formRef.value?.clearValidate();
};

// 取消
const handleCancel = () => {
  resetForm();
  emit('update:visible', false);
};

// 提交
const handleSubmit = async () => {
  const errors = await formRef.value?.validate();
  if (errors) {
    return;
  }

  submitting.value = true;
  try {
    if (isEdit.value) {
      // 编辑模式：只提交变更的字段
      const updateData: UpdateShortDomainRequest = {};
      if (formData.domain !== props.domain!.domain) {
        updateData.domain = formData.domain;
      }
      if (formData.isDefault !== props.domain!.isDefault) {
        updateData.isDefault = formData.isDefault;
      }
      if (formData.comment !== (props.domain!.comment || '')) {
        updateData.comment = formData.comment || undefined;
      }
      emit('submit', updateData);
    } else {
      // 新增模式
      const createData: CreateShortDomainRequest = {
        domain: formData.domain,
        isDefault: formData.isDefault,
        comment: formData.comment || undefined,
      };
      emit('submit', createData);
    }
    resetForm();
    emit('update:visible', false);
  } finally {
    submitting.value = false;
  }
};
</script>

<script lang="ts">
export default {
  name: 'DomainFormDrawer',
};
</script>
