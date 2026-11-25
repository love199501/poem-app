import { request } from './request';

type AppFlags = { commentEnabled: boolean; audioEnabled: boolean };

// 评论、在线播放由后端环境变量决定。未拉到配置前按关闭处理，避免审核包闪出入口。
let cached: AppFlags | null = null;
let inflight: Promise<AppFlags> | null = null;

function loadFlags(): Promise<AppFlags> {
  if (cached) return Promise.resolve(cached);
  if (!inflight) {
    inflight = request<any>({ url: '/app/config', silent: true })
      .then((d) => {
        cached = {
          commentEnabled: !!(d && d.commentEnabled),
          audioEnabled: !!(d && d.audioEnabled),
        };
        return cached;
      })
      .catch(() => {
        cached = { commentEnabled: false, audioEnabled: false };
        return cached;
      });
  }
  return inflight;
}

export function loadCommentEnabled(): Promise<boolean> {
  return loadFlags().then((f) => f.commentEnabled);
}

export function loadAudioEnabled(): Promise<boolean> {
  return loadFlags().then((f) => f.audioEnabled);
}
