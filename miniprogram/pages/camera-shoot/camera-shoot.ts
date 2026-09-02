import { showError } from '../../utils/request';

const CROP_ASPECT_WIDTH = 5;
const CROP_ASPECT_HEIGHT = 7;
const CROPPED_OUTPUT_WIDTH = 590;
const CROPPED_OUTPUT_HEIGHT = 826;
const MIN_PREVIEW_SCALE = 1;
const MAX_PREVIEW_SCALE = 4;

function clamp(value: number, min: number, max: number) {
  return Math.max(min, Math.min(max, value));
}

function getTouchPoint(touch: any) {
  return {
    x: Number(touch.clientX || touch.pageX || 0),
    y: Number(touch.clientY || touch.pageY || 0)
  };
}

function getTouchDistance(firstTouch: any, secondTouch: any) {
  const first = getTouchPoint(firstTouch);
  const second = getTouchPoint(secondTouch);
  const deltaX = second.x - first.x;
  const deltaY = second.y - first.y;
  return Math.sqrt(deltaX * deltaX + deltaY * deltaY);
}

function getTouchCenter(firstTouch: any, secondTouch: any) {
  const first = getTouchPoint(firstTouch);
  const second = getTouchPoint(secondTouch);
  return {
    x: (first.x + second.x) / 2,
    y: (first.y + second.y) / 2
  };
}

function getPreviewRect(imageWidth: number, imageHeight: number, containerWidth: number, containerHeight: number) {
  const imageRatio = imageWidth / imageHeight;
  const containerRatio = containerWidth / containerHeight;

  let drawWidth = containerWidth;
  let drawHeight = containerHeight;
  let offsetX = 0;
  let offsetY = 0;

  if (imageRatio > containerRatio) {
    drawHeight = containerHeight;
    drawWidth = containerHeight * imageRatio;
    offsetX = (containerWidth - drawWidth) / 2;
  } else {
    drawWidth = containerWidth;
    drawHeight = containerWidth / imageRatio;
    offsetY = (containerHeight - drawHeight) / 2;
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
    entrySource: 'camera' as 'camera' | 'album',
    disallowAlbumPhotos: false,
    tempImagePath: '',
    screenWidth: 375,
    screenHeight: 667,
    cropBoxLeft: 0,
    cropBoxTop: 180,
    cropBoxWidth: 375,
    cropBoxHeight: 525,
    previewImageWidth: 375,
    previewImageHeight: 667,
    previewImageOffsetX: 0,
    previewImageOffsetY: 0,
    previewImageScale: 1
  },

  onLoad(options: any) {
    const systemInfo = wx.getSystemInfoSync();
    const screenWidth = systemInfo.windowWidth || 375;
    const screenHeight = systemInfo.windowHeight || 667;
    const cropBoxWidth = screenWidth;
    const cropBoxHeight = cropBoxWidth * CROP_ASPECT_HEIGHT / CROP_ASPECT_WIDTH;
    const cropBoxLeft = 0;
    const cropBoxTop = (screenHeight - cropBoxHeight) / 2 - screenHeight * 0.03;
    const disallowAlbumPhotos = !!(options && options.disallowAlbumPhotos === '1');
    const requestedAlbum = options && options.source === 'album';
    const entrySource = requestedAlbum && !disallowAlbumPhotos ? 'album' : 'camera';

    this.setData({
      statusBarHeight: systemInfo.statusBarHeight || 20,
      entrySource,
      disallowAlbumPhotos,
      screenWidth,
      screenHeight,
      cropBoxLeft,
      cropBoxTop,
      cropBoxWidth,
      cropBoxHeight
    }, () => {
      if (requestedAlbum && disallowAlbumPhotos) {
        showError('当前任务不允许使用相册照片，请直接拍照');
      } else if (entrySource === 'album') {
        this.chooseFromAlbum();
      }
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

  setPreviewImage(filePath: string) {
    wx.getImageInfo({
      src: filePath,
      success: (imageInfo) => {
        const previewRect = getPreviewRect(
          imageInfo.width,
          imageInfo.height,
          this.data.cropBoxWidth,
          this.data.cropBoxHeight
        );
        (this as any).cropGesture = null;
        this.setData({
          tempImagePath: filePath,
          previewImageWidth: previewRect.drawWidth,
          previewImageHeight: previewRect.drawHeight,
          previewImageOffsetX: this.data.cropBoxLeft + previewRect.offsetX,
          previewImageOffsetY: this.data.cropBoxTop + previewRect.offsetY,
          previewImageScale: MIN_PREVIEW_SCALE
        });
      },
      fail: () => {
        showError('\u8bfb\u53d6\u7167\u7247\u5931\u8d25');
      }
    });
  },

  clampPreviewOffset(offsetX: number, offsetY: number, scale: number) {
    const scaledWidth = this.data.previewImageWidth * scale;
    const scaledHeight = this.data.previewImageHeight * scale;
    const cropRight = this.data.cropBoxLeft + this.data.cropBoxWidth;
    const cropBottom = this.data.cropBoxTop + this.data.cropBoxHeight;
    const minOffsetX = cropRight - scaledWidth;
    const maxOffsetX = this.data.cropBoxLeft;
    const minOffsetY = cropBottom - scaledHeight;
    const maxOffsetY = this.data.cropBoxTop;

    return {
      offsetX: clamp(offsetX, minOffsetX, maxOffsetX),
      offsetY: clamp(offsetY, minOffsetY, maxOffsetY)
    };
  },

  onCropTouchStart(e: any) {
    const touches = (e && e.touches) || [];
    if (touches.length === 0) {
      return;
    }

    const gesture: any = {
      touchCount: touches.length >= 2 ? 2 : 1,
      startOffsetX: this.data.previewImageOffsetX,
      startOffsetY: this.data.previewImageOffsetY,
      startScale: this.data.previewImageScale
    };

    if (touches.length >= 2) {
      const center = getTouchCenter(touches[0], touches[1]);
      gesture.startDistance = getTouchDistance(touches[0], touches[1]);
      gesture.startCenterX = center.x;
      gesture.startCenterY = center.y;
    } else {
      const point = getTouchPoint(touches[0]);
      gesture.startX = point.x;
      gesture.startY = point.y;
    }

    (this as any).cropGesture = gesture;
  },

  onCropTouchMove(e: any) {
    const touches = (e && e.touches) || [];
    const gesture = (this as any).cropGesture;
    if (!gesture || touches.length === 0) {
      return;
    }

    if (touches.length >= 2 && gesture.touchCount === 2) {
      const distance = getTouchDistance(touches[0], touches[1]);
      if (gesture.startDistance <= 0) {
        return;
      }

      const center = getTouchCenter(touches[0], touches[1]);
      const nextScale = clamp(
        gesture.startScale * distance / gesture.startDistance,
        MIN_PREVIEW_SCALE,
        MAX_PREVIEW_SCALE
      );
      const imagePointX = (gesture.startCenterX - gesture.startOffsetX) / gesture.startScale;
      const imagePointY = (gesture.startCenterY - gesture.startOffsetY) / gesture.startScale;
      const nextOffsetX = center.x - imagePointX * nextScale;
      const nextOffsetY = center.y - imagePointY * nextScale;
      const clampedOffset = this.clampPreviewOffset(nextOffsetX, nextOffsetY, nextScale);

      this.setData({
        previewImageOffsetX: clampedOffset.offsetX,
        previewImageOffsetY: clampedOffset.offsetY,
        previewImageScale: nextScale
      });
      return;
    }

    if (touches.length === 1 && gesture.touchCount === 1) {
      const point = getTouchPoint(touches[0]);
      const nextOffsetX = gesture.startOffsetX + point.x - gesture.startX;
      const nextOffsetY = gesture.startOffsetY + point.y - gesture.startY;
      const clampedOffset = this.clampPreviewOffset(
        nextOffsetX,
        nextOffsetY,
        this.data.previewImageScale
      );

      this.setData({
        previewImageOffsetX: clampedOffset.offsetX,
        previewImageOffsetY: clampedOffset.offsetY
      });
    }
  },

  onCropTouchEnd(e: any) {
    const touches = (e && e.touches) || [];
    if (touches.length > 0) {
      this.onCropTouchStart({ touches });
      return;
    }
    (this as any).cropGesture = null;
  },

  chooseFromAlbum() {
    if (this.data.disallowAlbumPhotos) {
      showError('当前任务不允许使用相册照片，请直接拍照');
      return;
    }
    wx.chooseMedia({
      count: 1,
      mediaType: ['image'],
      sizeType: ['original'],
      sourceType: ['album'],
      success: (res) => {
        this.setPreviewImage(res.tempFiles[0].tempFilePath);
      },
      fail: (err) => {
        if (err && err.errMsg && err.errMsg.indexOf('cancel') >= 0) {
          if (this.data.entrySource === 'album' && !this.data.tempImagePath) {
            wx.navigateBack();
          }
          return;
        }
        showError('打开相册失败');
      }
    });
  },

  takePhoto() {
    const cameraContext = wx.createCameraContext();
    cameraContext.takePhoto({
      quality: 'high',
      success: (res) => {
        this.setPreviewImage(res.tempImagePath);
      },
      fail: () => {
        showError('拍照失败，请重试');
      }
    });
  },

  retakePhoto() {
    (this as any).cropGesture = null;
    this.setData({
      tempImagePath: ''
    }, () => {
      if (this.data.entrySource === 'album') {
        this.chooseFromAlbum();
      }
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
          const cropLeft = this.data.cropBoxLeft;
          const cropTop = this.data.cropBoxTop;
          const cropWidth = this.data.cropBoxWidth;
          const cropHeight = this.data.cropBoxHeight;
          const scale = this.data.previewImageScale || MIN_PREVIEW_SCALE;
          const renderedWidth = this.data.previewImageWidth * scale;
          const renderedHeight = this.data.previewImageHeight * scale;

          const relativeLeft = (cropLeft - this.data.previewImageOffsetX) / renderedWidth;
          const relativeTop = (cropTop - this.data.previewImageOffsetY) / renderedHeight;
          const relativeWidth = cropWidth / renderedWidth;
          const relativeHeight = cropHeight / renderedHeight;

          const sx = clamp(Math.round(relativeLeft * imageInfo.width), 0, imageInfo.width - 1);
          const sy = clamp(Math.round(relativeTop * imageInfo.height), 0, imageInfo.height - 1);
          const sWidth = Math.max(
            1,
            Math.min(imageInfo.width - sx, Math.round(relativeWidth * imageInfo.width))
          );
          const sHeight = Math.max(
            1,
            Math.min(imageInfo.height - sy, Math.round(relativeHeight * imageInfo.height))
          );

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
