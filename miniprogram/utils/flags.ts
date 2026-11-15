import { request } from './request';

// 评论开关由后端环境变量决定。未拉到配置前按关闭处理，避免审核包闪出入口。
let cached: boolean | null = null;
let inflight: Promise<boolean> | null = null;

export function loadCommentEnabled(): Promise<boolean> {
  if (cached !== null) return Promise.resolve(cached);
  if (!inflight) {
    inflight = request<{ commentEnabled: boolean }>({ url: '/app/config', silent: true })
      .then((d) => {
        cached = !!(d && d.commentEnabled);
        return cached;
      })
      .catch(() => {
        cached = false;
        return false;
      });
  }
  return inflight;
}
