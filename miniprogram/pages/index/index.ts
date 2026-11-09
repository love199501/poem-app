import { request } from '../../utils/request';
import { BASE_URL } from '../../config';

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

// verseDisplay 五七言一联在逗号处换行，保证卡片上看到的是完整一句而不是半句。
function verseDisplay(s: string) {
  const t = String(s || '').replace(/\s+/g, '').trim();
  const i = t.search(/[，,]/);
  if (i <= 0) return t;
  const left = t.slice(0, i);
  const han = (left.match(/[\u4e00-\u9fff]/g) || []).length;
  if (han === 5 || han === 7) return t.slice(0, i + 1) + '\n' + t.slice(i + 1);
  return t;
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
  },

  onLoad() {
    // 冷启动（页面首次创建）随机展示一首。
    this._firstLoad = true;
    this.fetchPoems();
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
    // 首帧由 onLoad 负责；之后切回只刷新数据并保持当前位置。
    if (!this._firstLoad) this.fetchPoems();
  },

  fetchPoems() {
    request<any[]>({ url: '/poems' }).then((list) => {
      if (!list || !list.length) return;
      // 预处理展示字段：补全插画地址、裁剪标签，避免 wxml 里做复杂逻辑。
      const poems = list.map((p) => ({
        ...p,
        illust: toFull(p.illustrationUrl || ''),
        tagText: (p.tags && p.tags.length ? p.tags : p.moods || []).slice(0, 2),
        verseText: verseDisplay(p.firstVerse),
      }));
      let idx: number;
      if (this._firstLoad) {
        idx = Math.floor(Math.random() * poems.length); // 冷启动随机一次
        this._firstLoad = false;
      } else {
        idx = Math.min(this.data.idx, poems.length - 1); // 同一次使用中保持位置
      }
      this.setData({ poems, total: poems.length });
      this.setHero(idx);
    });
  },

  setHero(i: number) {
    const p = this.data.poems[i];
    if (!p) return;
    this.setData({ idx: i, hero: p, heroIllust: p.illust || '' });
  },

  // 滑动卡片：swiper 切换时同步当前诗与背景。
  onSwiperChange(e: any) {
    this.setHero(e.detail.current);
  },

  // 换一换：随机切到另一首（swiper current 绑定 idx，会自动滑过去）。
  shuffle() {
    const list = this.data.poems;
    if (!list || list.length <= 1) {
      if (list && list.length === 1) this.fetchPoems();
      return;
    }
    let i = this.data.idx;
    while (i === this.data.idx) {
      i = Math.floor(Math.random() * list.length);
    }
    this.setHero(i);
  },

  goDetail(e: any) {
    const id = (e && e.currentTarget && e.currentTarget.dataset.id) || (this.data.hero && this.data.hero.id);
    if (id) wx.navigateTo({ url: '/pages/poem/detail?id=' + id });
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
