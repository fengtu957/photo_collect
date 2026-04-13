// app.ts
import { login } from './services/auth';
import { UserEntitlements } from './types/vip';

interface IAppGlobalData {
  userInfo?: WechatMiniprogram.UserInfo;
  customFields: any[];
  entitlements?: UserEntitlements;
}

interface ICustomAppOption {
  globalData: IAppGlobalData;
  autoLogin(): void;
}

App<ICustomAppOption>({
  globalData: { customFields: [] },
  onLaunch() {
    this.autoLogin();
  },
  autoLogin() {
    login().then(() => {
      console.log('登录成功');
    }).catch((err) => {
      console.error('登录失败:', err);
    });
  }
})
