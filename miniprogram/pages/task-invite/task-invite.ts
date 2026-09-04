import { acceptTaskInvitation, getTaskInvitation, TaskInvitation } from '../../services/task';
import { showError } from '../../utils/request';

function normalizeInvitationToken(value: any): string {
  const token = String(value || '').trim();
  return /^[A-Za-z0-9_-]{20,128}$/.test(token) ? token : '';
}

Page({
  data: {
    token: '',
    invitation: null as TaskInvitation | null,
    pageLoading: true,
    accepting: false,
    loadError: ''
  },

  async onLoad(options: any) {
    const token = normalizeInvitationToken(options && options.token);
    this.setData({ token });
    if (!token) {
      this.setData({ pageLoading: false, loadError: '邀请不存在或已失效' });
      return;
    }

    try {
      const appInstance = getApp<any>();
      if (appInstance && typeof appInstance.autoLogin === 'function') {
        await appInstance.autoLogin();
      }
    } catch (err) {}

    await this.loadInvitation();
  },

  async loadInvitation() {
    this.setData({ pageLoading: true, loadError: '' });
    try {
      const invitation = await getTaskInvitation(this.data.token);
      this.setData({ invitation, pageLoading: false });
    } catch (err: any) {
      this.setData({
        pageLoading: false,
        loadError: (err && err.message) || '邀请不存在或已失效'
      });
    }
  },

  async acceptInvitation() {
    if (this.data.accepting || !this.data.invitation || !this.data.invitation.valid) {
      return;
    }

    this.setData({ accepting: true });
    wx.showLoading({ title: '确认中...', mask: true });
    try {
      const invitation = await acceptTaskInvitation(this.data.token);
      this.setData({ invitation });
      wx.showToast({ title: '领取成功', icon: 'success' });
    } catch (err: any) {
      showError((err && err.message) || '邀请领取失败');
      await this.loadInvitation();
    } finally {
      wx.hideLoading();
      this.setData({ accepting: false });
    }
  },

  enterTask() {
    const invitation = this.data.invitation;
    if (!invitation || !invitation.task_id) {
      return;
    }
    wx.redirectTo({
      url: `/pages/task-detail/task-detail?id=${invitation.task_id}&fromShare=1`
    });
  },

  goHome() {
    wx.reLaunch({ url: '/pages/task-list/task-list' });
  }
});
