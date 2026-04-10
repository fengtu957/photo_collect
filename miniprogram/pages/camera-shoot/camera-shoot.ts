import { showError } from '../../utils/request';

Page({
  data: {
    statusBarHeight: 20,
    devicePosition: 'front',
    tempImagePath: ''
  },

  onLoad() {
    const systemInfo = wx.getSystemInfoSync();
    this.setData({
      statusBarHeight: systemInfo.statusBarHeight || 20
    });
  },

  goBack() {
    wx.navigateBack();
  },

  onCameraError() {
    showError('相机打开失败，请检查权限');
  },

  switchCamera() {
    this.setData({
      devicePosition: this.data.devicePosition === 'front' ? 'back' : 'front'
    });
  },

  chooseFromAlbum() {
    wx.chooseMedia({
      count: 1,
      mediaType: ['image'],
      sizeType: ['original'],
      sourceType: ['album'],
      success: (res) => {
        this.setData({
          tempImagePath: res.tempFiles[0].tempFilePath
        });
      }
    });
  },

  takePhoto() {
    const cameraContext = wx.createCameraContext();
    cameraContext.takePhoto({
      quality: 'high',
      success: (res) => {
        this.setData({
          tempImagePath: res.tempImagePath
        });
      },
      fail: () => {
        showError('拍照失败，请重试');
      }
    });
  },

  retakePhoto() {
    this.setData({
      tempImagePath: ''
    });
  },

  usePhoto() {
    if (!this.data.tempImagePath) {
      showError('请先拍摄照片');
      return;
    }

    const eventChannel = this.getOpenerEventChannel();
    eventChannel.emit('photoSelected', {
      tempFilePath: this.data.tempImagePath
    });
    wx.navigateBack();
  }
});
