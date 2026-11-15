import { request } from '../../utils/request';
import { ensureLogin } from '../../utils/auth';
import { rememberRead } from '../../utils/history';
import { loadCommentEnabled } from '../../utils/flags';
import { ensureAskCredit } from '../../utils/askAd';
import { BASE_URL } from '../../config';

function fmtLike(n: number) {
  return n >= 1000 ? (n / 1000).toFixed(1) + 'k' : String(n);
}

// lvTier 按作者等级映射徽章配色档（与个人中心、全局样式一致）。
function lvTier(level: number): string {
  if (level >= 9) return 'lv-t5';
  if (level >= 7) return 'lv-t4';
  if (level >= 5) return 'lv-t3';
  if (level >= 3) return 'lv-t2';
  return 'lv-t1';
}

function toFull(u: string) {
  if (!u) return '';
  return /^https?:/.test(u) ? u : BASE_URL + u;
}

function userAskPrompts(poem: any) {
  const title = (poem && poem.title) || '这首诗';
  const stored = ((poem && poem.aiPrompts) || [])
    .map((s: string) => String(s || '').trim())
    .filter((s: string) => s.indexOf('你') >= 0 && /[？?]$/.test(s));
  const fallback = [
    '你写《' + title + '》的时候，心里在想什么？',
    '这句你最想让谁看见？',
    '有什么当时没写进诗里？',
  ];
  const out: string[] = [];
  stored.concat(fallback).forEach((s: string) => {
    if (out.length < 3 && out.indexOf(s) < 0) out.push(s);
  });
  return out;
}

function fmtTime(sec: number) {
  sec = Math.floor(sec || 0);
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  return (m < 10 ? '0' + m : m) + ':' + (s < 10 ? '0' + s : s);
}

Page({
  data: {
    id: 0,
    poem: null as any,
    activeLine: 1,
    note: null as any,
    showTrans: false,
    hasTrans: false,
    hasScene: false,
    hasIntent: false,
    hasWu: false,
    commentEnabled: false,
    isMember: false,
    authorChar: '诗',
    askInput: '',
    askBusy: false,
    asks: [] as any[],
    askPrompts: [] as string[],
    askRemain: 0,
    askPerAd: 3,

    menus: [] as any[],
    panel: 'note',
    intoView: '',

    comments: [] as any[],
    sort: 'hot',
    onlyShihua: false,
    input: '',
    inputFocus: false,
    replyPrefix: '',
    replyParentId: 0,
    replyTo: '',
    sending: false,

    // 朗诵
    audio: { playing: false, loading: false, cur: '00:00', dur: '00:00', pct: 0, ready: false },

    // 收藏 / 点赞（受欢迎度）
    favored: false,
    liked: false,
    likeCount: 0,
    favoriteCount: 0,

    shareOpen: false,
    shareTheme: 'illust',
    shareCols: [] as string[][],
    shareCode: '',
    shareIllust: '',
    shareQuote: null as any,
    themes: [
      { key: 'illust', name: '贴图' },
      { key: 'paper', name: '宣纸' },
      { key: 'night', name: '墨夜' },
      { key: 'green', name: '青绿' },
      { key: 'cinnabar', name: '朱砂' },
    ],
  },

  onLoad(options: any) {
    let id = Number(options.id || 0);
    if (!id && options.scene) {
      const scene = decodeURIComponent(options.scene);
      const m = scene.match(/id=(\d+)/);
      if (m) id = Number(m[1]);
    }
    if (!id) id = 1;
    this.setData({ id });
    wx.showShareMenu({ menus: ['shareAppMessage', 'shareTimeline'] });
    this.initAudio();
    this.fetchPoem();
  },

  onShareAppMessage() {
    const poem = this.data.poem || {};
    const line = ((this.data.shareCols && this.data.shareCols[0]) || []).join('');
    const title = line ? line + ' · ' + (poem.author || '诗境') : '诗境 · ' + (poem.title || '一首诗');
    return { title, path: '/pages/poem/detail?id=' + this.data.id };
  },

  onShareTimeline() {
    const poem = this.data.poem || {};
    const line = ((this.data.shareCols && this.data.shareCols[0]) || []).join('');
    return {
      title: line ? line + ' · ' + (poem.author || '') : '诗境 · ' + (poem.title || ''),
      query: 'id=' + this.data.id,
    };
  },

  onShow() {
    // 开始计阅读时长；每次展示都刷新评论，开通会员后历史评论能立刻带上皇冠。
    this._readStart = Date.now();
    if (this.data.id && this.data.commentEnabled) this.fetchComments();
    this.refreshMember();
    this.loadAskCredit();
  },

  loadAskCredit() {
    const poem = this.data.poem;
    if (!poem || !poem.author) return;
    ensureLogin().then((uid) => {
      if (!uid) return;
      request<any>({
        url: '/users/me/ask-credit',
        data: { author: poem.author, dynasty: poem.dynasty || '' },
        silent: true,
      })
        .then((r) => {
          if (!r) return;
          const patch: any = { askRemain: r.remain, askPerAd: r.perAd || 3 };
          if (r.remain < 0) patch.isMember = true;
          this.setData(patch);
        })
        .catch(() => {});
    });
  },

  refreshMember() {
    ensureLogin().then((uid) => {
      if (!uid) return;
      request<any>({ url: '/users/me', silent: true }).then((res) => {
        const u = (res && res.user) || res || {};
        const app = getApp();
        if (app && app.globalData) app.globalData.user = u;
        this.setData({ isMember: !!u.isMember });
      }).catch(() => {});
    });
  },
  onHide() {
    this.pauseAudio();
    this.reportRead();
  },
  onUnload() {
    this.reportRead();
    if (this.audioCtx) {
      try {
        this.audioCtx.stop();
        this.audioCtx.destroy();
      } catch (e) {}
      this.audioCtx = null;
    }
  },

  // reportRead 上报本次阅读。停留满 1 秒记入阅读历史；满 5 秒才累计时长和经验。
  reportRead() {
    if (!this._readStart) return;
    const sec = Math.floor((Date.now() - this._readStart) / 1000);
    this._readStart = 0;
    const poemId = Number(this.data.id || 0);
    if (sec < 1 || !poemId) return;
    rememberRead(poemId);
    ensureLogin().then(() => {
      request<any>({
        url: '/users/me/read',
        method: 'POST',
        data: { seconds: sec >= 5 ? sec : 0, poemId },
        silent: true,
      }).catch(() => {});
    });
  },

  // 收藏 / 取消收藏当前诗。
  toggleFav() {
    ensureLogin().then((uid) => {
      if (!uid) return;
      request<any>({ url: '/poems/' + this.data.id + '/favorite', method: 'POST' }).then((r) => {
        this.setData({ favored: r.favored, favoriteCount: r.favoriteCount });
        let title = '已取消收藏';
        if (r.favored) title = r.gainExp > 0 ? '已收藏 · 经验+' + r.gainExp : '已收藏';
        wx.showToast({ title, icon: 'none' });
      });
    });
  },

  // 给这首诗点赞 / 取消（乐观更新 + 后端同步，体现受欢迎度）。
  toggleLikePoem() {
    ensureLogin().then((uid) => {
      if (!uid) return;
      const prevLiked = this.data.liked;
      const prevCount = this.data.likeCount;
      const liked = !prevLiked;
      this.setData({ liked, likeCount: Math.max(0, prevCount + (liked ? 1 : -1)) });
      request<any>({ url: '/poems/' + this.data.id + '/like', method: 'POST', silent: true })
        .then((r) => {
          this.setData({ liked: r.liked, likeCount: r.likeCount });
          if (r.liked) {
            wx.showToast({ title: r.gainExp > 0 ? '已点赞 · 经验+' + r.gainExp : '已点赞', icon: 'none' });
          }
        })
        .catch(() => this.setData({ liked: prevLiked, likeCount: prevCount }));
    });
  },

  // —— 朗诵 ——
  initAudio() {
    const ctx = wx.createInnerAudioContext();
    this.audioCtx = ctx;
    ctx.obeyMuteSwitch = false;
    if (wx.setInnerAudioOption) {
      wx.setInnerAudioOption({ obeyMuteSwitch: false });
    }
    ctx.onPlay(() => this.setData({ 'audio.playing': true, 'audio.loading': false }));
    ctx.onPause(() => this.setData({ 'audio.playing': false }));
    ctx.onStop(() => this.setData({ 'audio.playing': false }));
    ctx.onEnded(() => this.setData({ 'audio.playing': false, 'audio.pct': 0, 'audio.cur': '00:00' }));
    ctx.onError(() => {
      // 部署后音频文件会丢，但地址还在。先自动再合成一次，仍失败再提示。
      if (!this._resynth && this.data.id) {
        this._resynth = true;
        this.setData({ 'audio.playing': false, 'audio.ready': false, 'audio.loading': true });
        ensureLogin().then((uid) => {
          if (!uid) {
            this.setData({ 'audio.loading': false });
            wx.showToast({ title: '请先登录', icon: 'none' });
            return;
          }
          request<any>({ url: '/poems/' + this.data.id + '/tts', method: 'POST', silent: true })
          .then((r) => {
            if (r && r.audioUrl) {
              this.setData({ 'poem.audioUrl': r.audioUrl });
              this.playSrc(r.audioUrl);
              return;
            }
            this.setData({ 'audio.loading': false });
            wx.showToast({ title: '朗诵文件已丢失', icon: 'none' });
          })
          .catch((err: any) => {
            this.setData({ 'audio.loading': false });
            wx.showToast({ title: (err && err.msg) || '朗诵合成失败', icon: 'none' });
          });
        });
        return;
      }
      this.setData({ 'audio.playing': false, 'audio.loading': false });
      wx.showToast({ title: '播放失败', icon: 'none' });
    });
    ctx.onTimeUpdate(() => {
      const cur = ctx.currentTime || 0;
      const dur = ctx.duration || 0;
      this.setData({
        'audio.cur': fmtTime(cur),
        'audio.dur': fmtTime(dur),
        'audio.pct': dur ? Math.min(100, (cur / dur) * 100) : 0,
      });
    });
  },

  toggleAudio() {
    const a = this.data.audio;
    if (a.playing) {
      this.pauseAudio();
      return;
    }
    if (a.ready) {
      this.audioCtx.play();
      return;
    }
    // 先问后端要可播放的地址。文件被清掉时会自动再合成，避免播一个已经不存在的链接。
    this.setData({ 'audio.loading': true });
    ensureLogin().then((uid) => {
      if (!uid) {
        this.setData({ 'audio.loading': false });
        wx.showToast({ title: '请先登录', icon: 'none' });
        return;
      }
      request<any>({ url: '/poems/' + this.data.id + '/tts', method: 'POST', silent: true })
      .then((r) => {
        this.setData({ 'poem.audioUrl': r.audioUrl });
        this.playSrc(r.audioUrl);
      })
      .catch((err: any) => {
        // 把后端真实原因弹出来，便于定位（如火山鉴权失败/未配置供应商）。
        this.setData({ 'audio.loading': false });
        const msg = err && err.msg ? err.msg : '朗诵合成失败';
        wx.showModal({ title: '情感朗诵未生效', content: msg, showCancel: false });
      });
    });
  },

  playSrc(url: string) {
    const full = /^https?:/.test(url) ? url : BASE_URL + url;
    this.audioCtx.src = full;
    this.setData({ 'audio.ready': true });
    this.audioCtx.play();
  },

  pauseAudio() {
    if (this.audioCtx && this.data.audio.playing) this.audioCtx.pause();
  },

  // 回退：微信同声传译插件（基础 TTS，偏机械）。
  ttsPlayFallback() {
    const poem = this.data.poem;
    if (!poem || !poem.lines || !poem.lines.length) {
      wx.showToast({ title: '暂无可朗诵内容', icon: 'none' });
      return;
    }
    const content = poem.lines.map((l: any) => l.text).join('');
    let plugin: any;
    try {
      plugin = requirePlugin('WechatSI');
    } catch (e) {
      wx.showToast({ title: '朗诵插件未启用', icon: 'none' });
      return;
    }
    this.setData({ 'audio.loading': true });
    plugin.textToSpeech({
      lang: 'zh_CN',
      tts: true,
      content,
      success: (res: any) => {
        this.audioCtx.src = res.filename;
        this.setData({ 'audio.ready': true });
        this.audioCtx.play();
      },
      fail: (err: any) => {
        this.setData({ 'audio.loading': false });
        wx.showToast({ title: '合成失败：' + (err.msg || ''), icon: 'none' });
      },
    });
  },

  fetchPoem() {
    Promise.all([request<any>({ url: '/poems/' + this.data.id }), loadCommentEnabled()]).then(([poem, commentEnabled]) => {
      const lines = ((poem && poem.lines) || []).filter((l: any) => String(l.text || '').trim());
      if (!lines.length && !String((poem && poem.firstVerse) || '').trim()) {
        wx.showToast({ title: '这首诗暂无原文', icon: 'none' });
        setTimeout(() => wx.navigateBack(), 400);
        return;
      }
      const len = (a: any) => (a && a.length) || 0;
      const hasTrans = len(poem.translation) > 0;
      const hasScene = !!(len(poem.sceneRows) || poem.scenePunch || poem.sceneBackground || poem.sceneTitle);
      const hasIntent = !!(poem.intentLead || len(poem.audiences) || poem.void);
      const hasWu = !!(len(poem.layers) || len(poem.quotes) || len(poem.duels));

      const menus: any[] = [{ key: 'note', label: '注释' }];
      if (hasTrans) menus.push({ key: 'trans', label: '译文' });
      if (hasScene) menus.push({ key: 'scene', label: '创作现场' });
      if (hasIntent) menus.push({ key: 'intent', label: '创作意图' });
      if (hasWu) menus.push({ key: 'wu', label: '解析' });
      const keep = menus.some((m) => m.key === this.data.panel);
      const panel = keep ? this.data.panel : menus[0].key;

      this.setData({
        poem,
        commentEnabled,
        hasTrans,
        hasScene,
        hasIntent,
        hasWu,
        menus,
        panel,
        favored: !!poem.favored,
        liked: !!poem.liked,
        likeCount: poem.likeCount || 0,
        favoriteCount: poem.favoriteCount || 0,
        authorChar: String(poem.author || '诗').slice(0, 1),
        askPrompts: userAskPrompts(poem),
      });
      this.loadAskCredit();
      this.preloadIllust(poem);
      this.loadWxaCode();

      // 默认高亮「首页展示的那句」(poem.firstVerse 对应的诗句)；找不到再退回第 2 句/第 1 句。
      if (poem.lines && poem.lines.length) {
        const fv = (poem.firstVerse || '').trim();
        let idx = -1;
        if (fv) {
          idx = poem.lines.findIndex((l: any) => (l.text || '').trim() === fv);
          if (idx < 0) {
            idx = poem.lines.findIndex(
              (l: any) => (l.text || '').indexOf(fv) >= 0 || fv.indexOf((l.text || '').trim()) >= 0
            );
          }
        }
        if (idx < 0) idx = poem.lines.length > 1 ? 1 : 0;
        this.pickLineByIndex(idx);
      }
    });
  },

  // —— 读：点击诗句切换注释 ——
  pickLine(e: any) {
    this.pickLineByIndex(e.currentTarget.dataset.idx);
  },
  pickLineByIndex(idx: number) {
    const line = this.data.poem.lines[idx];
    this.setData({
      activeLine: idx,
      panel: 'note',
      note: { text: line.text, body: line.noteBody, src: line.noteSrc },
    });
  },

  askText(e: any) {
    const q = (e.currentTarget.dataset.q || '').trim();
    if (!q) return;
    this.setData({ askInput: q });
    this.sendAsk();
  },

  onAskInput(e: any) {
    this.setData({ askInput: e.detail.value });
  },

  sendAsk() {
    const q = String(this.data.askInput || '').trim();
    if (!q || this.data.askBusy) return;
    const poem = this.data.poem || {};
    ensureAskCredit({
      isMember: this.data.isMember,
      remain: this.data.askRemain,
      author: poem.author || '',
      dynasty: poem.dynasty || '',
    }).then((remain) => {
      if (remain == null) return;
      this.setData({ askRemain: remain });
      this.postAsk(q);
    });
  },

  postAsk(q: string) {
    const history = (this.data.asks || [])
      .filter((m: any) => m.role === 'user' || m.role === 'ai')
      .reduce((acc: any[], m: any, i: number, arr: any[]) => {
        if (m.role === 'user') {
          const next = arr[i + 1];
          acc.push({ question: m.content, answer: next && next.role === 'ai' ? next.content : '' });
        }
        return acc;
      }, [])
      .filter((h: any) => h.answer);
    const asks = (this.data.asks || []).concat([{ role: 'user', content: q }]);
    this.setData({ asks, askInput: '', askBusy: true });
    ensureLogin().then((uid) => {
      if (!uid) {
        this.setData({ askBusy: false });
        return;
      }
      request<any>({
        url: '/poems/' + this.data.id + '/ask',
        method: 'POST',
        data: { question: q, history },
        silent: true,
      })
        .then((r) => {
          const patch: any = {
            askBusy: false,
            asks: (this.data.asks || []).concat([{ role: 'ai', content: r.answer || '', source: r.source || '' }]),
          };
          if (r && typeof r.askRemain === 'number') patch.askRemain = r.askRemain;
          this.setData(patch);
        })
        .catch((err: any) => {
          const asks = (this.data.asks || []).slice();
          if (asks.length && asks[asks.length - 1].role === 'user') asks.pop();
          const needAd = !!(err && String(err.msg || '').indexOf('广告') >= 0);
          this.setData({
            askBusy: false,
            asks,
            askInput: q,
            askRemain: needAd ? 0 : this.data.askRemain,
          });
          wx.showToast({ title: (err && err.msg) || '暂时无法回答', icon: 'none' });
        });
    });
  },

  pickPanel(e: any) {
    const key = e.currentTarget.dataset.key;
    if (!key || key === this.data.panel) return;
    if (key !== 'ask') {
      this.setData({ panel: key, intoView: '' });
      return;
    }
    const poem = this.data.poem || {};
    ensureAskCredit({
      isMember: this.data.isMember,
      remain: this.data.askRemain,
      author: poem.author || '',
      dynasty: poem.dynasty || '',
      forceAd: true,
    }).then((remain) => {
      if (remain == null) return;
      this.setData({ askRemain: remain, panel: 'ask', intoView: '' });
    });
  },
  toggleTrans() {
    this.setData({ showTrans: !this.data.showTrans });
  },

  // —— 步骤条：点击跳转 + 跟随滚动高亮 ——
  goPoet() {
    const poem = this.data.poem || {};
    const author = poem.author;
    if (!author) return;
    const dynasty = poem.dynasty || '';
    wx.navigateTo({
      url: '/pages/poet/scroll?author=' + encodeURIComponent(author) + '&dynasty=' + encodeURIComponent(dynasty),
    });
  },

  goStep(e: any) {
    const key = e.currentTarget.dataset.key;
    // 点击后短暂锁定滚动监听，避免动画途中高亮来回跳。
    this._lockSpyUntil = Date.now() + 500;
    this.setData({ curStep: key, intoView: key });
  },

  onScroll() {
    const now = Date.now();
    if (now < (this._lockSpyUntil || 0)) return;
    if (now - (this._lastSpy || 0) < 100) return;
    this._lastSpy = now;
    this.updateSpy();
  },

  // updateSpy 量取各段相对滚动容器顶部的位置，判定当前所在段。
  updateSpy() {
    const q = wx.createSelectorQuery().in(this);
    this.data.steps.forEach((s: any) => q.select('#' + s.key).boundingClientRect());
    q.select('.body').boundingClientRect();
    q.exec((res: any[]) => {
      const container = res[res.length - 1];
      if (!container) return;
      const line = container.top + 72; // 距容器顶 ~72px 处视为“当前段”
      let cur = this.data.steps[0].key;
      for (let i = 0; i < this.data.steps.length; i++) {
        const r = res[i];
        if (r && r.top <= line) cur = this.data.steps[i].key;
      }
      if (cur !== this.data.curStep) this.setData({ curStep: cur });
    });
  },

  // —— 议：讨论区 ——
  fetchComments() {
    request<any[]>({
      url: '/poems/' + this.data.id + '/comments',
      data: { sort: this.data.sort, kind: this.data.onlyShihua ? 'shihua' : '' },
    }).then((list) => this.setData({ comments: this.decorate(list || []) }));
  },

  // decorate 格式化赞数、初始化展开态，并处理楼中楼回复。
  // 当前用户已是会员时，其历史评论一律打上皇冠（不依赖后端快照字段）。
  decorate(list: any[]) {
    const me = (getApp() && getApp().globalData && getApp().globalData.user) || {};
    const myId = Number(me.id || 0);
    const iAmMember = !!me.isMember;
    const stamp = (c: any) => {
      const mine = myId > 0 && Number(c.userId) === myId;
      return {
        ...c,
        likeText: fmtLike(c.likeCount),
        lvTier: lvTier(c.authorLevel),
        authorMember: !!(c.authorMember || (mine && iAmMember)),
      };
    };
    return list.map((c) => ({
      ...stamp(c),
      expanded: false,
      replies: (c.replies || []).map((r: any) => stamp(r)),
    }));
  },

  switchSort(e: any) {
    this.setData({ sort: e.currentTarget.dataset.v }, () => this.fetchComments());
  },
  switchKind() {
    this.setData({ onlyShihua: !this.data.onlyShihua }, () => this.fetchComments());
  },

  // 展开 / 收起某条评论的回复。
  toggleExpand(e: any) {
    const id = Number(e.currentTarget.dataset.id);
    const idx = this.data.comments.findIndex((c: any) => Number(c.id) === id);
    if (idx < 0) return;
    this.setData({ [`comments[${idx}].expanded`]: !this.data.comments[idx].expanded });
  },

  // locate 在顶层与楼中楼里按 id 定位评论，返回 setData 路径与对象。
  locate(id: number): { path: string; obj: any } | null {
    const cs = this.data.comments;
    for (let i = 0; i < cs.length; i++) {
      if (Number(cs[i].id) === id) return { path: `comments[${i}]`, obj: cs[i] };
      const rs = cs[i].replies || [];
      for (let j = 0; j < rs.length; j++) {
        if (Number(rs[j].id) === id) return { path: `comments[${i}].replies[${j}]`, obj: rs[j] };
      }
    }
    return null;
  },

  // like：顶层与回复均可点赞，乐观更新 + 后端同步。
  like(e: any) {
    const id = Number(e.currentTarget.dataset.id);
    const loc = this.locate(id);
    if (!loc) return;
    const liked = !loc.obj.liked;
    const likeCount = Math.max(0, loc.obj.likeCount + (liked ? 1 : -1));
    this.setData({ [loc.path]: { ...loc.obj, liked, likeCount, likeText: fmtLike(likeCount) } });

    ensureLogin().then(() => {
      request<any>({ url: '/comments/' + id + '/like', method: 'POST', silent: true })
        .then((r) => {
          const l = this.locate(id);
          if (l) {
            this.setData({
              [l.path]: { ...l.obj, liked: r.liked, likeCount: r.likeCount, isFeatured: r.isFeatured, likeText: fmtLike(r.likeCount) },
            });
          }
        })
        .catch(() => {});
    });
  },

  onInput(e: any) {
    this.setData({ input: e.detail.value });
  },
  onBlur() {
    this.setData({ inputFocus: false });
  },

  // 点「回复」：定位到要挂载的父评论，聚焦输入框并把 @指向切到该作者，保留已输入正文。
  focusComment(e: any) {
    const pid = Number(e.currentTarget.dataset.pid || 0);
    const name = e.currentTarget.dataset.name || '';
    const newPrefix = name ? '回复 @' + name + '：' : '';

    let body = this.data.input || '';
    const old = this.data.replyPrefix;
    if (old && body.indexOf(old) === 0) {
      body = body.slice(old.length);
    } else {
      body = body.replace(/^回复\s?@[^：]*：/, '');
    }

    this.setData({
      panel: 'talk',
      intoView: 'sec-write',
      inputFocus: true,
      replyParentId: pid,
      replyTo: name,
      replyPrefix: newPrefix,
      input: newPrefix + body,
    });
  },

  submit() {
    let content = this.data.input.trim();
    // 去掉「回复 @X：」前缀，真正内容单独提交，@指向用 replyToName 字段携带。
    content = content.replace(/^回复\s?@[^：]*：/, '').trim();
    if (!content || this.data.sending) {
      if (!content) wx.showToast({ title: '说点什么吧', icon: 'none' });
      return;
    }
    const parentId = this.data.replyParentId;
    const replyTo = this.data.replyTo;
    this.setData({ sending: true });
    ensureLogin().then(() => {
      request<any>({
        url: '/poems/' + this.data.id + '/comments',
        method: 'POST',
        data: { content, kind: content.length > 40 ? 'shihua' : 'short', parentId, replyToName: replyTo },
      })
        .then(() => {
          this.setData({ input: '', replyPrefix: '', replyParentId: 0, replyTo: '', inputFocus: false, sending: false });
          wx.showToast({ title: parentId ? '已回复' : '已发布', icon: 'none' });
          this.fetchComments();
        })
        .catch(() => this.setData({ sending: false }));
    });
  },

  noop() {},

  // 诗签分享计入每日任务。hint 有值时用它做提示前缀（保存相册）。
  reportShare(hint?: any) {
    const prefix = typeof hint === 'string' ? hint : '';
    ensureLogin().then((uid) => {
      if (!uid) {
        if (prefix) wx.showToast({ title: prefix, icon: 'none' });
        return;
      }
      request<any>({ url: '/poems/' + this.data.id + '/share', method: 'POST', silent: true })
        .then((r) => {
          const exp = r && r.gainExp > 0 ? ' · 经验+' + r.gainExp : '';
          if (prefix || exp) wx.showToast({ title: (prefix || '诗签分享') + exp, icon: 'none' });
        })
        .catch(() => {
          if (prefix) wx.showToast({ title: prefix, icon: 'none' });
        });
    });
  },

  openShare() {
    if (!this.data.poem) return;
    const cols = this.pickShareLines().map((s) => s.split(''));
    if (!cols.length) {
      wx.showToast({ title: '这首诗暂无原文', icon: 'none' });
      return;
    }
    if (!this.data.shareIllust) this.preloadIllust(this.data.poem);
    this.setData({ shareOpen: true, shareCols: cols, shareQuote: this.pickShareQuote() });
    this.loadWxaCode();
  },

  closeShare() {
    this.setData({ shareOpen: false });
  },

  pickTheme(e: any) {
    const key = e.currentTarget.dataset.key;
    if (key) this.setData({ shareTheme: key });
  },

  // 诗签用当前这句。词按短句补全整句，例如「大雨落幽燕」要带上「白浪滔天」。
  pickShareLines() {
    const poem = this.data.poem || {};
    const raw = ((poem.lines || []) as any[]).map((l) => String(l.text || '').trim()).filter(Boolean);
    const phrases = (s: string) =>
      String(s || '')
        .replace(/\s+/g, '')
        .split(/[，、；。！？,;!?]/)
        .map((p) => p.replace(/[「」“”''《》]/g, ''))
        .filter(Boolean);
    const han = (s: string) => (s.match(/[\u4e00-\u9fff]/g) || []).length;
    const verse = phrases(poem.firstVerse || '');
    let i = Number(this.data.activeLine || 0);
    if (i < 0 || i >= raw.length) i = 0;
    const parts = phrases(raw[i] || '');
    if (!parts.length) return verse.slice(0, 2);
    if (parts.length >= 2) return parts.slice(0, 2);
    if (verse.length >= 2 && verse.indexOf(parts[0]) >= 0) return verse.slice(0, 2);

    const prev = phrases(raw[i - 1] || '')[0] || '';
    const next = phrases(raw[i + 1] || '')[0] || '';
    const h = han(parts[0]);
    const hp = han(prev);
    const hn = han(next);
    const ci = /词|曲|令/.test(poem.form || '');
    if (ci) {
      if (h <= 5 && hp >= 6) return [prev, parts[0]];
      if (h <= 5 && hn > 0 && hn <= 5) return [parts[0], next];
      if (h >= 6 && hn > 0 && hn <= 5) return [parts[0], next];
      if (h <= 5 && hp > 0 && hp <= 5 && (hn === 0 || hn > 5)) return [prev, parts[0]];
      return [parts[0]];
    }
    const couplet = (a: number, b: number) => (a === 5 && b === 5) || (a === 7 && b === 7);
    if (couplet(h, hn)) return [parts[0], next];
    if (couplet(hp, h)) return [prev, parts[0]];
    return [parts[0]];
  },

  pickShareQuote() {
    const quotes = (this.data.poem && this.data.poem.quotes) || [];
    const q = quotes.find((item: any) => String(item.text || '').trim());
    if (!q) return null;
    let text = String(q.text).replace(/\s+/g, '').trim();
    if (text.length > 46) text = text.slice(0, 46) + '…';
    return { text, source: String(q.source || '').trim() };
  },

  preloadIllust(poem: any) {
    const src = toFull((poem && poem.illustrationUrl) || '');
    if (!src) {
      this._illustLocal = '';
      this._illustSrc = '';
      this.setData({ shareIllust: '' });
      return;
    }
    if (this._illustSrc === src && this._illustLocal) {
      this.setData({ shareIllust: this._illustLocal });
      return;
    }
    this._illustSrc = src;
    this._illustLocal = '';
    this.setData({ shareIllust: src });
    wx.getImageInfo({
      src,
      success: (res) => {
        if (this._illustSrc !== src) return;
        this._illustLocal = res.path;
        this._illustSize = { w: res.width, h: res.height };
        this.setData({ shareIllust: res.path });
      },
    });
  },

  loadWxaCode() {
    if (this._codePath && this._codeFor === this.data.id) {
      this.setData({ shareCode: this._codePath });
      return;
    }
    if (this._codeLoading === this.data.id) return;
    this._codeLoading = this.data.id;
    let env = 'release';
    try {
      const info = wx.getAccountInfoSync();
      if (info && info.miniProgram && info.miniProgram.envVersion) env = info.miniProgram.envVersion;
    } catch (e) {}
    request<any>({ url: '/poems/' + this.data.id + '/wxacode', data: { env }, silent: true })
      .then((r) => {
        const b64 = r && r.image;
        if (!b64) {
          this._codeLoading = 0;
          return;
        }
        const path = wx.env.USER_DATA_PATH + '/shijing-code-' + this.data.id + '.png';
        wx.getFileSystemManager().writeFile({
          filePath: path,
          data: b64,
          encoding: 'base64',
          success: () => {
            this._codePath = path;
            this._codeFor = this.data.id;
            this._codeLoading = 0;
            this.setData({ shareCode: path });
          },
          fail: () => {
            this._codeLoading = 0;
          },
        });
      })
      .catch(() => {
        this._codeLoading = 0;
      });
  },

  saveShareCard() {
    const lines = (this.data.shareCols || []).map((col: string[]) => col.join(''));
    if (!lines.length) return;
    wx.showLoading({ title: '生成图片', mask: true });
    const cardH = this.data.shareQuote ? 1000 : 840;
    this.drawShareCard(lines, () => {
      wx.canvasToTempFilePath(
        {
          canvasId: 'shareCard',
          x: 0,
          y: 0,
          width: 640,
          height: cardH,
          destWidth: 1280,
          destHeight: cardH * 2,
          success: (res) => {
            wx.saveImageToPhotosAlbum({
              filePath: res.tempFilePath,
              success: () => {
                wx.hideLoading();
                wx.showToast({ title: '已保存到相册', icon: 'none' });
              },
              fail: () => {
                wx.hideLoading();
                wx.showModal({
                  title: '需要相册权限',
                  content: '保存诗签需要允许写入相册',
                  confirmText: '去设置',
                  success: (r) => {
                    if (r.confirm) wx.openSetting({});
                  },
                });
              },
            });
          },
          fail: () => {
            wx.hideLoading();
            wx.showToast({ title: '生成失败', icon: 'none' });
          },
        },
        this
      );
    });
  },

  drawShareCard(lines: string[], done: () => void) {
    const themes: Record<string, any> = {
      illust: { bg: '#2a261f', bg2: 'rgba(255,255,255,0.16)', ink: '#f0e8d6', seal: '#f4f0e6', mute: 'rgba(240,232,214,0.82)', ring: 'rgba(240,232,214,0.2)' },
      paper: { bg: '#f7f1e4', bg2: '#efe4cc', ink: '#2e2a22', seal: '#b0392c', mute: '#a49d8e', ring: 'rgba(176,57,44,0.16)' },
      night: { bg: '#2b2822', bg2: '#1c1914', ink: '#f0e8d6', seal: '#e6c98a', mute: '#a89878', ring: 'rgba(230,201,138,0.2)' },
      green: { bg: '#e7f0ea', bg2: '#d3e4da', ink: '#1e3a32', seal: '#2f6f5e', mute: '#6d8a80', ring: 'rgba(47,111,94,0.18)' },
      cinnabar: { bg: '#f8ebe6', bg2: '#f0d4cb', ink: '#4a221c', seal: '#b0392c', mute: '#a07870', ring: 'rgba(176,57,44,0.18)' },
    };
    const theme = themes[this.data.shareTheme] || themes.illust;
    const usePhoto = this.data.shareTheme === 'illust' && !!this.data.shareIllust;
    const paint = (path: string, iw: number, ih: number) => {
    const ctx = wx.createCanvasContext('shareCard', this);
    const w = 640;
    const quote = this.data.shareQuote;
    const h = quote ? 1000 : 840;
    if (usePhoto && path && iw > 0 && ih > 0) {
      const scale = Math.max(w / iw, h / ih);
      const dw = iw * scale;
      const dh = ih * scale;
      ctx.drawImage(path, (w - dw) / 2, (h - dh) / 2, dw, dh);
      ctx.setFillStyle('rgba(26,23,18,0.58)');
      ctx.fillRect(0, 0, w, h);
    } else {
      ctx.setFillStyle(theme.bg);
      ctx.fillRect(0, 0, w, h);
    }
    if (!usePhoto) {
      ctx.setStrokeStyle(theme.ring);
      ctx.setLineWidth(2);
      ctx.beginPath();
      ctx.arc(560, 760, 150, 0, Math.PI * 2);
      ctx.stroke();
    }

    const font = 44;
    const step = 58;
    const colGap = 78;
    const rightX = 280;
    ctx.setFillStyle(theme.ink);
    ctx.setFontSize(font);
    ctx.setTextAlign('center');
    lines.forEach((line, col) => {
      const x = rightX - col * colGap;
      line.split('').forEach((ch, i) => {
        ctx.fillText(ch, x, 120 + i * step);
      });
    });

    ctx.save();
    ctx.translate(500, 150);
    ctx.rotate((-6 * Math.PI) / 180);
    ctx.setStrokeStyle(theme.seal);
    ctx.setLineWidth(4);
    roundRect(ctx, -36, -36, 72, 72, 8);
    ctx.stroke();
    ctx.setFillStyle(theme.seal);
    ctx.setFontSize(22);
    ctx.setTextAlign('center');
    ctx.fillText('诗趣', 0, -2);
    ctx.fillText('小园', 0, 26);
    ctx.restore();

    const drawRest = () => {
      const poem = this.data.poem || {};
      ctx.setFillStyle(theme.mute);
      ctx.setFontSize(22);
      ctx.setTextAlign('center');
      ctx.fillText(this.data.shareCode ? '长按识别' : '诗趣小园', 500, 360);
      if (quote && quote.text) {
        const top = 620;
        if (usePhoto) {
          ctx.setFillStyle('rgba(12,10,8,0.42)');
          ctx.fillRect(0, top - 24, w, h - top + 24);
        }
        ctx.setStrokeStyle(theme.ring);
        ctx.setLineWidth(1);
        ctx.beginPath();
        ctx.moveTo(48, top);
        ctx.lineTo(w - 48, top);
        ctx.stroke();
        ctx.setFillStyle(theme.seal);
        ctx.setFontSize(20);
        ctx.setTextAlign('left');
        ctx.fillText('名家诗话', 48, top + 36);
        ctx.setFillStyle(theme.ink);
        ctx.setFontSize(24);
        const body = wrapByCount(quote.text, 22);
        body.forEach((row, i) => ctx.fillText(row, 48, top + 74 + i * 34));
        if (quote.source) {
          ctx.setFillStyle(theme.mute);
          ctx.setFontSize(20);
          ctx.fillText('—— ' + quote.source, 48, top + 74 + body.length * 34 + 8);
        }
      }
      ctx.setFillStyle(theme.ink);
      ctx.setFontSize(24);
      ctx.setTextAlign('left');
      ctx.fillText('《' + (poem.title || '') + '》 · ' + (poem.author || ''), 48, h - 46);
      let called = false;
      const finish = () => {
        if (called) return;
        called = true;
        done();
      };
      ctx.draw(false, finish);
      setTimeout(finish, 350);
    };
    if (this.data.shareCode) {
      ctx.drawImage(this.data.shareCode, 440, 210, 120, 120);
    } else {
      ctx.setFillStyle(theme.bg2);
      roundRect(ctx, 440, 210, 120, 120, 8);
      ctx.fill();
    }
    drawRest();
    };
    if (!usePhoto) {
      paint('', 0, 0);
      return;
    }
    if (this._illustLocal && this._illustSize) {
      paint(this._illustLocal, this._illustSize.w, this._illustSize.h);
      return;
    }
    wx.getImageInfo({
      src: this.data.shareIllust,
      success: (res) => {
        this._illustLocal = res.path;
        this._illustSize = { w: res.width, h: res.height };
        paint(res.path, res.width, res.height);
      },
      fail: () => paint('', 0, 0),
    });
  },
});

function wrapByCount(text: string, size: number) {
  const rows: string[] = [];
  let rest = text || '';
  while (rest && rows.length < 3) {
    rows.push(rest.slice(0, size));
    rest = rest.slice(size);
  }
  return rows;
}

function roundRect(ctx: any, x: number, y: number, w: number, h: number, r: number) {
  ctx.beginPath();
  ctx.moveTo(x + r, y);
  ctx.arcTo(x + w, y, x + w, y + h, r);
  ctx.arcTo(x + w, y + h, x, y + h, r);
  ctx.arcTo(x, y + h, x, y, r);
  ctx.arcTo(x, y, x + w, y, r);
  ctx.closePath();
}
