import { API } from '../config';

// ensureLogin 确保已登录并返回 userId；未登录则走 wx.login 换取并缓存。
// like / 发评论前调用，避免因登录是异步导致首次操作拿不到 userId。
export function ensureLogin(): Promise<number> {
  const app = getApp();
  if (app && app.globalData && app.globalData.userId) {
    return Promise.resolve(app.globalData.userId as number);
  }
  return new Promise((resolve) => {
    wx.login({
      success: (res: any) => {
        wx.request({
          url: API + '/login',
          method: 'POST',
          data: { code: res.code || 'guest', nickname: '诗友' },
          success: (r: any) => {
            const user = r.data && r.data.data;
            if (user && user.id) {
              app.globalData.userId = user.id;
              app.globalData.user = user;
              wx.setStorageSync('user', user);
              resolve(user.id);
            } else {
              resolve(0);
            }
          },
          fail: () => resolve(0),
        });
      },
      fail: () => resolve(0),
    });
  });
}
