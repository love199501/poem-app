import { API } from '../config';

type Method = 'GET' | 'POST' | 'PUT' | 'DELETE';

interface ReqOptions {
  url: string;
  method?: Method;
  data?: IAnyObject;
  silent?: boolean; // 为 true 时失败不弹 toast（用于乐观更新场景）
}

// request 统一封装：自动带上登录用户头 X-User-Id，并解包 {code,msg,data}。
export function request<T = any>(opts: ReqOptions): Promise<T> {
  const app = getApp();
  const userId = (app && app.globalData && app.globalData.userId) || '';
  return new Promise((resolve, reject) => {
    wx.request({
      url: API + opts.url,
      method: opts.method || 'GET',
      data: opts.data || {},
      header: {
        'Content-Type': 'application/json',
        'X-User-Id': String(userId),
      },
      success(res: any) {
        const body = res.data || {};
        if (res.statusCode >= 200 && res.statusCode < 300 && body.code === 0) {
          resolve(body.data as T);
        } else {
          if (!opts.silent) wx.showToast({ title: body.msg || '请求失败', icon: 'none' });
          reject(body);
        }
      },
      fail(err: any) {
        if (!opts.silent) wx.showToast({ title: '网络异常', icon: 'none' });
        reject(err);
      },
    });
  });
}
