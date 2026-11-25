import { CLOUD_ENV, useCloud } from './config';
import { ensureLogin } from './utils/auth';

App({
  globalData: {
    userId: 0,
    token: '',
    user: null as any,
  },

  onLaunch() {
    // 云托管原生调用需先初始化 wx.cloud。
    if (useCloud() && wx.cloud && wx.cloud.init) {
      wx.cloud.init({ env: CLOUD_ENV, traceUser: true });
    }

    ensureLogin();
  },
});
