import { request } from '../../utils/request';
import { ensureLogin } from '../../utils/auth';
import { loadCommentEnabled } from '../../utils/flags';
import { API, BASE_URL } from '../../config';

function fullAvatar(u: string): string {
  if (!u) return '';
  return /^https?:/.test(u) ? u : BASE_URL + u;
}

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

function profileReady(u: any): boolean {
  if (!u) return false;
  const nick = String(u.nickname || '').trim();
  return nick !== '' && nick !== '诗友' && !!u.avatarUrl && !!u.phone;
}

function canSheetClaim(u: any, nick: string): boolean {
  if (!u || u.memberClaimed) return false;
  const name = String(nick || '').trim();
  return name !== '' && name !== '诗友' && !!u.avatarUrl && !!u.phone;
}

// memberStatusText 会员状态文案（含剩余天数）。
function memberStatusText(u: any): string {
  const until = (u.memberUntil || '').slice(0, 10);
  if (u.isMember) {
    const left = Math.max(1, Math.ceil((new Date(u.memberUntil).getTime() - Date.now()) / 86400000));
    return '有效期至 ' + until + ' · 剩余 ' + left + ' 天';
  }
  if (u.memberClaimed) return '会员已于 ' + until + ' 到期';
  return '首次可免费领取 7 天会员';
}

Page({
  data: {
    user: null as any,
    stat: { readMinutes: 0, readCount: 0, favoriteCount: 0, commentCount: 0, likeReceived: 0 },
    commentEnabled: false,
    progress: 0, // 当前等级进度百分比
    expInLevel: 0, // 当前等级内已获经验
    expNeed: 0, // 当前等级升级所需经验
    toNext: 0, // 距下一级还需经验
    nextName: '', // 下一身份称号
    badgeTier: 'lv-t1', // 身份徽章配色档
    tasks: [] as any[], // 今日任务
    taskGot: 0, // 今日已得经验
    taskTotal: 0, // 今日可得经验上限
    checkin: { today: false, monthCount: 0, need: 18, canClaim: false, claimed: false, bonusExp: 15 },
    showRanks: false, // 身份阶梯弹层
    ranks: [] as any[], // 阶梯数据
    memberText: '', // 会员状态文案
    memberPriceFen: 0,
    profileReady: false, // 昵称、头像、手机号是否已齐
    sheetClaim: false, // 资料弹层里是否可以保存并领取
    claiming: false, // 领取中
    avatarSrc: '', // 微信头像完整地址（空则用文字头像）
    showEdit: false, // 资料编辑弹层
    editNick: '', // 编辑中的昵称
    showFeedback: false,
    feedbackText: '',
  },

  onShow() {
    loadCommentEnabled().then((on) => this.setData({ commentEnabled: on }));
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
          const checkin = res.checkin || { today: false, monthCount: 0, need: 18, canClaim: false, claimed: false, bonusExp: 15 };
          const need = checkin.need || 18;
          checkin.pct = Math.min(100, Math.round(((checkin.monthCount || 0) / need) * 100));
          this.setData({ tasks, taskGot: res.gotExp || 0, taskTotal: res.totalExp || 0, checkin });
          if (res.checkinGain > 0) {
            wx.showToast({ title: '今日打卡 · 经验+' + res.checkinGain, icon: 'none' });
            this.fetchMe();
          }
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
              readCount: res.readCount || 0,
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
            memberPriceFen: res.memberMonthPrice || 0,
            profileReady: profileReady(u),
            sheetClaim: canSheetClaim(u, this.data.editNick || u.nickname),
            avatarSrc: fullAvatar(u.avatarUrl),
          });
          const app = getApp();
          if (app && app.globalData) app.globalData.user = u;
          wx.setStorageSync('user', u);
        })
        .catch(() => {});
    });
  },

  // 微信手机号授权：code 交给后端换成号码并绑定到当前登录用户。
  onGetPhone(e: any) {
    const detail = (e && e.detail) || {};
    const msg = String(detail.errMsg || '');
    if (msg && msg.indexOf('ok') < 0) {
      wx.showToast({ title: '未授权手机号', icon: 'none' });
      return;
    }
    if (!detail.code) {
      wx.showToast({ title: '请在真机上授权手机号', icon: 'none' });
      return;
    }
    ensureLogin().then((uid) => {
      if (!uid) return;
      request<any>({ url: '/users/me/phone', method: 'POST', data: { code: detail.code } })
        .then((u) => {
          this.setData({
            user: u,
            memberText: memberStatusText(u),
            profileReady: profileReady(u),
            sheetClaim: canSheetClaim(u, this.data.editNick || u.nickname),
          });
          const app = getApp();
          if (app && app.globalData) app.globalData.user = u;
          wx.setStorageSync('user', u);
          wx.showToast({ title: '手机号已绑定', icon: 'success' });
        })
        .catch((err: any) => wx.showToast({ title: (err && err.msg) || '绑定失败', icon: 'none' }));
    });
  },

  // —— 微信资料（头像昵称填写能力） ——
  openEdit() {
    const nick = (this.data.user && this.data.user.nickname) || '';
    this.setData({ showEdit: true, editNick: nick, sheetClaim: canSheetClaim(this.data.user, nick) });
  },
  closeEdit() {
    this.setData({ showEdit: false });
  },
  noopEdit() {},
  onNickInput(e: any) {
    const nick = e.detail.value;
    this.setData({ editNick: nick, sheetClaim: canSheetClaim(this.data.user, nick) });
  },

  // 选择微信头像 → 上传到后端 → 更新。超过 2MB 先在本地拦住。
  onChooseAvatar(e: any) {
    const temp = e.detail.avatarUrl;
    if (!temp) return;
    wx.getFileSystemManager().getFileInfo({
      filePath: temp,
      success: (info) => {
        if (info.size > 2 * 1024 * 1024) {
          wx.showToast({ title: '头像请小于 2MB', icon: 'none' });
          return;
        }
        this.uploadAvatar(temp);
      },
      fail: () => this.uploadAvatar(temp),
    });
  },

  uploadAvatar(temp: string) {
    ensureLogin().then((uid) => {
      if (!uid) return;
      wx.showLoading({ title: '上传中' });
      wx.uploadFile({
        url: API + '/users/me/avatar',
        filePath: temp,
        name: 'file',
        header: { 'X-Auth-Token': String((getApp().globalData && getApp().globalData.token) || '') },
        success: (r: any) => {
          wx.hideLoading();
          let data: any = {};
          try {
            data = JSON.parse(r.data);
          } catch (err) {}
          if (data.code === 0 && data.data && data.data.avatarUrl) {
            const user = Object.assign({}, this.data.user, { avatarUrl: data.data.avatarUrl });
            this.setData({
              user,
              avatarSrc: fullAvatar(data.data.avatarUrl),
              profileReady: profileReady(user),
              memberText: memberStatusText(user),
              sheetClaim: canSheetClaim(user, this.data.editNick || user.nickname),
            });
            wx.showToast({ title: '头像已更新', icon: 'none' });
          } else {
            wx.showToast({ title: data.msg || '上传失败', icon: 'none' });
          }
        },
        fail: () => {
          wx.hideLoading();
          wx.showToast({ title: '上传失败', icon: 'none' });
        },
      });
    });
  },

  // 保存昵称。资料已齐且尚未领过会员时，保存后接着领取 7 天。
  saveProfile() {
    const nick = (this.data.editNick || '').trim();
    const claimAfter = !!this.data.sheetClaim;
    ensureLogin().then((uid) => {
      if (!uid) return;
      request<any>({ url: '/users/me/profile', method: 'POST', data: { nickname: nick } })
        .then((u) => {
          this.setData({
            user: u,
            memberText: memberStatusText(u),
            profileReady: profileReady(u),
            sheetClaim: canSheetClaim(u, nick),
            avatarSrc: fullAvatar(u.avatarUrl),
          });
          const app = getApp();
          if (app && app.globalData) app.globalData.user = u;
          wx.setStorageSync('user', u);
          if (claimAfter && profileReady(u)) {
            this.claimMember();
            return;
          }
          this.setData({ showEdit: false });
          wx.showToast({ title: '已保存', icon: 'none' });
        })
        .catch((err: any) => wx.showToast({ title: (err && err.msg) || '保存失败', icon: 'none' }));
    });
  },

  claimCheckinMember() {
    const c = this.data.checkin;
    if (!c || !c.canClaim || this.data.claiming) return;
    this.setData({ claiming: true });
    ensureLogin().then((uid) => {
      if (!uid) {
        this.setData({ claiming: false });
        return;
      }
      request<any>({ url: '/users/me/checkin/member', method: 'POST' })
        .then(() => {
          wx.showToast({ title: '已领取 1 个月会员', icon: 'success' });
          this.setData({ claiming: false });
          this.fetchMe();
          this.fetchTasks();
        })
        .catch((err: any) => {
          wx.showToast({ title: (err && err.msg) || '领取失败', icon: 'none' });
          this.setData({ claiming: false });
        });
    });
  },

  markMemberClaimed() {
    const user = Object.assign({}, this.data.user || {}, { memberClaimed: true });
    this.setData({
      user,
      memberText: memberStatusText(user),
      sheetClaim: false,
      showEdit: false,
      claiming: false,
    });
    const app = getApp();
    if (app && app.globalData) app.globalData.user = user;
    wx.setStorageSync('user', user);
    this.fetchMe();
  },

  // 完善资料后，首次免费领取 7 天会员。每个账号只能领一次。
  claimMember() {
    if (this.data.claiming) return;
    if (this.data.user && this.data.user.memberClaimed) {
      this.markMemberClaimed();
      wx.showToast({ title: '已领取过免费会员', icon: 'none' });
      return;
    }
    if (!profileReady(this.data.user)) {
      this.openEdit();
      wx.showToast({ title: '请先完善昵称、头像和手机号', icon: 'none' });
      return;
    }
    this.setData({ claiming: true });
    ensureLogin().then((uid) => {
      if (!uid) {
        this.setData({ claiming: false });
        return;
      }
      request<any>({ url: '/users/me/member/claim', method: 'POST' })
        .then(() => {
          wx.showToast({ title: '已领取 7 天会员', icon: 'success' });
          this.markMemberClaimed();
        })
        .catch((err: any) => {
          const msg = (err && err.msg) || '领取失败';
          if (msg.indexOf('已领取') >= 0) this.markMemberClaimed();
          else this.setData({ claiming: false });
          wx.showToast({ title: msg, icon: 'none' });
        });
    });
  },

  // 打开阅读历史 / 收藏 / 评论 / 被赞 列表页。
  goList(e: any) {
    const type = e.currentTarget.dataset.type;
    if (!type) return;
    if (!this.data.commentEnabled && (type === 'comments' || type === 'likes')) return;
    wx.navigateTo({ url: '/pages/mylist/index?type=' + type });
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

  confirmRecharge() {
    if (this.data.claiming) return;
    this.setData({ claiming: true });
    ensureLogin().then((uid) => {
      if (!uid) {
        this.setData({ claiming: false });
        return;
      }
      request<any>({ url: '/users/me/member/recharge', method: 'POST', silent: true })
        .then((r) => {
          const p = (r && r.payment) || {};
          wx.requestPayment({
            timeStamp: String(p.timeStamp || ''),
            nonceStr: p.nonceStr || '',
            package: p['package'] || '',
            signType: p.signType || 'MD5',
            paySign: p.paySign || '',
            success: () => {
              request<any>({
                url: '/users/me/member/recharge/confirm',
                method: 'POST',
                data: { outTradeNo: r.outTradeNo },
                silent: true,
              })
                .then(() => {
                  this.setData({ claiming: false });
                  wx.showToast({ title: '充值成功', icon: 'success' });
                  this.fetchMe();
                })
                .catch(() => {
                  this.setData({ claiming: false });
                  wx.showToast({ title: '支付成功，会员到账可能稍有延迟', icon: 'none' });
                  this.fetchMe();
                });
            },
            fail: () => {
              this.setData({ claiming: false });
              wx.showToast({ title: '已取消支付', icon: 'none' });
            },
          });
        })
        .catch((err: any) => {
          this.setData({ claiming: false });
          wx.showToast({ title: (err && err.msg) || '暂时无法充值', icon: 'none' });
        });
    });
  },

  openFeedback() {
    this.setData({ showFeedback: true, feedbackText: '' });
  },
  closeFeedback() {
    this.setData({ showFeedback: false });
  },
  onFeedbackInput(e: any) {
    this.setData({ feedbackText: e.detail.value });
  },
  submitFeedback() {
    const content = String(this.data.feedbackText || '').trim();
    if (!content) {
      wx.showToast({ title: '请写下建议或反馈', icon: 'none' });
      return;
    }
    ensureLogin().then((uid) => {
      if (!uid) return;
      request<any>({ url: '/users/me/feedback', method: 'POST', data: { content } })
        .then(() => {
          this.setData({ showFeedback: false, feedbackText: '' });
          wx.showToast({ title: '已收到，谢谢', icon: 'none' });
        })
        .catch((err: any) => wx.showToast({ title: (err && err.msg) || '提交失败', icon: 'none' }));
    });
  },
});
