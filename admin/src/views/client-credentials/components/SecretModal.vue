<template>
  <a-modal
    v-model:visible="modalVisible"
    :title="$t('clientCredentials.secret.title')"
    :footer="false"
    :width="600"
    @cancel="handleClose"
  >
    <a-alert type="warning" style="margin-bottom: 16px">
      {{ $t('clientCredentials.secret.warning') }}
    </a-alert>
    
    <a-descriptions :column="1" bordered>
      <a-descriptions-item :label="$t('clientCredentials.secret.name')">
        {{ secretData.name }}
      </a-descriptions-item>
      <a-descriptions-item :label="$t('clientCredentials.secret.clientKey')">
        <a-space>
          <a-typography-text copyable>
            {{ secretData.clientKey }}
          </a-typography-text>
        </a-space>
      </a-descriptions-item>
      <a-descriptions-item :label="$t('clientCredentials.secret.clientSecret')">
        <a-space>
          <a-typography-text code copyable style="word-break: break-all">
            {{ secretData.clientSecret }}
          </a-typography-text>
        </a-space>
      </a-descriptions-item>
      <a-descriptions-item
        v-if="secretData.description"
        :label="$t('clientCredentials.secret.description')"
      >
        {{ secretData.description }}
      </a-descriptions-item>
    </a-descriptions>

    <div style="margin-top: 16px; text-align: right">
      <a-button type="primary" @click="handleClose">
        {{ $t('clientCredentials.secret.close') }}
      </a-button>
    </div>
  </a-modal>
</template>

<script lang="ts" setup>
import { ref, computed } from 'vue';

interface SecretData {
  name: string;
  clientKey: string;
  clientSecret: string;
  description?: string;
}

const props = defineProps<{
  visible: boolean;
  secretData: SecretData;
}>();

const emit = defineEmits<{
  (e: 'update:visible', visible: boolean): void;
}>();

const modalVisible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value),
});

const handleClose = () => {
  modalVisible.value = false;
};
</script>

<style scoped lang="less">
:deep(.arco-descriptions-item-label) {
  font-weight: 600;
}
</style>






