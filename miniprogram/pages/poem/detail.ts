import { request } from '../../utils/request';
import { ensureLogin } from '../../utils/auth';
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

    steps: [
      { key: 'sec-read', k: '读', label: '原文' },
      { key: 'sec-scene', k: '境', label: '现场' },
      { key: 'sec-intent', k: '意', label: '想说' },
      { key: 'sec-wu', k: '悟', label: '解析' },
      { key: 'sec-talk', k: '议', label: '讨论' },
    ],
    curStep: 'sec-read',
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
  },

  onLoad(options: any) {
    const id = Number(options.id || 1);
    this.setData({ id });
    this.initAudio();
    this.fetchPoem();
  },

  onShow() {
    // 开始计阅读时长；每次展示都刷新评论，开通会员后历史评论能立刻带上皇冠。
    this._readStart = Date.now();
    if (this.data.id) this.fetchComments();
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

  // reportRead 上报本次阅读停留时长，用于累计经验（过短则忽略）。
  reportRead() {
    if (!this._readStart) return;
    const sec = Math.floor((Date.now() - this._readStart) / 1000);
    this._readStart = 0;
    if (sec < 5) return;
    ensureLogin().then(() => {
      request<any>({ url: '/users/me/read', method: 'POST', data: { seconds: sec }, silent: true }).catch(() => {});
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
        .then((r) => this.setData({ liked: r.liked, likeCount: r.likeCount }))
        .catch(() => this.setData({ liked: prevLiked, likeCount: prevCount }));
    });
  },

  // —— 朗诵 ——
  initAudio() {
    const ctx = wx.createInnerAudioContext();
    this.audioCtx = ctx;
    ctx.onPlay(() => this.setData({ 'audio.playing': true, 'audio.loading': false }));
    ctx.onPause(() => this.setData({ 'audio.playing': false }));
    ctx.onStop(() => this.setData({ 'audio.playing': false }));
    ctx.onEnded(() => this.setData({ 'audio.playing': false, 'audio.pct': 0, 'audio.cur': '00:00' }));
    ctx.onError((e: any) => {
      this.setData({ 'audio.playing': false, 'audio.loading': false });
      wx.showToast({ title: '播放失败：' + (e.errMsg || ''), icon: 'none' });
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
    const poem = this.data.poem;
    // 已有音频（名家录音或已合成的情感朗诵）直接播。
    if (poem && poem.audioUrl) {
      this.playSrc(poem.audioUrl);
      return;
    }
    // 否则请求后端「情感朗诵」合成（有感情，非机械音）。
    this.setData({ 'audio.loading': true });
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
    request<any>({ url: '/poems/' + this.data.id }).then((poem) => {
      const len = (a: any) => (a && a.length) || 0;
      const hasTrans = len(poem.translation) > 0;
      const hasScene = !!(len(poem.sceneRows) || poem.scenePunch || poem.sceneBackground || poem.sceneTitle);
      const hasIntent = !!(poem.intentLead || len(poem.audiences) || poem.void);
      const hasWu = !!(len(poem.layers) || len(poem.quotes) || len(poem.duels));

      // 步骤条只保留有内容的段（读/议 恒在）。
      const steps: any[] = [{ key: 'sec-read', k: '读', label: '原文' }];
      if (hasScene) steps.push({ key: 'sec-scene', k: '境', label: '现场' });
      if (hasIntent) steps.push({ key: 'sec-intent', k: '意', label: '想说' });
      if (hasWu) steps.push({ key: 'sec-wu', k: '悟', label: '解析' });
      steps.push({ key: 'sec-talk', k: '议', label: '讨论' });

      this.setData({
        poem,
        hasTrans,
        hasScene,
        hasIntent,
        hasWu,
        steps,
        favored: !!poem.favored,
        liked: !!poem.liked,
        likeCount: poem.likeCount || 0,
        favoriteCount: poem.favoriteCount || 0,
      });

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
      note: { text: line.text, body: line.noteBody, src: line.noteSrc },
    });
  },
  toggleTrans() {
    this.setData({ showTrans: !this.data.showTrans });
  },

  // —— 步骤条：点击跳转 + 跟随滚动高亮 ——
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

  // 置顶 / 取消置顶（精品精选），成功后按新顺序刷新。
  pin(e: any) {
    const id = e.currentTarget.dataset.id;
    request<any>({ url: '/comments/' + id + '/pin', method: 'POST' }).then(() => {
      wx.showToast({ title: '已更新置顶', icon: 'none' });
      this.fetchComments();
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
});
