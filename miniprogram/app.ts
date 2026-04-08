// app.ts
import { login } from './services/auth';

App<IAppOption>({
  globalData: {},
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