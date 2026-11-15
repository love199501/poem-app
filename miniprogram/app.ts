import { CLOUD_ENV, useCloud } from './config';
import { rawCall } from './utils/request';

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

    // 复用本地缓存的用户，否则走 wx.login 换取。
    const cached = wx.getStorageSync('user');
    if (cached && cached.id && cached.token) {
      this.globalData.userId = cached.id;
      this.globalData.token = cached.token;
      this.globalData.user = cached;
      return;
    }
    wx.login({
      success: (res: any) => {
        rawCall('/api/login', 'POST', { code: res.code || 'guest' }, {})
          .then((r) => {
            const user = r.data && r.data.data;
            if (user && user.id && user.token) {
              this.globalData.userId = user.id;
              this.globalData.token = user.token;
              this.globalData.user = user;
              wx.setStorageSync('user', user);
            }
          })
          .catch(() => {});
      },
    });
  },
});
