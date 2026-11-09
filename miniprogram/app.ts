import { API } from './config';

App({
  globalData: {
    userId: 0,
    user: null as any,
  },

  onLaunch() {
    // 复用本地缓存的用户，否则走 wx.login 换取（开发版用 code 当 openid）。
    const cached = wx.getStorageSync('user');
    if (cached && cached.id) {
      this.globalData.userId = cached.id;
      this.globalData.user = cached;
      return;
    }
    wx.login({
      success: (res: any) => {
        wx.request({
          url: API + '/login',
          method: 'POST',
          data: { code: res.code || 'guest', nickname: '诗友' },
          success: (r: any) => {
            const user = r.data && r.data.data;
            if (user && user.id) {
              this.globalData.userId = user.id;
              this.globalData.user = user;
              wx.setStorageSync('user', user);
            }
          },
        });
      },
    });
  },
});
