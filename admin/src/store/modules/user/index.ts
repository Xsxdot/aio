import { defineStore } from 'pinia';
import {
  login as userLogin,
  getUserInfo,
  LoginData,
  AdminInfo,
} from '@/api/user';
import { setToken, clearToken } from '@/utils/auth';
import { removeRouteListener } from '@/utils/route-listener';
import { UserState, RoleType } from './types';
import useAppStore from '../app';

const useUserStore = defineStore('user', {
  state: (): UserState => ({
    name: undefined,
    avatar: undefined,
    job: undefined,
    organization: undefined,
    location: undefined,
    email: undefined,
    introduction: undefined,
    personalWebsite: undefined,
    jobName: undefined,
    organizationName: undefined,
    locationName: undefined,
    phone: undefined,
    registrationDate: undefined,
    accountId: undefined,
    certification: undefined,
    role: '',
  }),

  getters: {
    userInfo(state: UserState): UserState {
      return { ...state };
    },
  },

  actions: {
    switchRoles() {
      return new Promise((resolve) => {
        this.role = this.role === 'user' ? 'admin' : 'user';
        resolve(this.role);
      });
    },
    // Set user's information
    setInfo(partial: Partial<UserState>) {
      this.$patch(partial);
    },

    // Reset user's information
    resetInfo() {
      this.$reset();
    },

    // 将后端管理员信息映射到前端 UserState
    mapAdminToUserState(adminInfo: AdminInfo): Partial<UserState> {
      // 角色映射：isSuper=true -> admin，否则 user
      const role: RoleType = adminInfo.isSuper ? 'admin' : 'user';

      return {
        name: adminInfo.account,
        accountId: adminInfo.account,
        role,
      };
    },

    // Get user's information
    async info() {
      const res = await getUserInfo();
      const userState = this.mapAdminToUserState(res.data);
      this.setInfo(userState);
    },

    // Login
    async login(loginForm: LoginData) {
      try {
        const res = await userLogin(loginForm);
        setToken(res.data.accessToken);
        // 登录成功后用返回的 admin 信息填充 store
        const userState = this.mapAdminToUserState(res.data.admin);
        this.setInfo(userState);
      } catch (err) {
        clearToken();
        throw err;
      }
    },
    logoutCallBack() {
      const appStore = useAppStore();
      this.resetInfo();
      clearToken();
      removeRouteListener();
      appStore.clearServerMenu();
    },
    // Logout
    async logout() {
      this.logoutCallBack();
    },
  },
});

export default useUserStore;
