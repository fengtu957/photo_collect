import { createTaskInvitation, getTaskCollaborators, removeTaskMember, TaskInvitation } from '../../services/task';
import { showError } from '../../utils/request';

Page({
  data: {
    taskId: '',
    viewerRole: '' as 'creator' | 'admin' | '',
    ownerUserId: '',
    currentUserId: '',
    activeTab: 'collaborator',
    adminUserIds: [] as string[],
    collaboratorUserIds: [] as string[],
    canInviteAdmin: false,
    canInviteCollaborator: false,
    pageLoading: true,
    loadError: '',
    preparingRole: '',
    shareReadyRole: '',
    invitationTokens: {} as Record<string, TaskInvitation>
  },

  async onLoad(options: any) {
    const taskId = String((options && options.taskId) || '').trim();
    const currentUserId = String(wx.getStorageSync('openid') || '');
    this.setData({ taskId, currentUserId });
    if (!taskId) {
      this.setData({ pageLoading: false, loadError: '任务参数无效' });
      return;
    }
    try {
      const appInstance = getApp<any>();
      if (appInstance && typeof appInstance.autoLogin === 'function') await appInstance.autoLogin();
    } catch (err) {}
    this.setData({ currentUserId: String(wx.getStorageSync('openid') || '') });
    await this.loadData();
  },

  async loadData() {
    this.setData({ pageLoading: true, loadError: '' });
    try {
      const result = await getTaskCollaborators(this.data.taskId);
      const activeTab = result.viewer_role === 'creator' ? this.data.activeTab : 'collaborator';
      this.setData({
        viewerRole: result.viewer_role,
        activeTab,
        adminUserIds: result.admin_user_ids || [],
        collaboratorUserIds: result.collaborator_user_ids || [],
        canInviteAdmin: !!result.can_invite_admin,
        canInviteCollaborator: !!result.can_invite_collaborator,
        pageLoading: false
      });
    } catch (err: any) {
      this.setData({ pageLoading: false, loadError: (err && err.message) || '加载协作成员失败' });
    }
  },

  switchTab(e: any) {
    const tab = String(e.currentTarget.dataset.tab || 'collaborator');
    if (tab === 'admin' && this.data.viewerRole !== 'creator') return;
    this.setData({ activeTab: tab });
  },

  async prepareInvite(e: any) {
    const role = String(e.currentTarget.dataset.role || '') as 'admin' | 'collaborator';
    if (!role || this.data.preparingRole) return;
    this.setData({ preparingRole: role, shareReadyRole: '' });
    wx.showLoading({ title: '准备邀请...', mask: true });
    try {
      const invitation = await createTaskInvitation(this.data.taskId, role);
      this.setData({
        invitationTokens: { ...this.data.invitationTokens, [role]: invitation },
        shareReadyRole: role
      });
    } catch (err: any) {
      showError((err && err.message) || '邀请创建失败');
    } finally {
      wx.hideLoading();
      this.setData({ preparingRole: '' });
    }
  },

  removeMember(e: any) {
    const role = String(e.currentTarget.dataset.role || '') as 'admin' | 'collaborator';
    const userID = String(e.currentTarget.dataset.userId || '');
    if (!role || !userID) return;
    wx.showModal({
      title: '解除成员',
      content: '确定要解除该成员的协作权限吗？',
      success: async (res) => {
        if (!res.confirm) return;
        wx.showLoading({ title: '处理中...', mask: true });
        try {
          await removeTaskMember(this.data.taskId, role, userID);
          wx.showToast({ title: '已解除', icon: 'success' });
          await this.loadData();
        } catch (err: any) {
          showError((err && err.message) || '解除失败');
        } finally {
          wx.hideLoading();
        }
      }
    });
  },

  onShareAppMessage(options: any) {
    const role = String((options && options.target && options.target.dataset && options.target.dataset.role) || '');
    const invitation = role ? this.data.invitationTokens[role] : null;
    if (invitation && invitation.token) {
      this.setData({ shareReadyRole: '' });
      return {
        title: `邀请你成为任务的${invitation.role_text || (role === 'admin' ? '管理员' : '协作员')}`,
        path: `/pages/task-invite/task-invite?token=${encodeURIComponent(invitation.token)}`,
        imageUrl: '/imgs/invited_2.png'
      };
    }
    return { title: '邀请你参与照片采集', path: `/pages/task-detail/task-detail?id=${this.data.taskId}&fromShare=1` };
  }
});
