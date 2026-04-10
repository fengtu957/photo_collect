import { showError } from '../../utils/request';

const CROP_ASPECT_WIDTH = 5;
const CROP_ASPECT_HEIGHT = 7;
const CROPPED_OUTPUT_WIDTH = 590;
const CROPPED_OUTPUT_HEIGHT = 826;

function getPreviewRect(imageWidth: number, imageHeight: number, containerWidth: number, containerHeight: number) {
  const imageRatio = imageWidth / imageHeight;
  const containerRatio = containerWidth / containerHeight;

  let drawWidth = containerWidth;
  let drawHeight = containerHeight;
  let offsetX = 0;
  let offsetY = 0;

  if (imageRatio > containerRatio) {
    drawWidth = containerWidth;
    drawHeight = containerWidth / imageRatio;
    offsetY = (containerHeight - drawHeight) / 2;
  } else {
    drawHeight = containerHeight;
    drawWidth = containerHeight * imageRatio;
    offsetX = (containerWidth - drawWidth) / 2;
  }

  return {
    offsetX,
    offsetY,
    drawWidth,
    drawHeight
  };
}

Page({
  data: {
    statusBarHeight: 20,
    devicePosition: 'front',
    tempImagePath: '',
    screenWidth: 375,
    screenHeight: 667,
    cropBoxLeft: 0,
    cropBoxTop: 180,
    cropBoxWidth: 375,
    cropBoxHeight: 525
  },

  onLoad() {
    const systemInfo = wx.getSystemInfoSync();
    const screenWidth = systemInfo.windowWidth || 375;
    const screenHeight = systemInfo.windowHeight || 667;
    const cropBoxWidth = screenWidth;
    const cropBoxHeight = cropBoxWidth * CROP_ASPECT_HEIGHT / CROP_ASPECT_WIDTH;
    const cropBoxLeft = 0;
    const cropBoxTop = (screenHeight - cropBoxHeight) / 2 - screenHeight * 0.03;

    this.setData({
      statusBarHeight: systemInfo.statusBarHeight || 20,
      screenWidth,
      screenHeight,
      cropBoxLeft,
      cropBoxTop,
      cropBoxWidth,
      cropBoxHeight
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

  async usePhoto() {
    if (!this.data.tempImagePath) {
      showError('请先拍摄照片');
      return;
    }

    wx.showLoading({
      title: '裁剪中...',
      mask: true
    });

    try {
      const croppedPath = await this.cropPhoto(this.data.tempImagePath);
      const eventChannel = this.getOpenerEventChannel();
      eventChannel.emit('photoSelected', {
        tempFilePath: croppedPath
      });
      wx.navigateBack();
    } catch (err: any) {
      showError((err && err.message) || '裁剪失败，请重试');
    } finally {
      wx.hideLoading();
    }
  },

  cropPhoto(filePath: string): Promise<string> {
    return new Promise((resolve, reject) => {
      wx.getImageInfo({
        src: filePath,
        success: (imageInfo) => {
          const containerWidth = this.data.screenWidth;
          const containerHeight = this.data.screenHeight;
          const cropLeft = this.data.cropBoxLeft;
          const cropTop = this.data.cropBoxTop;
          const cropWidth = this.data.cropBoxWidth;
          const cropHeight = this.data.cropBoxHeight;

          const previewRect = getPreviewRect(
            imageInfo.width,
            imageInfo.height,
            containerWidth,
            containerHeight
          );

          const relativeLeft = (cropLeft - previewRect.offsetX) / previewRect.drawWidth;
          const relativeTop = (cropTop - previewRect.offsetY) / previewRect.drawHeight;
          const relativeWidth = cropWidth / previewRect.drawWidth;
          const relativeHeight = cropHeight / previewRect.drawHeight;

          const sx = Math.max(0, Math.round(relativeLeft * imageInfo.width));
          const sy = Math.max(0, Math.round(relativeTop * imageInfo.height));
          const sWidth = Math.min(imageInfo.width - sx, Math.round(relativeWidth * imageInfo.width));
          const sHeight = Math.min(imageInfo.height - sy, Math.round(relativeHeight * imageInfo.height));

          const ctx = wx.createCanvasContext('cropCanvas', this);
          ctx.clearRect(0, 0, CROPPED_OUTPUT_WIDTH, CROPPED_OUTPUT_HEIGHT);
          ctx.drawImage(
            filePath,
            sx,
            sy,
            sWidth,
            sHeight,
            0,
            0,
            CROPPED_OUTPUT_WIDTH,
            CROPPED_OUTPUT_HEIGHT
          );
          ctx.draw(false, () => {
            wx.canvasToTempFilePath({
              canvasId: 'cropCanvas',
              x: 0,
              y: 0,
              width: CROPPED_OUTPUT_WIDTH,
              height: CROPPED_OUTPUT_HEIGHT,
              destWidth: CROPPED_OUTPUT_WIDTH,
              destHeight: CROPPED_OUTPUT_HEIGHT,
              fileType: 'jpg',
              quality: 1,
              success: (res) => resolve(res.tempFilePath),
              fail: (err) => reject(err)
            }, this);
          });
        },
        fail: (err) => reject(err)
      });
    });
  }
});
