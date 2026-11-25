import { API, BASE_URL, CLOUD_ENV, CLOUD_SERVICE, useCloud } from '../config';
import { clearLogin, ensureLogin } from './auth';

type Method = 'GET' | 'POST' | 'PUT' | 'DELETE';

interface ReqOptions {
  url: string;
  method?: Method;
  data?: IAnyObject;
  silent?: boolean; // 为 true 时失败不弹 toast（用于乐观更新场景）
  retried?: boolean;
}

// rawCall 底层请求：云托管模式走 wx.cloud.callContainer（自动带 openid），否则走 wx.request。
// apiPath 形如 '/api/poems'。返回 HTTP 层结果 {statusCode, data}。
export function rawCall(
  apiPath: string,
  method: Method,
  data: IAnyObject,
  header: IAnyObject
): Promise<{ statusCode: number; data: any }> {
  return new Promise((resolve, reject) => {
    const common = {
      method,
      data: data || {},
      success: (res: any) => resolve({ statusCode: res.statusCode, data: res.data }),
      fail: (err: any) => reject(err),
    };
    if (useCloud()) {
      wx.cloud.callContainer({
        config: { env: CLOUD_ENV },
        path: apiPath,
        header: { ...header, 'X-WX-SERVICE': CLOUD_SERVICE, 'content-type': 'application/json' },
        ...common,
      });
    } else {
      wx.request({
        url: BASE_URL + apiPath,
        header: { ...header, 'content-type': 'application/json' },
        ...common,
      });
    }
  });
}

// request 统一封装：自动带上登录凭证 X-Auth-Token，并解包 {code,msg,data}。
export function request<T = any>(opts: ReqOptions): Promise<T> {
  const app = getApp();
  const g = app && app.globalData;
  const token = (g && (g.token || (g.user && g.user.token))) || '';
  const path = API_PATH(opts.url);
  return new Promise((resolve, reject) => {
    rawCall(path, opts.method || 'GET', opts.data || {}, { 'X-Auth-Token': String(token) })
      .then((res) => {
        const body = res.data || {};
        if (res.statusCode >= 200 && res.statusCode < 300 && body.code === 0) {
          resolve(body.data as T);
          return;
        }
        const msg = body && body.msg ? body.msg : '请求失败(' + res.statusCode + ')';
        const unauthorized = res.statusCode === 401 || msg === '未登录';
        if (unauthorized && !opts.retried) {
          clearLogin();
          ensureLogin(true).then((uid) => {
            if (!uid) {
              if (!opts.silent) wx.showToast({ title: '登录失败', icon: 'none' });
              reject(body);
              return;
            }
            request<T>({ ...opts, retried: true }).then(resolve, reject);
          });
          return;
        }
        console.error('[request fail]', path, 'status=', res.statusCode, 'body=', res.data);
        if (!opts.silent) wx.showToast({ title: msg, icon: 'none' });
        reject(body);
      })
      .catch((err) => {
        console.error('[request error]', path, err);
        if (!opts.silent) wx.showToast({ title: '网络异常', icon: 'none' });
        reject(err);
      });
  });
}

// API_PATH 把业务路径拼成 /api 前缀路径（与 config.API 保持一致）。
function API_PATH(url: string): string {
  // API = BASE_URL + '/api'，这里只取 '/api' 段 + 业务路径，云/普通两种模式通用。
  return '/api' + url;
}

// 兼容旧引用
export { API };
