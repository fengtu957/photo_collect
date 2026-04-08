// app.ts
import { login } from './services/auth';

App<IAppOption>({
  globalData: {},
  onLaunch() {
    this.autoLogin();
  },
  async autoLogin() {
    try {
      await login();
      console.log('登录成功');
    } catch (err) {
      console.error('登录失败:', err);
    }
  }
})