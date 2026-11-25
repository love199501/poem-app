import { rawCall } from './request';

let pendingLogin: Promise<number> | null = null;

export function clearLogin() {
  const app = getApp();
  if (app && app.globalData) {
    app.globalData.userId = 0;
    app.globalData.token = '';
    app.globalData.user = null;
  }
  try {
    wx.removeStorageSync('user');
  } catch (e) {}
}

function restoreLogin(): number {
  const app = getApp();
  const g = app && app.globalData;
  if (g && g.userId && g.token) return g.userId as number;
  const cached = wx.getStorageSync('user');
  if (cached && cached.id && cached.token && g) {
    g.userId = cached.id;
    g.token = cached.token;
    g.user = cached;
    return cached.id;
  }
  return 0;
}

// ensureLogin 确保已登录并返回 userId。force 为 true 时丢掉本地凭证重新登录。
// 同时只发一次登录，避免并发登录把后发的凭证冲掉。
export function ensureLogin(force?: boolean): Promise<number> {
  if (!force) {
    const uid = restoreLogin();
    if (uid) return Promise.resolve(uid);
  }
  if (pendingLogin) return pendingLogin;
  const app = getApp();
  pendingLogin = new Promise((resolve) => {
    wx.login({
      success: (res: any) => {
        rawCall('/api/login', 'POST', { code: res.code || '' }, {})
          .then((r) => {
            const user = r.data && r.data.data;
            if (user && user.id && user.token && app && app.globalData) {
              app.globalData.userId = user.id;
              app.globalData.token = user.token;
              app.globalData.user = user;
              wx.setStorageSync('user', user);
              resolve(user.id);
            } else {
              resolve(0);
            }
          })
          .catch(() => resolve(0));
      },
      fail: () => resolve(0),
    });
  }).then((uid) => {
    pendingLogin = null;
    return uid;
  });
  return pendingLogin;
}
