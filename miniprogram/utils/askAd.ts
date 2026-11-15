import { request } from './request';
import { REWARD_AD_UNIT } from '../config';

let ad: any = null;
let adUnit = '';
let waiting: ((ok: boolean) => void) | null = null;

function loadAdUnit(): Promise<string> {
  if (REWARD_AD_UNIT) return Promise.resolve(REWARD_AD_UNIT);
  if (adUnit) return Promise.resolve(adUnit);
  return request<any>({ url: '/app/config', silent: true })
    .then((d) => {
      adUnit = (d && d.rewardAdUnit) || '';
      return adUnit;
    })
    .catch(() => '');
}

function ensureAd(unit: string) {
  if (ad) return ad;
  ad = wx.createRewardedVideoAd({ adUnitId: unit });
  ad.onError(() => {});
  ad.onClose((res: any) => {
    wx.hideLoading();
    const ended = !!(res && res.isEnded);
    if (!ended) wx.showToast({ title: '看完广告后才能对话', icon: 'none' });
    const done = waiting;
    waiting = null;
    if (done) done(ended);
  });
  return ad;
}

const MEMBER_ONLY = '此功能为会员专属，请开通会员后体验';

function showRewardAd(unit: string): Promise<boolean> {
  return new Promise((resolve) => {
    if (!unit || !wx.createRewardedVideoAd) {
      wx.showToast({ title: MEMBER_ONLY, icon: 'none' });
      resolve(false);
      return;
    }
    if (waiting) {
      resolve(false);
      return;
    }
    const video = ensureAd(unit);
    waiting = resolve;
    wx.showLoading({ title: '广告加载中', mask: true });
    const fail = () => {
      wx.hideLoading();
      wx.showToast({ title: '暂时没有广告，请稍后再试', icon: 'none' });
      const done = waiting;
      waiting = null;
      if (done) done(false);
    };
    video
      .show()
      .then(() => wx.hideLoading())
      .catch(() => {
        video
          .load()
          .then(() => video.show())
          .then(() => wx.hideLoading())
          .catch(fail);
      });
  });
}

// ensureAskCredit 会员直接进入。非会员在点击入口时：没配广告就提示开通会员，配了就立刻播激励视频。
// 发消息时若还有剩余次数，不再播广告。
export function ensureAskCredit(opts: {
  isMember: boolean;
  remain: number;
  author: string;
  dynasty: string;
  forceAd?: boolean;
}): Promise<number | null> {
  if (opts.isMember) return Promise.resolve(-1);
  if (!opts.forceAd && opts.remain > 0) return Promise.resolve(opts.remain);
  if (!opts.author) return Promise.resolve(null);
  return loadAdUnit().then((unit) => {
    if (!unit) {
      wx.showToast({ title: MEMBER_ONLY, icon: 'none' });
      return null;
    }
    return showRewardAd(unit).then((ended) => {
      if (!ended) return null;
      return request<any>({
        url: '/users/me/ask-credit',
        method: 'POST',
        data: { author: opts.author, dynasty: opts.dynasty || '' },
        silent: true,
      })
        .then((r) => (r && typeof r.remain === 'number' ? r.remain : opts.remain))
        .catch((err: any) => {
          wx.showToast({ title: (err && err.msg) || '暂时无法增加次数', icon: 'none' });
          return null;
        });
    });
  });
}
