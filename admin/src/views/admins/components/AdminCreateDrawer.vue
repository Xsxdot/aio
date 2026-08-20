<template>
  <a-drawer
    :visible="visible"
    :title="$t('admins.create.title')"
    :width="480"
    :footer="false"
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
      <a-form-item
        field="account"
        :label="$t('admins.form.account')"
      >
        <a-input
          v-model="formData.account"
          :placeholder="$t('admins.form.accountPlaceholder')"
          allow-clear
        />
      </a-form-item>

      <a-form-item
        field="password"
        :label="$t('admins.form.password')"
      >
        <a-input-password
          v-model="formData.password"
          :placeholder="$t('admins.form.passwordPlaceholder')"
          allow-clear
        />
      </a-form-item>

      <a-form-item
        field="remark"
        :label="$t('admins.form.remark')"
      >
        <a-textarea
          v-model="formData.remark"
          :placeholder="$t('admins.form.remarkPlaceholder')"
          :max-length="500"
          show-word-limit
          :auto-size="{ minRows: 3, maxRows: 5 }"
        />
      </a-form-item>

      <a-form-item>
        <a-space>
          <a-button type="primary" html-type="submit" :loading="loading">
            {{ $t('admins.create.submit') }}
          </a-button>
          <a-button @click="handleCancel">
            {{ $t('admins.create.cancel') }}
          </a-button>
        </a-space>
      </a-form-item>
    </a-form>
  </a-drawer>
</template>

<script lang="ts" setup>
import { ref, reactive, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import type { FormInstance } from '@arco-design/web-vue';

const { t } = useI18n();

interface Props {
  visible: boolean;
}

interface Emits {
  (e: 'update:visible', value: boolean): void;
  (e: 'submit', formData: any): void;
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const formRef = ref<FormInstance>();
const loading = ref(false);

const formData = reactive({
  account: '',
  password: '',
  remark: '',
});

const rules = {
  account: [
    { required: true, message: t('admins.form.accountRequired') },
    { min: 3, max: 50, message: t('admins.form.accountLength') },
  ],
  password: [
    { required: true, message: t('admins.form.passwordRequired') },
    { min: 6, message: t('admins.form.passwordLength') },
  ],
};

// 重置表单
const resetForm = () => {
  formData.account = '';
  formData.password = '';
  formData.remark = '';
  formRef.value?.clearValidate();
};

// 监听 visible 变化，打开时重置表单
watch(() => props.visible, (val) => {
  if (val) {
    resetForm();
  }
});

const handleCancel = () => {
  emit('update:visible', false);
};

const handleSubmit = async () => {
  try {
    const valid = await formRef.value?.validate();
    if (!valid) {
      loading.value = true;
      emit('submit', { ...formData });
      // 等待父组件处理完成后关闭
      setTimeout(() => {
        loading.value = false;
        emit('update:visible', false);
      }, 300);
    }
  } catch (error) {
    loading.value = false;
    console.error('Form validation failed:', error);
  }
};
</script>

<style scoped lang="less">
:deep(.arco-form-item) {
  margin-bottom: 24px;
}
</style>






