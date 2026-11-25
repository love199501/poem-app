import { request } from '../../utils/request';
import { ensureLogin } from '../../utils/auth';
import { localReadIds, saveReadIds } from '../../utils/history';
import { loadAudioEnabled, loadCommentEnabled } from '../../utils/flags';
import { BASE_URL } from '../../config';

// 未读不足这个数量（或已经全部读完）时，首页不再按阅读历史剔除。
const UNREAD_FLOOR = 10;

// 心情 → 图标映射：心情标签来自库里实时聚合，这里只负责配图标，未命中给默认。
const MOOD_ICONS: Record<string, string> = {
  思乡: '🌙',
  忧国: '🏔',
  旷达: '🍃',
  孤独: '🕯',
  喜悦: '🌸',
  离别: '🍂',
  爱情: '💗',
  闲适: '🍵',
  哲理: '📖',
  愤懑: '⚡',
  感伤: '🌧',
  豪放: '🗡',
  豪迈: '🗡',
};
const DEFAULT_MOOD_ICON = '🏷';

// greeting 按当前时辰给出问候语，丰富首页的「此刻」氛围。
function greeting() {
  const h = new Date().getHours();
  if (h < 6) return { title: '夜深了', sub: '读一首诗，再入梦乡' };
  if (h < 11) return { title: '早安', sub: '以一首诗开启今日' };
  if (h < 14) return { title: '午安', sub: '偷得浮生半日闲' };
  if (h < 18) return { title: '午后', sub: '一盏清茶，一首好诗' };
  if (h < 23) return { title: '晚上好', sub: '灯下读诗，心自安宁' };
  return { title: '夜深了', sub: '读一首诗，再入梦乡' };
}

// toFull 把后端相对图片路径补全为可访问地址。
function toFull(u: string) {
  if (!u) return '';
  return /^https?:/.test(u) ? u : BASE_URL + u;
}

function unionIds(a: number[], b: number[]) {
  const out: number[] = [];
  const seen: Record<number, boolean> = {};
  a.concat(b).forEach((n) => {
    const id = Number(n);
    if (id > 0 && !seen[id]) {
      seen[id] = true;
      out.push(id);
    }
  });
  return out;
}

const FIRST_BATCH = 20;
// 全库上屏时只挂这么多张，当前卡居中。滑到边缘再平移，避免每滑一次都拆掉图片重下。
const WINDOW = 9;

// pickDeck 未读不少于 10 首时只留未读；已全部读完或未读不足 10 首时用全部诗。
function pickDeck(poems: any[], readIds: number[]) {
  const read: Record<number, boolean> = {};
  readIds.forEach((id) => {
    read[Number(id)] = true;
  });
  const unread = poems.filter((p) => !read[Number(p.id)]);
  if (unread.length < UNREAD_FLOOR) return poems;
  return unread;
}

// verseLines 五七言一联拆成上下两行。用独立文本节点排，避免一个文本里塞换行后高度被压成 0。
function verseLines(s: string) {
  const t = String(s || '').replace(/\s+/g, '').trim();
  if (!t) return [] as string[];
  const i = t.search(/[，,]/);
  if (i <= 0) return [t];
  const left = t.slice(0, i);
  const han = (left.match(/[\u4e00-\u9fff]/g) || []).length;
  if (han !== 5 && han !== 7) return [t];
  const right = t.slice(i + 1);
  return right ? [t.slice(0, i + 1), right] : [t];
}

Page({
  data: {
    greet: greeting(),
    moods: [] as any[],
    poems: [] as any[],
    idx: 0,
    hero: null as any,
    heroIllust: '',
    total: 0,
    circular: false,
    commentEnabled: false,
    audioEnabled: false,
    swiperDuration: 320,
    preloadSrcs: [] as string[],
  },

  onLoad() {
    // 先出 20 首，再在后台补齐全库。滑动和换一换都走完整诗单。
    this._firstLoad = true;
    this._fullReady = false;
    this._catalog = [] as any[];
    this._cursor = 0;
    this.loadHome(false);
    this.loadHome(true);
    this.fetchMoods();
  },

  // 心情速选项取自库里实际涉及的心情标签（与诗库筛选一致）。
  fetchMoods() {
    request<any>({ url: '/poems/facets' })
      .then((f) => {
        const moods = (f.moods || [])
          .filter((m: string) => m && m !== '全部')
          .map((m: string) => ({ key: m, icon: MOOD_ICONS[m] || DEFAULT_MOOD_ICON }));
        this.setData({ moods });
      })
      .catch(() => {});
  },

  onShow() {
    this.setData({ greet: greeting() });
    loadCommentEnabled().then((on) => this.setData({ commentEnabled: on }));
    loadAudioEnabled().then((on) => this.setData({ audioEnabled: on }));
    // 从详情、诗库或其他页面回来时，屏幕上的这首诗保持不动。
    if (this.data.hero && this.data.poems && this.data.poems.length) return;
    if (!this._firstLoad) this.loadHome(true);
  },

  fetchReadIds() {
    const local = localReadIds();
    return ensureLogin()
      .then((uid) => {
        if (!uid) return local;
        return request<number[]>({ url: '/users/me/reads', silent: true })
          .then((ids) => {
            const merged = unionIds(local, ids || []);
            saveReadIds(merged);
            return merged;
          })
          .catch(() => local);
      })
      .catch(() => local);
  },

  // full=false 只取前 20 首先显示；full=true 拉全库，滑动和换一换都基于这份诗单。
  loadHome(full: boolean) {
    const data = full ? {} : { sample: FIRST_BATCH };
    Promise.all([request<any[]>({ url: '/poems', data }), this.fetchReadIds()])
      .then(([list, readIds]) => {
        if (!full && this._fullReady) return;
        if (!list || !list.length) return;
        const all = list.map((p) => ({
          ...p,
          illust: toFull(p.illustrationUrl || ''),
          tagText: (p.tags && p.tags.length ? p.tags : p.moods || []).slice(0, 2),
          verseLines: verseLines(p.firstVerse),
        }));
        const catalog = pickDeck(all, readIds || []).filter((p) => p.verseLines && p.verseLines.length);
        if (!catalog.length) return;
        // 以返回这一刻屏幕上的诗为准。请求发出时还没有诗，不能据此再随机换一首。
        const curId = (this.data.hero && this.data.hero.id) || 0;
        const wasFull = this._fullReady;
        this._catalog = catalog;
        this._shuffleBuf = [] as number[];
        if (full) this._fullReady = true;
        const keep = catalog.findIndex((p) => Number(p.id) === Number(curId));
        // 全库已经铺好后又回来：只对齐游标，不重画卡片。
        if (wasFull && curId && this.data.poems && this.data.poems.length) {
          if (keep >= 0) this._cursor = keep;
          this._firstLoad = false;
          return;
        }
        let cursor: number;
        if (!curId) {
          cursor = Math.floor(Math.random() * catalog.length);
          this._firstLoad = false;
        } else if (keep >= 0) {
          cursor = keep;
        } else {
          cursor = Math.min(this._cursor || 0, catalog.length - 1);
        }
        this.showAt(cursor);
      })
      .catch(() => {});
  },

  // 全库超过 20 首时，屏幕上只挂当前这首和左右各几张，避免 swiper 一次渲染几百张。
  // 平时只改下标，图片节点留着；滑到窗口边缘才整窗平移。
  showAt(cursor: number) {
    const catalog = this._catalog || [];
    const n = catalog.length;
    if (!n) return;
    cursor = ((cursor % n) + n) % n;
    this._cursor = cursor;
    if (n <= FIRST_BATCH) {
      const hero = catalog[cursor];
      this.setData({
        poems: catalog,
        total: n,
        idx: cursor,
        circular: n > 2,
        swiperDuration: 320,
        hero,
        heroIllust: hero ? hero.illust || '' : '',
        preloadSrcs: this.collectPreload(cursor),
      });
      return;
    }
    this.paintWindow(cursor, true);
  },

  paintWindow(cursor: number, jump: boolean) {
    const catalog = this._catalog || [];
    const n = catalog.length;
    if (!n) return;
    const half = Math.floor(WINDOW / 2);
    const poems: any[] = [];
    for (let i = -half; i <= half; i++) poems.push(catalog[(cursor + i + n) % n]);
    const hero = catalog[cursor];
    const data: Record<string, any> = {
      poems,
      total: n,
      idx: half,
      circular: false,
      hero,
      heroIllust: hero ? hero.illust || '' : '',
      preloadSrcs: this.collectPreload(cursor),
    };
    if (jump) data.swiperDuration = 0;
    this.setData(data);
    if (jump) setTimeout(() => this.setData({ swiperDuration: 320 }), 50);
  },

  // 当前卡前后，再加上几首预备给换一换的诗，先把背景图下载进缓存。
  collectPreload(cursor: number) {
    const catalog = this._catalog || [];
    const n = catalog.length;
    const urls: string[] = [];
    const push = (p: any) => {
      if (!p || !p.illust || urls.indexOf(p.illust) >= 0) return;
      urls.push(p.illust);
    };
    if (!n) return urls;
    for (let d = -3; d <= 5; d++) push(catalog[(cursor + d + n) % n]);
    this.ensureShuffleBuffer();
    (this._shuffleBuf || []).forEach((i: number) => push(catalog[i]));
    return urls;
  },

  ensureShuffleBuffer() {
    const n = (this._catalog || []).length;
    if (n <= 1) return;
    if (!this._shuffleBuf) this._shuffleBuf = [] as number[];
    let guard = 0;
    while (this._shuffleBuf.length < 4 && guard < 24) {
      guard += 1;
      const i = Math.floor(Math.random() * n);
      if (i === this._cursor || this._shuffleBuf.indexOf(i) >= 0) continue;
      this._shuffleBuf.push(i);
    }
  },

  setHero(i: number, list?: any[]) {
    const poems = list || this.data.poems;
    const p = poems[i];
    if (!p) return;
    this.setData({ idx: i, hero: p, heroIllust: p.illust || '' });
  },

  // 滑动卡片：只接受手指滑动。程序改 current 也会触发 change，若再 setData 回去，
  // 会和 circular 打成死循环，页面卡死，点击和日志都不再执行。
  onSwiperChange(e: any) {
    const source = e.detail && e.detail.source;
    const to = e.detail && e.detail.current;
    if (source !== 'touch') return;
    const catalog = this._catalog || [];
    const n = catalog.length;
    if (!n) return;
    if (n <= FIRST_BATCH) {
      if (to === this.data.idx) return;
      this._cursor = to;
      this.setHero(to);
      this.setData({ preloadSrcs: this.collectPreload(to) });
      return;
    }
    if (to === this.data.idx) return;
    const delta = to - this.data.idx;
    if (!delta) return;
    const cursor = (this._cursor + delta + n) % n;
    this._cursor = cursor;
    const last = ((this.data.poems && this.data.poems.length) || WINDOW) - 1;
    // 窗口中间只改下标，左右图片已经在屏幕上，不再拆掉重挂。
    if (to > 1 && to < last - 1) {
      this.setHero(to);
      this.setData({ preloadSrcs: this.collectPreload(cursor) });
      return;
    }
    this.paintWindow(cursor, true);
  },

  // 换一换：优先用已经预取过背景的那几首，再补上下一批。
  shuffle() {
    const n = (this._catalog || []).length;
    if (n <= 1) {
      if (n === 1) this.loadHome(true);
      return;
    }
    this.ensureShuffleBuffer();
    const buf: number[] = this._shuffleBuf || [];
    let i = buf.length ? buf.shift() : -1;
    if (i == null || i < 0 || i === this._cursor || i >= n) {
      i = this._cursor;
      let guard = 0;
      while (i === this._cursor && guard < 8) {
        guard += 1;
        i = Math.floor(Math.random() * n);
      }
    }
    this.showAt(i as number);
  },

  goDetail(e: any) {
    const id = (e && e.currentTarget && e.currentTarget.dataset.id) || (this.data.hero && this.data.hero.id);
    if (!id) return;
    this._keepOnBack = true;
    wx.navigateTo({ url: '/pages/poem/detail?id=' + id });
  },

  goLibrary() {
    wx.switchTab({ url: '/pages/library/index' });
  },

  // 心情速选：写入 storage，切到诗库 tab 后由诗库读取并筛选。
  goMood(e: any) {
    const mood = e.currentTarget.dataset.mood;
    if (mood) wx.setStorageSync('lib_mood', mood);
    wx.switchTab({ url: '/pages/library/index' });
  },
});
