// app.ts
import { login } from './services/auth';
import { UserEntitlements } from './types/entitlement';

interface IAppGlobalData {
  userInfo?: WechatMiniprogram.UserInfo;
  customFields: any[];
  entitlements?: UserEntitlements;
  entryOptions?: WechatMiniprogram.App.LaunchShowOption | null;
}

interface ICustomAppOption {
  globalData: IAppGlobalData;
  loginPromise?: Promise<any> | null;
  autoLogin(): Promise<any>;
  saveEntryOptions(options: WechatMiniprogram.App.LaunchShowOption): void;
}

App<ICustomAppOption>({
  globalData: {
    customFields: [],
    entryOptions: null
  },
  loginPromise: null,
  onLaunch(options) {
    this.saveEntryOptions(options);
    this.autoLogin();
  },
  onShow(options) {
    this.saveEntryOptions(options);
  },
  saveEntryOptions(options) {
    this.globalData.entryOptions = options || null;
  },
  autoLogin() {
    if (this.loginPromise) {
      return this.loginPromise;
    }

    this.loginPromise = login().then((res) => {
      console.log('登录成功');
      this.loginPromise = null;
      return res;
    }, (err) => {
      console.error('登录失败:', err);
      this.loginPromise = null;
      throw err;
    });

    return this.loginPromise;
  }
})
