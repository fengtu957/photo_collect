import { getUserEntitlements, redeemVIPCode } from '../../services/vip';
import { showError, showLoading, hideLoading } from '../../utils/request';

function formatExpireText(entitlements: any): string {
  if (!entitlements) return '';
  if (entitlements.is_vip && entitlements.expire_at) {
    return `有效期至 ${String(entitlements.expire_at).replace('T', ' ').slice(0, 16)}`;
  }
  if (!entitlements.is_vip && entitlements.expire_at) {
    return `已于 ${String(entitlements.expire_at).replace('T', ' ').slice(0, 16)} 到期`;
  }
  return '开通后立即解锁任务数、收集人数和 AI 分析权限';
}

function showRedeemResultModal(title: string, content: string) {
  wx.showModal({
    title,
    content,
    showCancel: false,
    confirmText: '我知道了'
  });
}

Page({
  data: {
    loading: true,
    entitlements: null as any,
    statusTitle: '普通用户',
    expireText: '',
    redeemCode: '',
    contactLabel: '',
    contactValue: ''
  },

  onLoad() {
    this.loadEntitlements();
  },

  onShow() {
    this.loadEntitlements();
  },

  async loadEntitlements() {
    try {
      const entitlements = await getUserEntitlements();
      const appInstance = getApp<any>();
      appInstance.globalData.entitlements = entitlements;
      this.setData({
        loading: false,
        entitlements,
        statusTitle: entitlements.is_vip ? 'VIP会员' : '普通用户',
        expireText: formatExpireText(entitlements),
        contactLabel: entitlements.contact_label || '微信',
        contactValue: entitlements.contact_value || '请联系管理员获取开通方式'
      });
    } catch (err: any) {
      this.setData({ loading: false });
      showError(err.message || '加载会员信息失败');
    }
  },

  onRedeemInput(e: any) {
    this.setData({ redeemCode: e.detail.value });
  },

  copyContact() {
    wx.setClipboardData({
      data: this.data.contactValue,
      success: () => {
        wx.showToast({ title: '联系方式已复制', icon: 'success' });
      }
    });
  },

  async redeemCode() {
    const code = String(this.data.redeemCode || '').trim();
    if (!code) {
      showError('请输入激活码');
      return;
    }

    showLoading('兑换中...');
    try {
      const entitlements = await redeemVIPCode(code);
      const appInstance = getApp<any>();
      appInstance.globalData.entitlements = entitlements;
      this.setData({
        entitlements,
        redeemCode: '',
        statusTitle: entitlements.is_vip ? 'VIP会员' : '普通用户',
        expireText: formatExpireText(entitlements),
        contactLabel: entitlements.contact_label || '微信',
        contactValue: entitlements.contact_value || '请联系管理员获取开通方式'
      });
      showRedeemResultModal('兑换成功', 'VIP 已生效，可立即使用对应权益。');
    } catch (err: any) {
      showRedeemResultModal('兑换失败', err.message || '兑换失败');
    } finally {
      hideLoading();
    }
  }
});
