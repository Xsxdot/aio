import { useRouter } from 'vue-router';
import { Message } from '@arco-design/web-vue';

export default function useUser() {
  const router = useRouter();
  const logout = async (logoutTo?: string) => {
    const currentRoute = router.currentRoute.value;
    Message.success('登出成功');
    router.push({
      name: logoutTo && typeof logoutTo === 'string' ? logoutTo : 'login',
      query: {
        ...router.currentRoute.value.query,
        redirect: currentRoute.name as string,
      },
    });
  };
  return {
    logout,
  };
}
