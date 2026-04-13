import { VIP_CONTACT_LABEL, VIP_CONTACT_VALUE } from '../../constants/vip';
import { getUserEntitlements, redeemVIPCode } from '../../services/vip';
import { showError, showLoading, hideLoading, showSuccess } from '../../utils/request';

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

Page({
  data: {
    loading: true,
    entitlements: null as any,
    statusTitle: '普通用户',
    expireText: '',
    redeemCode: '',
    contactLabel: VIP_CONTACT_LABEL,
    contactValue: VIP_CONTACT_VALUE
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
        expireText: formatExpireText(entitlements)
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
        showSuccess('联系方式已复制');
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
        expireText: formatExpireText(entitlements)
      });
      showSuccess('兑换成功');
    } catch (err: any) {
      showError(err.message || '兑换失败');
    } finally {
      hideLoading();
    }
  }
});
