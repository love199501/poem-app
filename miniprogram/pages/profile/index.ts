import { request } from '../../utils/request';
import { ensureLogin } from '../../utils/auth';

// 身份称号阶梯与升级门槛（需与后端 model/user.go 的 levels 保持一致）。
const RANKS = ['白丁', '学童', '秀才', '举人', '进士', '翰林', '诗人', '诗豪', '诗仙'];
const THRESH = [0, 50, 150, 400, 900, 1800, 3500, 6000, 10000];

// lvTier 按等级映射到徽章配色档位（1-2→t1 … 9→t5），等级越高辨识度越强。
function lvTier(level: number): string {
  if (level >= 9) return 'lv-t5';
  if (level >= 7) return 'lv-t4';
  if (level >= 5) return 'lv-t3';
  if (level >= 3) return 'lv-t2';
  return 'lv-t1';
}

// memberStatusText 会员状态文案（含剩余天数）。
function memberStatusText(u: any): string {
  const until = (u.memberUntil || '').slice(0, 10);
  if (u.isMember) {
    const left = Math.max(1, Math.ceil((new Date(u.memberUntil).getTime() - Date.now()) / 86400000));
    return '有效期至 ' + until + ' · 剩余 ' + left + ' 天';
  }
  if (u.memberClaimed) return '会员已于 ' + until + ' 到期';
  return '首次可免费领取 1 个月会员体验';
}

Page({
  data: {
    user: null as any,
    stat: { readMinutes: 0, favoriteCount: 0, commentCount: 0, likeReceived: 0 },
    progress: 0, // 当前等级进度百分比
    expInLevel: 0, // 当前等级内已获经验
    expNeed: 0, // 当前等级升级所需经验
    toNext: 0, // 距下一级还需经验
    nextName: '', // 下一身份称号
    badgeTier: 'lv-t1', // 身份徽章配色档
    tasks: [] as any[], // 今日任务
    taskGot: 0, // 今日已得经验
    taskTotal: 0, // 今日可得经验上限
    showRanks: false, // 身份阶梯弹层
    ranks: [] as any[], // 阶梯数据
    memberText: '', // 会员状态文案
    claiming: false, // 领取中
  },

  onShow() {
    this.fetchMe();
    this.fetchTasks();
  },

  fetchTasks() {
    ensureLogin().then((uid) => {
      if (!uid) return;
      request<any>({ url: '/users/me/tasks' })
        .then((res) => {
          const tasks = (res.tasks || []).map((t: any) => ({
            ...t,
            pct: t.cap > 0 ? Math.min(100, Math.round((t.count / t.cap) * 100)) : 0,
          }));
          this.setData({ tasks, taskGot: res.gotExp || 0, taskTotal: res.totalExp || 0 });
        })
        .catch(() => {});
    });
  },

  fetchMe() {
    ensureLogin().then((uid) => {
      if (!uid) return;
      request<any>({ url: '/users/me' })
        .then((res) => {
          const u = res.user || {};
          let progress = 100;
          let expInLevel = 0;
          let expNeed = 0;
          let toNext = 0;
          if (!u.isMax) {
            expInLevel = u.exp - u.curBase;
            expNeed = u.nextExp - u.curBase;
            toNext = Math.max(0, u.nextExp - u.exp);
            progress = expNeed > 0 ? Math.min(100, Math.round((expInLevel / expNeed) * 100)) : 0;
          }
          this.setData({
            user: u,
            stat: {
              readMinutes: res.readMinutes || 0,
              favoriteCount: res.favoriteCount || 0,
              commentCount: res.commentCount || 0,
              likeReceived: res.likeReceived || 0,
            },
            progress,
            expInLevel,
            expNeed,
            toNext,
            nextName: RANKS[Math.min(u.level, RANKS.length - 1)] || '',
            badgeTier: lvTier(u.level),
            memberText: memberStatusText(u),
          });
          const app = getApp();
          if (app && app.globalData) app.globalData.user = u;
        })
        .catch(() => {});
    });
  },

  // 首次免费领取 1 个月会员。
  claimMember() {
    if (this.data.claiming) return;
    this.setData({ claiming: true });
    ensureLogin().then((uid) => {
      if (!uid) {
        this.setData({ claiming: false });
        return;
      }
      request<any>({ url: '/users/me/member/claim', method: 'POST' })
        .then(() => {
          wx.showToast({ title: '已领取 1 个月会员', icon: 'success' });
          this.setData({ claiming: false });
          this.fetchMe();
        })
        .catch((err: any) => {
          wx.showToast({ title: (err && err.msg) || '领取失败', icon: 'none' });
          this.setData({ claiming: false });
        });
    });
  },

  // 打开收藏 / 评论 / 被赞 列表页。
  goList(e: any) {
    const type = e.currentTarget.dataset.type;
    if (type) wx.navigateTo({ url: '/pages/mylist/index?type=' + type });
  },

  // 点击身份徽章，弹出完整身份阶梯并标出当前所处（自定义弹层，带分档视觉）。
  openRanks() {
    const u = this.data.user;
    const cur = u ? u.level : 0;
    const ranks = RANKS.map((name, i) => {
      const level = i + 1;
      const reached = level <= cur;
      const current = level === cur;
      let note = `需 ${THRESH[i]} 经验解锁`;
      if (current) note = `${THRESH[i]} 经验 · 当前身份`;
      else if (reached) note = `${THRESH[i]} 经验 · 已达成`;
      return { level, name, threshold: THRESH[i], tier: lvTier(level), reached, current, note };
    });
    this.setData({ ranks, showRanks: true });
  },
  closeRanks() {
    this.setData({ showRanks: false });
  },
  noop() {},
});
